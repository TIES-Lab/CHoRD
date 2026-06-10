package runner

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"sync"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/mgpusim/v4/amd/driver"
)

type kernelStatsTracer struct {
	sim.TimeTeller

	l2Counter     *memAccessCounter
	dramCounter   *dramAccessCounter
	cacheCounters *cacheEventGroups
	rdmaCounter   *rdmaTrafficCounter
	ptwCounter    *ptwStatsCounter
	tlbCounters   *tlbEventGroups

	mu       sync.Mutex
	inflight map[string]inflightKernel
	records  []kernelMetricRecord
	nextIdx  int
}

type inflightKernel struct {
	index     int
	name      string
	cmdID     string
	startTime sim.VTimeInSec

	startPTW ptwStatsSnapshot

	// Launch configuration (copied from the LaunchKernelCommand).
	gridSize         [3]uint32
	wgSize           [3]uint16
	numWG            [3]uint32
	numWGTotal       uint64
	effectiveWGTotal uint64
	managed          bool
	launchAbsID      int
	launchRelID      int
	programLaunchID  int
	subWGFactor      int
	wgWindowStart    int
	wgWindowCount    int
	activeCUHint     int

	startL2Snap    memAccessSnapshot
	startDramSnap  dramAccessSnapshot
	startCacheSnap cacheEventGroupsSnapshot
	startRdmaSnap  rdmaTrafficSnapshot
	startTLBSnap   tlbEventGroupsSnapshot
}

type kernelMetricRecord struct {
	Index   int
	Name    string
	CmdID   string
	Start   sim.VTimeInSec
	End     sim.VTimeInSec
	Latency sim.VTimeInSec

	// Launch configuration
	GridSize         [3]uint32
	WGSize           [3]uint16
	NumWG            [3]uint32
	NumWGTotal       uint64
	EffectiveWGTotal uint64
	Managed          bool
	LaunchAbsID      int
	LaunchRelID      int
	ProgramLaunchID  int
	SubWGFactor      int
	WGWindowStart    int
	WGWindowCount    int
	ActiveCUHint     int

	rawStartL2Snap   memAccessSnapshot
	rawEndL2Snap     memAccessSnapshot
	rawStartDramSnap dramAccessSnapshot
	rawEndDramSnap   dramAccessSnapshot
	rawStartPTW      ptwStatsSnapshot
	rawEndPTW        ptwStatsSnapshot
	rawStartTLBSnap  tlbEventGroupsSnapshot
	rawEndTLBSnap    tlbEventGroupsSnapshot

	LocalRead     uint64
	LocalWrite    uint64
	RemoteRead    uint64
	RemoteWrite   uint64
	ExternalRead  uint64
	ExternalWrite uint64

	LocalBytes    uint64
	RemoteBytes   uint64
	ExternalBytes uint64

	DRAMReadReq    uint64
	DRAMWriteReq   uint64
	DRAMReadBytes  uint64
	DRAMWriteBytes uint64

	// RDMA / NoP / NoC traffic (delta during this kernel)
	NoPSendPackets uint64
	NoPSendBytes   uint64
	NoPRecvPackets uint64
	NoPRecvBytes   uint64

	NoCSendPackets uint64
	NoCSendBytes   uint64
	NoCRecvPackets uint64
	NoCRecvBytes   uint64

	RDMAReqSendPackets uint64
	RDMAReqSendBytes   uint64

	// PTW / PWC statistics (delta during this kernel)
	PTWTranslationReqs  uint64
	PTWWalkCompleted    uint64
	PTWWalkFailed       uint64
	PTWPWCLookups       uint64
	PTWPWCHits          uint64
	PTWPWCMisses        uint64
	PTWPWCHitLevel0     uint64
	PTWPWCHitLevel1     uint64
	PTWPWCHitLevel2     uint64
	PTWPTEReads         uint64
	PTWPTEReadsLocal    uint64
	PTWPTEReadsRemote   uint64
	PTWWalkLatencySumPS uint64
	PTWWalkLatencyCount uint64

	// Cache events (delta during this kernel)
	L1VReadHit      uint64
	L1VReadMiss     uint64
	L1VReadMSHRHit  uint64
	L1VWriteHit     uint64
	L1VWriteMiss    uint64
	L1VWriteMSHRHit uint64

	L1IReadHit      uint64
	L1IReadMiss     uint64
	L1IReadMSHRHit  uint64
	L1IWriteHit     uint64
	L1IWriteMiss    uint64
	L1IWriteMSHRHit uint64

	L1SReadHit      uint64
	L1SReadMiss     uint64
	L1SReadMSHRHit  uint64
	L1SWriteHit     uint64
	L1SWriteMiss    uint64
	L1SWriteMSHRHit uint64

	L2ReadHit      uint64
	L2ReadMiss     uint64
	L2ReadMSHRHit  uint64
	L2WriteHit     uint64
	L2WriteMiss    uint64
	L2WriteMSHRHit uint64

	// TLB events (delta during this kernel)
	L1VTLBHit     uint64
	L1VTLBMiss    uint64
	L1VTLBMSHRHit uint64

	L1ITLBHit     uint64
	L1ITLBMiss    uint64
	L1ITLBMSHRHit uint64

	L1STLBHit     uint64
	L1STLBMiss    uint64
	L1STLBMSHRHit uint64

	L2TLBHit     uint64
	L2TLBMiss    uint64
	L2TLBMSHRHit uint64
}

func newKernelStatsTracer(
	t sim.TimeTeller,
	l2Counter *memAccessCounter,
	dramCounter *dramAccessCounter,
	cacheCounters *cacheEventGroups,
	rdmaCounter *rdmaTrafficCounter,
	ptwCounter *ptwStatsCounter,
	tlbCounters *tlbEventGroups,
) *kernelStatsTracer {
	return &kernelStatsTracer{
		TimeTeller:    t,
		l2Counter:     l2Counter,
		dramCounter:   dramCounter,
		cacheCounters: cacheCounters,
		rdmaCounter:   rdmaCounter,
		ptwCounter:    ptwCounter,
		tlbCounters:   tlbCounters,
		inflight:      make(map[string]inflightKernel),
	}
}

func kernelMetricAttributionWeight(r kernelMetricRecord) int {
	if r.ActiveCUHint > 0 {
		return r.ActiveCUHint
	}
	if r.Managed && r.EffectiveWGTotal > 0 {
		if r.EffectiveWGTotal > uint64(^uint(0)>>1) {
			return int(^uint(0) >> 1)
		}
		return int(r.EffectiveWGTotal)
	}
	if r.NumWGTotal > 0 {
		if r.NumWGTotal > uint64(^uint(0)>>1) {
			return int(^uint(0) >> 1)
		}
		return int(r.NumWGTotal)
	}
	return 1
}

func splitCounter(total uint64, weights []int) []uint64 {
	out := make([]uint64, len(weights))
	if total == 0 || len(weights) == 0 {
		return out
	}
	sumW := 0
	for _, w := range weights {
		if w > 0 {
			sumW += w
		}
	}
	if sumW <= 0 {
		base := total / uint64(len(weights))
		rem := total % uint64(len(weights))
		for i := range out {
			out[i] = base
			if uint64(i) < rem {
				out[i]++
			}
		}
		return out
	}
	var assigned uint64
	type fracPart struct {
		idx  int
		frac uint64
	}
	fracs := make([]fracPart, 0, len(weights))
	den := uint64(sumW)
	for i, w := range weights {
		if w < 0 {
			w = 0
		}
		numer := total * uint64(w)
		out[i] = numer / den
		assigned += out[i]
		fracs = append(fracs, fracPart{idx: i, frac: numer % den})
	}
	sort.SliceStable(fracs, func(i, j int) bool {
		if fracs[i].frac != fracs[j].frac {
			return fracs[i].frac > fracs[j].frac
		}
		return fracs[i].idx < fracs[j].idx
	})
	rem := total - assigned
	for i := uint64(0); i < rem && int(i) < len(fracs); i++ {
		out[fracs[i].idx]++
	}
	return out
}

func distributeMemSnapshot(delta memAccessSnapshot, weights []int) []memAccessSnapshot {
	n := len(weights)
	out := make([]memAccessSnapshot, n)
	assign := func(vals []uint64, set func(i int, v uint64)) {
		for i, v := range vals {
			set(i, v)
		}
	}
	assign(splitCounter(delta.localRead, weights), func(i int, v uint64) { out[i].localRead = v })
	assign(splitCounter(delta.localWrite, weights), func(i int, v uint64) { out[i].localWrite = v })
	assign(splitCounter(delta.remoteRead, weights), func(i int, v uint64) { out[i].remoteRead = v })
	assign(splitCounter(delta.remoteWrite, weights), func(i int, v uint64) { out[i].remoteWrite = v })
	assign(splitCounter(delta.externalRead, weights), func(i int, v uint64) { out[i].externalRead = v })
	assign(splitCounter(delta.externalWrite, weights), func(i int, v uint64) { out[i].externalWrite = v })
	assign(splitCounter(delta.localBytes, weights), func(i int, v uint64) { out[i].localBytes = v })
	assign(splitCounter(delta.remoteBytes, weights), func(i int, v uint64) { out[i].remoteBytes = v })
	assign(splitCounter(delta.externalBytes, weights), func(i int, v uint64) { out[i].externalBytes = v })
	return out
}

func distributeDramSnapshot(delta dramAccessSnapshot, weights []int) []dramAccessSnapshot {
	n := len(weights)
	out := make([]dramAccessSnapshot, n)
	assign := func(vals []uint64, set func(i int, v uint64)) {
		for i, v := range vals {
			set(i, v)
		}
	}
	assign(splitCounter(delta.readReq, weights), func(i int, v uint64) { out[i].readReq = v })
	assign(splitCounter(delta.writeReq, weights), func(i int, v uint64) { out[i].writeReq = v })
	assign(splitCounter(delta.readBytes, weights), func(i int, v uint64) { out[i].readBytes = v })
	assign(splitCounter(delta.writeBytes, weights), func(i int, v uint64) { out[i].writeBytes = v })
	return out
}

func distributePTWSnapshot(delta ptwStatsSnapshot, weights []int) []ptwStatsSnapshot {
	n := len(weights)
	out := make([]ptwStatsSnapshot, n)
	assign := func(vals []uint64, set func(i int, v uint64)) {
		for i, v := range vals {
			set(i, v)
		}
	}
	assign(splitCounter(delta.translationReqCount, weights), func(i int, v uint64) { out[i].translationReqCount = v })
	assign(splitCounter(delta.walkCompletedCount, weights), func(i int, v uint64) { out[i].walkCompletedCount = v })
	assign(splitCounter(delta.walkFailedCount, weights), func(i int, v uint64) { out[i].walkFailedCount = v })
	assign(splitCounter(delta.pwcLookupCount, weights), func(i int, v uint64) { out[i].pwcLookupCount = v })
	assign(splitCounter(delta.pwcHitCount, weights), func(i int, v uint64) { out[i].pwcHitCount = v })
	assign(splitCounter(delta.pwcMissCount, weights), func(i int, v uint64) { out[i].pwcMissCount = v })
	assign(splitCounter(delta.pwcHitLvl0, weights), func(i int, v uint64) { out[i].pwcHitLvl0 = v })
	assign(splitCounter(delta.pwcHitLvl1, weights), func(i int, v uint64) { out[i].pwcHitLvl1 = v })
	assign(splitCounter(delta.pwcHitLvl2, weights), func(i int, v uint64) { out[i].pwcHitLvl2 = v })
	assign(splitCounter(delta.pteReadCount, weights), func(i int, v uint64) { out[i].pteReadCount = v })
	assign(splitCounter(delta.pteReadLocalCount, weights), func(i int, v uint64) { out[i].pteReadLocalCount = v })
	assign(splitCounter(delta.pteReadRemoteCount, weights), func(i int, v uint64) { out[i].pteReadRemoteCount = v })
	assign(splitCounter(delta.walkLatencySumPS, weights), func(i int, v uint64) { out[i].walkLatencySumPS = v })
	assign(splitCounter(delta.walkLatencyCount, weights), func(i int, v uint64) { out[i].walkLatencyCount = v })
	return out
}

func distributeTLBSnapshot(delta tlbEventGroupsSnapshot, weights []int) []tlbEventGroupsSnapshot {
	n := len(weights)
	out := make([]tlbEventGroupsSnapshot, n)
	assign := func(vals []uint64, set func(i int, v uint64)) {
		for i, v := range vals {
			set(i, v)
		}
	}
	assign(splitCounter(delta.L1V.hit, weights), func(i int, v uint64) { out[i].L1V.hit = v })
	assign(splitCounter(delta.L1V.miss, weights), func(i int, v uint64) { out[i].L1V.miss = v })
	assign(splitCounter(delta.L1V.mshrHit, weights), func(i int, v uint64) { out[i].L1V.mshrHit = v })
	assign(splitCounter(delta.L1I.hit, weights), func(i int, v uint64) { out[i].L1I.hit = v })
	assign(splitCounter(delta.L1I.miss, weights), func(i int, v uint64) { out[i].L1I.miss = v })
	assign(splitCounter(delta.L1I.mshrHit, weights), func(i int, v uint64) { out[i].L1I.mshrHit = v })
	assign(splitCounter(delta.L1S.hit, weights), func(i int, v uint64) { out[i].L1S.hit = v })
	assign(splitCounter(delta.L1S.miss, weights), func(i int, v uint64) { out[i].L1S.miss = v })
	assign(splitCounter(delta.L1S.mshrHit, weights), func(i int, v uint64) { out[i].L1S.mshrHit = v })
	assign(splitCounter(delta.L2.hit, weights), func(i int, v uint64) { out[i].L2.hit = v })
	assign(splitCounter(delta.L2.miss, weights), func(i int, v uint64) { out[i].L2.miss = v })
	assign(splitCounter(delta.L2.mshrHit, weights), func(i int, v uint64) { out[i].L2.mshrHit = v })
	return out
}

func memSnapshotActivity(delta memAccessSnapshot) float64 {
	bytes := delta.localBytes + delta.remoteBytes + delta.externalBytes
	if bytes > 0 {
		return float64(bytes)
	}
	ops := delta.localRead + delta.localWrite + delta.remoteRead + delta.remoteWrite + delta.externalRead + delta.externalWrite
	return float64(ops)
}

func dramSnapshotActivity(delta dramAccessSnapshot) float64 {
	bytes := delta.readBytes + delta.writeBytes
	if bytes > 0 {
		return float64(bytes)
	}
	return float64(delta.readReq + delta.writeReq)
}

func ptwSnapshotActivity(delta ptwStatsSnapshot) float64 {
	ops := delta.translationReqCount + delta.walkCompletedCount + delta.walkFailedCount + delta.pteReadCount
	if ops > 0 {
		return float64(ops)
	}
	return float64(delta.walkLatencyCount)
}

func tlbSnapshotActivity(delta tlbEventGroupsSnapshot) float64 {
	total := delta.L1V.hit + delta.L1V.miss + delta.L1V.mshrHit +
		delta.L1I.hit + delta.L1I.miss + delta.L1I.mshrHit +
		delta.L1S.hit + delta.L1S.miss + delta.L1S.mshrHit +
		delta.L2.hit + delta.L2.miss + delta.L2.mshrHit
	return float64(total)
}

func normalizeAttributionWeights(baseWeights []int, activeIdx []int, rates []float64) []int {
	out := make([]int, len(baseWeights))
	copy(out, baseWeights)
	if len(baseWeights) == 0 || len(activeIdx) != len(baseWeights) {
		return out
	}

	minPositive := 0.0
	maxRaw := 0.0
	raw := make([]float64, len(baseWeights))
	for _, idx := range activeIdx {
		rate := 0.0
		if idx >= 0 && idx < len(rates) {
			rate = rates[idx]
		}
		if rate > 0 && (minPositive == 0.0 || rate < minPositive) {
			minPositive = rate
		}
	}
	if minPositive <= 0.0 {
		return out
	}
	for pos, idx := range activeIdx {
		bw := 1.0
		if baseWeights[pos] > 0 {
			bw = float64(baseWeights[pos])
		}
		rate := 0.0
		if idx >= 0 && idx < len(rates) {
			rate = rates[idx]
		}
		if rate <= 0.0 {
			rate = minPositive * 0.25
		}
		raw[pos] = bw * rate
		if raw[pos] > maxRaw {
			maxRaw = raw[pos]
		}
	}
	if maxRaw <= 0.0 {
		return out
	}
	scale := 1024.0 / maxRaw
	for i, v := range raw {
		w := int(math.Round(v * scale))
		if w <= 0 {
			w = 1
		}
		out[i] = w
	}
	return out
}

func zeroAttributedFields(r *kernelMetricRecord) {
	r.LocalRead = 0
	r.LocalWrite = 0
	r.RemoteRead = 0
	r.RemoteWrite = 0
	r.ExternalRead = 0
	r.ExternalWrite = 0
	r.LocalBytes = 0
	r.RemoteBytes = 0
	r.ExternalBytes = 0

	r.DRAMReadReq = 0
	r.DRAMWriteReq = 0
	r.DRAMReadBytes = 0
	r.DRAMWriteBytes = 0

	r.PTWTranslationReqs = 0
	r.PTWWalkCompleted = 0
	r.PTWWalkFailed = 0
	r.PTWPWCLookups = 0
	r.PTWPWCHits = 0
	r.PTWPWCMisses = 0
	r.PTWPWCHitLevel0 = 0
	r.PTWPWCHitLevel1 = 0
	r.PTWPWCHitLevel2 = 0
	r.PTWPTEReads = 0
	r.PTWPTEReadsLocal = 0
	r.PTWPTEReadsRemote = 0
	r.PTWWalkLatencySumPS = 0
	r.PTWWalkLatencyCount = 0

	r.L1VTLBHit = 0
	r.L1VTLBMiss = 0
	r.L1VTLBMSHRHit = 0
	r.L1ITLBHit = 0
	r.L1ITLBMiss = 0
	r.L1ITLBMSHRHit = 0
	r.L1STLBHit = 0
	r.L1STLBMiss = 0
	r.L1STLBMSHRHit = 0
	r.L2TLBHit = 0
	r.L2TLBMiss = 0
	r.L2TLBMSHRHit = 0
}

func applyMemDelta(r *kernelMetricRecord, d memAccessSnapshot) {
	r.LocalRead += d.localRead
	r.LocalWrite += d.localWrite
	r.RemoteRead += d.remoteRead
	r.RemoteWrite += d.remoteWrite
	r.ExternalRead += d.externalRead
	r.ExternalWrite += d.externalWrite
	r.LocalBytes += d.localBytes
	r.RemoteBytes += d.remoteBytes
	r.ExternalBytes += d.externalBytes
}

func applyDramDelta(r *kernelMetricRecord, d dramAccessSnapshot) {
	r.DRAMReadReq += d.readReq
	r.DRAMWriteReq += d.writeReq
	r.DRAMReadBytes += d.readBytes
	r.DRAMWriteBytes += d.writeBytes
}

func applyPTWDelta(r *kernelMetricRecord, d ptwStatsSnapshot) {
	r.PTWTranslationReqs += d.translationReqCount
	r.PTWWalkCompleted += d.walkCompletedCount
	r.PTWWalkFailed += d.walkFailedCount
	r.PTWPWCLookups += d.pwcLookupCount
	r.PTWPWCHits += d.pwcHitCount
	r.PTWPWCMisses += d.pwcMissCount
	r.PTWPWCHitLevel0 += d.pwcHitLvl0
	r.PTWPWCHitLevel1 += d.pwcHitLvl1
	r.PTWPWCHitLevel2 += d.pwcHitLvl2
	r.PTWPTEReads += d.pteReadCount
	r.PTWPTEReadsLocal += d.pteReadLocalCount
	r.PTWPTEReadsRemote += d.pteReadRemoteCount
	r.PTWWalkLatencySumPS += d.walkLatencySumPS
	r.PTWWalkLatencyCount += d.walkLatencyCount
}

func applyTLBDelta(r *kernelMetricRecord, d tlbEventGroupsSnapshot) {
	r.L1VTLBHit += d.L1V.hit
	r.L1VTLBMiss += d.L1V.miss
	r.L1VTLBMSHRHit += d.L1V.mshrHit
	r.L1ITLBHit += d.L1I.hit
	r.L1ITLBMiss += d.L1I.miss
	r.L1ITLBMSHRHit += d.L1I.mshrHit
	r.L1STLBHit += d.L1S.hit
	r.L1STLBMiss += d.L1S.miss
	r.L1STLBMSHRHit += d.L1S.mshrHit
	r.L2TLBHit += d.L2.hit
	r.L2TLBMiss += d.L2.miss
	r.L2TLBMSHRHit += d.L2.mshrHit
}

func attributeKernelOverlapMetrics(records []kernelMetricRecord) []kernelMetricRecord {
	if len(records) <= 1 {
		return records
	}
	out := make([]kernelMetricRecord, len(records))
	copy(out, records)
	for i := range out {
		zeroAttributedFields(&out[i])
	}

	timeSet := make(map[sim.VTimeInSec]bool)
	l2At := make(map[sim.VTimeInSec]memAccessSnapshot)
	dramAt := make(map[sim.VTimeInSec]dramAccessSnapshot)
	ptwAt := make(map[sim.VTimeInSec]ptwStatsSnapshot)
	tlbAt := make(map[sim.VTimeInSec]tlbEventGroupsSnapshot)

	for _, r := range records {
		timeSet[r.Start] = true
		timeSet[r.End] = true
		l2At[r.Start] = r.rawStartL2Snap
		l2At[r.End] = r.rawEndL2Snap
		dramAt[r.Start] = r.rawStartDramSnap
		dramAt[r.End] = r.rawEndDramSnap
		ptwAt[r.Start] = r.rawStartPTW
		ptwAt[r.End] = r.rawEndPTW
		tlbAt[r.Start] = r.rawStartTLBSnap
		tlbAt[r.End] = r.rawEndTLBSnap
	}

	bounds := make([]sim.VTimeInSec, 0, len(timeSet))
	for t := range timeSet {
		bounds = append(bounds, t)
	}
	sort.Slice(bounds, func(i, j int) bool { return bounds[i] < bounds[j] })

	type overlapSegment struct {
		start       sim.VTimeInSec
		end         sim.VTimeInSec
		activeIdx   []int
		baseWeights []int
		l2Delta     memAccessSnapshot
		dramDelta   dramAccessSnapshot
		ptwDelta    ptwStatsSnapshot
		tlbDelta    tlbEventGroupsSnapshot
	}

	segments := make([]overlapSegment, 0, len(bounds))
	memSoloRate := make([]float64, len(records))
	dramSoloRate := make([]float64, len(records))
	ptwSoloRate := make([]float64, len(records))
	tlbSoloRate := make([]float64, len(records))
	soloDur := make([]float64, len(records))

	for bi := 0; bi+1 < len(bounds); bi++ {
		segStart := bounds[bi]
		segEnd := bounds[bi+1]
		if !(segEnd > segStart) {
			continue
		}
		activeIdx := make([]int, 0, len(records))
		baseWeights := make([]int, 0, len(records))
		for i, r := range records {
			if r.Start < segEnd && r.End > segStart {
				activeIdx = append(activeIdx, i)
				baseWeights = append(baseWeights, kernelMetricAttributionWeight(r))
			}
		}
		if len(activeIdx) == 0 {
			continue
		}
		l2Delta := l2At[segEnd].Sub(l2At[segStart])
		dramDelta := dramAt[segEnd].Sub(dramAt[segStart])
		ptwDelta := ptwAt[segEnd].sub(ptwAt[segStart])
		tlbDelta := tlbAt[segEnd].Sub(tlbAt[segStart])

		segments = append(segments, overlapSegment{
			start:       segStart,
			end:         segEnd,
			activeIdx:   activeIdx,
			baseWeights: baseWeights,
			l2Delta:     l2Delta,
			dramDelta:   dramDelta,
			ptwDelta:    ptwDelta,
			tlbDelta:    tlbDelta,
		})

		if len(activeIdx) == 1 {
			idx := activeIdx[0]
			dur := float64(segEnd - segStart)
			if dur > 0 {
				soloDur[idx] += dur
				memSoloRate[idx] += memSnapshotActivity(l2Delta)
				dramSoloRate[idx] += dramSnapshotActivity(dramDelta)
				ptwSoloRate[idx] += ptwSnapshotActivity(ptwDelta)
				tlbSoloRate[idx] += tlbSnapshotActivity(tlbDelta)
			}
		}
	}

	for i := range records {
		if soloDur[i] > 0 {
			memSoloRate[i] /= soloDur[i]
			dramSoloRate[i] /= soloDur[i]
			ptwSoloRate[i] /= soloDur[i]
			tlbSoloRate[i] /= soloDur[i]
		}
	}

	for _, seg := range segments {
		if len(seg.activeIdx) == 1 {
			idx := seg.activeIdx[0]
			applyMemDelta(&out[idx], seg.l2Delta)
			applyDramDelta(&out[idx], seg.dramDelta)
			applyPTWDelta(&out[idx], seg.ptwDelta)
			applyTLBDelta(&out[idx], seg.tlbDelta)
			continue
		}

		memWeights := normalizeAttributionWeights(seg.baseWeights, seg.activeIdx, memSoloRate)
		dramWeights := normalizeAttributionWeights(seg.baseWeights, seg.activeIdx, dramSoloRate)
		ptwWeights := normalizeAttributionWeights(seg.baseWeights, seg.activeIdx, ptwSoloRate)
		tlbWeights := normalizeAttributionWeights(seg.baseWeights, seg.activeIdx, tlbSoloRate)

		l2Split := distributeMemSnapshot(seg.l2Delta, memWeights)
		dramSplit := distributeDramSnapshot(seg.dramDelta, dramWeights)
		ptwSplit := distributePTWSnapshot(seg.ptwDelta, ptwWeights)
		tlbSplit := distributeTLBSnapshot(seg.tlbDelta, tlbWeights)

		for pos, idx := range seg.activeIdx {
			applyMemDelta(&out[idx], l2Split[pos])
			applyDramDelta(&out[idx], dramSplit[pos])
			applyPTWDelta(&out[idx], ptwSplit[pos])
			applyTLBDelta(&out[idx], tlbSplit[pos])
		}
	}

	return out
}

func (t *kernelStatsTracer) StartTask(task tracing.Task) {
	if !isKernelCommandTask(task) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	startTime := t.CurrentTime()

	l2Snap := memAccessSnapshot{}
	if t.l2Counter != nil {
		l2Snap = t.l2Counter.Snapshot()
	}

	dramSnap := dramAccessSnapshot{}
	if t.dramCounter != nil {
		dramSnap = t.dramCounter.Snapshot()
	}

	cacheSnap := cacheEventGroupsSnapshot{}
	if t.cacheCounters != nil {
		cacheSnap = t.cacheCounters.Snapshot()
	}

	rdmaSnap := rdmaTrafficSnapshot{}
	if t.rdmaCounter != nil {
		rdmaSnap = t.rdmaCounter.Snapshot()
	}

	ptwSnap := ptwStatsSnapshot{}
	if t.ptwCounter != nil {
		ptwSnap = t.ptwCounter.Snapshot()
	}

	tlbSnap := tlbEventGroupsSnapshot{}
	if t.tlbCounters != nil {
		tlbSnap = t.tlbCounters.Snapshot()
	}

	name := kernelNameFromTask(task)

	gridSize, wgSize := kernelLaunchDimsFromTask(task)
	numWG, numWGTotal := deriveNumWG(gridSize, wgSize)
	managed, launchAbsID, launchRelID, programLaunchID, subWGFactor, wgWindowStart, wgWindowCount, effectiveWGTotal := kernelLaunchRuntimeDetailFromTask(task)
	activeCUHint := kernelLaunchActiveCUHintFromTask(task)
	if effectiveWGTotal == 0 {
		effectiveWGTotal = numWGTotal
	}
	if subWGFactor > 1 {
		effectiveWGTotal *= uint64(subWGFactor)
	}

	t.inflight[task.ID] = inflightKernel{
		index:            t.nextIdx,
		name:             name,
		cmdID:            task.ID,
		startTime:        startTime,
		gridSize:         gridSize,
		wgSize:           wgSize,
		numWG:            numWG,
		numWGTotal:       numWGTotal,
		effectiveWGTotal: effectiveWGTotal,
		managed:          managed,
		launchAbsID:      launchAbsID,
		launchRelID:      launchRelID,
		programLaunchID:  programLaunchID,
		subWGFactor:      subWGFactor,
		wgWindowStart:    wgWindowStart,
		wgWindowCount:    wgWindowCount,
		activeCUHint:     activeCUHint,
		startL2Snap:      l2Snap,
		startDramSnap:    dramSnap,
		startCacheSnap:   cacheSnap,
		startRdmaSnap:    rdmaSnap,
		startPTW:         ptwSnap,
		startTLBSnap:     tlbSnap,
	}
	t.nextIdx++
}

func (t *kernelStatsTracer) StepTask(task tracing.Task) {}

func (t *kernelStatsTracer) AddMilestone(milestone tracing.Milestone) {}

// EndTask is called with a Task that only contains the Task ID (Akita's
// tracing.EndTask constructs Task{ID: ...}). Therefore, we must NOT rely on
// task.What/task.Kind here. Instead, use the inflight map populated in StartTask.
func (t *kernelStatsTracer) EndTask(task tracing.Task) {
	t.mu.Lock()
	defer t.mu.Unlock()

	inflight, ok := t.inflight[task.ID]
	if !ok {
		return
	}

	endTime := t.CurrentTime()

	endL2 := memAccessSnapshot{}
	if t.l2Counter != nil {
		endL2 = t.l2Counter.Snapshot()
	}
	deltaL2 := endL2.Sub(inflight.startL2Snap)

	endDram := dramAccessSnapshot{}
	if t.dramCounter != nil {
		endDram = t.dramCounter.Snapshot()
	}
	deltaDram := endDram.Sub(inflight.startDramSnap)

	endCache := cacheEventGroupsSnapshot{}
	if t.cacheCounters != nil {
		endCache = t.cacheCounters.Snapshot()
	}
	deltaCache := endCache.Sub(inflight.startCacheSnap)

	endRdma := rdmaTrafficSnapshot{}
	if t.rdmaCounter != nil {
		endRdma = t.rdmaCounter.Snapshot()
	}
	deltaRdma := endRdma.Sub(inflight.startRdmaSnap)

	endPTW := ptwStatsSnapshot{}
	if t.ptwCounter != nil {
		endPTW = t.ptwCounter.Snapshot()
	}
	deltaPTW := endPTW.sub(inflight.startPTW)

	endTLB := tlbEventGroupsSnapshot{}
	if t.tlbCounters != nil {
		endTLB = t.tlbCounters.Snapshot()
	}
	deltaTLB := endTLB.Sub(inflight.startTLBSnap)

	rec := kernelMetricRecord{
		Index:   inflight.index,
		Name:    inflight.name,
		CmdID:   inflight.cmdID,
		Start:   inflight.startTime,
		End:     endTime,
		Latency: endTime - inflight.startTime,

		GridSize:         inflight.gridSize,
		WGSize:           inflight.wgSize,
		NumWG:            inflight.numWG,
		NumWGTotal:       inflight.numWGTotal,
		EffectiveWGTotal: inflight.effectiveWGTotal,
		Managed:          inflight.managed,
		LaunchAbsID:      inflight.launchAbsID,
		LaunchRelID:      inflight.launchRelID,
		ProgramLaunchID:  inflight.programLaunchID,
		SubWGFactor:      inflight.subWGFactor,
		WGWindowStart:    inflight.wgWindowStart,
		WGWindowCount:    inflight.wgWindowCount,
		ActiveCUHint:     inflight.activeCUHint,

		rawStartL2Snap:   inflight.startL2Snap,
		rawEndL2Snap:     endL2,
		rawStartDramSnap: inflight.startDramSnap,
		rawEndDramSnap:   endDram,
		rawStartPTW:      inflight.startPTW,
		rawEndPTW:        endPTW,
		rawStartTLBSnap:  inflight.startTLBSnap,
		rawEndTLBSnap:    endTLB,

		LocalRead:     deltaL2.localRead,
		LocalWrite:    deltaL2.localWrite,
		RemoteRead:    deltaL2.remoteRead,
		RemoteWrite:   deltaL2.remoteWrite,
		ExternalRead:  deltaL2.externalRead,
		ExternalWrite: deltaL2.externalWrite,

		LocalBytes:    deltaL2.localBytes,
		RemoteBytes:   deltaL2.remoteBytes,
		ExternalBytes: deltaL2.externalBytes,

		DRAMReadReq:    deltaDram.readReq,
		DRAMWriteReq:   deltaDram.writeReq,
		DRAMReadBytes:  deltaDram.readBytes,
		DRAMWriteBytes: deltaDram.writeBytes,

		NoPSendPackets: deltaRdma.nopSendPackets,
		NoPSendBytes:   deltaRdma.nopSendBytes,
		NoPRecvPackets: deltaRdma.nopRecvPackets,
		NoPRecvBytes:   deltaRdma.nopRecvBytes,

		NoCSendPackets: deltaRdma.nocSendPackets,
		NoCSendBytes:   deltaRdma.nocSendBytes,
		NoCRecvPackets: deltaRdma.nocRecvPackets,
		NoCRecvBytes:   deltaRdma.nocRecvBytes,

		RDMAReqSendPackets: deltaRdma.rdmaReqSendPackets,
		RDMAReqSendBytes:   deltaRdma.rdmaReqSendBytes,

		PTWTranslationReqs:  deltaPTW.translationReqCount,
		PTWWalkCompleted:    deltaPTW.walkCompletedCount,
		PTWWalkFailed:       deltaPTW.walkFailedCount,
		PTWPWCLookups:       deltaPTW.pwcLookupCount,
		PTWPWCHits:          deltaPTW.pwcHitCount,
		PTWPWCMisses:        deltaPTW.pwcMissCount,
		PTWPWCHitLevel0:     deltaPTW.pwcHitLvl0,
		PTWPWCHitLevel1:     deltaPTW.pwcHitLvl1,
		PTWPWCHitLevel2:     deltaPTW.pwcHitLvl2,
		PTWPTEReads:         deltaPTW.pteReadCount,
		PTWPTEReadsLocal:    deltaPTW.pteReadLocalCount,
		PTWPTEReadsRemote:   deltaPTW.pteReadRemoteCount,
		PTWWalkLatencySumPS: deltaPTW.walkLatencySumPS,
		PTWWalkLatencyCount: deltaPTW.walkLatencyCount,

		L1VReadHit:      deltaCache.L1V.readHit,
		L1VReadMiss:     deltaCache.L1V.readMiss,
		L1VReadMSHRHit:  deltaCache.L1V.readMSHRHit,
		L1VWriteHit:     deltaCache.L1V.writeHit,
		L1VWriteMiss:    deltaCache.L1V.writeMiss,
		L1VWriteMSHRHit: deltaCache.L1V.writeMSHRHit,

		L1IReadHit:      deltaCache.L1I.readHit,
		L1IReadMiss:     deltaCache.L1I.readMiss,
		L1IReadMSHRHit:  deltaCache.L1I.readMSHRHit,
		L1IWriteHit:     deltaCache.L1I.writeHit,
		L1IWriteMiss:    deltaCache.L1I.writeMiss,
		L1IWriteMSHRHit: deltaCache.L1I.writeMSHRHit,

		L1SReadHit:      deltaCache.L1S.readHit,
		L1SReadMiss:     deltaCache.L1S.readMiss,
		L1SReadMSHRHit:  deltaCache.L1S.readMSHRHit,
		L1SWriteHit:     deltaCache.L1S.writeHit,
		L1SWriteMiss:    deltaCache.L1S.writeMiss,
		L1SWriteMSHRHit: deltaCache.L1S.writeMSHRHit,

		L2ReadHit:      deltaCache.L2.readHit,
		L2ReadMiss:     deltaCache.L2.readMiss,
		L2ReadMSHRHit:  deltaCache.L2.readMSHRHit,
		L2WriteHit:     deltaCache.L2.writeHit,
		L2WriteMiss:    deltaCache.L2.writeMiss,
		L2WriteMSHRHit: deltaCache.L2.writeMSHRHit,

		L1VTLBHit:     deltaTLB.L1V.hit,
		L1VTLBMiss:    deltaTLB.L1V.miss,
		L1VTLBMSHRHit: deltaTLB.L1V.mshrHit,

		L1ITLBHit:     deltaTLB.L1I.hit,
		L1ITLBMiss:    deltaTLB.L1I.miss,
		L1ITLBMSHRHit: deltaTLB.L1I.mshrHit,

		L1STLBHit:     deltaTLB.L1S.hit,
		L1STLBMiss:    deltaTLB.L1S.miss,
		L1STLBMSHRHit: deltaTLB.L1S.mshrHit,

		L2TLBHit:     deltaTLB.L2.hit,
		L2TLBMiss:    deltaTLB.L2.miss,
		L2TLBMSHRHit: deltaTLB.L2.mshrHit,
	}

	t.records = append(t.records, rec)
	delete(t.inflight, task.ID)
}

func isKernelCommandTask(task tracing.Task) bool {
	return task.What == "*driver.LaunchKernelCommand" ||
		task.What == "*driver.LaunchUnifiedMultiGPUKernelCommand"
}

func kernelNameFromTask(task tracing.Task) string {
	// Default name if we cannot extract more information.
	name := "unknown"

	if task.Detail == nil {
		return name
	}

	switch cmd := task.Detail.(type) {
	case *driver.LaunchKernelCommand:
		if cmd.CodeObject != nil && cmd.CodeObject.Symbol != nil && cmd.CodeObject.Symbol.Name != "" {
			name = cmd.CodeObject.Symbol.Name
		}
	case *driver.LaunchUnifiedMultiGPUKernelCommand:
		if cmd.CodeObject != nil && cmd.CodeObject.Symbol != nil && cmd.CodeObject.Symbol.Name != "" {
			name = cmd.CodeObject.Symbol.Name
		}
	}

	return name
}

func kernelLaunchDimsFromTask(task tracing.Task) (grid [3]uint32, wg [3]uint16) {
	if task.Detail == nil {
		return
	}

	switch cmd := task.Detail.(type) {
	case *driver.LaunchKernelCommand:
		grid = cmd.GridSize
		wg = cmd.WGSize
	case *driver.LaunchUnifiedMultiGPUKernelCommand:
		grid = cmd.GridSize
		wg = cmd.WGSize
	}

	return
}

func summarizeLaunchCommandWGWindow(cmd *driver.LaunchKernelCommand) (start int, count int, effectiveWGTotal uint64) {
	if cmd == nil {
		return 0, 0, 0
	}
	if len(cmd.WGWindows) == 1 {
		w := cmd.WGWindows[0]
		if w.Count > 0 {
			return int(w.Start), int(w.Count), uint64(w.Count)
		}
	}
	if len(cmd.WGWindows) > 1 {
		total := 0
		for _, w := range cmd.WGWindows {
			if w.Count > 0 {
				total += int(w.Count)
			}
		}
		if total > 0 {
			return int(cmd.WGStart), int(total), uint64(total)
		}
	}
	if cmd.Managed && cmd.WGCount > 0 {
		return int(cmd.WGStart), int(cmd.WGCount), uint64(cmd.WGCount)
	}
	return 0, 0, 0
}

func kernelLaunchRuntimeDetailFromTask(task tracing.Task) (managed bool, launchAbsID, launchRelID, programLaunchID, subWGFactor, wgWindowStart, wgWindowCount int, effectiveWGTotal uint64) {
	if task.Detail == nil {
		return
	}
	switch cmd := task.Detail.(type) {
	case *driver.LaunchKernelCommand:
		managed = cmd.Managed
		launchAbsID = cmd.LaunchAbsID
		launchRelID = cmd.LaunchRelID
		programLaunchID = cmd.ProgramLaunchID
		subWGFactor = cmd.SubWGFactor
		wgWindowStart, wgWindowCount, effectiveWGTotal = summarizeLaunchCommandWGWindow(cmd)
	}
	return
}

func kernelLaunchActiveCUHintFromTask(task tracing.Task) int {
	if task.Detail == nil {
		return 0
	}
	sumPositive := func(xs []int) int {
		total := 0
		for _, x := range xs {
			if x > 0 {
				total += x
			}
		}
		return total
	}
	switch cmd := task.Detail.(type) {
	case *driver.LaunchKernelCommand:
		if len(cmd.AllowedCUIDs) > 0 {
			return len(cmd.AllowedCUIDs)
		}
		if s := sumPositive(cmd.ChipletCUs); s > 0 {
			return s
		}
		if cmd.EstCUs > 0 {
			return cmd.EstCUs
		}
	}
	return 0
}

func deriveNumWG(grid [3]uint32, wg [3]uint16) (numWG [3]uint32, total uint64) {
	dims := 0
	total = 1

	for i := 0; i < 3; i++ {
		if wg[i] == 0 {
			continue
		}
		dims++
		w := uint32(wg[i])
		if w == 0 {
			continue
		}
		n := (grid[i] + w - 1) / w
		numWG[i] = n
		total *= uint64(n)
	}

	if dims == 0 {
		total = 0
	}
	return
}

// Dump writes per-kernel metrics into "<baseName>.csv".
// The default baseName used by the reporter is "<metric-file-name>_kernels".
func (t *kernelStatsTracer) Dump(baseName string) error {
	t.mu.Lock()
	records := make([]kernelMetricRecord, len(t.records))
	copy(records, t.records)
	t.mu.Unlock()

	records = attributeKernelOverlapMetrics(records)
	sort.Slice(records, func(i, j int) bool { return records[i].Index < records[j].Index })

	f, err := os.Create(baseName + ".csv")
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	header := []string{
		"kernel_index",
		"kernel_name",
		"command_id",
		"start_time_sec",
		"end_time_sec",
		"latency_sec",
		"local_reads",
		"local_writes",
		"remote_reads",
		"remote_writes",
		"external_reads",
		"external_writes",
		"local_bytes_est",
		"remote_bytes_est",
		"external_bytes_est",
		"dram_read_reqs",
		"dram_write_reqs",
		"dram_read_bytes",
		"dram_write_bytes",
		"nop_send_packets",
		"nop_send_bytes",
		"nop_recv_packets",
		"nop_recv_bytes",
		"noc_send_packets",
		"noc_send_bytes",
		"noc_recv_packets",
		"noc_recv_bytes",
		"rdma_req_send_packets",
		"rdma_req_send_bytes",
		"ptw_translation_reqs",
		"ptw_walk_completed",
		"ptw_walk_failed",
		"ptw_pwc_lookups",
		"ptw_pwc_hits",
		"ptw_pwc_misses",
		"ptw_pwc_hit_level0",
		"ptw_pwc_hit_level1",
		"ptw_pwc_hit_level2",
		"ptw_pte_reads",
		"ptw_pte_reads_local",
		"ptw_pte_reads_remote",
		"ptw_walk_latency_sum_ps",
		"ptw_walk_latency_count",
		"l1v_read_hit",
		"l1v_read_miss",
		"l1v_read_mshr_hit",
		"l1v_write_hit",
		"l1v_write_miss",
		"l1v_write_mshr_hit",
		"l1i_read_hit",
		"l1i_read_miss",
		"l1i_read_mshr_hit",
		"l1i_write_hit",
		"l1i_write_miss",
		"l1i_write_mshr_hit",
		"l1s_read_hit",
		"l1s_read_miss",
		"l1s_read_mshr_hit",
		"l1s_write_hit",
		"l1s_write_miss",
		"l1s_write_mshr_hit",
		"l2_read_hit",
		"l2_read_miss",
		"l2_read_mshr_hit",
		"l2_write_hit",
		"l2_write_miss",
		"l2_write_mshr_hit",
		"l1v_tlb_hit",
		"l1v_tlb_miss",
		"l1v_tlb_mshr_hit",
		"l1i_tlb_hit",
		"l1i_tlb_miss",
		"l1i_tlb_mshr_hit",
		"l1s_tlb_hit",
		"l1s_tlb_miss",
		"l1s_tlb_mshr_hit",
		"l2_tlb_hit",
		"l2_tlb_miss",
		"l2_tlb_mshr_hit",
		"grid_x",
		"grid_y",
		"grid_z",
		"wg_x",
		"wg_y",
		"wg_z",
		"num_wgx",
		"num_wgy",
		"num_wgz",
		"num_wg_total",
		"effective_num_wg_total",
		"managed",
		"launch_abs_id",
		"launch_rel_id",
		"program_launch_id",
		"sub_wg_factor",
		"wg_window_start",
		"wg_window_count",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range records {
		row := []string{
			strconv.Itoa(r.Index),
			r.Name,
			r.CmdID,
			fmt.Sprintf("%.12f", float64(r.Start)),
			fmt.Sprintf("%.12f", float64(r.End)),
			fmt.Sprintf("%.12f", float64(r.Latency)),
			strconv.FormatUint(r.LocalRead, 10),
			strconv.FormatUint(r.LocalWrite, 10),
			strconv.FormatUint(r.RemoteRead, 10),
			strconv.FormatUint(r.RemoteWrite, 10),
			strconv.FormatUint(r.ExternalRead, 10),
			strconv.FormatUint(r.ExternalWrite, 10),
			strconv.FormatUint(r.LocalBytes, 10),
			strconv.FormatUint(r.RemoteBytes, 10),
			strconv.FormatUint(r.ExternalBytes, 10),
			strconv.FormatUint(r.DRAMReadReq, 10),
			strconv.FormatUint(r.DRAMWriteReq, 10),
			strconv.FormatUint(r.DRAMReadBytes, 10),
			strconv.FormatUint(r.DRAMWriteBytes, 10),
			strconv.FormatUint(r.NoPSendPackets, 10),
			strconv.FormatUint(r.NoPSendBytes, 10),
			strconv.FormatUint(r.NoPRecvPackets, 10),
			strconv.FormatUint(r.NoPRecvBytes, 10),
			strconv.FormatUint(r.NoCSendPackets, 10),
			strconv.FormatUint(r.NoCSendBytes, 10),
			strconv.FormatUint(r.NoCRecvPackets, 10),
			strconv.FormatUint(r.NoCRecvBytes, 10),
			strconv.FormatUint(r.RDMAReqSendPackets, 10),
			strconv.FormatUint(r.RDMAReqSendBytes, 10),
			strconv.FormatUint(r.PTWTranslationReqs, 10),
			strconv.FormatUint(r.PTWWalkCompleted, 10),
			strconv.FormatUint(r.PTWWalkFailed, 10),
			strconv.FormatUint(r.PTWPWCLookups, 10),
			strconv.FormatUint(r.PTWPWCHits, 10),
			strconv.FormatUint(r.PTWPWCMisses, 10),
			strconv.FormatUint(r.PTWPWCHitLevel0, 10),
			strconv.FormatUint(r.PTWPWCHitLevel1, 10),
			strconv.FormatUint(r.PTWPWCHitLevel2, 10),
			strconv.FormatUint(r.PTWPTEReads, 10),
			strconv.FormatUint(r.PTWPTEReadsLocal, 10),
			strconv.FormatUint(r.PTWPTEReadsRemote, 10),
			strconv.FormatUint(r.PTWWalkLatencySumPS, 10),
			strconv.FormatUint(r.PTWWalkLatencyCount, 10),
			strconv.FormatUint(r.L1VReadHit, 10),
			strconv.FormatUint(r.L1VReadMiss, 10),
			strconv.FormatUint(r.L1VReadMSHRHit, 10),
			strconv.FormatUint(r.L1VWriteHit, 10),
			strconv.FormatUint(r.L1VWriteMiss, 10),
			strconv.FormatUint(r.L1VWriteMSHRHit, 10),
			strconv.FormatUint(r.L1IReadHit, 10),
			strconv.FormatUint(r.L1IReadMiss, 10),
			strconv.FormatUint(r.L1IReadMSHRHit, 10),
			strconv.FormatUint(r.L1IWriteHit, 10),
			strconv.FormatUint(r.L1IWriteMiss, 10),
			strconv.FormatUint(r.L1IWriteMSHRHit, 10),
			strconv.FormatUint(r.L1SReadHit, 10),
			strconv.FormatUint(r.L1SReadMiss, 10),
			strconv.FormatUint(r.L1SReadMSHRHit, 10),
			strconv.FormatUint(r.L1SWriteHit, 10),
			strconv.FormatUint(r.L1SWriteMiss, 10),
			strconv.FormatUint(r.L1SWriteMSHRHit, 10),
			strconv.FormatUint(r.L2ReadHit, 10),
			strconv.FormatUint(r.L2ReadMiss, 10),
			strconv.FormatUint(r.L2ReadMSHRHit, 10),
			strconv.FormatUint(r.L2WriteHit, 10),
			strconv.FormatUint(r.L2WriteMiss, 10),
			strconv.FormatUint(r.L2WriteMSHRHit, 10),
			strconv.FormatUint(r.L1VTLBHit, 10),
			strconv.FormatUint(r.L1VTLBMiss, 10),
			strconv.FormatUint(r.L1VTLBMSHRHit, 10),
			strconv.FormatUint(r.L1ITLBHit, 10),
			strconv.FormatUint(r.L1ITLBMiss, 10),
			strconv.FormatUint(r.L1ITLBMSHRHit, 10),
			strconv.FormatUint(r.L1STLBHit, 10),
			strconv.FormatUint(r.L1STLBMiss, 10),
			strconv.FormatUint(r.L1STLBMSHRHit, 10),
			strconv.FormatUint(r.L2TLBHit, 10),
			strconv.FormatUint(r.L2TLBMiss, 10),
			strconv.FormatUint(r.L2TLBMSHRHit, 10),
			strconv.FormatUint(uint64(r.GridSize[0]), 10),
			strconv.FormatUint(uint64(r.GridSize[1]), 10),
			strconv.FormatUint(uint64(r.GridSize[2]), 10),
			strconv.FormatUint(uint64(r.WGSize[0]), 10),
			strconv.FormatUint(uint64(r.WGSize[1]), 10),
			strconv.FormatUint(uint64(r.WGSize[2]), 10),
			strconv.FormatUint(uint64(r.NumWG[0]), 10),
			strconv.FormatUint(uint64(r.NumWG[1]), 10),
			strconv.FormatUint(uint64(r.NumWG[2]), 10),
			strconv.FormatUint(r.NumWGTotal, 10),
			strconv.FormatUint(r.EffectiveWGTotal, 10),
			strconv.FormatBool(r.Managed),
			strconv.Itoa(r.LaunchAbsID),
			strconv.Itoa(r.LaunchRelID),
			strconv.Itoa(r.ProgramLaunchID),
			strconv.Itoa(r.SubWGFactor),
			strconv.Itoa(r.WGWindowStart),
			strconv.Itoa(r.WGWindowCount),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	w.Flush()
	return w.Error()
}
