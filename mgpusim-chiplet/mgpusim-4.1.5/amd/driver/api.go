package driver

import (
	"log"
	"math"
	"strconv"
	"strings"
	"sync/atomic"

	// embed hsaco files
	_ "embed"

	"github.com/sarchlab/akita/v4/mem/vm"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/mgpusim/v4/amd/driver/internal"
	"github.com/sarchlab/mgpusim/v4/amd/kernels"
)

var nextPID uint64

// Init creates a context to be used in the following API calls.
func (d *Driver) Init() *Context {
	atomic.AddUint64(&nextPID, 1)

	c := &Context{
		pid:                            vm.PID(nextPID),
		currentGPUID:                   1,
		managedLaunches:                make(map[int]*managedLaunchState),
		managedActiveCUsByGPU:          make(map[int]int),
		managedActiveCUsByGPUChiplet:   make(map[int]map[int]int),
		managedReservedCUsByGPU:        make(map[int]map[int]bool),
		managedCompletedAbsIDs:         make(map[int]bool),
		managedPendingCompletionAbsIDs: make(map[int]bool),
	}

	d.contextMutex.Lock()
	d.contexts = append(d.contexts, c)
	d.contextMutex.Unlock()

	return c
}

// InitWithExistingPID creates a new context that shares the same process ID
// with the given context.
func (d *Driver) InitWithExistingPID(ctx *Context) *Context {
	c := &Context{
		pid:                            ctx.pid,
		currentGPUID:                   1,
		managedLaunches:                make(map[int]*managedLaunchState),
		managedActiveCUsByGPU:          make(map[int]int),
		managedActiveCUsByGPUChiplet:   make(map[int]map[int]int),
		managedReservedCUsByGPU:        make(map[int]map[int]bool),
		managedCompletedAbsIDs:         make(map[int]bool),
		managedPendingCompletionAbsIDs: make(map[int]bool),
	}

	d.contextMutex.Lock()
	d.contexts = append(d.contexts, c)
	d.contextMutex.Unlock()

	return c
}

// GetNumGPUs return the number of GPUs in the platform
func (d *Driver) GetNumGPUs() int {
	return len(d.GPUs)
}

// SelectGPU requires the driver to perform the following APIs on a selected
// GPU
func (d *Driver) SelectGPU(c *Context, gpuID int) {
	if gpuID >= len(d.devices) {
		log.Panicf("GPU %d is not available", gpuID)
	}
	c.currentGPUID = gpuID
}

// CreateUnifiedGPU can create a virtual GPU that bundles multiple GPUs
// together. It returns the DeviceID of the created unified multi-GPU device.
func (d *Driver) CreateUnifiedGPU(c *Context, gpuIDs []int) int {
	d.mustNotBeAnEmptyList(gpuIDs)
	d.mustBeAllActualGPUs(gpuIDs)

	dev := &internal.Device{
		ID:            len(d.devices),
		Type:          internal.DeviceTypeUnifiedGPU,
		UnifiedGPUIDs: gpuIDs,
		MemState:      internal.NewDeviceMemoryState(d.Log2PageSize),
	}

	for _, gpuID := range gpuIDs {
		dev.ActualGPUs = append(dev.ActualGPUs, d.devices[gpuID])
	}

	d.devices = append(d.devices, dev)
	d.memAllocator.RegisterDevice(dev)

	return dev.ID
}

func (d *Driver) mustNotBeAnEmptyList(gpuIDs []int) {
	if len(gpuIDs) == 0 {
		panic("must unify at least 1 GPU")
	}
}

func (d *Driver) mustBeAllActualGPUs(gpuIDs []int) {
	for _, gpuID := range gpuIDs {
		dev := d.devices[gpuID]
		if dev.Type != internal.DeviceTypeGPU {
			panic("can only unify GPUs")
		}
	}
}

// CreateCommandQueue creates a command queue in the driver
func (d *Driver) CreateCommandQueue(c *Context) *CommandQueue {
	q := new(CommandQueue)
	q.GPUID = c.currentGPUID
	q.Context = c

	c.queueMutex.Lock()
	c.queues = append(c.queues, q)
	c.queueMutex.Unlock()

	return q
}

// DrainCommandQueue will return when there is no command to execute
func (d *Driver) DrainCommandQueue(q *CommandQueue) {
	listener := q.Subscribe()
	defer q.Unsubscribe(listener)

	d.enqueueSignal <- true

	for {
		if q.NumCommand() == 0 {
			return
		}
		listener.Wait()
	}
}

// AllocateMemory allocates a chunk of memory of size byteSize in storage.
// It returns the pointer pointing to the newly allocated memory in the GPU
// memory space.
func (d *Driver) AllocateMemory(
	ctx *Context,
	byteSize uint64,
) Ptr {
	ptr := d.memAllocator.Allocate(ctx.pid, byteSize, ctx.currentGPUID)

	ctx.buffers = append(ctx.buffers, &buffer{
		vAddr:   Ptr(ptr),
		size:    byteSize,
		freed:   false,
		l2Dirty: false,
	})

	// log.Printf("Allocate %d\n", ptr)
	return Ptr(ptr)
}

// AllocateMemoryWithTag allocates a chunk of GPU memory and associates it with a
// plan buffer tag.
//
// If a plan is installed in the driver and it contains page placement
// information for this tag, the driver will attempt to place each page on the
// specified chiplet.
func (d *Driver) AllocateMemoryWithTag(
	ctx *Context,
	byteSize uint64,
	tag string,
) Ptr {
	pageSize := uint64(1 << d.Log2PageSize)
	numPages := int((byteSize-1)/pageSize + 1)

	var pageChiplets []int
	if d.plan != nil && tag != "" {
		if d.plan.BufferPlacement != nil {
			if _, ok := d.plan.BufferPlacement[tag]; ok {
				placements, err := d.plan.BufferPagePlacements(tag, numPages)
				if err != nil {
					// Plan exists but is incomplete / inconsistent for this tag.
					// Fall back to the allocator baseline (RR / interleaved) instead
					// of crashing the simulation.
					log.Printf(
						"[plan] buffer placement for tag %q failed: %v; falling back to baseline placement",
						tag, err,
					)
				} else {
					pageChiplets = placements
				}
			}
		}
	}

	ptr := d.memAllocator.AllocateWithPagePlacement(ctx.pid, byteSize, ctx.currentGPUID, pageChiplets)

	ctx.buffers = append(ctx.buffers, &buffer{
		vAddr:   Ptr(ptr),
		size:    byteSize,
		tag:     tag,
		freed:   false,
		l2Dirty: false,
	})

	return Ptr(ptr)
}

// AllocateMemoryLASP allocates GPU memory and optionally pins pages to
// chiplets following a simple MGVM/LASP-style partitioning scheme.
//
// Supported partition strings (case-insensitive):
//   - "modN": page i -> chiplet i % N
//   - "divN": page i -> chiplet floor(i / ceil(numPages/N))
//
// If N is omitted, N defaults to the configured number of chiplets per GPU.
//
// This API is provided for MGVM compatibility. For new work, prefer
// AllocateMemoryWithTag + plan JSON.
func (d *Driver) AllocateMemoryLASP(
	ctx *Context,
	byteSize uint64,
	partition string,
) Ptr {
	pageSize := uint64(1 << d.Log2PageSize)
	numPages := int((byteSize-1)/pageSize + 1)

	numChiplets, _ := GetChipletMemoryConfig()
	if numChiplets <= 0 {
		numChiplets = 1
	}

	pageChiplets := make([]int, numPages)
	for i := 0; i < numPages; i++ {
		pageChiplets[i] = chipletForLASPPage(i, numPages, partition, numChiplets)
	}

	ptr := d.memAllocator.AllocateWithPagePlacement(ctx.pid, byteSize, ctx.currentGPUID, pageChiplets)

	ctx.buffers = append(ctx.buffers, &buffer{
		vAddr:   Ptr(ptr),
		size:    byteSize,
		freed:   false,
		l2Dirty: false,
	})

	return Ptr(ptr)
}

func chipletForLASPPage(pageIndex int, numPages int, partition string, numChiplets int) int {
	if numChiplets <= 0 {
		numChiplets = 1
	}
	if numPages <= 0 {
		return 0
	}

	p := strings.ToLower(strings.TrimSpace(partition))
	if p == "" {
		p = "mod"
	}

	// Parse optional numeric suffix.
	parseN := func(prefix string) int {
		rest := strings.TrimPrefix(p, prefix)
		rest = strings.TrimSpace(rest)
		if rest == "" {
			return numChiplets
		}
		n, err := strconv.Atoi(rest)
		if err != nil || n <= 0 {
			return numChiplets
		}
		return n
	}

	// modN: i % N
	if strings.HasPrefix(p, "mod") {
		n := parseN("mod")
		chiplet := pageIndex % n
		if chiplet < 0 {
			chiplet = 0
		}
		return chiplet
	}

	// divN: floor(i / ceil(numPages/N))
	if strings.HasPrefix(p, "div") {
		n := parseN("div")
		pagesPerPart := int(math.Ceil(float64(numPages) / float64(n)))
		if pagesPerPart <= 0 {
			pagesPerPart = 1
		}
		chiplet := pageIndex / pagesPerPart
		if chiplet >= n {
			chiplet = n - 1
		}
		if chiplet < 0 {
			chiplet = 0
		}
		return chiplet
	}

	// Fallback: mod over configured chiplets.
	return pageIndex % numChiplets
}

// AllocateUnifiedMemory allocates a unified memory. Allocation is done on CPU
func (d *Driver) AllocateUnifiedMemory(
	ctx *Context,
	byteSize uint64,
) Ptr {
	ptr := Ptr(d.memAllocator.AllocateUnified(ctx.pid, byteSize))

	ctx.buffers = append(ctx.buffers, &buffer{
		vAddr:   ptr,
		size:    byteSize,
		freed:   false,
		l2Dirty: false,
	})

	return ptr
}

// Remap keeps the virtual address unchanged and moves the physical address to
// another GPU
func (d *Driver) Remap(ctx *Context, addr, size uint64, deviceID int) {
	d.memAllocator.Remap(ctx.pid, addr, size, deviceID)
}

// Distribute rearranges a consecutive virtual memory space and re-allocate the
// memory on designated GPUs. This function returns the number of bytes
// allocated to each GPU.
func (d *Driver) Distribute(
	ctx *Context,
	addr Ptr,
	byteSize uint64,
	gpuIDs []int,
) []uint64 {
	if len(gpuIDs) == 1 {
		return []uint64{byteSize}
	}

	return d.distributor.Distribute(ctx, uint64(addr), byteSize, gpuIDs)
}

func unique(in []int) []int {
	keys := make(map[int]bool)
	list := make([]int, 0)

	for _, entry := range in {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}

	return list
}

// FreeMemory frees the memory pointed by ptr. The pointer must be allocated
// with the function AllocateMemory earlier. Error will be returned if the ptr
// provided is invalid.
func (d *Driver) FreeMemory(ctx *Context, ptr Ptr) error {
	d.drainManagedLaunches(ctx)

	// log.Printf("Free %d\n", ptr)
	d.memAllocator.Free(uint64(ptr))

	for i, buffer := range ctx.buffers {
		if buffer.vAddr == ptr {
			ctx.buffers[i].freed = true
		}
	}

	return nil
}

// EnqueueMemCopyH2D registers a MemCopyH2DCommand in the queue.
func (d *Driver) EnqueueMemCopyH2D(
	queue *CommandQueue,
	dst Ptr,
	src interface{},
) {
	cmd := &MemCopyH2DCommand{
		ID:  sim.GetIDGenerator().Generate(),
		Dst: dst,
		Src: src,
	}

	d.Enqueue(queue, cmd)
}

// EnqueueMemCopyD2H registers a MemCopyD2HCommand in the queue.
func (d *Driver) EnqueueMemCopyD2H(
	queue *CommandQueue,
	dst interface{},
	src Ptr,
) {
	cmd := &MemCopyD2HCommand{
		ID:  sim.GetIDGenerator().Generate(),
		Dst: dst,
		Src: src,
	}
	d.Enqueue(queue, cmd)
}

//go:embed memcopy.hsaco
var kernelBytes []byte

// EnqueueMemCopyD2D registers a MemCopyD2DCommand (LaunchKernelCommand) in the
// queue.
func (d *Driver) EnqueueMemCopyD2D(
	queue *CommandQueue,
	dst Ptr,
	src Ptr,
	num int,
) {
	co := kernels.LoadProgramFromMemory(
		kernelBytes, "copyKernel")
	if co == nil {
		panic("fail to load copyKernel kernel")
	}
	gridSize := [3]uint32{uint32(math.Ceil(float64(num) / float64(4))), 1, 1}
	//total_bytes / (wgSize * 4). Each thread copies 4 bytes.

	wgSize := [3]uint16{64, 1, 1}
	kernelArgs := KernelMemCopyArgs{src, dst, int64(num)}

	d.EnqueueLaunchKernel(queue, co, gridSize, wgSize, &kernelArgs)
}

// MemCopyH2D copies a memory from the host to a GPU device.
func (d *Driver) MemCopyH2D(ctx *Context, dst Ptr, src interface{}) {
	d.drainManagedLaunches(ctx)

	queue := d.CreateCommandQueue(ctx)
	d.EnqueueMemCopyH2D(queue, dst, src)
	d.DrainCommandQueue(queue)
}

// MemCopyD2H copies a memory from a GPU device to the host
func (d *Driver) MemCopyD2H(ctx *Context, dst interface{}, src Ptr) {
	d.drainManagedLaunches(ctx)

	queue := d.CreateCommandQueue(ctx)
	d.EnqueueMemCopyD2H(queue, dst, src)
	d.DrainCommandQueue(queue)
}

// MemCopyD2D copies a memory from a GPU device to another GPU device. num is
// the total number of bytes.
func (d *Driver) MemCopyD2D(ctx *Context, dst Ptr, src Ptr, num int) {
	d.drainManagedLaunches(ctx)

	queue := d.CreateCommandQueue(ctx)
	d.EnqueueMemCopyD2D(queue, dst, src, num)
	d.DrainCommandQueue(queue)
}
