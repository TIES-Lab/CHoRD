package driver

import (
	"strings"

	"github.com/sarchlab/mgpusim/v4/amd/driver/internal"
)

// SetMemoryAllocatorType configures the global memory allocator type used when
// building the platform.
//
// This must be called before building the platform (before GPUs are registered
// to the driver).
//
// Supported values (case-insensitive):
//   - "default": sequential free-list allocator
//   - "buddy": buddy allocator
//   - "lasp"/"partitioned"/"chiplet": MGVM-style chiplet interleaved allocator
func SetMemoryAllocatorType(allocatorType string) {
	switch strings.ToLower(allocatorType) {
	case "", "default":
		internal.MemoryAllocatorType = internal.AllocatorTypeDefault
	case "buddy":
		internal.MemoryAllocatorType = internal.AllocatorTypeBuddy
	case "lasp", "partitioned", "interleaved", "chiplet", "chiplets":
		internal.MemoryAllocatorType = internal.AllocatorTypePartitioned
	default:
		panic("unknown memory allocator type: " + allocatorType)
	}
}

// SetChipletMemoryConfig configures the chiplet memory layout parameters used by
// the partitioned allocator.
func SetChipletMemoryConfig(numChiplets int, banksPerChiplet int) {
	if numChiplets <= 0 {
		numChiplets = 1
	}
	if banksPerChiplet <= 0 {
		banksPerChiplet = 1
	}

	internal.NumChipletsPerGPU = numChiplets
	internal.NumMemoryBanksPerChiplet = banksPerChiplet
}

// GetChipletMemoryConfig returns the current chiplet memory layout parameters.
func GetChipletMemoryConfig() (numChiplets int, banksPerChiplet int) {
	return internal.NumChipletsPerGPU, internal.NumMemoryBanksPerChiplet
}
