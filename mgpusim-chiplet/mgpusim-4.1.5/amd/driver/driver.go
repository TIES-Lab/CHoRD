package driver

import (
	"log"
	"reflect"
	"runtime/debug"
	"sync"

	"github.com/rs/xid"
	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/mem/vm"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/mgpusim/v4/amd/driver/internal"
	"github.com/sarchlab/mgpusim/v4/amd/kernels"
	"github.com/sarchlab/mgpusim/v4/amd/plan"
	"github.com/sarchlab/mgpusim/v4/amd/protocol"
	"github.com/tebeka/atexit"
)

// Driver is an Akita component that controls the simulated GPUs
type Driver struct {
	*sim.TickingComponent

	memAllocator  internal.MemoryAllocator
	distributor   distributor
	globalStorage *mem.Storage

	GPUs        []sim.Port
	devices     []*internal.Device
	pageTable   vm.PageTable
	middlewares []Middleware

	requestsToSend []sim.Msg

	contextMutex sync.Mutex
	contexts     []*Context

	mmuPort sim.Port
	gpuPort sim.Port

	driverStopped      chan bool
	enqueueSignal      chan bool
	engineMutex        sync.Mutex
	engineRunning      bool
	engineRunningMutex sync.Mutex
	simulationID       string

	Log2PageSize uint64

	// plan optionally provides per-buffer page placement and per-kernel
	// scheduling hints. It is loaded by the runner (or by user code) and can be
	// nil if no plan is installed.
	plan *plan.Plan

	currentPageMigrationReq         *vm.PageMigrationReqToDriver
	toSendToMMU                     *vm.PageMigrationRspFromDriver
	migrationReqToSendToCP          []*protocol.PageMigrationReqToCP
	isCurrentlyHandlingMigrationReq bool
	numRDMADrainACK                 uint64
	numRDMARestartACK               uint64
	numShootDownACK                 uint64
	numRestartACK                   uint64
	numPagesMigratingACK            uint64
	isCurrentlyMigratingOnePage     bool

	RemotePMCPorts []sim.Port
}

// SetPlan installs a scheduling / placement plan into the driver.
//
// The plan is optional. If nil, the driver behaves as the default simulator.
func (d *Driver) SetPlan(p *plan.Plan) {
	d.plan = p
}

// Plan returns the currently installed plan. It can be nil.
func (d *Driver) Plan() *plan.Plan {
	return d.plan
}

// Run starts a new threads that handles all commands in the command queues
func (d *Driver) Run() {
	d.logSimulationStart()
	go d.runAsync()
}

// NextDeviceInitialAddress returns the physical address that will be used
// as the base address for the next device registered with the driver's
// memory allocator.
//
// This method is mainly used by timing platform builders to align their
// internal memAddrOffset with the driver's allocator.
func (d *Driver) NextDeviceInitialAddress() uint64 {
	return d.memAllocator.NextDeviceInitialAddress()
}

// Terminate stops the driver thread execution.
func (d *Driver) Terminate() {
	d.driverStopped <- true
	d.logSimulationTerminate()
}

func (d *Driver) logSimulationStart() {
	d.simulationID = xid.New().String()
	tracing.StartTask(
		d.simulationID,
		"",
		d,
		"Simulation", "Simulation",
		nil,
	)
}

func (d *Driver) logSimulationTerminate() {
	tracing.EndTask(d.simulationID, d)
}

func (d *Driver) runAsync() {
	for {
		select {
		case <-d.driverStopped:
			return
		case <-d.enqueueSignal:
			d.Engine.Pause()
			d.TickLater()
			d.Engine.Continue()

			d.engineRunningMutex.Lock()
			if d.engineRunning {
				d.engineRunningMutex.Unlock()
				continue
			}

			d.engineRunning = true
			go d.runEngine()
			d.engineRunningMutex.Unlock()
		}
	}
}

func (d *Driver) runEngine() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic: %v", r)
			debug.PrintStack()
			atexit.Exit(1)
		}
	}()

	d.engineMutex.Lock()
	defer d.engineMutex.Unlock()
	err := d.Engine.Run()
	if err != nil {
		panic(err)
	}

	d.engineRunningMutex.Lock()
	d.engineRunning = false
	d.engineRunningMutex.Unlock()
}

// DeviceProperties defines the properties of a device
type DeviceProperties struct {
	CUCount  int
	DRAMSize uint64
}

// RegisterGPU tells the driver about the existence of a GPU
func (d *Driver) RegisterGPU(
	commandProcessorPort sim.Port,
	properties DeviceProperties,
) {
	d.GPUs = append(d.GPUs, commandProcessorPort)

	gpuDevice := &internal.Device{
		ID:       len(d.GPUs),
		Type:     internal.DeviceTypeGPU,
		MemState: internal.NewDeviceMemoryState(d.Log2PageSize),
		Properties: internal.DeviceProperties{
			CUCount:  properties.CUCount,
			DRAMSize: properties.DRAMSize,
		},
	}
	gpuDevice.SetTotalMemSize(properties.DRAMSize)
	d.memAllocator.RegisterDevice(gpuDevice)

	d.devices = append(d.devices, gpuDevice)
}

// Tick ticks
func (d *Driver) Tick() bool {
	madeProgress := false

	madeProgress = d.sendToGPUs() || madeProgress
	madeProgress = d.sendToMMU() || madeProgress
	madeProgress = d.sendMigrationReqToCP() || madeProgress

	for _, mw := range d.middlewares {
		madeProgress = mw.Tick() || madeProgress
	}

	madeProgress = d.processReturnReq() || madeProgress
	madeProgress = d.processNewCommand() || madeProgress
	madeProgress = d.parseFromMMU() || madeProgress

	return madeProgress
}

func (d *Driver) sendToGPUs() bool {
	if len(d.requestsToSend) == 0 {
		return false
	}

	req := d.requestsToSend[0]
	err := d.gpuPort.Send(req)
	if err == nil {
		d.requestsToSend = d.requestsToSend[1:]
		return true
	}

	return false
}

//nolint:gocyclo
func (d *Driver) processReturnReq() bool {
	req := d.gpuPort.PeekIncoming()
	if req == nil {
		return false
	}

	switch req := req.(type) {
	case *protocol.LaunchKernelRsp:
		d.gpuPort.RetrieveIncoming()
		return d.processLaunchKernelReturn(req)
	case *protocol.RDMADrainRspToDriver:
		d.gpuPort.RetrieveIncoming()
		return d.processRDMADrainRsp(req)
	case *protocol.ShootDownCompleteRsp:
		d.gpuPort.RetrieveIncoming()
		return d.processShootdownCompleteRsp(req)
	case *protocol.PageMigrationRspToDriver:
		d.gpuPort.RetrieveIncoming()
		return d.processPageMigrationRspFromCP(req)
	case *protocol.RDMARestartRspToDriver:
		d.gpuPort.RetrieveIncoming()
		return d.processRDMARestartRspToDriver(req)
	case *protocol.GPURestartRsp:
		d.gpuPort.RetrieveIncoming()
		return d.handleGPURestartRsp(req)
	}

	return false
}

func (d *Driver) processNewCommand() bool {
	madeProgress := false

	d.contextMutex.Lock()
	for _, ctx := range d.contexts {
		madeProgress = d.processNewCommandFromContext(ctx) || madeProgress
	}
	d.contextMutex.Unlock()

	return madeProgress
}

func (d *Driver) ensureManagedCompletionFlush(ctx *Context) bool {
	if ctx == nil || !ctx.shouldStartManagedCompletionFlush() {
		return false
	}

	queue := d.CreateCommandQueue(ctx)
	cmd := &FlushCommand{
		ID:                       sim.GetIDGenerator().Generate(),
		L1Only:                   true,
		ManagedCompletionBarrier: true,
	}
	if !ctx.beginManagedCompletionFlush(queue) {
		return false
	}
	d.Enqueue(queue, cmd)
	return true
}

func (d *Driver) processNewCommandFromContext(
	ctx *Context,
) bool {
	madeProgress := false
	madeProgress = d.ensureManagedCompletionFlush(ctx) || madeProgress

	ctx.queueMutex.Lock()
	queues := append([]*CommandQueue(nil), ctx.queues...)
	ctx.queueMutex.Unlock()

	// Phase 1: process all non-managed head commands first. This lets command
	// preparation work (for example H2D copies of kernel args / packets) advance
	// even when the actual launch is still waiting on dependencies or resources.
	for _, q := range queues {
		if q == nil || q.NumCommand() == 0 || q.IsRunning {
			continue
		}
		if _, _, ok := d.managedLaunchAtQueueHead(q); ok {
			continue
		}
		madeProgress = d.processOneCommand(q) || madeProgress
	}

	if ctx.hasManagedCompletionBarrier() {
		return madeProgress
	}

	// Phase 2: among managed launch commands that are now ready, start them in
	// priority order while there is available estimated CU capacity.
	for {
		ready := d.readyManagedLaunchQueues(ctx, queues)
		if len(ready) == 0 {
			break
		}
		if !d.processOneCommand(ready[0]) {
			break
		}
		madeProgress = true
	}

	return madeProgress
}

func (d *Driver) processNewCommandFromCmdQueue(
	q *CommandQueue,
) bool {
	if q.NumCommand() == 0 {
		return false
	}

	if q.IsRunning {
		return false
	}

	return d.processOneCommand(q)
}

func (d *Driver) processOneCommand(
	cmdQueue *CommandQueue,
) bool {
	cmd := cmdQueue.Peek()

	switch cmd := cmd.(type) {
	case *LaunchKernelCommand:
		if !d.processLaunchKernelCommand(cmd, cmdQueue) {
			return false
		}
		d.logCmdStart(cmd)
		return true
	case *NoopCommand:
		if !d.processNoopCommand(cmd, cmdQueue) {
			return false
		}
		d.logCmdStart(cmd)
		d.logCmdComplete(cmd)
		return true
	case *LaunchUnifiedMultiGPUKernelCommand:
		if !d.processUnifiedMultiGPULaunchKernelCommand(cmd, cmdQueue) {
			return false
		}
		d.logCmdStart(cmd)
		return true
	default:
		return d.processCommandWithMiddleware(cmd, cmdQueue)
	}
}

func (d *Driver) processCommandWithMiddleware(
	cmd Command,
	cmdQueue *CommandQueue,
) bool {
	for _, m := range d.middlewares {
		processed := m.ProcessCommand(cmd, cmdQueue)

		if processed {
			d.logCmdStart(cmd)
			return true
		}
	}

	return false
}

func (d *Driver) logCmdStart(cmd Command) {
	// Only attach the command object to the trace when it is needed.
	//
	// Some commands (e.g., MemCopy) may hold large buffers. Attaching them as
	// trace details can unnecessarily increase memory footprint. For per-kernel
	// reporting we only need kernel launch commands.
	var detail interface{}
	switch cmd.(type) {
	case *LaunchKernelCommand, *LaunchUnifiedMultiGPUKernelCommand:
		detail = cmd
	default:
		detail = nil
	}

	tracing.StartTask(
		cmd.GetID(),
		d.simulationID,
		d,
		"Driver Command",
		reflect.TypeOf(cmd).String(),
		detail,
	)
}

func (d *Driver) logCmdComplete(cmd Command) {
	tracing.EndTask(cmd.GetID(), d)
}

func (d *Driver) logCmdCompleteByID(cmdID string) {
	if cmdID == "" {
		return
	}
	tracing.EndTask(cmdID, d)
}

func managedLaunchWindowSummary(cmd *LaunchKernelCommand) (start int, count int) {
	if cmd == nil {
		return 0, 0
	}
	if len(cmd.WGWindows) == 1 {
		w := cmd.WGWindows[0]
		if w.Count > 0 {
			return int(w.Start), int(w.Count)
		}
	}
	if cmd.WGCount > 0 {
		return int(cmd.WGStart), int(cmd.WGCount)
	}
	return 0, 0
}

func (d *Driver) processNoopCommand(
	cmd *NoopCommand,
	queue *CommandQueue,
) bool {
	queue.Dequeue()
	return true
}

func (d *Driver) logTaskToGPUInitiate(
	cmd Command,
	req sim.Msg,
) {
	tracing.TraceReqInitiate(req, d, cmd.GetID())
}

func (d *Driver) logTaskToGPUClear(
	req sim.Msg,
) {
	tracing.TraceReqFinalize(req, d)
}

func sanitizeSubWGFactor(cmd *LaunchKernelCommand) int {
	if cmd == nil {
		return 1
	}
	factor := int(cmd.SubWGFactor)
	if factor <= 1 {
		return 1
	}
	if cmd.CodeObject == nil || cmd.Packet == nil {
		return 1
	}
	if cmd.CodeObject.WGGroupSegmentByteSize > 0 || cmd.CodeObject.WGFBarrierCount > 0 {
		return 1
	}
	return kernels.SanitizeSubWGFactor(cmd.Packet, factor)
}

func (d *Driver) processLaunchKernelCommand(
	cmd *LaunchKernelCommand,
	queue *CommandQueue,
) bool {
	if cmd.Managed {
		st := queue.Context.managedLaunchState(cmd.LaunchAbsID)
		if st == nil {
			return false
		}
		wgCount, estCUs, chipletCUs, allowedCUs, ok := d.chooseManagedLaunchChunk(queue.Context, st)
		if !ok {
			return false
		}
		startWG := st.NextWG
		st.CurrentChunkWGStart = startWG
		st.CurrentChunkWGCount = wgCount
		st.CurrentChunkEstCUs = estCUs
		st.CurrentChunkChipletCU = append([]int(nil), chipletCUs...)
		st.CurrentChunkAllowedCUs = append([]int(nil), allowedCUs...)
		st.NextWG += wgCount
		st.RemainingWGs -= wgCount
		if st.RemainingWGs < 0 {
			st.RemainingWGs = 0
		}
		cmd.WGStart = startWG
		cmd.WGCount = wgCount
		if len(st.ExplicitWGWindows) > 0 {
			cmd.WGWindows = append([]plan.WGWindow(nil), st.ExplicitWGWindows...)
			if st.AdaptiveWGChunks {
				cmd.WGFilter = makeWGWindowUnionSliceFilter(st.ExplicitWGWindows, startWG, wgCount)
			} else {
				cmd.WGFilter = makeWGWindowUnionFilter(st.ExplicitWGWindows)
			}
		} else {
			cmd.WGFilter = makeWGWindowFilter(startWG, wgCount)
		}
		cmd.EstCUs = estCUs
		cmd.ChipletCUs = append([]int(nil), chipletCUs...)
		cmd.AllowedCUIDs = append([]int(nil), allowedCUs...)
		queue.Context.managedLaunchStarted(cmd.LaunchAbsID, queue.GPUID, cmd.EstCUs, cmd.ChipletCUs, cmd.AllowedCUIDs)
	}

	req := protocol.NewLaunchKernelReq(d.gpuPort,
		d.GPUs[queue.GPUID-1])
	req.PID = queue.Context.pid
	req.HsaCo = cmd.CodeObject

	// Propagate per-kernel scheduling hints from the plan (if installed).
	if cmd.CodeObject != nil && cmd.CodeObject.Symbol != nil {
		req.KernelName = cmd.CodeObject.Symbol.Name
	}
	if d.plan != nil {
		if policy, partition, ok := d.plan.KernelSchedulingHint(req.KernelName); ok {
			req.SchedulingPolicy = policy
			req.SchedPartition = partition
		}
	}
	if cmd.PolicyOverride != "" {
		req.SchedulingPolicy = cmd.PolicyOverride
	}
	if cmd.PartitionOverride != "" {
		req.SchedPartition = cmd.PartitionOverride
	}

	req.Packet = cmd.Packet
	req.PacketAddress = uint64(cmd.DPacket)
	req.WGFilter = cmd.WGFilter
	req.ProgramLaunchID = cmd.ProgramLaunchID
	req.LaunchAbsID = cmd.LaunchAbsID
	req.LaunchRelID = cmd.LaunchRelID
	req.WGWindowStart, req.WGWindowCount = managedLaunchWindowSummary(cmd)
	req.AllowedCUIDs = append([]int(nil), cmd.AllowedCUIDs...)
	req.SubWGFactor = sanitizeSubWGFactor(cmd)

	queue.IsRunning = true
	cmd.Reqs = append(cmd.Reqs, req)

	d.requestsToSend = append(d.requestsToSend, req)

	queue.Context.l2Dirty = true
	queue.Context.markAllBuffersDirty()

	d.logTaskToGPUInitiate(cmd, req)

	return true
}

func (d *Driver) processUnifiedMultiGPULaunchKernelCommand(
	cmd *LaunchUnifiedMultiGPUKernelCommand,
	queue *CommandQueue,
) bool {
	wgDist := d.distributeWGToGPUs(queue, cmd)

	dev := d.devices[queue.GPUID]
	for i, gpuID := range dev.UnifiedGPUIDs {
		if wgDist[i+1]-wgDist[i] == 0 {
			continue
		}

		req := protocol.NewLaunchKernelReq(d.gpuPort, d.GPUs[gpuID-1])
		req.PID = queue.Context.pid
		req.HsaCo = cmd.CodeObject

		// Propagate per-kernel scheduling hints from the plan (if installed).
		if cmd.CodeObject != nil && cmd.CodeObject.Symbol != nil {
			req.KernelName = cmd.CodeObject.Symbol.Name
		}
		if d.plan != nil {
			if policy, partition, ok := d.plan.KernelSchedulingHint(req.KernelName); ok {
				req.SchedulingPolicy = policy
				req.SchedPartition = partition
			}
		}
		req.Packet = cmd.PacketArray[i]
		req.PacketAddress = uint64(cmd.DPacketArray[i])

		currentGPUIndex := i
		req.WGFilter = func(
			pkt *kernels.HsaKernelDispatchPacket,
			wg *kernels.WorkGroup,
		) bool {
			numWGX := (pkt.GridSizeX-1)/uint32(pkt.WorkgroupSizeX) + 1
			numWGY := (pkt.GridSizeY-1)/uint32(pkt.WorkgroupSizeY) + 1

			flattenedID :=
				wg.IDZ*int(numWGX)*int(numWGY) +
					wg.IDY*int(numWGX) +
					wg.IDX

			if flattenedID >= wgDist[currentGPUIndex] &&
				flattenedID < wgDist[currentGPUIndex+1] {
				return true
			}

			return false
		}

		queue.IsRunning = true
		cmd.Reqs = append(cmd.Reqs, req)

		d.requestsToSend = append(d.requestsToSend, req)

		queue.Context.l2Dirty = true
		queue.Context.markAllBuffersDirty()

		d.logTaskToGPUInitiate(cmd, req)
	}

	return true
}

func (d *Driver) enqueueKernelBoundaryL1Flushes(cmd Command) {
	for _, gpu := range d.GPUs {
		req := protocol.NewL1FlushReq(d.gpuPort, gpu)
		d.requestsToSend = append(d.requestsToSend, req)
		cmd.AddReq(req)
		d.logTaskToGPUInitiate(cmd, req)
	}
}

func (d *Driver) distributeWGToGPUs(
	queue *CommandQueue,
	cmd *LaunchUnifiedMultiGPUKernelCommand,
) []int {
	dev := d.devices[queue.GPUID]
	actualGPUs := dev.UnifiedGPUIDs
	wgAllocated := 0
	wgDist := make([]int, len(actualGPUs)+1)

	totalCUCount := 0
	for _, devID := range actualGPUs {
		totalCUCount += d.devices[devID].Properties.CUCount
	}

	numWGX := (cmd.PacketArray[0].GridSizeX-1)/uint32(cmd.PacketArray[0].WorkgroupSizeX) + 1
	numWGY := (cmd.PacketArray[0].GridSizeY-1)/uint32(cmd.PacketArray[0].WorkgroupSizeY) + 1
	numWGZ := (cmd.PacketArray[0].GridSizeZ-1)/uint32(cmd.PacketArray[0].WorkgroupSizeZ) + 1
	totalWGCount := int(numWGX * numWGY * numWGZ)
	wgPerCU := (totalWGCount-1)/totalCUCount + 1

	for i, devID := range actualGPUs {
		cuCount := d.devices[devID].Properties.CUCount
		wgToAllocate := cuCount * wgPerCU
		wgDist[i+1] = wgAllocated + wgToAllocate
		wgAllocated += wgToAllocate
	}

	if wgAllocated < totalWGCount {
		panic("not all wg allocated")
	}

	return wgDist
}

func (d *Driver) processLaunchKernelReturn(
	rsp *protocol.LaunchKernelRsp,
) bool {
	req, cmd, cmdQueue := d.findCommandByReqID(rsp.RspTo)
	cmd.RemoveReq(req)

	d.logTaskToGPUClear(req)

	if len(cmd.GetReqs()) == 0 {
		// Normal synchronous kernel launches are OpenCL kernel-boundary barriers.
		// The timing model has private per-CU L1 caches, so invalidate/flush L1V/L1S
		// before allowing the next queued kernel to start. This does not touch L2.
		if launchCmd, ok := cmd.(*LaunchKernelCommand); ok && !launchCmd.Managed {
			d.enqueueKernelBoundaryL1Flushes(cmd)
			return true
		}

		keepCommand := false
		completedCmdID := cmd.GetID()
		if launchCmd, ok := cmd.(*LaunchKernelCommand); ok && launchCmd.Managed {
			completed := cmdQueue.Context.managedLaunchChunkCompleted(
				launchCmd.LaunchAbsID,
				cmdQueue.GPUID,
				launchCmd.EstCUs,
				launchCmd.ChipletCUs,
				launchCmd.AllowedCUIDs,
			)
			if !completed {
				keepCommand = true
				launchCmd.ID = sim.GetIDGenerator().Generate()
				launchCmd.WGStart = 0
				launchCmd.WGCount = 0
				launchCmd.WGFilter = nil
				launchCmd.EstCUs = 0
				launchCmd.ChipletCUs = nil
				launchCmd.AllowedCUIDs = nil
			}
		}

		cmdQueue.IsRunning = false
		if !keepCommand {
			cmdQueue.Dequeue()
		}

		d.logCmdCompleteByID(completedCmdID)
	}

	return true
}

func (d *Driver) findCommandByReq(req sim.Msg) (Command, *CommandQueue) {
	d.contextMutex.Lock()
	defer d.contextMutex.Unlock()

	for _, ctx := range d.contexts {
		ctx.queueMutex.Lock()
		for _, q := range ctx.queues {
			cmd := q.Peek()
			if cmd == nil {
				continue
			}

			reqs := cmd.GetReqs()
			for _, r := range reqs {
				if r == req {
					ctx.queueMutex.Unlock()
					return cmd, q
				}
			}
		}
		ctx.queueMutex.Unlock()
	}

	panic("cannot find command")
}

func (d *Driver) findCommandByReqID(reqID string) (
	sim.Msg,
	Command,
	*CommandQueue,
) {
	d.contextMutex.Lock()
	defer d.contextMutex.Unlock()

	for _, ctx := range d.contexts {
		ctx.queueMutex.Lock()

		for _, q := range ctx.queues {
			cmd := q.Peek()
			if cmd == nil {
				continue
			}

			reqs := cmd.GetReqs()
			for _, r := range reqs {
				if r.Meta().ID == reqID {
					ctx.queueMutex.Unlock()
					return r, cmd, q
				}
			}
		}

		ctx.queueMutex.Unlock()
	}

	panic("cannot find command")
}

func (d *Driver) parseFromMMU() bool {
	if d.isCurrentlyHandlingMigrationReq {
		return false
	}

	req := d.mmuPort.RetrieveIncoming()
	if req == nil {
		return false
	}

	switch req := req.(type) {
	case *vm.PageMigrationReqToDriver:
		d.currentPageMigrationReq = req
		d.isCurrentlyHandlingMigrationReq = true
		d.initiateRDMADrain()
	default:
		log.Panicf("Driver cannot handle request of type %s",
			reflect.TypeOf(req))
	}

	return true
}

func (d *Driver) initiateRDMADrain() bool {
	for i := 0; i < len(d.GPUs); i++ {
		req := protocol.NewRDMADrainCmdFromDriver(d.gpuPort,
			d.GPUs[i])
		d.requestsToSend = append(d.requestsToSend, req)
		d.numRDMADrainACK++
	}

	return true
}

func (d *Driver) processRDMADrainRsp(
	req *protocol.RDMADrainRspToDriver,
) bool {
	d.numRDMADrainACK--

	if d.numRDMADrainACK == 0 {
		d.sendShootDownReqs()
	}

	return true
}

func (d *Driver) sendShootDownReqs() bool {
	vAddr := make([]uint64, 0)
	migrationInfo := d.currentPageMigrationReq.MigrationInfo

	numReqsGPUInMap := 0
	for i := 1; i < d.GetNumGPUs()+1; i++ {
		pages, found := migrationInfo.GPUReqToVAddrMap[uint64(i)]

		if found {
			numReqsGPUInMap++
			for j := 0; j < len(pages); j++ {
				vAddr = append(vAddr, pages[j])
			}
		}
	}

	accessingGPUs := d.currentPageMigrationReq.CurrAccessingGPUs
	pid := d.currentPageMigrationReq.PID
	d.numShootDownACK = uint64(len(accessingGPUs))

	for i := 0; i < len(accessingGPUs); i++ {
		toShootdownGPU := accessingGPUs[i] - 1
		shootDownReq := protocol.NewShootdownCommand(
			d.gpuPort, d.GPUs[toShootdownGPU],
			vAddr, pid)
		d.requestsToSend = append(d.requestsToSend, shootDownReq)
	}

	return true
}

func (d *Driver) processShootdownCompleteRsp(
	req *protocol.ShootDownCompleteRsp,
) bool {
	d.numShootDownACK--

	if d.numShootDownACK == 0 {
		toRequestFromGPU := d.currentPageMigrationReq.CurrPageHostGPU
		toRequestFromPMCPort := d.RemotePMCPorts[toRequestFromGPU-1]

		migrationInfo := d.currentPageMigrationReq.MigrationInfo

		requestingGPUs := d.findRequestingGPUs(migrationInfo)
		context := d.findContext(d.currentPageMigrationReq.PID)

		pageVaddrs := make(map[uint64][]uint64)

		for i := 0; i < len(requestingGPUs); i++ {
			pageVaddrs[requestingGPUs[i]] =
				migrationInfo.GPUReqToVAddrMap[requestingGPUs[i]+1]
		}

		for gpuID, vAddrs := range pageVaddrs {
			for i := 0; i < len(vAddrs); i++ {
				vAddr := vAddrs[i]
				page, oldPAddr :=
					d.preparePageForMigration(vAddr, context, gpuID)

				req := protocol.NewPageMigrationReqToCP(d.gpuPort,
					d.GPUs[gpuID])
				req.DestinationPMCPort = toRequestFromPMCPort
				req.ToReadFromPhysicalAddress = oldPAddr
				req.ToWriteToPhysicalAddress = page.PAddr
				req.PageSize = d.currentPageMigrationReq.PageSize

				d.migrationReqToSendToCP = append(d.migrationReqToSendToCP, req)
				d.numPagesMigratingACK++
			}
		}
		return true
	}

	return false
}

func (d *Driver) findRequestingGPUs(
	migrationInfo *vm.PageMigrationInfo,
) []uint64 {
	requestingGPUs := make([]uint64, 0)

	for i := 1; i < d.GetNumGPUs()+1; i++ {
		_, found := migrationInfo.GPUReqToVAddrMap[uint64(i)]
		if found {
			requestingGPUs = append(requestingGPUs, uint64(i-1))
		}
	}
	return requestingGPUs
}

func (d *Driver) findContext(pid vm.PID) *Context {
	context := &Context{}
	for i := 0; i < len(d.contexts); i++ {
		if d.contexts[i].pid == d.currentPageMigrationReq.PID {
			context = d.contexts[i]
		}
	}
	if context == nil {
		log.Panicf("Process does not exist")
	}
	return context
}

func (d *Driver) preparePageForMigration(
	vAddr uint64,
	context *Context,
	gpuID uint64,
) (*vm.Page, uint64) {
	page, found := d.pageTable.Find(context.pid, vAddr)
	if !found {
		panic("page not founds")
	}
	oldPAddr := page.PAddr

	newPage := d.memAllocator.AllocatePageWithGivenVAddr(
		context.pid, int(gpuID+1), vAddr, true)
	newPage.DeviceID = gpuID + 1

	newPage.IsMigrating = true
	d.pageTable.Update(newPage)

	return &newPage, oldPAddr
}

func (d *Driver) sendMigrationReqToCP() bool {
	if len(d.migrationReqToSendToCP) == 0 {
		return false
	}

	if d.isCurrentlyMigratingOnePage {
		return false
	}

	req := d.migrationReqToSendToCP[0]

	err := d.gpuPort.Send(req)
	if err == nil {
		d.migrationReqToSendToCP = d.migrationReqToSendToCP[1:]
		d.isCurrentlyMigratingOnePage = true
		return true
	}

	return false
}

func (d *Driver) processPageMigrationRspFromCP(
	rsp *protocol.PageMigrationRspToDriver,
) bool {
	d.numPagesMigratingACK--
	d.isCurrentlyMigratingOnePage = false

	if d.numPagesMigratingACK == 0 {
		d.prepareGPURestartReqs()
		d.preparePageMigrationRspToMMU()
	}

	return true
}

func (d *Driver) prepareGPURestartReqs() {
	accessingGPUs := d.currentPageMigrationReq.CurrAccessingGPUs

	for i := 0; i < len(accessingGPUs); i++ {
		restartGPUID := accessingGPUs[i] - 1
		restartReq := protocol.NewGPURestartReq(
			d.gpuPort,
			d.GPUs[restartGPUID])
		d.requestsToSend = append(d.requestsToSend, restartReq)
		d.numRestartACK++
	}
}

func (d *Driver) preparePageMigrationRspToMMU() {
	requestingGPUs := make([]uint64, 0)

	migrationInfo := d.currentPageMigrationReq.MigrationInfo

	for i := 1; i < d.GetNumGPUs()+1; i++ {
		_, found := migrationInfo.GPUReqToVAddrMap[uint64(i)]
		if found {
			requestingGPUs = append(requestingGPUs, uint64(i-1))
		}
	}

	pageVaddrs := make(map[uint64][]uint64)

	for i := 0; i < len(requestingGPUs); i++ {
		pageVaddrs[requestingGPUs[i]] = migrationInfo.GPUReqToVAddrMap[requestingGPUs[i]+1]
	}

	req := vm.NewPageMigrationRspFromDriver(d.mmuPort.AsRemote(),
		d.currentPageMigrationReq.Src, d.currentPageMigrationReq)

	for _, vAddrs := range pageVaddrs {
		for j := 0; j < len(vAddrs); j++ {
			req.VAddr = append(req.VAddr, vAddrs[j])
		}
	}
	req.RspToTop = d.currentPageMigrationReq.RespondToTop
	d.toSendToMMU = req
}

func (d *Driver) handleGPURestartRsp(
	req *protocol.GPURestartRsp,
) bool {
	d.numRestartACK--
	if d.numRestartACK == 0 {
		d.prepareRDMARestartReqs()
	}
	return true
}

func (d *Driver) prepareRDMARestartReqs() {
	for i := 0; i < len(d.GPUs); i++ {
		req := protocol.NewRDMARestartCmdFromDriver(d.gpuPort, d.GPUs[i])
		d.requestsToSend = append(d.requestsToSend, req)
		d.numRDMARestartACK++
	}
}

func (d *Driver) processRDMARestartRspToDriver(
	rsp *protocol.RDMARestartRspToDriver) bool {
	d.numRDMARestartACK--

	if d.numRDMARestartACK == 0 {
		d.currentPageMigrationReq = nil
		d.isCurrentlyHandlingMigrationReq = false
		return true
	}
	return true
}

func (d *Driver) sendToMMU() bool {
	if d.toSendToMMU == nil {
		return false
	}
	req := d.toSendToMMU
	err := d.mmuPort.Send(req)
	if err == nil {
		d.toSendToMMU = nil
		return true
	}

	return false
}

// BufferInfo describes a user-visible buffer allocated via the driver API.
// It is primarily used by profilers and external analysis tools.
type BufferInfo struct {
	PID   vm.PID `json:"pid"`
	Tag   string `json:"tag"`
	VAddr uint64 `json:"vaddr"`
	Size  uint64 `json:"size"`
}

// ListBuffers returns a snapshot of all buffers allocated in all contexts.
// The returned slice is safe to use after the call returns.
func (d *Driver) ListBuffers() []BufferInfo {
	d.contextMutex.Lock()
	defer d.contextMutex.Unlock()

	out := make([]BufferInfo, 0, 128)
	for _, ctx := range d.contexts {
		for _, b := range ctx.buffers {
			out = append(out, BufferInfo{
				PID:   ctx.pid,
				Tag:   b.tag,
				VAddr: uint64(b.vAddr),
				Size:  b.size,
			})
		}
	}
	return out
}
