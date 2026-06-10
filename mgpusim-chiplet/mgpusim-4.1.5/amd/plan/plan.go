// Package plan provides a lightweight JSON plan format to control
// (1) per-buffer page placement,
// (2) per-kernel scheduling hints, and
// (3) per-program-launch execution / overlap hints.
//
// This package is intentionally small and forward-compatible. Unknown JSON
// fields are ignored by default JSON unmarshalling.
package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Plan is the top-level plan structure.
//
// A plan file can specify:
//   - Hardware parameters (for reporting / cross-checking only).
//   - Per-kernel scheduling policy hints.
//   - Per-buffer page placement (per page -> chiplet mapping).
//   - Per-program-launch execution hints used by the simulator-side launch
//     manager to overlap, reorder, and workgroup-chunk a launch.
//
// Unknown fields in the JSON are ignored.
type Plan struct {
	Tool                  string   `json:"tool"`
	Version               int      `json:"version"`
	Objective             string   `json:"objective"`
	Hardware              Hardware `json:"hardware"`
	DefaultSchedPartition string   `json:"default_sched_partition"`

	// LaunchSequenceLength is the number of host-visible LaunchKernel calls in a
	// repeated benchmark sequence.
	LaunchSequenceLength int `json:"launch_sequence_length,omitempty"`

	// DispatchSequenceLength is the number of internal dispatch units in one
	// repeated sequence. For simple plans this equals LaunchSequenceLength. It is
	// larger when a program launch is intended to be workgroup-chunked.
	DispatchSequenceLength int `json:"dispatch_sequence_length,omitempty"`

	// LaunchHints are execution hints keyed by ProgramLaunchID. Multiple hints may
	// refer to the same ProgramLaunchID when the launch is expected to be
	// adaptively chunked / re-entered by the simulator.
	LaunchHints []LaunchHint `json:"launch_hints,omitempty"`

	KernelScheduling []KernelSchedulingEntry    `json:"kernel_scheduling"`
	BufferPlacement  map[string]BufferPlacement `json:"buffer_placement"`
}

// Hardware describes the assumed hardware configuration in the plan.
type Hardware struct {
	Chiplets  int    `json:"chiplets"`
	PageBytes uint64 `json:"page_bytes"`
}

// KernelSchedulingEntry describes the policy hint for one kernel.
type KernelSchedulingEntry struct {
	Kernel    string `json:"kernel"`
	Policy    string `json:"policy"`
	Partition string `json:"partition"`
}

// BufferPlacement describes how pages of a buffer should be placed.
type BufferPlacement struct {
	Policy string `json:"policy"`
	Pages  []int  `json:"pages"`
}

// WGWindow describes one contiguous flattened workgroup-id window [Start, Start+Count).
type WGWindow struct {
	Start int `json:"start"`
	Count int `json:"count"`
}

// LaunchHint describes execution hints for one dispatch unit in a repeated
// benchmark sequence.
//
// Semantics:
//   - ProgramLaunchID identifies the host-visible LaunchKernel call this hint
//     belongs to.
//   - ID identifies an internal dispatch unit within the repeated sequence. A
//     simple non-chunked launch uses exactly one hint and usually has
//     ID==ProgramLaunchID.
//   - DependsOn lists dispatch-unit IDs in the same repeated sequence.
//   - AdaptiveWGChunks allows the simulator to split a launch into multiple
//     workgroup windows at runtime, using real-time free-CU capacity.
type LaunchHint struct {
	ID              int    `json:"id"`
	ProgramLaunchID int    `json:"program_launch_id,omitempty"`
	Kernel          string `json:"kernel,omitempty"`
	DependsOn       []int  `json:"depends_on,omitempty"`
	Priority        int    `json:"priority,omitempty"`
	EstCUs          int    `json:"est_cus,omitempty"`
	ChipletCUs      []int  `json:"chiplet_cus,omitempty"`
	EstDuration     int64  `json:"est_duration_ps,omitempty"`
	Policy          string `json:"policy,omitempty"`
	Partition       string `json:"partition,omitempty"`

	AdaptiveWGChunks bool `json:"adaptive_wg_chunks,omitempty"`
	MinChunkWG       int  `json:"min_chunk_wg,omitempty"`

	// Optional explicit flattened-workgroup windows for item-level dispatch.
	// When present, the simulator dispatches exactly these WG windows for this
	// item, instead of the whole launch or an adaptive prefix window.
	WGWindows []WGWindow `json:"wg_windows,omitempty"`

	// Optional safe micro-splitting factor for low-parallelism kernels.
	// The simulator may split each original work-group into multiple smaller
	// sub-work-groups that stay on the same chiplet, if the kernel uses neither
	// LDS nor work-group barriers.
	SubWGFactor int `json:"sub_wg_factor,omitempty"`
}

// KernelSchedulingHint returns the scheduling policy and partition hint for a
// kernel name.
//
// If an entry is found but the partition is empty, DefaultSchedPartition is
// returned.
func (p *Plan) KernelSchedulingHint(kernel string) (policy string, partition string, ok bool) {
	if p == nil || kernel == "" {
		return "", "", false
	}

	for _, e := range p.KernelScheduling {
		if e.Kernel == kernel {
			policy = e.Policy
			partition = e.Partition
			if partition == "" {
				partition = p.DefaultSchedPartition
			}
			return policy, partition, true
		}
	}

	return "", "", false
}

// HasLaunchHints returns true if the plan includes per-launch overlap / reorder
// / chunking hints.
func (p *Plan) HasLaunchHints() bool {
	if p == nil {
		return false
	}
	if p.LaunchSequenceLength <= 0 {
		return false
	}
	return len(p.LaunchHints) > 0
}

// DispatchSequenceLengthOrDefault returns the internal dispatch sequence length
// declared by the plan.
func (p *Plan) DispatchSequenceLengthOrDefault(defaultVal int) int {
	if p == nil {
		return defaultVal
	}
	if p.DispatchSequenceLength > 0 {
		return p.DispatchSequenceLength
	}
	if p.LaunchSequenceLength > 0 {
		return p.LaunchSequenceLength
	}
	if len(p.LaunchHints) > 0 {
		maxID := 0
		for _, h := range p.LaunchHints {
			if h.ID > maxID {
				maxID = h.ID
			}
		}
		return maxID + 1
	}
	return defaultVal
}

// LaunchHintsByProgramLaunchID returns all hints attached to a relative
// host-visible program launch id.
//
// Returned hints are sorted by ID so the driver sees a stable order.
func (p *Plan) LaunchHintsByProgramLaunchID(programLaunchID int) []LaunchHint {
	if p == nil || programLaunchID < 0 {
		return nil
	}

	out := make([]LaunchHint, 0, 2)
	for _, h := range p.LaunchHints {
		pid := h.ProgramLaunchID
		if pid == 0 && h.ID >= 0 && p.DispatchSequenceLengthOrDefault(0) == p.LaunchSequenceLength {
			// Backward-compatibility with v6 plans that did not store
			// ProgramLaunchID explicitly.
			pid = h.ID
		}
		if pid == programLaunchID {
			out = append(out, h)
		}
	}

	if len(out) == 0 {
		return nil
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].ProgramLaunchID < out[j].ProgramLaunchID
	})
	return out
}

// validateLaunchHints checks that per-dispatch dependencies are well-formed and acyclic.
func (p *Plan) validateLaunchHints() error {
	if p == nil || len(p.LaunchHints) == 0 {
		return nil
	}

	ids := make(map[int]struct{}, len(p.LaunchHints))
	indeg := make(map[int]int, len(p.LaunchHints))
	succ := make(map[int][]int, len(p.LaunchHints))
	for _, h := range p.LaunchHints {
		if _, exists := ids[h.ID]; exists {
			return fmt.Errorf("duplicate launch_hints id %d", h.ID)
		}
		ids[h.ID] = struct{}{}
		indeg[h.ID] = 0
		succ[h.ID] = nil
	}

	for _, h := range p.LaunchHints {
		seen := make(map[int]struct{}, len(h.DependsOn))
		for _, dep := range h.DependsOn {
			if dep == h.ID {
				return fmt.Errorf("launch_hint id %d depends on itself", h.ID)
			}
			if _, ok := ids[dep]; !ok {
				return fmt.Errorf("launch_hint id %d depends on unknown id %d", h.ID, dep)
			}
			if _, dup := seen[dep]; dup {
				continue
			}
			seen[dep] = struct{}{}
			indeg[h.ID]++
			succ[dep] = append(succ[dep], h.ID)
		}
	}

	ready := make([]int, 0, len(ids))
	for id, deg := range indeg {
		if deg == 0 {
			ready = append(ready, id)
		}
	}
	sort.Ints(ready)
	visited := 0
	for head := 0; head < len(ready); head++ {
		cur := ready[head]
		visited++
		for _, nxt := range succ[cur] {
			indeg[nxt]--
			if indeg[nxt] == 0 {
				ready = append(ready, nxt)
			}
		}
	}
	if visited == len(ids) {
		return nil
	}

	remaining := make([]int, 0, len(ids)-visited)
	for id, deg := range indeg {
		if deg > 0 {
			remaining = append(remaining, id)
		}
	}
	sort.Ints(remaining)
	return fmt.Errorf("launch_hints dependency cycle detected; unresolved ids=%v", remaining)
}

// Load reads a JSON plan file.
func Load(path string) (*Plan, error) {
	if path == "" {
		return nil, fmt.Errorf("empty plan path")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var p Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}

	if p.BufferPlacement == nil {
		p.BufferPlacement = make(map[string]BufferPlacement)
	}

	if err := p.validateLaunchHints(); err != nil {
		return nil, err
	}

	return &p, nil
}

// NumChipletsOrDefault returns the chiplet count declared in the plan.
func (p *Plan) NumChipletsOrDefault(defaultVal int) int {
	if p == nil {
		return defaultVal
	}
	if p.Hardware.Chiplets <= 0 {
		return defaultVal
	}
	return p.Hardware.Chiplets
}

// PageBytesOrDefault returns the page size declared in the plan.
func (p *Plan) PageBytesOrDefault(defaultVal uint64) uint64 {
	if p == nil {
		return defaultVal
	}
	if p.Hardware.PageBytes == 0 {
		return defaultVal
	}
	return p.Hardware.PageBytes
}

// BufferPagePlacements returns the per-page chiplet assignment for a buffer tag.
//
// Supported policies:
//   - explicit (default): buffer_placement[tag].pages provides chiplet id per page
//   - rr/interleaved: page i is assigned to chiplet (i % chiplets)
func (p *Plan) BufferPagePlacements(tag string, numPages int) ([]int, error) {
	if p == nil {
		return nil, fmt.Errorf("nil plan")
	}
	if numPages <= 0 {
		return []int{}, nil
	}

	bp, ok := p.BufferPlacement[tag]
	if !ok {
		return nil, fmt.Errorf("buffer tag %q not found in plan", tag)
	}

	policy := bp.Policy
	if policy == "" {
		policy = "explicit"
	}

	switch policy {
	case "explicit":
		if len(bp.Pages) < numPages {
			return nil, fmt.Errorf(
				"buffer tag %q policy explicit but pages length %d < required %d",
				tag, len(bp.Pages), numPages,
			)
		}
		out := make([]int, numPages)
		copy(out, bp.Pages[:numPages])

		chiplets := p.NumChipletsOrDefault(1)
		for i := 0; i < numPages; i++ {
			if out[i] < 0 || out[i] >= chiplets {
				return nil, fmt.Errorf(
					"buffer tag %q page %d has invalid chiplet id %d (chiplets=%d)",
					tag, i, out[i], chiplets,
				)
			}
		}

		return out, nil

	case "interleaved", "rr", "roundrobin", "round-robin":
		chiplets := p.NumChipletsOrDefault(1)
		out := make([]int, numPages)
		for i := 0; i < numPages; i++ {
			out[i] = i % chiplets
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unsupported buffer placement policy %q for tag %q", policy, tag)
	}
}
