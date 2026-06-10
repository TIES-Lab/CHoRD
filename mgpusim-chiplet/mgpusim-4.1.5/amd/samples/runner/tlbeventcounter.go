package runner

import "sync/atomic"

// tlbEventCounter tracks TLB hit/miss/mshr-hit events.
//
// The TLB components in Akita add task steps "hit", "miss", and
// "mshr-hit" (see akita/mem/vm/tlb/tlbMiddleware.go).
type tlbEventCounter struct {
	hit     uint64
	miss    uint64
	mshrHit uint64
}

type tlbEventSnapshot struct {
	hit     uint64
	miss    uint64
	mshrHit uint64
}

func (c *tlbEventCounter) Snapshot() tlbEventSnapshot {
	if c == nil {
		return tlbEventSnapshot{}
	}
	return tlbEventSnapshot{
		hit:     atomic.LoadUint64(&c.hit),
		miss:    atomic.LoadUint64(&c.miss),
		mshrHit: atomic.LoadUint64(&c.mshrHit),
	}
}

func (s tlbEventSnapshot) Sub(prev tlbEventSnapshot) tlbEventSnapshot {
	sub := func(a, b uint64) uint64 {
		if a >= b {
			return a - b
		}
		return 0
	}
	return tlbEventSnapshot{
		hit:     sub(s.hit, prev.hit),
		miss:    sub(s.miss, prev.miss),
		mshrHit: sub(s.mshrHit, prev.mshrHit),
	}
}

// tlbEventGroups aggregates counters for different TLB levels/types.
//
// This mirrors the cache grouping used for cache hit/miss, enabling per-kernel
// deltas for each TLB type.
type tlbEventGroups struct {
	L1V tlbEventCounter
	L1I tlbEventCounter
	L1S tlbEventCounter
	L2  tlbEventCounter
}

type tlbEventGroupsSnapshot struct {
	L1V tlbEventSnapshot
	L1I tlbEventSnapshot
	L1S tlbEventSnapshot
	L2  tlbEventSnapshot
}

func (g *tlbEventGroups) Snapshot() tlbEventGroupsSnapshot {
	if g == nil {
		return tlbEventGroupsSnapshot{}
	}
	return tlbEventGroupsSnapshot{
		L1V: g.L1V.Snapshot(),
		L1I: g.L1I.Snapshot(),
		L1S: g.L1S.Snapshot(),
		L2:  g.L2.Snapshot(),
	}
}

func (s tlbEventGroupsSnapshot) Sub(prev tlbEventGroupsSnapshot) tlbEventGroupsSnapshot {
	return tlbEventGroupsSnapshot{
		L1V: s.L1V.Sub(prev.L1V),
		L1I: s.L1I.Sub(prev.L1I),
		L1S: s.L1S.Sub(prev.L1S),
		L2:  s.L2.Sub(prev.L2),
	}
}
