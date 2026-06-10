package vm

// PageTablePagePool is an optional interface that a PageTable can implement to
// accept pre-allocated physical pages that will be used as page-table pages.
//
// The MGVM page-walk model allocates page-table pages from the same physical
// memory space as data pages. In the v3.7 codebase, the driver memory allocator
// is responsible for reserving a small portion of each device memory and
// providing those pages to the page table implementation.
//
// A PageTable implementation that does not need separate page-table pages can
// ignore this interface.
type PageTablePagePool interface {
	// AddPageTablePages registers physical pages that can be used to store page
	// table nodes for a given device.
	AddPageTablePages(deviceID uint64, pAddrs []uint64)
}
