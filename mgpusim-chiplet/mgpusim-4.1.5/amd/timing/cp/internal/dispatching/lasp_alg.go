package dispatching

import (
	"sort"
	"strconv"
	"strings"

	"github.com/sarchlab/mgpusim/v4/amd/kernels"
	"github.com/sarchlab/mgpusim/v4/amd/protocol"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cp/internal/resource"
)

// chipletLASPAlgorithm implements a lightweight LASP-style work-group
// partitioning and dispatching policy for chiplet-based GPUs.
//
// It first assigns each WG to a chiplet based on its (IDX, IDY) coordinates
// and the configured partition string (e.g., Xdiv, Ydiv, Xblk4, Yblk4). It
// then dispatches WGs to CUs within their assigned chiplet.
//
// This is adapted from the MGVM (MICRO'22) artifact's implementation but is
// rewritten to match the v4 kernels.GridBuilder interface and to support an
// arbitrary number of chiplets.
type chipletLASPAlgorithm struct {
	gridBuilder kernels.GridBuilder
	cuPool      resource.CUResourcePool

	numChiplets int
	partition   string

	perChipletWGs [][]*kernels.WorkGroup
	nextWGIndex   []int
	nextChiplet   int

	// Next CU offset per chiplet (within that chiplet's CU range).
	nextCUOffset []int

	// Intra-chiplet CU scheduling policy for LASP.
	//   - "rr" (default): round-robin across CUs in the chiplet
	//   - "block": assign contiguous WGs to the same CU as much as possible
	cuPolicy string

	allowedCUByChiplet [][]int

	numWG           int
	numDispatchedWG int
}

func (a *chipletLASPAlgorithm) RegisterCU(cu resource.DispatchableCU) {
	a.cuPool.RegisterCU(cu)
}

func laspChipletCURange(totalCU, numChiplets, chiplet int) (startCU, endCU int) {
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

func laspChipletOfCU(cuID, totalCU, numChiplets int) int {
	for chiplet := 0; chiplet < numChiplets; chiplet++ {
		startCU, endCU := laspChipletCURange(totalCU, numChiplets, chiplet)
		if cuID >= startCU && cuID < endCU {
			return chiplet
		}
	}
	if numChiplets <= 0 {
		return 0
	}
	return numChiplets - 1
}

func buildLASPAllowedCULists(allowed []int, totalCU, numChiplets int) [][]int {
	out := make([][]int, max(1, numChiplets))
	if totalCU <= 0 || numChiplets <= 0 {
		return out
	}
	if len(allowed) == 0 {
		for chiplet := 0; chiplet < numChiplets; chiplet++ {
			startCU, endCU := laspChipletCURange(totalCU, numChiplets, chiplet)
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
		chiplet := laspChipletOfCU(cuID, totalCU, numChiplets)
		out[chiplet] = append(out[chiplet], cuID)
	}
	for chiplet := range out {
		sort.Ints(out[chiplet])
	}
	return out
}

func fullLASPChipletCUList(totalCU, numChiplets, chiplet int) []int {
	if totalCU <= 0 || numChiplets <= 0 || chiplet < 0 || chiplet >= numChiplets {
		return nil
	}
	startCU, endCU := laspChipletCURange(totalCU, numChiplets, chiplet)
	out := make([]int, 0, max(0, endCU-startCU))
	for cuID := startCU; cuID < endCU; cuID++ {
		out = append(out, cuID)
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a *chipletLASPAlgorithm) StartNewKernel(info kernels.KernelLaunchInfo) {
	if a.numChiplets <= 0 {
		a.numChiplets = 1
	}
	if strings.TrimSpace(a.partition) == "" {
		a.partition = "Xdiv"
	}
	if strings.TrimSpace(a.cuPolicy) == "" {
		a.cuPolicy = "rr"
	}

	a.gridBuilder.SetKernel(info)

	a.numWG = a.gridBuilder.NumWG()
	a.numDispatchedWG = 0
	a.nextChiplet = 0

	a.perChipletWGs = make([][]*kernels.WorkGroup, a.numChiplets)
	a.nextWGIndex = make([]int, a.numChiplets)
	a.nextCUOffset = make([]int, a.numChiplets)
	a.allowedCUByChiplet = buildLASPAllowedCULists(info.AllowedCUIDs, a.cuPool.NumCU(), a.numChiplets)

	numWGX, numWGY := computeNumWGXY(info)
	for i := 0; i < a.numWG; i++ {
		wg := a.gridBuilder.NextWG()
		if wg == nil {
			break
		}

		chiplet := a.assignWGToChiplet(wg, numWGX, numWGY)
		if chiplet < 0 {
			chiplet = 0
		}
		if chiplet >= a.numChiplets {
			chiplet = chiplet % a.numChiplets
		}
		a.perChipletWGs[chiplet] = append(a.perChipletWGs[chiplet], wg)
	}
}

func (a *chipletLASPAlgorithm) NumWG() int {
	return a.numWG
}

func (a *chipletLASPAlgorithm) HasNext() bool {
	return a.numDispatchedWG < a.numWG
}

func (a *chipletLASPAlgorithm) Next() dispatchLocation {
	// Try each chiplet once per call, starting from nextChiplet.
	for t := 0; t < a.numChiplets; t++ {
		chiplet := (a.nextChiplet + t) % a.numChiplets
		if a.nextWGIndex[chiplet] >= len(a.perChipletWGs[chiplet]) {
			continue
		}

		wg := a.perChipletWGs[chiplet][a.nextWGIndex[chiplet]]
		loc, ok := a.tryDispatchWGOnChiplet(wg, chiplet)
		if ok {
			a.nextWGIndex[chiplet]++
			a.numDispatchedWG++
			a.nextChiplet = (chiplet + 1) % a.numChiplets
			return loc
		}
	}

	// No WG can be dispatched due to resource constraints.
	return dispatchLocation{}
}

func (a *chipletLASPAlgorithm) tryDispatchWGOnChiplet(
	wg *kernels.WorkGroup,
	chiplet int,
) (dispatchLocation, bool) {
	totalCU := a.cuPool.NumCU()
	if totalCU == 0 {
		return dispatchLocation{}, false
	}

	allowedCUList := []int(nil)
	if chiplet < len(a.allowedCUByChiplet) {
		allowedCUList = append([]int(nil), a.allowedCUByChiplet[chiplet]...)
	}

	tryList := func(cuList []int) (dispatchLocation, bool) {
		if len(cuList) == 0 {
			return dispatchLocation{}, false
		}
		numCUInChiplet := len(cuList)
		cuPolicy := strings.ToLower(strings.TrimSpace(a.cuPolicy))
		startOffset := 0
		if cuPolicy == "block" {
			localWGIdx := a.nextWGIndex[chiplet]
			wgTotalInChiplet := len(a.perChipletWGs[chiplet])
			blk := ceilDiv(wgTotalInChiplet, numCUInChiplet)
			if blk <= 0 {
				blk = 1
			}
			startOffset = localWGIdx / blk
			if startOffset >= numCUInChiplet {
				startOffset = numCUInChiplet - 1
			}
		} else {
			startOffset = a.nextCUOffset[chiplet] % numCUInChiplet
		}

		for i := 0; i < numCUInChiplet; i++ {
			localID := (startOffset + i) % numCUInChiplet
			cuID := cuList[localID]
			cu := a.cuPool.GetCU(cuID)
			locations, ok := cu.ReserveResourceForWG(wg)
			if !ok {
				continue
			}
			if cuPolicy != "block" {
				a.nextCUOffset[chiplet] = (localID + 1) % numCUInChiplet
			}
			dispatch := dispatchLocation{
				valid: true,
				cu:    cu.DispatchingPort(),
				cuID:  cuID,
				wg:    wg,
			}
			dispatch.locations = make([]protocol.WfDispatchLocation, len(locations))
			for i, loc := range locations {
				dispatch.locations[i] = protocol.WfDispatchLocation(loc)
			}
			return dispatch, true
		}
		return dispatchLocation{}, false
	}

	if loc, ok := tryList(allowedCUList); ok {
		return loc, true
	}

	// Robustness fallback:
	// If the driver-selected concrete CU subset cannot make progress (for example,
	// because a previously chosen CU is still transiently busy), fall back to any
	// CU within the same chiplet. This preserves the chiplet-level partition while
	// preventing the kernel from deadlocking on an overly strict exact-CU binding.
	fullChipletList := fullLASPChipletCUList(totalCU, a.numChiplets, chiplet)
	if len(fullChipletList) == 0 {
		return dispatchLocation{}, false
	}
	if len(allowedCUList) == len(fullChipletList) {
		return dispatchLocation{}, false
	}
	return tryList(fullChipletList)
}

func (a *chipletLASPAlgorithm) FreeResources(location dispatchLocation) {
	a.cuPool.GetCU(location.cuID).FreeResourcesForWG(location.wg)
}

func computeNumWGXY(info kernels.KernelLaunchInfo) (numWGX, numWGY int) {
	// Defensive defaults.
	if info.Packet == nil {
		return 1, 1
	}

	wgSizeX := int(info.Packet.WorkgroupSizeX)
	wgSizeY := int(info.Packet.WorkgroupSizeY)
	if wgSizeX <= 0 {
		wgSizeX = 1
	}
	if wgSizeY <= 0 {
		wgSizeY = 1
	}

	gridX := int(info.Packet.GridSizeX)
	gridY := int(info.Packet.GridSizeY)
	if gridX <= 0 {
		gridX = 1
	}
	if gridY <= 0 {
		gridY = 1
	}

	numWGX = (gridX + wgSizeX - 1) / wgSizeX
	numWGY = (gridY + wgSizeY - 1) / wgSizeY
	if numWGX <= 0 {
		numWGX = 1
	}
	if numWGY <= 0 {
		numWGY = 1
	}
	return
}

func (a *chipletLASPAlgorithm) assignWGToChiplet(
	wg *kernels.WorkGroup,
	numWGX, numWGY int,
) int {
	part := strings.ToLower(strings.TrimSpace(a.partition))
	if part == "" {
		part = "xdiv"
	}

	// Divide the grid into numChiplets contiguous ranges.
	if strings.HasPrefix(part, "xdiv") {
		divisor := ceilDiv(numWGX, a.numChiplets)
		if divisor <= 0 {
			divisor = 1
		}
		chiplet := wg.IDX / divisor
		if chiplet >= a.numChiplets {
			chiplet = a.numChiplets - 1
		}
		return chiplet
	}
	if strings.HasPrefix(part, "ydiv") {
		// Align with MGVM's implementation: Ydiv uses floor(numWGY/numChiplets).
		// Most of the MGVM workloads have numWGY divisible by numChiplets, so this
		// matches their behavior. We still guard against divisor==0 to avoid a
		// crash for tiny grids.
		divisor := numWGY / a.numChiplets
		if divisor <= 0 {
			divisor = 1
		}
		chiplet := wg.IDY / divisor
		if chiplet >= a.numChiplets {
			chiplet = a.numChiplets - 1
		}
		return chiplet
	}

	// Block-cyclic scheduling: assign blocks of size N in X/Y cyclically.
	if strings.HasPrefix(part, "xblk") {
		blk := parseTrailingPositiveInt(part, 1)
		return (wg.IDX / blk) % a.numChiplets
	}
	if strings.HasPrefix(part, "yblk") {
		blk := parseTrailingPositiveInt(part, 1)
		return (wg.IDY / blk) % a.numChiplets
	}

	// Fallback: simple modulo on flattened WG ID.
	flat := wg.IDX + wg.IDY*numWGX + wg.IDZ*numWGX*numWGY
	return flat % a.numChiplets
}

func ceilDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

func parseTrailingPositiveInt(s string, def int) int {
	// Extract trailing digits.
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] < '0' || s[i] > '9' {
			if i == len(s)-1 {
				return def
			}
			v, err := strconv.Atoi(s[i+1:])
			if err != nil || v <= 0 {
				return def
			}
			return v
		}
	}
	// All digits.
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return def
	}
	return v
}

// Ensure chipletLASPAlgorithm implements the algorithm interface.
var _ algorithm = (*chipletLASPAlgorithm)(nil)

// Make sure we reference protocol so the import is not optimized away in case
// this file is copied into environments that use build tags.
var _ = protocol.WfDispatchLocation{}
