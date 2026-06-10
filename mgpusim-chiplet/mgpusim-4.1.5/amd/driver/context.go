package driver

import (
	"sync"

	"github.com/sarchlab/akita/v4/mem/vm"
	"github.com/sarchlab/mgpusim/v4/amd/plan"
)

type managedLaunchState struct {
	AbsID           int
	RelID           int
	ProgramLaunchID int
	Queue           *CommandQueue
	KernelName      string
	DependsOn       []int
	Priority        int

	// Full-launch runtime profile / planner hint.
	EstCUs        int
	ChipletCUs    []int
	EstDurationPs int64

	// Adaptive workgroup-chunking state.
	AdaptiveWGChunks bool
	MinChunkWG       int
	TotalWGs         int
	NextWG           int
	RemainingWGs     int

	// Current chunk in flight (valid while Active=true).
	CurrentChunkWGStart    int
	CurrentChunkWGCount    int
	CurrentChunkEstCUs     int
	CurrentChunkChipletCU  []int
	CurrentChunkAllowedCUs []int

	// Optional explicit WG windows for an item-level dispatch plan.
	ExplicitWGWindows []plan.WGWindow
	SubWGFactor       int

	Started   bool
	Active    bool
	Completed bool
	GPUID     int
	Policy    string
	Partition string
}

type buffer struct {
	vAddr Ptr
	size  uint64
	// tag optionally binds this buffer to a plan buffer tag.
	tag   string
	freed bool

	// After a kernel is launched, the l2 cache contain dirty data that belongs
	// to this buffer. Therefore, copying from or to this buffer triggers L2
	// flushing.
	l2Dirty bool
}

// Context is an opaque struct that carries the information used by the driver.
type Context struct {
	pid           vm.PID
	currentGPUID  int
	prevPageVAddr uint64
	l2Dirty       bool

	queueMutex sync.Mutex
	queues     []*CommandQueue

	pipelineMutex                  sync.Mutex
	nextLaunchAbsID                int
	managedLaunches                map[int]*managedLaunchState
	managedActiveCUsByGPU          map[int]int
	managedActiveCUsByGPUChiplet   map[int]map[int]int
	managedReservedCUsByGPU        map[int]map[int]bool
	managedCompletedAbsIDs         map[int]bool
	managedPendingCompletionAbsIDs map[int]bool
	managedCompletionFlushInFlight bool
	managedCompletionFlushQueue    *CommandQueue

	buffers []*buffer
}

func (c *Context) markAllBuffersDirty() {
	for _, b := range c.buffers {
		b.l2Dirty = true
	}
}

func (c *Context) markAllBuffersClean() {
	for _, b := range c.buffers {
		b.l2Dirty = false
	}
}

func (c *Context) removeFreedBuffers() {
	kept := c.buffers[:0]
	for _, b := range c.buffers {
		if !b.freed {
			kept = append(kept, b)
		}
	}

	for i := len(kept); i < len(c.buffers); i++ {
		c.buffers[i] = nil
	}

	c.buffers = kept
}

func (c *Context) nextLaunchIDs(sequenceLength int) (absID, relID int) {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()

	absID = c.nextLaunchAbsID
	c.nextLaunchAbsID++
	relID = absID
	if sequenceLength > 0 {
		relID = absID % sequenceLength
	}

	c.garbageCollectCompletedLaunchesLocked(sequenceLength)
	return absID, relID
}

func (c *Context) registerManagedLaunch(state *managedLaunchState) {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()

	if c.managedLaunches == nil {
		c.managedLaunches = make(map[int]*managedLaunchState)
	}
	if c.managedActiveCUsByGPU == nil {
		c.managedActiveCUsByGPU = make(map[int]int)
	}
	if c.managedActiveCUsByGPUChiplet == nil {
		c.managedActiveCUsByGPUChiplet = make(map[int]map[int]int)
	}
	if c.managedCompletedAbsIDs == nil {
		c.managedCompletedAbsIDs = make(map[int]bool)
	}
	if c.managedPendingCompletionAbsIDs == nil {
		c.managedPendingCompletionAbsIDs = make(map[int]bool)
	}
	if c.managedReservedCUsByGPU == nil {
		c.managedReservedCUsByGPU = make(map[int]map[int]bool)
	}

	c.managedLaunches[state.AbsID] = state
}

func (c *Context) managedLaunchState(absID int) *managedLaunchState {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()
	return c.managedLaunches[absID]
}

func (c *Context) ensureChipletMapLocked(gpuID int) map[int]int {
	if c.managedActiveCUsByGPUChiplet == nil {
		c.managedActiveCUsByGPUChiplet = make(map[int]map[int]int)
	}
	m := c.managedActiveCUsByGPUChiplet[gpuID]
	if m == nil {
		m = make(map[int]int)
		c.managedActiveCUsByGPUChiplet[gpuID] = m
	}
	return m
}

func (c *Context) ensureReservedCUSetLocked(gpuID int) map[int]bool {
	if c.managedReservedCUsByGPU == nil {
		c.managedReservedCUsByGPU = make(map[int]map[int]bool)
	}
	m := c.managedReservedCUsByGPU[gpuID]
	if m == nil {
		m = make(map[int]bool)
		c.managedReservedCUsByGPU[gpuID] = m
	}
	return m
}

func (c *Context) managedLaunchStarted(absID int, gpuID int, estCUs int, chipletCUs []int, allowedCUs []int) {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()

	st, ok := c.managedLaunches[absID]
	if !ok || st == nil || st.Active {
		return
	}

	st.Started = true
	st.Active = true
	st.GPUID = gpuID
	st.CurrentChunkEstCUs = estCUs
	st.CurrentChunkChipletCU = append([]int(nil), chipletCUs...)
	st.CurrentChunkAllowedCUs = append([]int(nil), allowedCUs...)

	if c.managedActiveCUsByGPU == nil {
		c.managedActiveCUsByGPU = make(map[int]int)
	}
	c.managedActiveCUsByGPU[gpuID] += estCUs

	if len(chipletCUs) > 0 {
		m := c.ensureChipletMapLocked(gpuID)
		for chiplet, v := range chipletCUs {
			if v <= 0 {
				continue
			}
			m[chiplet] += v
		}
	}
	if len(allowedCUs) > 0 {
		m := c.ensureReservedCUSetLocked(gpuID)
		for _, cuID := range allowedCUs {
			if cuID >= 0 {
				m[int(cuID)] = true
			}
		}
	}
}

func (c *Context) managedLaunchChunkCompleted(absID int, gpuID int, estCUs int, chipletCUs []int, allowedCUs []int) (launchCompleted bool) {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()

	st, ok := c.managedLaunches[absID]
	if !ok || st == nil {
		return false
	}

	if st.Active {
		if c.managedActiveCUsByGPU != nil {
			c.managedActiveCUsByGPU[gpuID] -= estCUs
			if c.managedActiveCUsByGPU[gpuID] < 0 {
				c.managedActiveCUsByGPU[gpuID] = 0
			}
		}
		if len(chipletCUs) > 0 && c.managedActiveCUsByGPUChiplet != nil {
			m := c.managedActiveCUsByGPUChiplet[gpuID]
			for chiplet, v := range chipletCUs {
				if v <= 0 || m == nil {
					continue
				}
				m[chiplet] -= v
				if m[chiplet] < 0 {
					m[chiplet] = 0
				}
			}
		}
	}

	st.Active = false
	if len(allowedCUs) > 0 && c.managedReservedCUsByGPU != nil {
		m := c.managedReservedCUsByGPU[gpuID]
		for _, cuID := range allowedCUs {
			delete(m, int(cuID))
		}
	}

	st.CurrentChunkEstCUs = 0
	st.CurrentChunkChipletCU = nil
	st.CurrentChunkAllowedCUs = nil
	st.CurrentChunkWGStart = 0
	st.CurrentChunkWGCount = 0

	if st.RemainingWGs <= 0 {
		st.Completed = true
		if c.managedPendingCompletionAbsIDs == nil {
			c.managedPendingCompletionAbsIDs = make(map[int]bool)
		}
		// Do not publish dependency completion immediately. A profiled/managed
		// launch can be followed by another managed launch without going through
		// the normal synchronous command-queue barrier. Publish completion only
		// after a quiescent L1-only cache barrier.
		c.managedPendingCompletionAbsIDs[absID] = true
		return true
	}

	return false
}

func (c *Context) isManagedLaunchCompleted(absID int) bool {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()

	if c.managedCompletedAbsIDs == nil {
		return false
	}
	return c.managedCompletedAbsIDs[absID]
}

func (c *Context) activeManagedCUsForGPU(gpuID int) int {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()
	return c.managedActiveCUsByGPU[gpuID]
}

func (c *Context) activeManagedCUsForGPUChiplet(gpuID int, chiplet int) int {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()
	if c.managedActiveCUsByGPUChiplet == nil {
		return 0
	}
	m := c.managedActiveCUsByGPUChiplet[gpuID]
	if m == nil {
		return 0
	}
	return m[chiplet]
}

func (c *Context) reservedManagedCUsSnapshot(gpuID int) map[int]bool {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()
	out := make(map[int]bool)
	if c.managedReservedCUsByGPU == nil {
		return out
	}
	m := c.managedReservedCUsByGPU[gpuID]
	for cuID, busy := range m {
		if busy {
			out[int(cuID)] = true
		}
	}
	return out
}

func (c *Context) managedLaunchesPending() []*managedLaunchState {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()

	out := make([]*managedLaunchState, 0, len(c.managedLaunches))
	for _, st := range c.managedLaunches {
		if st != nil && !st.Completed {
			out = append(out, st)
		}
	}
	return out
}

func (c *Context) markLaunchCompleted(absID int) {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()

	if c.managedLaunches == nil {
		c.managedLaunches = make(map[int]*managedLaunchState)
	}
	if c.managedCompletedAbsIDs == nil {
		c.managedCompletedAbsIDs = make(map[int]bool)
	}

	st, ok := c.managedLaunches[absID]
	if !ok || st == nil {
		st = &managedLaunchState{AbsID: absID}
		c.managedLaunches[absID] = st
	}
	st.Active = false
	st.Completed = true
	c.managedCompletedAbsIDs[absID] = true
}

func (c *Context) garbageCollectCompletedLaunches(sequenceLength int) {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()
	c.garbageCollectCompletedLaunchesLocked(sequenceLength)
}

func (c *Context) garbageCollectCompletedLaunchesLocked(sequenceLength int) {
	keepFloor := c.nextLaunchAbsID
	if sequenceLength > 0 {
		keepFloor = c.nextLaunchAbsID - (c.nextLaunchAbsID % sequenceLength)
	}

	// Important for dispatch/item-level plans: a completed launch/item may still
	// be required as a dependency by a later pending item. Keep such completion
	// records alive even if they are older than the current launch-sequence
	// window, otherwise dependent items can wait forever.
	requiredCompleted := make(map[int]struct{})
	for _, st := range c.managedLaunches {
		if st == nil || st.Completed {
			continue
		}
		for _, depAbsID := range st.DependsOn {
			requiredCompleted[depAbsID] = struct{}{}
		}
	}

	for absID, st := range c.managedLaunches {
		if st == nil || !st.Completed {
			continue
		}
		if _, needed := requiredCompleted[absID]; needed {
			continue
		}
		if _, pending := c.managedPendingCompletionAbsIDs[absID]; pending {
			continue
		}
		if absID >= keepFloor {
			continue
		}
		delete(c.managedLaunches, absID)
		delete(c.managedCompletedAbsIDs, absID)
	}
}

func (c *Context) hasManagedCompletionBarrierLocked() bool {
	return len(c.managedPendingCompletionAbsIDs) > 0 || c.managedCompletionFlushInFlight
}

func (c *Context) hasManagedCompletionBarrier() bool {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()
	return c.hasManagedCompletionBarrierLocked()
}

func (c *Context) activeManagedCUsTotalLocked() int {
	total := 0
	for _, v := range c.managedActiveCUsByGPU {
		if v > 0 {
			total += v
		}
	}
	return total
}

func (c *Context) shouldStartManagedCompletionFlush() bool {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()
	if len(c.managedPendingCompletionAbsIDs) == 0 {
		return false
	}
	if c.managedCompletionFlushInFlight {
		return false
	}
	return c.activeManagedCUsTotalLocked() == 0
}

func (c *Context) beginManagedCompletionFlush(q *CommandQueue) bool {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()
	if len(c.managedPendingCompletionAbsIDs) == 0 {
		return false
	}
	if c.managedCompletionFlushInFlight {
		return false
	}
	if c.activeManagedCUsTotalLocked() != 0 {
		return false
	}
	c.managedCompletionFlushInFlight = true
	c.managedCompletionFlushQueue = q
	return true
}

func (c *Context) finishManagedCompletionFlush() {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()
	if c.managedCompletedAbsIDs == nil {
		c.managedCompletedAbsIDs = make(map[int]bool)
	}
	for absID := range c.managedPendingCompletionAbsIDs {
		c.managedCompletedAbsIDs[absID] = true
	}
	c.managedPendingCompletionAbsIDs = make(map[int]bool)
	c.managedCompletionFlushInFlight = false
	c.managedCompletionFlushQueue = nil
}

func (c *Context) getManagedCompletionFlushQueue() *CommandQueue {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()
	return c.managedCompletionFlushQueue
}
