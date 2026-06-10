package runner

import (
	"sync/atomic"

	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/tracing"
)

// dramAccessCounter tracks the total number of requests that reach any DRAM
// controller (i.e., the traffic that actually goes to memory after caching).
//
// This is intentionally lightweight (atomic counters) so that it can be
// snapshotted at kernel boundaries.
type dramAccessCounter struct {
	readReq    uint64
	writeReq   uint64
	readBytes  uint64
	writeBytes uint64
}

type dramAccessSnapshot struct {
	readReq    uint64
	writeReq   uint64
	readBytes  uint64
	writeBytes uint64
}

func newDramAccessCounter() *dramAccessCounter {
	return &dramAccessCounter{}
}

func (c *dramAccessCounter) Snapshot() dramAccessSnapshot {
	return dramAccessSnapshot{
		readReq:    atomic.LoadUint64(&c.readReq),
		writeReq:   atomic.LoadUint64(&c.writeReq),
		readBytes:  atomic.LoadUint64(&c.readBytes),
		writeBytes: atomic.LoadUint64(&c.writeBytes),
	}
}

func (s dramAccessSnapshot) Sub(prev dramAccessSnapshot) dramAccessSnapshot {
	sub := func(a, b uint64) uint64 {
		if a >= b {
			return a - b
		}
		return 0
	}
	return dramAccessSnapshot{
		readReq:    sub(s.readReq, prev.readReq),
		writeReq:   sub(s.writeReq, prev.writeReq),
		readBytes:  sub(s.readBytes, prev.readBytes),
		writeBytes: sub(s.writeBytes, prev.writeBytes),
	}
}

// dramAccessTracer increments dramAccessCounter when DRAM receives ReadReq/WriteReq.
type dramAccessTracer struct {
	counter *dramAccessCounter
}

func (t *dramAccessTracer) StartTask(task tracing.Task) {
	if t.counter == nil {
		return
	}

	// DRAM components in Akita may use different task.Kind values for reads/writes.
	// For correctness, follow the same "What/Detail type" convention as the
	// original dramTracer in report.go and count based on request type.
	switch req := task.Detail.(type) {
	case *mem.ReadReq:
		atomic.AddUint64(&t.counter.readReq, 1)
		atomic.AddUint64(&t.counter.readBytes, req.AccessByteSize)
	case *mem.WriteReq:
		atomic.AddUint64(&t.counter.writeReq, 1)
		// In Akita v4, WriteReq does not expose AccessByteSize. Use the write
		// payload size. If the payload is empty (rare), fall back to the dirty mask
		// length (one bit per byte) as a best-effort estimate.
		sz := uint64(len(req.Data))
		if sz == 0 && len(req.DirtyMask) > 0 {
			sz = uint64(len(req.DirtyMask))
		}
		atomic.AddUint64(&t.counter.writeBytes, sz)
	default:
		// Ignore other task types.
	}
}

func (t *dramAccessTracer) StepTask(task tracing.Task) {}

func (t *dramAccessTracer) AddMilestone(milestone tracing.Milestone) {}

func (t *dramAccessTracer) EndTask(task tracing.Task) {}
