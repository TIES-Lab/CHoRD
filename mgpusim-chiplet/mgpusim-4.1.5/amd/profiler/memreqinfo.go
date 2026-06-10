package profiler

import "github.com/sarchlab/akita/v4/mem/vm"

// MemReqInfo is attached to mem.ReadReq/mem.WriteReq via req.Info.
//
// It preserves information that is otherwise lost when the VM address
// translator rewrites requests to physical addresses (PID becomes 0 and the
// original VAddr is not directly recoverable).
//
// This struct is intentionally small because it is on the hot path.
//
// All fields are optional; profilers should treat zero values as "unknown".
type MemReqInfo struct {
	// PID is the original process ID of the request (before translation).
	PID vm.PID `json:"pid"`

	// VAddr is the original virtual address (before translation).
	// For coalesced requests, this is typically cache-line aligned.
	VAddr uint64 `json:"vaddr"`

	// RequesterSA is the global shader-array (SA) ID of the requester.
	// For the chiplet platform, chiplet_id = RequesterSA / sa_per_chiplet.
	RequesterSA int `json:"requester_sa"`

	// PacketAddr identifies the kernel launch packet that originated this
	// memory request. It lets profilers attribute L2 traffic to the exact
	// launch even when multiple kernels overlap.
	PacketAddr uint64 `json:"packet_addr"`

	// Work-group coordinates of the requester. These are best-effort and let
	// profilers build per-workgroup page-touch summaries.
	WGX int `json:"wg_x"`
	WGY int `json:"wg_y"`
	WGZ int `json:"wg_z"`
}
