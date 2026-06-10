package runner

import (
	"sync/atomic"

	"github.com/sarchlab/akita/v4/tracing"
)

// cacheEventTracer increments cacheEventCounter for cache hit/miss events.
// It can also optionally increment a group counter (e.g., aggregate L1V/L2 totals)
// by sharing the same *cacheEventCounter pointer.
type cacheEventTracer struct {
	counter *cacheEventCounter
	group   *cacheEventCounter
}

func newCacheEventTracer(counter *cacheEventCounter, group *cacheEventCounter) *cacheEventTracer {
	return &cacheEventTracer{
		counter: counter,
		group:   group,
	}
}

// StartTask is unused for counting steps.
func (t *cacheEventTracer) StartTask(task tracing.Task) {}

// StepTask counts the cache pipeline step events.
func (t *cacheEventTracer) StepTask(task tracing.Task) {
	if t.counter == nil && t.group == nil {
		return
	}

	if len(task.Steps) == 0 {
		return
	}

	step := task.Steps[0]
	switch step.What {
		case "read-hit":
			if t.counter != nil {
				atomic.AddUint64(&t.counter.readHit, 1)
			}
			if t.group != nil {
				atomic.AddUint64(&t.group.readHit, 1)
			}
		case "read-miss":
			if t.counter != nil {
				atomic.AddUint64(&t.counter.readMiss, 1)
			}
			if t.group != nil {
				atomic.AddUint64(&t.group.readMiss, 1)
			}
		case "read-mshr-hit":
			if t.counter != nil {
				atomic.AddUint64(&t.counter.readMSHRHit, 1)
			}
			if t.group != nil {
				atomic.AddUint64(&t.group.readMSHRHit, 1)
			}
		case "write-hit":
			if t.counter != nil {
				atomic.AddUint64(&t.counter.writeHit, 1)
			}
			if t.group != nil {
				atomic.AddUint64(&t.group.writeHit, 1)
			}
		case "write-miss":
			if t.counter != nil {
				atomic.AddUint64(&t.counter.writeMiss, 1)
			}
			if t.group != nil {
				atomic.AddUint64(&t.group.writeMiss, 1)
			}
		case "write-mshr-hit":
			if t.counter != nil {
				atomic.AddUint64(&t.counter.writeMSHRHit, 1)
			}
			if t.group != nil {
				atomic.AddUint64(&t.group.writeMSHRHit, 1)
			}
		default:
			// Ignore other steps. This is intentionally strict to avoid
			// creating unbounded step name maps (like StepCountTracer does).
		}
}

// AddMilestone is unused.
func (t *cacheEventTracer) AddMilestone(milestone tracing.Milestone) {}

// EndTask is unused.
func (t *cacheEventTracer) EndTask(task tracing.Task) {}
