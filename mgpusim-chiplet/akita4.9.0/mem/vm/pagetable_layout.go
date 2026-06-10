package vm

// PageTableDeviceMemoryLayout is an optional interface that a PageTable can
// implement to receive the physical memory layout information of a device.
//
// This is useful for MGVM-style page table page placement. For example, a
// radix page table may want to allocate page-table pages on the same chiplet as
// the mapped data page (determined by the bank-index based chiplet mapping).
//
// The driver provides the layout information when registering devices.
type PageTableDeviceMemoryLayout interface {
	SetDeviceMemoryLayout(
		deviceID uint64,
		initialAddress uint64,
		numChiplets int,
		numBanksPerChiplet int,
	)
}
