package runner

import (
	"sync/atomic"

	"github.com/sarchlab/akita/v4/sim"
)

// rdmaTrafficCounter counts the number of messages and bytes that cross
// chiplet RDMA boundaries.
//
// We use sim.Port hooks (send/recv) to count actual message traffic bytes
// (msg.Meta().TrafficBytes).
//
// Note: These counters are not limited to a specific message type. In the
// current design, the RDMA ports only carry memory access requests and
// responses, so counting all messages is acceptable.
type rdmaTrafficCounter struct {
	// NoP traffic: messages sent/received on RDMA *Outside* ports.
	nopSendPackets uint64
	nopSendBytes   uint64
	nopRecvPackets uint64
	nopRecvBytes   uint64

	// NoC traffic at RDMA boundary: messages sent/received on RDMA *Inside* ports.
	nocSendPackets uint64
	nocSendBytes   uint64
	nocRecvPackets uint64
	nocRecvBytes   uint64

	// RDMA request traffic (subset of NoP send): only requests (sent on
	// RDMARequestOutside).
	rdmaReqSendPackets uint64
	rdmaReqSendBytes   uint64
}

type rdmaTrafficSnapshot struct {
	nopSendPackets uint64
	nopSendBytes   uint64
	nopRecvPackets uint64
	nopRecvBytes   uint64

	nocSendPackets uint64
	nocSendBytes   uint64
	nocRecvPackets uint64
	nocRecvBytes   uint64

	rdmaReqSendPackets uint64
	rdmaReqSendBytes   uint64
}

func newRDMATrafficCounter() *rdmaTrafficCounter {
	return &rdmaTrafficCounter{}
}

func (c *rdmaTrafficCounter) Snapshot() rdmaTrafficSnapshot {
	return rdmaTrafficSnapshot{
		nopSendPackets: atomic.LoadUint64(&c.nopSendPackets),
		nopSendBytes:   atomic.LoadUint64(&c.nopSendBytes),
		nopRecvPackets: atomic.LoadUint64(&c.nopRecvPackets),
		nopRecvBytes:   atomic.LoadUint64(&c.nopRecvBytes),

		nocSendPackets: atomic.LoadUint64(&c.nocSendPackets),
		nocSendBytes:   atomic.LoadUint64(&c.nocSendBytes),
		nocRecvPackets: atomic.LoadUint64(&c.nocRecvPackets),
		nocRecvBytes:   atomic.LoadUint64(&c.nocRecvBytes),

		rdmaReqSendPackets: atomic.LoadUint64(&c.rdmaReqSendPackets),
		rdmaReqSendBytes:   atomic.LoadUint64(&c.rdmaReqSendBytes),
	}
}

func (s rdmaTrafficSnapshot) Sub(prev rdmaTrafficSnapshot) rdmaTrafficSnapshot {
	sub := func(a, b uint64) uint64 {
		if a >= b {
			return a - b
		}
		return 0
	}
	return rdmaTrafficSnapshot{
		nopSendPackets: sub(s.nopSendPackets, prev.nopSendPackets),
		nopSendBytes:   sub(s.nopSendBytes, prev.nopSendBytes),
		nopRecvPackets: sub(s.nopRecvPackets, prev.nopRecvPackets),
		nopRecvBytes:   sub(s.nopRecvBytes, prev.nopRecvBytes),

		nocSendPackets: sub(s.nocSendPackets, prev.nocSendPackets),
		nocSendBytes:   sub(s.nocSendBytes, prev.nocSendBytes),
		nocRecvPackets: sub(s.nocRecvPackets, prev.nocRecvPackets),
		nocRecvBytes:   sub(s.nocRecvBytes, prev.nocRecvBytes),

		rdmaReqSendPackets: sub(s.rdmaReqSendPackets, prev.rdmaReqSendPackets),
		rdmaReqSendBytes:   sub(s.rdmaReqSendBytes, prev.rdmaReqSendBytes),
	}
}

// msgCounterHook increments a (packets, bytes) pair when a hook position is
// triggered.
type msgCounterHook struct {
	pos     *sim.HookPos
	packets *uint64
	bytes   *uint64
}

func (h *msgCounterHook) Func(ctx sim.HookCtx) {
	if h == nil || h.pos == nil || h.packets == nil || h.bytes == nil {
		return
	}
	if ctx.Pos != h.pos {
		return
	}
	msg, ok := ctx.Item.(sim.Msg)
	if !ok || msg == nil {
		return
	}

	atomic.AddUint64(h.packets, 1)
	atomic.AddUint64(h.bytes, uint64(msg.Meta().TrafficBytes))
}
