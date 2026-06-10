package driver

import (
	"encoding/binary"
	"reflect"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/mgpusim/v4/amd/driver/internal"
	"github.com/sarchlab/mgpusim/v4/amd/insts"
	"github.com/sarchlab/mgpusim/v4/amd/kernels"
	"github.com/sarchlab/mgpusim/v4/amd/plan"
)

type launchRuntimeIDs struct {
	AbsID           int
	RelID           int
	ProgramLaunchID int
}

// EnqueueLaunchKernel schedules kernel to be launched later
func (d *Driver) EnqueueLaunchKernel(
	queue *CommandQueue,
	co *insts.HsaCo,
	gridSize [3]uint32,
	wgSize [3]uint16,
	kernelArgs interface{},
) {
	d.enqueueLaunchKernelWithRuntimeIDs(queue, co, gridSize, wgSize, kernelArgs, nil)
}

func (d *Driver) enqueueLaunchKernelWithRuntimeIDs(
	queue *CommandQueue,
	co *insts.HsaCo,
	gridSize [3]uint32,
	wgSize [3]uint16,
	kernelArgs interface{},
	runtimeIDs *launchRuntimeIDs,
) {
	if runtimeIDs == nil && queue != nil && queue.Context != nil {
		sequenceLength := 0
		if d.plan != nil {
			sequenceLength = d.plan.LaunchSequenceLength
		}
		absID, relID := queue.Context.nextLaunchIDs(sequenceLength)
		runtimeIDs = &launchRuntimeIDs{
			AbsID:           absID,
			RelID:           relID,
			ProgramLaunchID: relID,
		}
	}
	dev := d.devices[queue.GPUID]

	if dev.Type == internal.DeviceTypeUnifiedGPU {
		d.enqueueLaunchUnifiedKernel(queue, co, gridSize, wgSize, kernelArgs)
	} else {
		dCoData, dKernArgData, dPacket := d.allocateGPUMemory(queue.Context, co)

		packet := d.createAQLPacket(gridSize, wgSize, dCoData, dKernArgData)
		newKernelArgs := d.prepareLocalMemory(co, kernelArgs, packet)

		d.EnqueueMemCopyH2D(queue, dCoData, co.Data)
		d.EnqueueMemCopyH2D(queue, dKernArgData, newKernelArgs)
		d.EnqueueMemCopyH2D(queue, dPacket, packet)

		d.enqueueLaunchKernelCommandDetailed(queue, co, packet, dPacket, nil, runtimeIDs)
	}
}

func (d *Driver) allocateGPUMemory(
	ctx *Context,
	co *insts.HsaCo,
) (dCoData, dKernArgData, dPacket Ptr) {
	dCoData = d.AllocateMemory(ctx, uint64(len(co.Data)))
	dKernArgData = d.AllocateMemory(ctx, co.KernargSegmentByteSize)

	packet := kernels.HsaKernelDispatchPacket{}
	dPacket = d.AllocateMemory(ctx, uint64(binary.Size(packet)))

	return dCoData, dKernArgData, dPacket
}

func (d *Driver) prepareLocalMemory(
	co *insts.HsaCo,
	kernelArgs interface{},
	packet *kernels.HsaKernelDispatchPacket,
) (newKernelArgs interface{}) {
	newKernelArgs = reflect.New(reflect.TypeOf(kernelArgs).Elem()).Interface()
	reflect.ValueOf(newKernelArgs).Elem().
		Set(reflect.ValueOf(kernelArgs).Elem())

	ldsSize := co.WGGroupSegmentByteSize

	if reflect.TypeOf(newKernelArgs).Kind() == reflect.Slice {
		// From server, do nothing
	} else {
		kernArgStruct := reflect.ValueOf(newKernelArgs).Elem()
		for i := 0; i < kernArgStruct.NumField(); i++ {
			arg := kernArgStruct.Field(i).Interface()

			switch ldsPtr := arg.(type) {
			case LocalPtr:
				kernArgStruct.Field(i).SetUint(uint64(ldsSize))
				ldsSize += uint32(ldsPtr)
			}
		}
	}

	packet.GroupSegmentSize = ldsSize

	return newKernelArgs
}

// LaunchKernel is an easy way to run a kernel on the GCN3 simulator. It
// launches the kernel immediately.
func (d *Driver) LaunchKernel(
	ctx *Context,
	co *insts.HsaCo,
	gridSize [3]uint32,
	wgSize [3]uint16,
	kernelArgs interface{},
) {
	absID, relID, hints, managed := d.nextLaunchHints(ctx, co)
	if managed && len(hints) > 0 {
		d.launchKernelManaged(ctx, co, gridSize, wgSize, kernelArgs, absID, relID, hints)
		return
	}

	// If a previous pipelined burst is still in flight, make the synchronous
	// launch act as a conservative barrier so host-visible semantics stay intact.
	d.drainManagedLaunches(ctx)

	queue := d.CreateCommandQueue(ctx)
	programLaunchID := relID
	if programLaunchID < 0 {
		programLaunchID = 0
	}
	d.enqueueLaunchKernelWithRuntimeIDs(queue, co, gridSize, wgSize, kernelArgs, &launchRuntimeIDs{
		AbsID:           absID,
		RelID:           relID,
		ProgramLaunchID: programLaunchID,
	})
	d.DrainCommandQueue(queue)

	if d.plan != nil && d.plan.HasLaunchHints() {
		ctx.markLaunchCompleted(absID)
	}
}

func totalWorkgroupsInPacket(packet *kernels.HsaKernelDispatchPacket) int {
	if packet == nil {
		return 0
	}
	wgx := (int(packet.GridSizeX)-1)/maxInt(int(packet.WorkgroupSizeX), 1) + 1
	wgy := (int(packet.GridSizeY)-1)/maxInt(int(packet.WorkgroupSizeY), 1) + 1
	wgz := (int(packet.GridSizeZ)-1)/maxInt(int(packet.WorkgroupSizeZ), 1) + 1
	if wgx <= 0 || wgy <= 0 || wgz <= 0 {
		return 0
	}
	return wgx * wgy * wgz
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func flattenedWGID(pkt *kernels.HsaKernelDispatchPacket, wg *kernels.WorkGroup) int {
	if pkt == nil || wg == nil {
		return -1
	}
	numWGX := (pkt.GridSizeX-1)/uint32(pkt.WorkgroupSizeX) + 1
	numWGY := (pkt.GridSizeY-1)/uint32(pkt.WorkgroupSizeY) + 1
	return wg.IDZ*int(numWGX)*int(numWGY) + wg.IDY*int(numWGX) + wg.IDX
}

func makeWGWindowFilter(startWG, wgCount int) kernels.WGFilterFunc {
	if startWG <= 0 && wgCount <= 0 {
		return nil
	}
	if wgCount <= 0 {
		return nil
	}
	endWG := startWG + wgCount
	return func(pkt *kernels.HsaKernelDispatchPacket, wg *kernels.WorkGroup) bool {
		if pkt == nil || wg == nil {
			return false
		}
		flattenedID := flattenedWGID(pkt, wg)
		return flattenedID >= startWG && flattenedID < endWG
	}
}

func makeWGWindowUnionFilter(windows []plan.WGWindow) kernels.WGFilterFunc {
	if len(windows) == 0 {
		return nil
	}
	type normWindow struct{ start, end int }
	norm := make([]normWindow, 0, len(windows))
	for _, w := range windows {
		if w.Count <= 0 {
			continue
		}
		norm = append(norm, normWindow{start: w.Start, end: w.Start + w.Count})
	}
	if len(norm) == 0 {
		return nil
	}
	return func(pkt *kernels.HsaKernelDispatchPacket, wg *kernels.WorkGroup) bool {
		if pkt == nil || wg == nil {
			return false
		}
		flattenedID := flattenedWGID(pkt, wg)
		for _, win := range norm {
			if flattenedID >= win.start && flattenedID < win.end {
				return true
			}
		}
		return false
	}
}

func makeWGWindowUnionSliceFilter(windows []plan.WGWindow, startWG, wgCount int) kernels.WGFilterFunc {
	if len(windows) == 0 {
		return nil
	}
	if wgCount <= 0 {
		return nil
	}
	type normWindow struct{ start, end int }
	norm := make([]normWindow, 0, len(windows))
	for _, w := range windows {
		if w.Count <= 0 {
			continue
		}
		norm = append(norm, normWindow{start: w.Start, end: w.Start + w.Count})
	}
	if len(norm) == 0 {
		return nil
	}
	endWG := startWG + wgCount
	return func(pkt *kernels.HsaKernelDispatchPacket, wg *kernels.WorkGroup) bool {
		if pkt == nil || wg == nil {
			return false
		}
		flat := flattenedWGID(pkt, wg)
		rank := 0
		for _, win := range norm {
			if flat < win.start {
				break
			}
			if flat >= win.end {
				rank += win.end - win.start
				continue
			}
			local := rank + (flat - win.start)
			return local >= startWG && local < endWG
		}
		return false
	}
}

func (d *Driver) managedDispatchBase(absID int, relID int) int {
	if d.plan == nil {
		return absID - relID
	}
	launchSeq := d.plan.LaunchSequenceLength
	dispatchSeq := d.plan.DispatchSequenceLengthOrDefault(launchSeq)
	if launchSeq <= 0 || dispatchSeq <= 0 {
		return absID - relID
	}
	iter := 0
	if absID >= relID {
		iter = (absID - relID) / launchSeq
	}
	return iter * dispatchSeq
}

func (d *Driver) launchKernelManaged(
	ctx *Context,
	co *insts.HsaCo,
	gridSize [3]uint32,
	wgSize [3]uint16,
	kernelArgs interface{},
	absID int,
	relID int,
	hints []plan.LaunchHint,
) {
	setupQueue := d.CreateCommandQueue(ctx)
	dev := d.devices[setupQueue.GPUID]

	if dev.Type == internal.DeviceTypeUnifiedGPU {
		// Conservative fallback for unified GPUs until the launch manager learns
		// how to reserve resources across multiple actual GPUs.
		d.EnqueueLaunchKernel(setupQueue, co, gridSize, wgSize, kernelArgs)
		d.DrainCommandQueue(setupQueue)
		ctx.markLaunchCompleted(absID)
		if d.plan != nil {
			ctx.garbageCollectCompletedLaunches(d.plan.LaunchSequenceLength)
		}
		return
	}

	dCoData, dKernArgData, dPacket := d.allocateGPUMemory(setupQueue.Context, co)
	packet := d.createAQLPacket(gridSize, wgSize, dCoData, dKernArgData)
	newKernelArgs := d.prepareLocalMemory(co, kernelArgs, packet)

	// Ensure kernel object / args / packet are uploaded once before any managed
	// item launches start. The actual parallelism we care about is on kernel
	// execution, not on these small setup copies.
	d.EnqueueMemCopyH2D(setupQueue, dCoData, co.Data)
	d.EnqueueMemCopyH2D(setupQueue, dKernArgData, newKernelArgs)
	d.EnqueueMemCopyH2D(setupQueue, dPacket, packet)
	d.DrainCommandQueue(setupQueue)

	totalWGs := totalWorkgroupsInPacket(packet)
	devTotalCUs := d.totalCUCountForDevice(setupQueue.GPUID)
	dispatchBase := d.managedDispatchBase(absID, relID)

	for _, hint := range hints {
		itemQueue := d.CreateCommandQueue(ctx)
		itemAbsID := dispatchBase + hint.ID
		itemRelID := hint.ID
		state := buildManagedLaunchState(itemQueue, co, packet, itemAbsID, itemRelID, hint, devTotalCUs, d.managedPlanChipletCount(), totalWGs)
		ctx.registerManagedLaunch(state)
		d.enqueueLaunchKernelCommandDetailed(itemQueue, co, packet, dPacket, state, nil)
	}

	d.enqueueSignal <- true
}

func (d *Driver) createAQLPacket(
	gridSize [3]uint32,
	wgSize [3]uint16,
	dCoData Ptr,
	dKernArgData Ptr,
) *kernels.HsaKernelDispatchPacket {
	packet := new(kernels.HsaKernelDispatchPacket)
	packet.GridSizeX = gridSize[0]
	packet.GridSizeY = gridSize[1]
	packet.GridSizeZ = gridSize[2]
	packet.WorkgroupSizeX = wgSize[0]
	packet.WorkgroupSizeY = wgSize[1]
	packet.WorkgroupSizeZ = wgSize[2]
	packet.KernelObject = uint64(dCoData)
	packet.KernargAddress = uint64(dKernArgData)
	return packet
}

func (d *Driver) enqueueLaunchKernelCommand(
	queue *CommandQueue,
	co *insts.HsaCo,
	packet *kernels.HsaKernelDispatchPacket,
	dPacket Ptr,
) {
	d.enqueueLaunchKernelCommandDetailed(queue, co, packet, dPacket, nil, nil)
}

func (d *Driver) enqueueLaunchKernelCommandDetailed(
	queue *CommandQueue,
	co *insts.HsaCo,
	packet *kernels.HsaKernelDispatchPacket,
	dPacket Ptr,
	managedState *managedLaunchState,
	runtimeIDs *launchRuntimeIDs,
) {
	var gridSize [3]uint32
	var wgSize [3]uint16
	if packet != nil {
		gridSize = [3]uint32{packet.GridSizeX, packet.GridSizeY, packet.GridSizeZ}
		wgSize = [3]uint16{packet.WorkgroupSizeX, packet.WorkgroupSizeY, packet.WorkgroupSizeZ}
	}

	cmd := &LaunchKernelCommand{
		ID:         sim.GetIDGenerator().Generate(),
		CodeObject: co,
		GridSize:   gridSize,
		WGSize:     wgSize,
		DPacket:    dPacket,
		Packet:     packet,
	}
	if managedState != nil {
		cmd.Managed = true
		cmd.LaunchAbsID = managedState.AbsID
		cmd.LaunchRelID = managedState.RelID
		cmd.ProgramLaunchID = managedState.ProgramLaunchID
		cmd.EstCUs = managedState.EstCUs
		cmd.ChipletCUs = append([]int(nil), managedState.ChipletCUs...)
		cmd.EstDurationPs = managedState.EstDurationPs
		cmd.Priority = managedState.Priority
		cmd.PolicyOverride = managedState.Policy
		cmd.PartitionOverride = managedState.Partition
		cmd.WGWindows = append([]plan.WGWindow(nil), managedState.ExplicitWGWindows...)
		cmd.SubWGFactor = managedState.SubWGFactor
	} else if runtimeIDs != nil {
		cmd.LaunchAbsID = runtimeIDs.AbsID
		cmd.LaunchRelID = runtimeIDs.RelID
		cmd.ProgramLaunchID = runtimeIDs.ProgramLaunchID
	}
	d.Enqueue(queue, cmd)
}

func (d *Driver) enqueueLaunchUnifiedKernelCommand(
	queue *CommandQueue,
	co *insts.HsaCo,
	packet []*kernels.HsaKernelDispatchPacket,
	dPacket []Ptr,
) {
	var gridSize [3]uint32
	var wgSize [3]uint16
	if len(packet) > 0 && packet[0] != nil {
		p0 := packet[0]
		gridSize = [3]uint32{p0.GridSizeX, p0.GridSizeY, p0.GridSizeZ}
		wgSize = [3]uint16{p0.WorkgroupSizeX, p0.WorkgroupSizeY, p0.WorkgroupSizeZ}
	}

	cmd := &LaunchUnifiedMultiGPUKernelCommand{
		ID:           sim.GetIDGenerator().Generate(),
		CodeObject:   co,
		GridSize:     gridSize,
		WGSize:       wgSize,
		DPacketArray: dPacket,
		PacketArray:  packet,
	}
	d.Enqueue(queue, cmd)
}
func (d *Driver) enqueueLaunchUnifiedKernel(
	queue *CommandQueue,
	co *insts.HsaCo,
	gridSize [3]uint32,
	wgSize [3]uint16,
	kernelArgs interface{},
) {
	dev := d.devices[queue.GPUID]
	initGPUID := queue.Context.currentGPUID
	queueArray := make([]*CommandQueue, len(dev.UnifiedGPUIDs)+1)
	dCoDataArray := make([]Ptr, len(dev.UnifiedGPUIDs)+1)
	dKernArgDataArray := make([]Ptr, len(dev.UnifiedGPUIDs)+1)
	dPacketArray := make([]Ptr, len(dev.UnifiedGPUIDs)+1)
	packetArray := make([]*kernels.HsaKernelDispatchPacket, len(dev.UnifiedGPUIDs)+1)
	// fmt.Printf("# of GPUs : %v \n", len(dev.UnifiedGPUIDs))

	for i, gpuID := range dev.UnifiedGPUIDs {
		queueArray[i] = queue
		queueArray[i].Context.currentGPUID = gpuID
		dCoData, dKernArgData, dPacket := d.allocateGPUMemory(queue.Context, co)

		packet := d.createAQLPacket(gridSize, wgSize, dCoData, dKernArgData)
		newKernelArgs := d.prepareLocalMemory(co, kernelArgs, packet)

		d.EnqueueMemCopyH2D(queue, dCoData, co.Data)
		d.EnqueueMemCopyH2D(queue, dKernArgData, newKernelArgs)
		d.EnqueueMemCopyH2D(queue, dPacket, packet)

		dCoDataArray[i] = dCoData
		dKernArgDataArray[i] = dKernArgData
		dPacketArray[i] = dPacket
		packetArray[i] = packet
		// fmt.Printf("packetArray: %v \n", packetArray[i])
	}

	queue.Context.currentGPUID = initGPUID
	d.enqueueLaunchUnifiedKernelCommand(queue, co, packetArray, dPacketArray)
}
