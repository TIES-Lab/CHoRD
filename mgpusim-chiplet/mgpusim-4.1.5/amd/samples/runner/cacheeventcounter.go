package runner

import "sync/atomic"

// cacheEventCounter tracks cache hit/miss events using a fixed set of counters.
// It is designed to be lightweight so we can snapshot it at kernel boundaries
// and compute per-kernel deltas.
type cacheEventCounter struct {
	readHit     uint64
	readMiss    uint64
	readMSHRHit uint64

	writeHit     uint64
	writeMiss    uint64
	writeMSHRHit uint64
}

type cacheEventSnapshot struct {
	readHit     uint64
	readMiss    uint64
	readMSHRHit uint64

	writeHit     uint64
	writeMiss    uint64
	writeMSHRHit uint64
}

func (c *cacheEventCounter) Snapshot() cacheEventSnapshot {
	if c == nil {
		return cacheEventSnapshot{}
	}
	return cacheEventSnapshot{
		readHit:      atomic.LoadUint64(&c.readHit),
		readMiss:     atomic.LoadUint64(&c.readMiss),
		readMSHRHit:  atomic.LoadUint64(&c.readMSHRHit),
		writeHit:     atomic.LoadUint64(&c.writeHit),
		writeMiss:    atomic.LoadUint64(&c.writeMiss),
		writeMSHRHit: atomic.LoadUint64(&c.writeMSHRHit),
	}
}

func (s cacheEventSnapshot) Sub(prev cacheEventSnapshot) cacheEventSnapshot {
	sub := func(a, b uint64) uint64 {
		if a >= b {
			return a - b
		}
		return 0
	}
	return cacheEventSnapshot{
		readHit:      sub(s.readHit, prev.readHit),
		readMiss:     sub(s.readMiss, prev.readMiss),
		readMSHRHit:  sub(s.readMSHRHit, prev.readMSHRHit),
		writeHit:     sub(s.writeHit, prev.writeHit),
		writeMiss:    sub(s.writeMiss, prev.writeMiss),
		writeMSHRHit: sub(s.writeMSHRHit, prev.writeMSHRHit),
	}
}

// cacheEventGroups provides aggregated cache counters per cache level.
type cacheEventGroups struct {
	L1V cacheEventCounter
	L1I cacheEventCounter
	L1S cacheEventCounter
	L2  cacheEventCounter
}

type cacheEventGroupsSnapshot struct {
	L1V cacheEventSnapshot
	L1I cacheEventSnapshot
	L1S cacheEventSnapshot
	L2  cacheEventSnapshot
}

func newCacheEventGroups() *cacheEventGroups {
	return &cacheEventGroups{}
}

func (g *cacheEventGroups) Snapshot() cacheEventGroupsSnapshot {
	if g == nil {
		return cacheEventGroupsSnapshot{}
	}
	return cacheEventGroupsSnapshot{
		L1V: g.L1V.Snapshot(),
		L1I: g.L1I.Snapshot(),
		L1S: g.L1S.Snapshot(),
		L2:  g.L2.Snapshot(),
	}
}

func (s cacheEventGroupsSnapshot) Sub(prev cacheEventGroupsSnapshot) cacheEventGroupsSnapshot {
	return cacheEventGroupsSnapshot{
		L1V: s.L1V.Sub(prev.L1V),
		L1I: s.L1I.Sub(prev.L1I),
		L1S: s.L1S.Sub(prev.L1S),
		L2:  s.L2.Sub(prev.L2),
	}
}
