package dispatching

import (
	"github.com/sarchlab/akita/v4/monitoring"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/mgpusim/v4/amd/kernels"
	"github.com/sarchlab/mgpusim/v4/amd/protocol"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cp/internal/resource"
)

// A Builder can build dispatchers
type Builder struct {
	cp              tracing.NamedHookable
	cuResourcePool  resource.CUResourcePool
	alg             string
	respondingPort  sim.Port
	dispatchingPort sim.Port
	monitor         *monitoring.Monitor

	// Chiplet-aware scheduling configuration (used by LASP/hybrid).
	defaultKernelSchedulingPolicy string
	schedPartition                string
	numChiplets                   int
}

// MakeBuilder creates a builder with default dispatching configurations.
func MakeBuilder() Builder {
	b := Builder{
		alg:                           "partition",
		defaultKernelSchedulingPolicy: "round-robin",
		schedPartition:                "Xdiv",
		numChiplets:                   1,
	}
	return b
}

// WithCP sets the Command Processor that the Dispatcher belongs to.
func (b Builder) WithCP(cp tracing.NamedHookable) Builder {
	b.cp = cp
	return b
}

// WithCUResourcePool sets the CU resource pool. It has to be given form
// outside, as all the dispatchers share the same CU resource pool.
func (b Builder) WithCUResourcePool(pool resource.CUResourcePool) Builder {
	b.cuResourcePool = pool
	return b
}

// WithRespondingPort sets the port that the dispatcher can send WFCompleteMsg
// to.
func (b Builder) WithRespondingPort(p sim.Port) Builder {
	b.respondingPort = p
	return b
}

// WithDispatchingPort sets the port that connects to the Compute Units.
func (b Builder) WithDispatchingPort(p sim.Port) Builder {
	b.dispatchingPort = p
	return b
}

// WithAlg sets the dispatching algorithm.
func (b Builder) WithAlg(alg string) Builder {
	switch alg {
	case "round-robin", "greedy", "partition", "lasp", "hybrid":
		b.alg = alg
	default:
		panic("unknown dispatching algorithm " + alg)
	}

	return b
}

// WithMonitor sets the monitor that manages progress bars.
func (b Builder) WithMonitor(monitor *monitoring.Monitor) Builder {
	b.monitor = monitor
	return b
}

// WithDefaultKernelSchedulingPolicy sets the default per-kernel scheduling
// policy used by the "hybrid" algorithm when a kernel does not provide an
// explicit hint.
func (b Builder) WithDefaultKernelSchedulingPolicy(policy string) Builder {
	b.defaultKernelSchedulingPolicy = policy
	return b
}

// WithSchedPartition sets the default partition parameter for LASP scheduling.
func (b Builder) WithSchedPartition(partition string) Builder {
	b.schedPartition = partition
	return b
}

// WithNumChiplets sets the number of chiplets in the GPU.
func (b Builder) WithNumChiplets(n int) Builder {
	b.numChiplets = n
	return b
}

// Build creates a dispatcher.
func (b Builder) Build(name string) Dispatcher {
	d := &DispatcherImpl{
		name:            name,
		cp:              b.cp,
		respondingPort:  b.respondingPort,
		dispatchingPort: b.dispatchingPort,
		inflightWGs:     make(map[string]dispatchLocation),
		originalReqs:    make(map[string]*protocol.MapWGReq),
		latencyTable: []int{
			1,
			4, 4, 4, 4,
			5, 6, 7, 8,
			9, 10, 11, 12,
			13, 14, 15, 16,
		},
		constantKernelOverhead: 0,
		monitor:                b.monitor,
	}

	switch b.alg {
	case "round-robin":
		d.alg = &roundRobinAlgorithm{
			gridBuilder: kernels.NewGridBuilder(),
			cuPool:      b.cuResourcePool,
		}
	case "greedy":
		d.alg = &greedyAlgorithm{
			gridBuilder: kernels.NewGridBuilder(),
			cuPool:      b.cuResourcePool,
		}
	case "partition":
		d.alg = &partitionAlgorithm{
			cuPool: b.cuResourcePool,
		}
	case "lasp":
		d.alg = &chipletLASPAlgorithm{
			gridBuilder: kernels.NewGridBuilder(),
			cuPool:      b.cuResourcePool,
			numChiplets: b.numChiplets,
			partition:   b.schedPartition,
		}
	case "hybrid":
		d.alg = newHybridAlgorithm(
			kernels.NewGridBuilder(),
			b.cuResourcePool,
			b.defaultKernelSchedulingPolicy,
			b.schedPartition,
			b.numChiplets,
		)
	default:
		panic("unknown dispatching algorithm " + b.alg)
	}

	return d
}
