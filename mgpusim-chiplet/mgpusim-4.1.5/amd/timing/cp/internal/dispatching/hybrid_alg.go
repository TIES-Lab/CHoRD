package dispatching

import (
	"strings"

	"github.com/sarchlab/mgpusim/v4/amd/kernels"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cp/internal/resource"
)

// hybridAlgorithm selects a scheduling algorithm per kernel based on
// KernelLaunchInfo hints, falling back to a default policy.
//
// Currently supported policies:
//   - "round-robin" / "rr"
//   - "lasp" (intra-chiplet CU RR)
//   - "lasp-blockcu" (intra-chiplet CU block mapping)
type hybridAlgorithm struct {
	defaultPolicy    string
	defaultPartition string
	numChiplets      int

	cuPool      resource.CUResourcePool
	gridBuilder kernels.GridBuilder

	rr   *chipletRoundRobinAlgorithm
	lasp *chipletLASPAlgorithm

	active algorithm
}

func newHybridAlgorithm(
	gridBuilder kernels.GridBuilder,
	cuPool resource.CUResourcePool,
	defaultPolicy string,
	defaultPartition string,
	numChiplets int,
) algorithm {
	h := &hybridAlgorithm{
		defaultPolicy:    defaultPolicy,
		defaultPartition: defaultPartition,
		numChiplets:      numChiplets,
		cuPool:           cuPool,
		gridBuilder:      gridBuilder,
	}

	if h.defaultPolicy == "" {
		h.defaultPolicy = "round-robin"
	}
	if h.defaultPartition == "" {
		h.defaultPartition = "Xdiv"
	}
	if h.numChiplets <= 0 {
		h.numChiplets = 1
	}

	h.rr = &chipletRoundRobinAlgorithm{
		gridBuilder: gridBuilder,
		cuPool:      cuPool,
		numChiplets: h.numChiplets,
	}

	h.lasp = &chipletLASPAlgorithm{
		gridBuilder: gridBuilder,
		cuPool:      cuPool,
		numChiplets: h.numChiplets,
		partition:   h.defaultPartition,
	}

	h.active = h.rr
	return h
}

func (h *hybridAlgorithm) RegisterCU(cu resource.DispatchableCU) {
	h.cuPool.RegisterCU(cu)
}

func (h *hybridAlgorithm) StartNewKernel(info kernels.KernelLaunchInfo) {
	// Pick a policy from the per-kernel hint; otherwise use the CLI default.
	//
	// Note: When a plan provides an invalid scheduling policy, we fall back to
	// round-robin instead of crashing.
	policyHint := strings.ToLower(strings.TrimSpace(info.SchedulingPolicy))
	hasHint := policyHint != ""

	policy := policyHint
	if policy == "" {
		policy = strings.ToLower(strings.TrimSpace(h.defaultPolicy))
	}
	if policy == "" {
		policy = "round-robin"
	}
	// Normalize synonyms.
	cuPolicy := ""
	switch policy {
	case "rr", "roundrobin", "round-robin":
		policy = "round-robin"
	case "lasp", "lasp-rr", "lasp_rr":
		policy = "lasp"
		cuPolicy = "rr"
	case "lasp-block", "lasp-blockcu", "lasp_block", "lasp_blockcu":
		policy = "lasp"
		cuPolicy = "block"
	default:
		// Unknown policy.
		if hasHint {
			// Invalid plan hint -> fall back to round-robin.
			policy = "round-robin"
		} else {
			// Unknown CLI default -> use round-robin.
			policy = "round-robin"
		}
	}
	if policy == "lasp" && cuPolicy == "" {
		cuPolicy = "rr"
	}

	partition := strings.TrimSpace(info.SchedPartition)
	if partition == "" {
		partition = strings.TrimSpace(h.defaultPartition)
	}
	if partition == "" {
		partition = "Xdiv"
	}
	if policy == "lasp" && !isValidLASPPartition(partition) {
		// Invalid partition -> use default instead of crashing.
		partition = strings.TrimSpace(h.defaultPartition)
		if partition == "" {
			partition = "Xdiv"
		}
	}

	if policy == "lasp" {
		h.lasp.numChiplets = h.numChiplets
		h.lasp.partition = partition
		h.lasp.cuPolicy = cuPolicy
		h.active = h.lasp
	} else {
		h.active = h.rr
	}

	h.active.StartNewKernel(info)
}

func isValidLASPPartition(partition string) bool {
	p := strings.ToLower(strings.TrimSpace(partition))
	return strings.HasPrefix(p, "xdiv") ||
		strings.HasPrefix(p, "ydiv") ||
		strings.HasPrefix(p, "xblk") ||
		strings.HasPrefix(p, "yblk")
}

func (h *hybridAlgorithm) NumWG() int {
	return h.active.NumWG()
}

func (h *hybridAlgorithm) HasNext() bool {
	return h.active.HasNext()
}

func (h *hybridAlgorithm) Next() dispatchLocation {
	return h.active.Next()
}

func (h *hybridAlgorithm) FreeResources(location dispatchLocation) {
	h.active.FreeResources(location)
}
