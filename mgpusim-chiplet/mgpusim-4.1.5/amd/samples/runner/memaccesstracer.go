package runner

import (
	"strings"
	"sync/atomic"

	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/tracing"
)

const (
	// The plan/ILP objective uses fixed byte costs per access:
	//   load : req 16B + rsp 64B
	//   store: req 16B + rsp 16B
	estReadReqBytes  uint64 = 16
	estReadRspBytes  uint64 = 64
	estWriteReqBytes uint64 = 16
	estWriteRspBytes uint64 = 16

	estReadTotalBytes  uint64 = estReadReqBytes + estReadRspBytes   // 80B
	estWriteTotalBytes uint64 = estWriteReqBytes + estWriteRspBytes // 32B
)

type memAccessCounter struct {
	localRead  uint64
	localWrite uint64

	remoteRead  uint64
	remoteWrite uint64

	externalRead  uint64
	externalWrite uint64

	localBytes    uint64
	remoteBytes   uint64
	externalBytes uint64
}

type memAccessSnapshot struct {
	localRead  uint64
	localWrite uint64

	remoteRead  uint64
	remoteWrite uint64

	externalRead  uint64
	externalWrite uint64

	localBytes    uint64
	remoteBytes   uint64
	externalBytes uint64
}

func newMemAccessCounter() *memAccessCounter {
	return &memAccessCounter{}
}

func (c *memAccessCounter) Snapshot() memAccessSnapshot {
	return memAccessSnapshot{
		localRead:     atomic.LoadUint64(&c.localRead),
		localWrite:    atomic.LoadUint64(&c.localWrite),
		remoteRead:    atomic.LoadUint64(&c.remoteRead),
		remoteWrite:   atomic.LoadUint64(&c.remoteWrite),
		externalRead:  atomic.LoadUint64(&c.externalRead),
		externalWrite: atomic.LoadUint64(&c.externalWrite),
		localBytes:    atomic.LoadUint64(&c.localBytes),
		remoteBytes:   atomic.LoadUint64(&c.remoteBytes),
		externalBytes: atomic.LoadUint64(&c.externalBytes),
	}
}

func (s memAccessSnapshot) Sub(prev memAccessSnapshot) memAccessSnapshot {
	delta := memAccessSnapshot{}

	// Guard against underflow just in case (counters should be monotonic).
	sub := func(a, b uint64) uint64 {
		if a >= b {
			return a - b
		}
		return 0
	}

	delta.localRead = sub(s.localRead, prev.localRead)
	delta.localWrite = sub(s.localWrite, prev.localWrite)

	delta.remoteRead = sub(s.remoteRead, prev.remoteRead)
	delta.remoteWrite = sub(s.remoteWrite, prev.remoteWrite)

	delta.externalRead = sub(s.externalRead, prev.externalRead)
	delta.externalWrite = sub(s.externalWrite, prev.externalWrite)

	delta.localBytes = sub(s.localBytes, prev.localBytes)
	delta.remoteBytes = sub(s.remoteBytes, prev.remoteBytes)
	delta.externalBytes = sub(s.externalBytes, prev.externalBytes)

	return delta
}

func (c *memAccessCounter) addLocal(msg sim.Msg) {
	if isReadReq(msg) {
		atomic.AddUint64(&c.localRead, 1)
		atomic.AddUint64(&c.localBytes, estReadTotalBytes)
		return
	}
	if isWriteReq(msg) {
		atomic.AddUint64(&c.localWrite, 1)
		atomic.AddUint64(&c.localBytes, estWriteTotalBytes)
		return
	}
}

func (c *memAccessCounter) addRemote(msg sim.Msg) {
	if isReadReq(msg) {
		atomic.AddUint64(&c.remoteRead, 1)
		atomic.AddUint64(&c.remoteBytes, estReadTotalBytes)
		return
	}
	if isWriteReq(msg) {
		atomic.AddUint64(&c.remoteWrite, 1)
		atomic.AddUint64(&c.remoteBytes, estWriteTotalBytes)
		return
	}
}

func (c *memAccessCounter) addExternal(msg sim.Msg) {
	if isReadReq(msg) {
		atomic.AddUint64(&c.externalRead, 1)
		atomic.AddUint64(&c.externalBytes, estReadTotalBytes)
		return
	}
	if isWriteReq(msg) {
		atomic.AddUint64(&c.externalWrite, 1)
		atomic.AddUint64(&c.externalBytes, estWriteTotalBytes)
		return
	}
}

func isReadReq(msg sim.Msg) bool {
	_, ok := msg.(*mem.ReadReq)
	return ok
}

func isWriteReq(msg sim.Msg) bool {
	_, ok := msg.(*mem.WriteReq)
	return ok
}

// isChipletRDMA tries to detect whether a port name belongs to an intra-GPU
// chiplet RDMA instance.
//
// We keep this as a heuristic because different branches use slightly different
// naming schemes:
//   - MGVM (v1.x):  "GPU0.Chiplet[0].RDMA"
//   - This port (v4.x): "GPU0.chiplet_00.RDMA" (preferred)
// The external (inter-GPU / CPU) RDMA is typically named "GPU0.RDMA" and does
// not contain "Chiplet" or "chiplet_".
func isChipletRDMA(src string) bool {
	if strings.Contains(src, ".Chiplet[") {
		return true
	}
	if strings.Contains(src, ".chiplet_") {
		return true
	}
	// A permissive fallback for other naming variants.
	if strings.Contains(src, ".Chiplet") && strings.Contains(src, ".RDMA") {
		return true
	}
	return false
}

// l2AccessTracer counts ReadReq/WriteReq arriving at each L2 cache.
// It classifies requests as:
//   - local    : from local (non-RDMA) sources (typically shader arrays / L1 caches)
//   - remote   : from an intra-GPU chiplet RDMA (remote bank access)
//   - external : from the (GPU-level) RDMA engine (CPU / other GPUs)
//
// This matches the chiplet model wiring:
//   local  -> direct L1->L2 connection
//   remote -> inter-chiplet network -> chiplet RDMA -> L2
//   external -> PCIe/NoC -> GPU RDMA -> L2
type l2AccessTracer struct {
	counter *memAccessCounter
}

func (t *l2AccessTracer) StartTask(task tracing.Task) {
	if t.counter == nil {
		return
	}
	if task.Kind != "req_in" {
		return
	}

	msg, ok := task.Detail.(sim.Msg)
	if !ok {
		return
	}

	// Only count memory access requests.
	if !isReadReq(msg) && !isWriteReq(msg) {
		return
	}

	src := string(msg.Meta().Src)
	fromRDMADataInside := strings.Contains(src, ".RDMADataInside")
	fromChipletRDMA := fromRDMADataInside && isChipletRDMA(src)
	fromExternalRDMA := fromRDMADataInside && !isChipletRDMA(src)

	switch {
	case fromChipletRDMA:
		t.counter.addRemote(msg)
	case fromExternalRDMA:
		t.counter.addExternal(msg)
	default:
		t.counter.addLocal(msg)
	}
}

func (t *l2AccessTracer) StepTask(task tracing.Task) {}

func (t *l2AccessTracer) AddMilestone(milestone tracing.Milestone) {}

func (t *l2AccessTracer) EndTask(task tracing.Task) {}
