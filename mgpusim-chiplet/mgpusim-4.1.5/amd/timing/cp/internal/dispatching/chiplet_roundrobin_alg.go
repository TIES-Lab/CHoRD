package dispatching

import (
	"sort"

	"github.com/sarchlab/mgpusim/v4/amd/kernels"
	"github.com/sarchlab/mgpusim/v4/amd/protocol"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cp/internal/resource"
)

// chipletRoundRobinAlgorithm dispatches work-groups in a chiplet-aware
// round-robin fashion.
//
// The baseline goal is: work-groups are striped across chiplets (WGIndex %
// NumChiplets), and within each chiplet, CUs are used in round-robin order.
//
// This avoids a common pitfall in a global CU-level round-robin scheduler: if
// CUs are registered in chiplet-major order, the scheduler may keep filling
// chiplet-0 before touching other chiplets, making "#chiplets" scaling look
// broken.
type chipletRoundRobinAlgorithm struct {
	gridBuilder kernels.GridBuilder
	cuPool      resource.CUResourcePool

	numChiplets int

	// Per-chiplet state.
	perChipletWGs    [][]*kernels.WorkGroup
	nextCUOffset     []int
	nextChiplet      int
	allowedByChiplet [][]int

	// Kernel bookkeeping.
	totalWG        int
	numDispatchedW int
}

func (a *chipletRoundRobinAlgorithm) RegisterCU(cu resource.DispatchableCU) {
	a.cuPool.RegisterCU(cu)
}

func rrChipletCURange(totalCU, numChiplets, chiplet int) (startCU, endCU int) {
	if totalCU <= 0 {
		return 0, 0
	}
	if numChiplets <= 0 {
		return 0, totalCU
	}
	cusPerChiplet := totalCU / numChiplets
	if cusPerChiplet <= 0 {
		cusPerChiplet = totalCU
	}
	startCU = chiplet * cusPerChiplet
	endCU = startCU + cusPerChiplet
	if chiplet == numChiplets-1 || endCU > totalCU {
		endCU = totalCU
	}
	if startCU < 0 {
		startCU = 0
	}
	if startCU > totalCU {
		startCU = totalCU
	}
	if endCU < startCU {
		endCU = startCU
	}
	return
}

func rrChipletOfCU(cuID, totalCU, numChiplets int) int {
	for chiplet := 0; chiplet < numChiplets; chiplet++ {
		startCU, endCU := rrChipletCURange(totalCU, numChiplets, chiplet)
		if cuID >= startCU && cuID < endCU {
			return chiplet
		}
	}
	if numChiplets <= 0 {
		return 0
	}
	return numChiplets - 1
}

func buildRRAllowedCULists(allowed []int, totalCU, numChiplets int) [][]int {
	out := make([][]int, maxRR(1, numChiplets))
	if totalCU <= 0 || numChiplets <= 0 {
		return out
	}
	if len(allowed) == 0 {
		for chiplet := 0; chiplet < numChiplets; chiplet++ {
			startCU, endCU := rrChipletCURange(totalCU, numChiplets, chiplet)
			for cuID := startCU; cuID < endCU; cuID++ {
				out[chiplet] = append(out[chiplet], cuID)
			}
		}
		return out
	}
	seen := make(map[int]bool)
	for _, cuID := range allowed {
		if cuID < 0 || cuID >= totalCU || seen[cuID] {
			continue
		}
		seen[cuID] = true
		chiplet := rrChipletOfCU(cuID, totalCU, numChiplets)
		out[chiplet] = append(out[chiplet], cuID)
	}
	for chiplet := range out {
		sort.Ints(out[chiplet])
	}
	return out
}

func maxRR(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fullRRChipletCUList(totalCU, numChiplets, chiplet int) []int {
	if totalCU <= 0 || numChiplets <= 0 || chiplet < 0 || chiplet >= numChiplets {
		return nil
	}
	startCU, endCU := rrChipletCURange(totalCU, numChiplets, chiplet)
	out := make([]int, 0, maxRR(0, endCU-startCU))
	for cuID := startCU; cuID < endCU; cuID++ {
		out = append(out, cuID)
	}
	return out
}

func (a *chipletRoundRobinAlgorithm) StartNewKernel(info kernels.KernelLaunchInfo) {
	a.gridBuilder.SetKernel(info)
	a.totalWG = a.gridBuilder.NumWG()
	a.numDispatchedW = 0

	if a.numChiplets <= 0 {
		a.numChiplets = 1
	}

	a.perChipletWGs = make([][]*kernels.WorkGroup, a.numChiplets)
	a.nextCUOffset = make([]int, a.numChiplets)
	a.allowedByChiplet = buildRRAllowedCULists(info.AllowedCUIDs, a.cuPool.NumCU(), a.numChiplets)
	for c := 0; c < a.numChiplets; c++ {
		a.perChipletWGs[c] = make([]*kernels.WorkGroup, 0)
		a.nextCUOffset[c] = 0
	}
	a.nextChiplet = 0

	numWGX, numWGY, _ := computeNumWGXYZ(info)

	// Pre-assign work-groups to chiplets by a deterministic striping scheme.
	for wg := a.gridBuilder.NextWG(); wg != nil; wg = a.gridBuilder.NextWG() {
		flat := wg.IDX + wg.IDY*numWGX + wg.IDZ*numWGX*numWGY
		chiplet := flat % a.numChiplets
		if chiplet < 0 {
			chiplet = 0
		}
		a.perChipletWGs[chiplet] = append(a.perChipletWGs[chiplet], wg)
	}
}

func (a *chipletRoundRobinAlgorithm) NumWG() int {
	return a.totalWG
}

func (a *chipletRoundRobinAlgorithm) HasNext() bool {
	return a.numDispatchedW < a.totalWG
}

func (a *chipletRoundRobinAlgorithm) Next() (location dispatchLocation) {
	if !a.HasNext() {
		return dispatchLocation{}
	}

	if a.numChiplets <= 0 {
		a.numChiplets = 1
	}

	totalCU := a.cuPool.NumCU()
	if totalCU == 0 {
		return dispatchLocation{}
	}

	cuPerChiplet := totalCU / a.numChiplets
	if cuPerChiplet <= 0 {
		// Degenerate config: fewer CUs than chiplets.
		cuPerChiplet = 1
	}

	startChiplet := a.nextChiplet % a.numChiplets
	for i := 0; i < a.numChiplets; i++ {
		chiplet := (startChiplet + i) % a.numChiplets
		if len(a.perChipletWGs[chiplet]) == 0 {
			continue
		}

		wg := a.perChipletWGs[chiplet][0]
		loc, ok := a.tryDispatchWGOnChiplet(wg, chiplet, cuPerChiplet)
		if !ok {
			continue
		}

		// Commit dispatch.
		a.perChipletWGs[chiplet] = a.perChipletWGs[chiplet][1:]
		a.numDispatchedW++
		a.nextChiplet = (chiplet + 1) % a.numChiplets
		return loc
	}

	return dispatchLocation{}
}

func (a *chipletRoundRobinAlgorithm) tryDispatchWGOnChiplet(
	wg *kernels.WorkGroup,
	chiplet int,
	cuPerChiplet int,
) (dispatchLocation, bool) {
	_ = cuPerChiplet // kept to minimize diff with the original call sites

	allowedList := []int(nil)
	if chiplet < len(a.allowedByChiplet) {
		allowedList = append([]int(nil), a.allowedByChiplet[chiplet]...)
	}

	tryList := func(cuList []int) (dispatchLocation, bool) {
		if len(cuList) == 0 {
			return dispatchLocation{}, false
		}

		nLocal := len(cuList)
		startOffset := a.nextCUOffset[chiplet] % nLocal
		for i := 0; i < nLocal; i++ {
			localID := (startOffset + i) % nLocal
			cuID := cuList[localID]
			cu := a.cuPool.GetCU(cuID)

			locations, ok := cu.ReserveResourceForWG(wg)
			if !ok {
				continue
			}

			a.nextCUOffset[chiplet] = (localID + 1) % nLocal

			dispatch := dispatchLocation{
				valid: true,
				cu:    cu.DispatchingPort(),
				cuID:  cuID,
				wg:    wg,
			}
			dispatch.locations = make([]protocol.WfDispatchLocation, len(locations))
			for i, l := range locations {
				dispatch.locations[i] = protocol.WfDispatchLocation(l)
			}
			return dispatch, true
		}

		return dispatchLocation{}, false
	}

	if loc, ok := tryList(allowedList); ok {
		return loc, true
	}

	// Deadlock-avoidance fallback:
	// If the exact CU subset cannot make forward progress, widen only within the
	// same chiplet. This preserves chiplet-level placement while avoiding kernels
	// getting stuck on an overly strict exact-CU binding.
	fullChipletList := fullRRChipletCUList(a.cuPool.NumCU(), a.numChiplets, chiplet)
	if len(fullChipletList) == 0 || len(fullChipletList) == len(allowedList) {
		return dispatchLocation{}, false
	}
	return tryList(fullChipletList)
}

func (a *chipletRoundRobinAlgorithm) FreeResources(location dispatchLocation) {
	a.cuPool.GetCU(location.cuID).FreeResourcesForWG(location.wg)
}

// computeNumWGXYZ returns the number of work-groups in each dimension.
//
// This mirrors kernels.GridBuilder's internal logic but is exposed here to
// avoid relying on unexported methods.
func computeNumWGXYZ(info kernels.KernelLaunchInfo) (numWGX, numWGY, numWGZ int) {
	packet := info.Packet

	gridX := int(packet.GridSizeX)
	gridY := int(packet.GridSizeY)
	gridZ := int(packet.GridSizeZ)

	wgX := int(packet.WorkgroupSizeX)
	wgY := int(packet.WorkgroupSizeY)
	wgZ := int(packet.WorkgroupSizeZ)

	if wgX <= 0 {
		wgX = 1
	}
	if wgY <= 0 {
		wgY = 1
	}
	if wgZ <= 0 {
		wgZ = 1
	}

	numWGX = (gridX + wgX - 1) / wgX
	numWGY = (gridY + wgY - 1) / wgY
	numWGZ = (gridZ + wgZ - 1) / wgZ

	if numWGX <= 0 {
		numWGX = 1
	}
	if numWGY <= 0 {
		numWGY = 1
	}
	if numWGZ <= 0 {
		numWGZ = 1
	}

	return
}

var _ algorithm = (*chipletRoundRobinAlgorithm)(nil)
