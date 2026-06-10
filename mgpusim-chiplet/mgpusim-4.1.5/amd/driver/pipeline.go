package driver

import (
	"sort"
	"strconv"
	"strings"

	"github.com/sarchlab/mgpusim/v4/amd/insts"
	"github.com/sarchlab/mgpusim/v4/amd/kernels"
	"github.com/sarchlab/mgpusim/v4/amd/plan"
)

func (d *Driver) totalCUCountForDevice(deviceID int) int {
	if deviceID < 0 || deviceID >= len(d.devices) {
		return 0
	}

	dev := d.devices[deviceID]
	if dev == nil {
		return 0
	}

	if dev.Properties.CUCount > 0 {
		return dev.Properties.CUCount
	}

	total := 0
	for _, actual := range dev.ActualGPUs {
		if actual != nil {
			total += actual.Properties.CUCount
		}
	}
	return total
}

func normalizedManagedEstCUs(estCUs, totalCUs int) int {
	if totalCUs <= 0 {
		if estCUs > 0 {
			return estCUs
		}
		return 1
	}
	if estCUs <= 0 {
		return 1
	}
	if estCUs > totalCUs {
		return totalCUs
	}
	return estCUs
}

func normalizedManagedChipletCUs(chipletCUs []int, numChiplets int, totalCUs int) []int {
	if len(chipletCUs) == 0 || numChiplets <= 0 || totalCUs <= 0 {
		return nil
	}

	capPerChiplet := totalCUs / numChiplets
	if capPerChiplet <= 0 {
		capPerChiplet = 1
	}

	out := make([]int, numChiplets)
	hasAny := false
	for i := 0; i < numChiplets && i < len(chipletCUs); i++ {
		v := chipletCUs[i]
		if v < 0 {
			v = 0
		}
		if v > capPerChiplet {
			v = capPerChiplet
		}
		out[i] = v
		if v > 0 {
			hasAny = true
		}
	}
	if !hasAny {
		return nil
	}
	return out
}

func ceilDivInt(a, b int) int {
	if b <= 0 {
		return 0
	}
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

func scaledManagedEstCUs(fullEstCUs, totalWGs, chunkWGs, totalCUs int) int {
	full := normalizedManagedEstCUs(fullEstCUs, totalCUs)
	if chunkWGs <= 0 {
		return 0
	}
	if totalWGs <= 0 || chunkWGs >= totalWGs {
		return full
	}
	need := ceilDivInt(full*chunkWGs, totalWGs)
	if need <= 0 {
		need = 1
	}
	if totalCUs > 0 && need > totalCUs {
		need = totalCUs
	}
	return need
}

func scaledManagedChipletCUs(base []int, totalWGs, chunkWGs, totalCUs, numChiplets int) []int {
	base = normalizedManagedChipletCUs(base, numChiplets, totalCUs)
	if len(base) == 0 || chunkWGs <= 0 {
		return nil
	}
	if totalWGs <= 0 || chunkWGs >= totalWGs {
		return append([]int(nil), base...)
	}

	out := make([]int, len(base))
	hasAny := false
	for i, v := range base {
		if v <= 0 {
			continue
		}
		scaled := ceilDivInt(v*chunkWGs, totalWGs)
		if scaled <= 0 {
			scaled = 1
		}
		out[i] = scaled
		hasAny = true
	}
	if !hasAny {
		return nil
	}
	return out
}

func sumPositiveInts(vals []int) int {
	total := 0
	for _, v := range vals {
		if v > 0 {
			total += v
		}
	}
	return total
}

func packetNumWGXYZ(packet *kernels.HsaKernelDispatchPacket) (numWGX, numWGY, numWGZ int) {
	if packet == nil {
		return 1, 1, 1
	}
	gridX := int(packet.GridSizeX)
	gridY := int(packet.GridSizeY)
	gridZ := int(packet.GridSizeZ)
	if gridX <= 0 {
		gridX = 1
	}
	if gridY <= 0 {
		gridY = 1
	}
	if gridZ <= 0 {
		gridZ = 1
	}
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
	numWGX = ceilDivInt(gridX, wgX)
	numWGY = ceilDivInt(gridY, wgY)
	numWGZ = ceilDivInt(gridZ, wgZ)
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

func parseManagedTrailingPositiveInt(s string, def int) int {
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
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func inferManagedWGCountsByChiplet(packet *kernels.HsaKernelDispatchPacket, policy, partition string, numChiplets int) []int {
	if packet == nil || numChiplets <= 0 {
		return nil
	}
	numWGX, numWGY, numWGZ := packetNumWGXYZ(packet)
	counts := make([]int, numChiplets)
	part := strings.ToLower(strings.TrimSpace(partition))
	pol := strings.ToLower(strings.TrimSpace(policy))
	if part == "" && strings.Contains(pol, "lasp") {
		part = "xdiv"
	}
	for z := 0; z < numWGZ; z++ {
		for y := 0; y < numWGY; y++ {
			for x := 0; x < numWGX; x++ {
				chiplet := 0
				switch {
				case strings.HasPrefix(part, "xdiv"):
					divisor := ceilDivInt(numWGX, numChiplets)
					if divisor <= 0 {
						divisor = 1
					}
					chiplet = x / divisor
				case strings.HasPrefix(part, "ydiv"):
					divisor := numWGY / numChiplets
					if divisor <= 0 {
						divisor = 1
					}
					chiplet = y / divisor
				case strings.HasPrefix(part, "xblk"):
					blk := parseManagedTrailingPositiveInt(part, 1)
					chiplet = (x / blk) % numChiplets
				case strings.HasPrefix(part, "yblk"):
					blk := parseManagedTrailingPositiveInt(part, 1)
					chiplet = (y / blk) % numChiplets
				default:
					flat := x + y*numWGX + z*numWGX*numWGY
					chiplet = flat % numChiplets
				}
				if chiplet < 0 {
					chiplet = 0
				}
				if chiplet >= numChiplets {
					chiplet = numChiplets - 1
				}
				counts[chiplet]++
			}
		}
	}
	return counts
}

func inferManagedChipletCUsFromGeometry(packet *kernels.HsaKernelDispatchPacket, policy, partition string, estCUs, totalCUs, numChiplets int) []int {
	counts := inferManagedWGCountsByChiplet(packet, policy, partition, numChiplets)
	if len(counts) == 0 {
		return nil
	}
	active := make([]int, 0, len(counts))
	totalWG := 0
	for i, c := range counts {
		if c > 0 {
			active = append(active, i)
			totalWG += c
		}
	}
	if len(active) == 0 || totalWG <= 0 {
		return nil
	}
	need := estCUs
	if need < len(active) {
		need = len(active)
	}
	if totalCUs > 0 && need > totalCUs {
		need = totalCUs
	}
	capPerChiplet := 0
	if totalCUs > 0 && numChiplets > 0 {
		capPerChiplet = totalCUs / numChiplets
	}
	if capPerChiplet <= 0 {
		capPerChiplet = need
		if capPerChiplet <= 0 {
			capPerChiplet = 1
		}
	}
	out := make([]int, numChiplets)
	for _, chiplet := range active {
		out[chiplet] = 1
	}
	remaining := need - len(active)
	if remaining < 0 {
		remaining = 0
	}
	type fracNeed struct {
		chiplet int
		base    int
		frac    int
	}
	fracs := make([]fracNeed, 0, len(active))
	for _, chiplet := range active {
		prod := remaining * counts[chiplet]
		base := 0
		frac := 0
		if totalWG > 0 {
			base = prod / totalWG
			frac = prod % totalWG
		}
		room := capPerChiplet - out[chiplet]
		if room < 0 {
			room = 0
		}
		if base > room {
			base = room
			frac = 0
		}
		out[chiplet] += base
		fracs = append(fracs, fracNeed{chiplet: chiplet, base: base, frac: frac})
	}
	used := sumPositiveInts(out) - len(active)
	left := remaining - used
	if left < 0 {
		left = 0
	}
	sort.SliceStable(fracs, func(i, j int) bool {
		if fracs[i].frac != fracs[j].frac {
			return fracs[i].frac > fracs[j].frac
		}
		if counts[fracs[i].chiplet] != counts[fracs[j].chiplet] {
			return counts[fracs[i].chiplet] > counts[fracs[j].chiplet]
		}
		return fracs[i].chiplet < fracs[j].chiplet
	})
	for left > 0 {
		progress := false
		for _, item := range fracs {
			if out[item.chiplet] >= capPerChiplet {
				continue
			}
			out[item.chiplet]++
			left--
			progress = true
			if left == 0 {
				break
			}
		}
		if !progress {
			break
		}
	}
	if sumPositiveInts(out) <= 0 {
		return nil
	}
	return out
}

func chooseManagedAllowedCUsStriped(needTotal int, available [][]int) ([]int, bool) {
	if needTotal <= 0 {
		return nil, true
	}
	allowed := make([]int, 0, needTotal)
	nextIdx := make([]int, len(available))
	remaining := needTotal
	for remaining > 0 {
		progress := false
		for chiplet := 0; chiplet < len(available) && remaining > 0; chiplet++ {
			if nextIdx[chiplet] >= len(available[chiplet]) {
				continue
			}
			allowed = append(allowed, available[chiplet][nextIdx[chiplet]])
			nextIdx[chiplet]++
			remaining--
			progress = true
		}
		if !progress {
			return nil, false
		}
	}
	return allowed, true
}

func managedChipletCURange(totalCUs, numChiplets, chiplet int) (startCU, endCU int) {
	if totalCUs <= 0 {
		return 0, 0
	}
	if numChiplets <= 0 {
		return 0, totalCUs
	}
	cusPerChiplet := totalCUs / numChiplets
	if cusPerChiplet <= 0 {
		cusPerChiplet = totalCUs
	}
	startCU = chiplet * cusPerChiplet
	endCU = startCU + cusPerChiplet
	if chiplet == numChiplets-1 || endCU > totalCUs {
		endCU = totalCUs
	}
	if startCU < 0 {
		startCU = 0
	}
	if startCU > totalCUs {
		startCU = totalCUs
	}
	if endCU < startCU {
		endCU = startCU
	}
	return
}

func availableManagedCUsByChiplet(ctx *Context, gpuID int, totalCUs int, numChiplets int) [][]int {
	out := make([][]int, maxInt(numChiplets, 0))
	if totalCUs <= 0 || numChiplets <= 0 {
		return out
	}
	reserved := ctx.reservedManagedCUsSnapshot(gpuID)
	for chiplet := 0; chiplet < numChiplets; chiplet++ {
		startCU, endCU := managedChipletCURange(totalCUs, numChiplets, chiplet)
		for cuID := startCU; cuID < endCU; cuID++ {
			if !reserved[int(cuID)] {
				out[chiplet] = append(out[chiplet], int(cuID))
			}
		}
	}
	return out
}

func chooseManagedAllowedCUs(needByChiplet []int, needTotal int, available [][]int) ([]int, bool) {
	allowed := make([]int, 0)
	if len(needByChiplet) > 0 {
		for chiplet, need := range needByChiplet {
			if need <= 0 {
				continue
			}
			if chiplet >= len(available) || len(available[chiplet]) < int(need) {
				return nil, false
			}
			allowed = append(allowed, available[chiplet][:need]...)
		}
		if len(allowed) > 0 {
			return allowed, true
		}
	}
	if needTotal <= 0 {
		return nil, true
	}
	return chooseManagedAllowedCUsStriped(needTotal, available)
}

func partialManagedChunkFromAvailable(baseNeed []int, remainingWGs int, minChunkWG int, availByChiplet []int) (wgBudget int, need []int, ok bool) {
	if len(baseNeed) == 0 || remainingWGs <= 0 {
		return 0, nil, false
	}
	totalBase := sumPositiveInts(baseNeed)
	if totalBase <= 0 {
		return 0, nil, false
	}
	need = make([]int, len(baseNeed))
	usable := 0
	for i, base := range baseNeed {
		if base <= 0 {
			continue
		}
		avail := 0
		if i < len(availByChiplet) && availByChiplet[i] > 0 {
			avail = availByChiplet[i]
		}
		if avail <= 0 {
			continue
		}
		if avail < base {
			need[i] = avail
		} else {
			need[i] = base
		}
		usable += need[i]
	}
	if usable <= 0 {
		return 0, nil, false
	}
	rawWG := (remainingWGs * usable) / totalBase
	if rawWG <= 0 {
		rawWG = 1
	}
	wgBudget = rawWG
	if minChunkWG > 1 {
		wgBudget = (wgBudget / minChunkWG) * minChunkWG
		if wgBudget <= 0 {
			wgBudget = minChunkWG
		}
	}
	if wgBudget > remainingWGs {
		wgBudget = remainingWGs
	}
	if wgBudget <= 0 {
		return 0, nil, false
	}
	if wgBudget < rawWG {
		for i, v := range need {
			if v <= 0 {
				continue
			}
			scaled := ceilDivInt(v*wgBudget, rawWG)
			if scaled <= 0 {
				scaled = 1
			}
			if scaled > need[i] {
				scaled = need[i]
			}
			need[i] = scaled
		}
	}
	if sumPositiveInts(need) <= 0 {
		return 0, nil, false
	}
	return wgBudget, need, true
}

func chunkWGFromAvailable(totalWGs, fullEstCUs, availableCUs, minChunkWG int) int {
	if totalWGs <= 0 || availableCUs <= 0 || fullEstCUs <= 0 {
		return 0
	}
	wg := (totalWGs * availableCUs) / fullEstCUs
	if wg <= 0 {
		wg = 1
	}
	if minChunkWG > 1 {
		wg = (wg / minChunkWG) * minChunkWG
		if wg <= 0 {
			wg = minChunkWG
		}
	}
	if wg > totalWGs {
		wg = totalWGs
	}
	return wg
}

func workgroupsInPacketGrid(packetWG [3]int) int {
	total := 1
	for _, v := range packetWG {
		if v <= 0 {
			return 0
		}
		total *= v
	}
	return total
}

func (d *Driver) nextLaunchHints(
	ctx *Context,
	co *insts.HsaCo,
) (absID int, relID int, hints []plan.LaunchHint, ok bool) {
	sequenceLength := 0
	if d.plan != nil {
		sequenceLength = d.plan.LaunchSequenceLength
	}

	absID, relID = ctx.nextLaunchIDs(sequenceLength)

	if d.plan == nil || !d.plan.HasLaunchHints() {
		return absID, relID, nil, false
	}

	hints = d.plan.LaunchHintsByProgramLaunchID(relID)
	if len(hints) == 0 {
		return absID, relID, nil, false
	}

	kernelName := ""
	if co != nil && co.Symbol != nil {
		kernelName = co.Symbol.Name
	}
	filtered := make([]plan.LaunchHint, 0, len(hints))
	for _, h := range hints {
		if h.Kernel != "" && kernelName != "" && h.Kernel != kernelName {
			continue
		}
		filtered = append(filtered, h)
	}
	if len(filtered) == 0 {
		return absID, relID, nil, false
	}
	return absID, relID, filtered, true
}

func (d *Driver) managedPlanChipletCount() int {
	if d.plan == nil {
		return 0
	}
	return d.plan.NumChipletsOrDefault(0)
}

func buildManagedLaunchState(
	queue *CommandQueue,
	co *insts.HsaCo,
	packet *kernels.HsaKernelDispatchPacket,
	absID int,
	relID int,
	hint plan.LaunchHint,
	totalCUs int,
	numChiplets int,
	totalWGs int,
) *managedLaunchState {
	kernelName := ""
	if co != nil && co.Symbol != nil {
		kernelName = co.Symbol.Name
	}

	baseAbsID := absID - relID
	dependsOn := make([]int, 0, len(hint.DependsOn))
	for _, depRelID := range hint.DependsOn {
		if depRelID < 0 {
			continue
		}
		dependsOn = append(dependsOn, baseAbsID+depRelID)
	}

	programLaunchID := hint.ProgramLaunchID
	if programLaunchID < 0 {
		programLaunchID = relID
	}
	if programLaunchID == 0 && relID != 0 && hint.ID == relID {
		programLaunchID = relID
	}

	if len(hint.WGWindows) > 0 {
		totalWGs = 0
		for _, w := range hint.WGWindows {
			if w.Count > 0 {
				totalWGs += w.Count
			}
		}
	}
	if totalWGs <= 0 {
		totalWGs = 1
	}
	minChunkWG := hint.MinChunkWG
	if minChunkWG <= 0 {
		minChunkWG = 1
	}
	adaptive := hint.AdaptiveWGChunks || len(hint.WGWindows) > 0

	estCUs := normalizedManagedEstCUs(hint.EstCUs, totalCUs)
	chipletCUs := normalizedManagedChipletCUs(hint.ChipletCUs, numChiplets, totalCUs)
	if len(chipletCUs) == 0 {
		chipletCUs = inferManagedChipletCUsFromGeometry(packet, hint.Policy, hint.Partition, estCUs, totalCUs, numChiplets)
	}
	if inferredTotal := sumPositiveInts(chipletCUs); inferredTotal > estCUs {
		estCUs = inferredTotal
	}

	return &managedLaunchState{
		AbsID:             absID,
		RelID:             relID,
		ProgramLaunchID:   programLaunchID,
		Queue:             queue,
		KernelName:        kernelName,
		DependsOn:         dependsOn,
		Priority:          hint.Priority,
		EstCUs:            estCUs,
		ChipletCUs:        chipletCUs,
		EstDurationPs:     hint.EstDuration,
		GPUID:             queue.GPUID,
		Policy:            hint.Policy,
		Partition:         hint.Partition,
		AdaptiveWGChunks:  adaptive,
		MinChunkWG:        minChunkWG,
		TotalWGs:          totalWGs,
		RemainingWGs:      totalWGs,
		ExplicitWGWindows: append([]plan.WGWindow(nil), hint.WGWindows...),
		SubWGFactor:       int(hint.SubWGFactor),
	}
}

func (d *Driver) managedLaunchAtQueueHead(
	q *CommandQueue,
) (*LaunchKernelCommand, *managedLaunchState, bool) {
	if q == nil || q.IsRunning {
		return nil, nil, false
	}

	cmd, ok := q.Peek().(*LaunchKernelCommand)
	if !ok || cmd == nil || !cmd.Managed {
		return nil, nil, false
	}

	st := q.Context.managedLaunchState(cmd.LaunchAbsID)
	if st == nil {
		return nil, nil, false
	}

	return cmd, st, true
}

func (d *Driver) managedLaunchPredsReady(ctx *Context, st *managedLaunchState) bool {
	if ctx == nil || st == nil || st.Completed || st.Active || st.RemainingWGs <= 0 {
		return false
	}
	for _, depAbsID := range st.DependsOn {
		if !ctx.isManagedLaunchCompleted(depAbsID) {
			return false
		}
	}
	return true
}

func (d *Driver) fitsChipletBudget(st *managedLaunchState, need []int, totalCUs int, numChiplets int, ctx *Context) bool {
	if len(need) == 0 || numChiplets <= 0 || totalCUs <= 0 {
		return true
	}
	capPerChiplet := totalCUs / numChiplets
	if capPerChiplet <= 0 {
		capPerChiplet = 1
	}
	for chiplet, v := range need {
		if v <= 0 {
			continue
		}
		active := ctx.activeManagedCUsForGPUChiplet(st.GPUID, chiplet)
		if active+v > capPerChiplet {
			return false
		}
	}
	return true
}

func (d *Driver) chooseManagedLaunchChunk(
	ctx *Context,
	st *managedLaunchState,
) (wgCount int, estCUs int, chipletCUs []int, allowedCUs []int, ok bool) {
	if ctx == nil || st == nil || st.Completed || st.Active || st.RemainingWGs <= 0 {
		return 0, 0, nil, nil, false
	}
	if !d.managedLaunchPredsReady(ctx, st) {
		return 0, 0, nil, nil, false
	}

	totalCUs := d.totalCUCountForDevice(st.GPUID)
	if totalCUs <= 0 {
		totalCUs = st.EstCUs
		if totalCUs <= 0 {
			totalCUs = 1
		}
	}
	activeTotal := ctx.activeManagedCUsForGPU(st.GPUID)
	availTotal := totalCUs - activeTotal
	if availTotal <= 0 {
		return 0, 0, nil, nil, false
	}

	numChiplets := d.managedPlanChipletCount()
	availableByChiplet := availableManagedCUsByChiplet(ctx, st.GPUID, totalCUs, numChiplets)
	remainingWGCount := st.RemainingWGs
	remainingNeedCUs := scaledManagedEstCUs(st.EstCUs, st.TotalWGs, remainingWGCount, totalCUs)
	remainingNeedChiplets := scaledManagedChipletCUs(st.ChipletCUs, st.TotalWGs, remainingWGCount, totalCUs, numChiplets)
	if remainingNeedCUs <= availTotal && d.fitsChipletBudget(st, remainingNeedChiplets, totalCUs, numChiplets, ctx) {
		allocCUs := remainingNeedCUs
		if len(remainingNeedChiplets) > 0 {
			allocCUs = sumPositiveInts(remainingNeedChiplets)
		}
		allowed, ok := chooseManagedAllowedCUs(remainingNeedChiplets, allocCUs, availableByChiplet)
		if ok {
			return remainingWGCount, allocCUs, remainingNeedChiplets, allowed, true
		}
	}

	if !st.AdaptiveWGChunks {
		return 0, 0, nil, nil, false
	}

	minChunkWG := st.MinChunkWG
	if minChunkWG <= 0 {
		minChunkWG = 1
	}

	// Runtime-managed partial-chiplet chunking:
	// If some chiplets are currently saturated but others are idle, issue a
	// smaller chunk that can occupy only the currently free chiplets. This is
	// what allows a branch kernel such as v_proj to run during scores/softmax
	// without the planner hard-coding chiplet subsets.
	if len(remainingNeedChiplets) > 0 && numChiplets > 0 {
		capPerChiplet := totalCUs / numChiplets
		if capPerChiplet <= 0 {
			capPerChiplet = 1
		}
		availByChiplet := make([]int, numChiplets)
		for chiplet := 0; chiplet < numChiplets; chiplet++ {
			availByChiplet[chiplet] = capPerChiplet - ctx.activeManagedCUsForGPUChiplet(st.GPUID, chiplet)
			if availByChiplet[chiplet] < 0 {
				availByChiplet[chiplet] = 0
			}
		}
		wgBudget, needChiplets, ok := partialManagedChunkFromAvailable(
			remainingNeedChiplets,
			remainingWGCount,
			minChunkWG,
			availByChiplet,
		)
		if ok {
			needCUs := sumPositiveInts(needChiplets)
			if needCUs > 0 && needCUs <= availTotal && d.fitsChipletBudget(st, needChiplets, totalCUs, numChiplets, ctx) {
				allowed, ok2 := chooseManagedAllowedCUs(needChiplets, needCUs, availableByChiplet)
				if ok2 {
					return wgBudget, needCUs, needChiplets, allowed, true
				}
			}
		}
	}

	// Fallback: total-CU-only chunking when we do not have a meaningful per-chiplet model.
	wgBudget := chunkWGFromAvailable(st.RemainingWGs, st.EstCUs, availTotal, minChunkWG)
	if wgBudget <= 0 {
		return 0, 0, nil, nil, false
	}
	if wgBudget > st.RemainingWGs {
		wgBudget = st.RemainingWGs
	}

	for wgBudget > 0 {
		if minChunkWG > 1 && wgBudget > minChunkWG {
			wgBudget = (wgBudget / minChunkWG) * minChunkWG
		}
		if wgBudget <= 0 {
			break
		}
		needCUs := scaledManagedEstCUs(st.EstCUs, st.TotalWGs, wgBudget, totalCUs)
		needChiplets := scaledManagedChipletCUs(st.ChipletCUs, st.TotalWGs, wgBudget, totalCUs, numChiplets)
		if needCUs <= availTotal && d.fitsChipletBudget(st, needChiplets, totalCUs, numChiplets, ctx) {
			allocCUs := needCUs
			if len(needChiplets) > 0 {
				allocCUs = sumPositiveInts(needChiplets)
			}
			allowed, ok2 := chooseManagedAllowedCUs(needChiplets, allocCUs, availableByChiplet)
			if ok2 {
				return wgBudget, allocCUs, needChiplets, allowed, true
			}
		}
		if wgBudget == 1 {
			break
		}
		wgBudget--
	}

	return 0, 0, nil, nil, false
}

func (d *Driver) managedLaunchReady(
	ctx *Context,
	st *managedLaunchState,
) bool {
	_, _, _, _, ok := d.chooseManagedLaunchChunk(ctx, st)
	return ok
}

func (d *Driver) readyManagedLaunchQueues(
	ctx *Context,
	queues []*CommandQueue,
) []*CommandQueue {
	ready := make([]*CommandQueue, 0, len(queues))
	for _, q := range queues {
		_, st, ok := d.managedLaunchAtQueueHead(q)
		if !ok {
			continue
		}
		if d.managedLaunchReady(ctx, st) {
			ready = append(ready, q)
		}
	}

	sort.SliceStable(ready, func(i, j int) bool {
		_, stI, _ := d.managedLaunchAtQueueHead(ready[i])
		_, stJ, _ := d.managedLaunchAtQueueHead(ready[j])
		if stI == nil || stJ == nil {
			return ready[i] == nil
		}
		if stI.Priority != stJ.Priority {
			return stI.Priority > stJ.Priority
		}
		return stI.AbsID < stJ.AbsID
	})

	return ready
}

func (d *Driver) drainManagedLaunches(ctx *Context) {
	if ctx == nil {
		return
	}

	sequenceLength := 0
	if d.plan != nil {
		sequenceLength = d.plan.LaunchSequenceLength
	}

	for {
		d.ensureManagedCompletionFlush(ctx)
		pending := ctx.managedLaunchesPending()
		if len(pending) == 0 && !ctx.hasManagedCompletionBarrier() {
			ctx.garbageCollectCompletedLaunches(sequenceLength)
			return
		}
		if q := ctx.getManagedCompletionFlushQueue(); q != nil {
			d.DrainCommandQueue(q)
		}

		for _, st := range pending {
			if st == nil || st.Queue == nil {
				continue
			}
			d.DrainCommandQueue(st.Queue)
		}

		ctx.garbageCollectCompletedLaunches(sequenceLength)
	}
}
