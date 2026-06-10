package runner

import "github.com/sarchlab/akita/v4/mem/vm/ptw"

// ptwStatsSnapshot is an aggregated snapshot over all PTW instances.
//
// All fields are monotonic counters.
type ptwStatsSnapshot struct {
	translationReqCount uint64
	walkCompletedCount  uint64
	walkFailedCount     uint64

	pwcLookupCount uint64
	pwcHitCount    uint64
	pwcMissCount   uint64
	pwcHitLvl0     uint64
	pwcHitLvl1     uint64
	pwcHitLvl2     uint64

	pteReadCount       uint64
	pteReadLocalCount  uint64
	pteReadRemoteCount uint64

	walkLatencySumPS uint64
	walkLatencyCount uint64
}

func (s ptwStatsSnapshot) sub(prev ptwStatsSnapshot) ptwStatsSnapshot {
	return ptwStatsSnapshot{
		translationReqCount: s.translationReqCount - prev.translationReqCount,
		walkCompletedCount:  s.walkCompletedCount - prev.walkCompletedCount,
		walkFailedCount:     s.walkFailedCount - prev.walkFailedCount,

		pwcLookupCount: s.pwcLookupCount - prev.pwcLookupCount,
		pwcHitCount:    s.pwcHitCount - prev.pwcHitCount,
		pwcMissCount:   s.pwcMissCount - prev.pwcMissCount,
		pwcHitLvl0:     s.pwcHitLvl0 - prev.pwcHitLvl0,
		pwcHitLvl1:     s.pwcHitLvl1 - prev.pwcHitLvl1,
		pwcHitLvl2:     s.pwcHitLvl2 - prev.pwcHitLvl2,

		pteReadCount:       s.pteReadCount - prev.pteReadCount,
		pteReadLocalCount:  s.pteReadLocalCount - prev.pteReadLocalCount,
		pteReadRemoteCount: s.pteReadRemoteCount - prev.pteReadRemoteCount,

		walkLatencySumPS: s.walkLatencySumPS - prev.walkLatencySumPS,
		walkLatencyCount: s.walkLatencyCount - prev.walkLatencyCount,
	}
}

// ptwStatsCounter collects PTW/PWC stats by sampling the PTW components.
type ptwStatsCounter struct {
	ptws []*ptw.Comp
}

func newPTWStatsCounter(ptws []*ptw.Comp) *ptwStatsCounter {
	return &ptwStatsCounter{ptws: ptws}
}

func (c *ptwStatsCounter) Snapshot() ptwStatsSnapshot {
	var s ptwStatsSnapshot
	for _, p := range c.ptws {
		st := p.Stats()

		s.translationReqCount += st.TranslationReqCount
		s.walkCompletedCount += st.WalkCompletedCount
		s.walkFailedCount += st.WalkFailedCount

		s.pwcLookupCount += st.PWCLookupCount
		s.pwcHitCount += st.PWCHitCount
		s.pwcMissCount += st.PWCMissCount
		s.pwcHitLvl0 += st.PWCHitLevel0
		s.pwcHitLvl1 += st.PWCHitLevel1
		s.pwcHitLvl2 += st.PWCHitLevel2

		s.pteReadCount += st.PTEReadCount
		s.pteReadLocalCount += st.PTEReadLocalCount
		s.pteReadRemoteCount += st.PTEReadRemoteCount

		s.walkLatencySumPS += st.WalkLatencySumPS
		s.walkLatencyCount += st.WalkLatencyCount
	}
	return s
}

func (c *ptwStatsCounter) PerPTWStats() []ptw.Stats {
	out := make([]ptw.Stats, 0, len(c.ptws))
	for _, p := range c.ptws {
		out = append(out, p.Stats())
	}
	return out
}
