package kernels

import "github.com/sarchlab/mgpusim/v4/amd/insts"

// WGFilterFunc is a filter
type WGFilterFunc func(
	*HsaKernelDispatchPacket,
	*WorkGroup,
) bool

// KernelLaunchInfo includes the necessary information to launch a kernel.
type KernelLaunchInfo struct {
	CodeObject *insts.HsaCo
	Packet     *HsaKernelDispatchPacket
	PacketAddr uint64
	WGFilter   WGFilterFunc

	// Optional scheduling hints propagated from the driver.
	KernelName       string
	SchedulingPolicy string
	SchedPartition   string
	ProgramLaunchID  int
	LaunchAbsID      int
	LaunchRelID      int
	WGWindowStart    int
	WGWindowCount    int

	// AllowedCUIDs restricts this launch to a concrete subset of CUs selected
	// by the driver runtime manager. When empty, the scheduler may use any CU
	// consistent with the partition/policy.
	AllowedCUIDs []int

	// SubWGFactor optionally micro-splits each original work-group into several
	// smaller sub-work-groups. The simulator keeps the original WG ID (hence the
	// same chiplet mapping) and only slices the local-ID space inside that WG.
	// Safe use currently requires no LDS/barrier and z==1; both 1D and simple 2D
	// work-groups are supported.
	SubWGFactor int
}

// A GridBuilder is the unit that can build a grid and its internal structure
// from a kernel and its launch parameters.
type GridBuilder interface {
	SetKernel(info KernelLaunchInfo)
	NumWG() int
	NextWG() *WorkGroup
	Skip(n int)
}

// NewGridBuilder creates a default grid builder
func NewGridBuilder() GridBuilder {
	return &gridBuilderImpl{}
}

type gridBuilderImpl struct {
	hsaco      *insts.HsaCo
	packet     *HsaKernelDispatchPacket
	filter     WGFilterFunc
	packetAddr uint64
	numWG      int

	xid, yid, zid int

	subWGFactor     int
	subWGIndex      int
	subWGXParts     int
	subWGYParts     int
	subSizeX        int
	subSizeY        int
	programLaunchID int
	launchAbsID     int
	launchRelID     int
	wgWindowStart   int
	wgWindowCount   int
}

type subWGShape struct {
	factor int
	xParts int
	yParts int
	subX   int
	subY   int
}

func betterSubWGShape(a, b subWGShape) bool {
	if a.factor != b.factor {
		return a.factor > b.factor
	}
	aMin := min(a.subX, a.subY)
	bMin := min(b.subX, b.subY)
	if aMin != bMin {
		return aMin > bMin
	}
	aAspect := abs(a.subX - a.subY)
	bAspect := abs(b.subX - b.subY)
	if aAspect != bAspect {
		return aAspect < bAspect
	}
	if a.subX != b.subX {
		return a.subX > b.subX
	}
	return a.subY > b.subY
}

func chooseSubWGShape(pkt *HsaKernelDispatchPacket, factor int) (subWGShape, bool) {
	if pkt == nil {
		return subWGShape{}, false
	}
	fullX := int(pkt.WorkgroupSizeX)
	fullY := int(pkt.WorkgroupSizeY)
	fullZ := int(pkt.WorkgroupSizeZ)
	if factor <= 1 {
		return subWGShape{factor: 1, xParts: 1, yParts: 1, subX: fullX, subY: fullY}, true
	}
	if fullX <= 0 || fullY <= 0 || fullZ != 1 {
		return subWGShape{}, false
	}
	// The current sub-WG implementation assumes every original work-group has
	// the same full shape. If the launch would generate edge work-groups, keep
	// the split path disabled so local-id slicing remains exact.
	if int(pkt.GridSizeX)%fullX != 0 || int(pkt.GridSizeY)%fullY != 0 || int(pkt.GridSizeZ)%fullZ != 0 {
		return subWGShape{}, false
	}
	if factor > fullX*fullY {
		factor = fullX * fullY
	}
	for f := factor; f > 1; f-- {
		best := subWGShape{}
		bestOK := false
		for xParts := 1; xParts <= min(f, fullX); xParts++ {
			if f%xParts != 0 || fullX%xParts != 0 {
				continue
			}
			yParts := f / xParts
			if yParts <= 0 || fullY%yParts != 0 {
				continue
			}
			cand := subWGShape{
				factor: f,
				xParts: xParts,
				yParts: yParts,
				subX:   fullX / xParts,
				subY:   fullY / yParts,
			}
			if fullY == 1 {
				if yParts != 1 || cand.subX < 4 {
					continue
				}
			} else {
				if cand.subX < 2 || cand.subY < 2 {
					continue
				}
			}
			if !bestOK || betterSubWGShape(cand, best) {
				best = cand
				bestOK = true
			}
		}
		if bestOK {
			return best, true
		}
	}
	return subWGShape{}, false
}

func effectiveSubWGFactor(pkt *HsaKernelDispatchPacket, factor int) int {
	shape, ok := chooseSubWGShape(pkt, factor)
	if !ok {
		return 1
	}
	if shape.factor <= 1 {
		return 1
	}
	return shape.factor
}

// SanitizeSubWGFactor exposes the same representability gate used by the
// grid builder so the driver and the dispatcher agree on which split factors are
// valid for a given launch packet.
func SanitizeSubWGFactor(pkt *HsaKernelDispatchPacket, factor int) int {
	return effectiveSubWGFactor(pkt, factor)
}

func (b *gridBuilderImpl) SetKernel(
	info KernelLaunchInfo,
) {
	b.hsaco = info.CodeObject
	b.packet = info.Packet
	b.packetAddr = info.PacketAddr
	b.filter = info.WGFilter
	b.xid = 0
	b.yid = 0
	b.zid = 0
	shape, ok := chooseSubWGShape(info.Packet, info.SubWGFactor)
	if ok {
		b.subWGFactor = max(1, shape.factor)
		b.subWGXParts = max(1, shape.xParts)
		b.subWGYParts = max(1, shape.yParts)
		b.subSizeX = max(1, shape.subX)
		b.subSizeY = max(1, shape.subY)
	} else {
		b.subWGFactor = 1
		b.subWGXParts = 1
		b.subWGYParts = 1
		b.subSizeX = max(1, int(info.Packet.WorkgroupSizeX))
		b.subSizeY = max(1, int(info.Packet.WorkgroupSizeY))
	}
	b.subWGIndex = 0
	b.programLaunchID = int(info.ProgramLaunchID)
	b.launchAbsID = int(info.LaunchAbsID)
	b.launchRelID = int(info.LaunchRelID)
	b.wgWindowStart = int(info.WGWindowStart)
	b.wgWindowCount = int(info.WGWindowCount)

	b.countWG()
}

func (b *gridBuilderImpl) Skip(n int) {
	for i := 0; i < n; i++ {
		b.NextWG()
	}
}

func (b *gridBuilderImpl) advanceOriginalWG() {
	xLeft := int(b.packet.GridSizeX) - b.xid*int(b.packet.WorkgroupSizeX)
	yLeft := int(b.packet.GridSizeY) - b.yid*int(b.packet.WorkgroupSizeY)
	zLeft := int(b.packet.GridSizeZ) - b.zid*int(b.packet.WorkgroupSizeZ)

	if xLeft <= 0 || yLeft <= 0 || zLeft <= 0 {
		return
	}

	xToAllocate := min(xLeft, int(b.packet.WorkgroupSizeX))
	yToAllocate := min(yLeft, int(b.packet.WorkgroupSizeY))
	zToAllocate := min(zLeft, int(b.packet.WorkgroupSizeZ))

	b.xid++
	xLeft -= xToAllocate
	if xLeft <= 0 {
		b.xid = 0
		b.yid++
		yLeft -= yToAllocate
		if yLeft <= 0 {
			b.yid = 0
			b.zid++
			_ = zToAllocate
		}
	}
	b.subWGIndex = 0
}

func (b *gridBuilderImpl) countWG() {
	x := int(b.packet.GridSizeX-1)/int(b.packet.WorkgroupSizeX) + 1
	y := int(b.packet.GridSizeY-1)/int(b.packet.WorkgroupSizeY) + 1
	z := int(b.packet.GridSizeZ-1)/int(b.packet.WorkgroupSizeZ) + 1
	factor := max(1, b.subWGFactor)

	if b.filter == nil {
		b.numWG = x * y * z * factor
		return
	}

	b.numWG = 0
	for i := 0; i < x; i++ {
		for j := 0; j < y; j++ {
			for k := 0; k < z; k++ {
				wg := WorkGroup{
					IDX: i,
					IDY: j,
					IDZ: k,
				}

				if b.filter(b.packet, &wg) {
					b.numWG += factor
				}
			}
		}
	}
}

func (b *gridBuilderImpl) NumWG() int {
	return b.numWG
}

func (b *gridBuilderImpl) NextWG() *WorkGroup {
	for {
		xLeft := int(b.packet.GridSizeX) - b.xid*int(b.packet.WorkgroupSizeX)
		yLeft := int(b.packet.GridSizeY) - b.yid*int(b.packet.WorkgroupSizeY)
		zLeft := int(b.packet.GridSizeZ) - b.zid*int(b.packet.WorkgroupSizeZ)

		if xLeft <= 0 || yLeft <= 0 || zLeft <= 0 {
			return nil
		}

		xToAllocate := min(xLeft, int(b.packet.WorkgroupSizeX))
		yToAllocate := min(yLeft, int(b.packet.WorkgroupSizeY))
		zToAllocate := min(zLeft, int(b.packet.WorkgroupSizeZ))

		probe := &WorkGroup{
			IDX: b.xid,
			IDY: b.yid,
			IDZ: b.zid,
		}

		if b.filter != nil && !b.filter(b.packet, probe) {
			b.advanceOriginalWG()
			continue
		}

		wg := NewWorkGroup()
		wg.Packet = b.packet
		wg.CodeObject = b.hsaco
		wg.SizeX = int(b.packet.WorkgroupSizeX)
		wg.SizeY = int(b.packet.WorkgroupSizeY)
		wg.SizeZ = int(b.packet.WorkgroupSizeZ)
		wg.IDX = b.xid
		wg.IDY = b.yid
		wg.IDZ = b.zid
		wg.CurrSizeX = xToAllocate
		wg.CurrSizeY = yToAllocate
		wg.CurrSizeZ = zToAllocate
		wg.ProgramLaunchID = b.programLaunchID
		wg.LaunchAbsID = b.launchAbsID
		wg.LaunchRelID = b.launchRelID
		wg.WGWindowStart = b.wgWindowStart
		wg.WGWindowCount = b.wgWindowCount

		if b.subWGFactor > 1 {
			xPart := b.subWGIndex % max(1, b.subWGXParts)
			yPart := (b.subWGIndex / max(1, b.subWGXParts)) % max(1, b.subWGYParts)
			subX := min(xToAllocate, max(1, b.subSizeX))
			subY := min(yToAllocate, max(1, b.subSizeY))
			wg.LocalXOffset = xPart * subX
			wg.LocalYOffset = yPart * subY
			wg.CurrSizeX = subX
			wg.CurrSizeY = subY
			wg.SubWGFactor = b.subWGFactor
			wg.SubWGIndex = b.subWGIndex
		}

		b.subWGIndex++
		if b.subWGIndex >= max(1, b.subWGFactor) {
			b.advanceOriginalWG()
		}

		b.spawnWorkItems(wg)
		b.formWavefronts(wg)
		return wg
	}
}

func (b *gridBuilderImpl) spawnWorkItems(wg *WorkGroup) {
	for z := 0; z < wg.CurrSizeZ; z++ {
		for y := 0; y < wg.CurrSizeY; y++ {
			for x := 0; x < wg.CurrSizeX; x++ {
				wi := new(WorkItem)
				wi.WG = wg
				wi.IDX = x + wg.LocalXOffset
				wi.IDY = y + wg.LocalYOffset
				wi.IDZ = z + wg.LocalZOffset
				wg.WorkItems = append(wg.WorkItems, wi)
			}
		}
	}
}

func (b *gridBuilderImpl) formWavefronts(wg *WorkGroup) {
	var wf *Wavefront
	wavefrontSize := 64
	for i, wi := range wg.WorkItems {
		if i%wavefrontSize == 0 {
			wf = NewWavefront()
			wf.FirstWiFlatID = wi.FlattenedID()
			wf.CodeObject = b.hsaco
			wf.Packet = b.packet
			wf.PacketAddress = b.packetAddr
			wf.WG = wg
			wg.Wavefronts = append(wg.Wavefronts, wf)
		}
		wf.WorkItems = append(wf.WorkItems, wi)
		laneID := len(wf.WorkItems) - 1
		wf.InitExecMask |= 1 << uint32(laneID)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
