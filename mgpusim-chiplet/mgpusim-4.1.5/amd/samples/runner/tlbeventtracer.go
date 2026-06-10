package runner

import (
	"sync/atomic"

	"github.com/sarchlab/akita/v4/tracing"
)

// tlbEventTracer increments a tlbEventCounter when it observes task steps.
//
// Akita's TLB middleware adds task steps with What values:
//   - "hit"
//   - "miss"
//   - "mshr-hit"
//
// This tracer is lightweight and intended for per-kernel delta accounting.
type tlbEventTracer struct {
	counter *tlbEventCounter
}

func (t *tlbEventTracer) StartTask(_ tracing.Task) {}

func (t *tlbEventTracer) StepTask(task tracing.Task) {
	if t == nil || t.counter == nil {
		return
	}
	if len(task.Steps) == 0 {
		return
	}
	step := task.Steps[0].What
	switch step {
	case "hit":
		atomic.AddUint64(&t.counter.hit, 1)
	case "miss":
		atomic.AddUint64(&t.counter.miss, 1)
	case "mshr-hit":
		atomic.AddUint64(&t.counter.mshrHit, 1)
	}
}

func (t *tlbEventTracer) AddMilestone(_ tracing.Milestone) {}

func (t *tlbEventTracer) EndTask(_ tracing.Task) {}
