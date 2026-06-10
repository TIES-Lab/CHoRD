// Package internal provides support for the driver implementation.
package internal

import (
	"sync"

	"github.com/sarchlab/akita/v4/mem/vm"
)

// A MemoryAllocator can allocate memory on the CPU and GPUs
type MemoryAllocator interface {
	RegisterDevice(device *Device)
	GetDeviceIDByPAddr(pAddr uint64) int
	NextDeviceInitialAddress() uint64
	Allocate(pid vm.PID, byteSize uint64, deviceID int) uint64
	// AllocateWithPagePlacement allocates a chunk of memory on a given device,
	// optionally pinning each page to a specific chiplet.
	//
	// If pageChiplets is nil or empty, the allocator behaves the same as
	// Allocate.
	//
	// If pageChiplets has fewer entries than the number of pages, remaining
	// pages use the default allocation policy.
	AllocateWithPagePlacement(
		pid vm.PID,
		byteSize uint64,
		deviceID int,
		pageChiplets []int,
	) uint64
	AllocateUnified(pid vm.PID, byteSize uint64) uint64
	Free(vAddr uint64)
	Remap(pid vm.PID, pageVAddr, byteSize uint64, deviceID int)
	RemovePage(vAddr uint64)
	AllocatePageWithGivenVAddr(
		pid vm.PID,
		deviceID int,
		vAddr uint64,
		unified bool,
	) vm.Page
}

// NewMemoryAllocator creates a new memory allocator.
func NewMemoryAllocator(
	pageTable vm.PageTable,
	log2PageSize uint64,
) MemoryAllocator {
	a := &memoryAllocatorImpl{
		pageTable:            pageTable,
		totalStorageByteSize: 1 << log2PageSize, // Starting with a page to avoid 0 address.
		log2PageSize:         log2PageSize,
		processMemoryStates:  make(map[vm.PID]*processMemoryState),
		vAddrToPageMapping:   make(map[uint64]vm.Page),
		allocationPageCounts: make(map[uint64]int),
		devices:              make(map[int]*Device),
	}
	return a
}

type processMemoryState struct {
	pid       vm.PID
	nextVAddr uint64
}

// A memoryAllocatorImpl provides the default implementation for
// memoryAllocator
type memoryAllocatorImpl struct {
	sync.Mutex
	pageTable            vm.PageTable
	log2PageSize         uint64
	vAddrToPageMapping   map[uint64]vm.Page
	allocationPageCounts map[uint64]int
	processMemoryStates  map[vm.PID]*processMemoryState
	devices              map[int]*Device
	totalStorageByteSize uint64
}

func (a *memoryAllocatorImpl) RegisterDevice(device *Device) {
	a.Lock()
	defer a.Unlock()

	state := device.MemState
	state.setInitialAddress(a.totalStorageByteSize)

	a.totalStorageByteSize += state.getStorageSize()

	a.devices[device.ID] = device

	// If the page table supports receiving device memory layout information,
	// provide it now. This enables MGVM-style page-table page placement (e.g.,
	// colocating page-table pages with data pages on the same chiplet).
	if layout, ok := a.pageTable.(vm.PageTableDeviceMemoryLayout); ok {
		layout.SetDeviceMemoryLayout(
			uint64(device.ID),
			state.getInitialAddress(),
			NumChipletsPerGPU,
			NumMemoryBanksPerChiplet,
		)
	}

	// If the page table supports page-table-page pre-allocation, reserve a pool
	// of physical pages for page-table pages on this device.
	//
	// Without this, an MGVM-style radix page table would have no memory to place
	// its page-table pages, causing a panic during the first allocations.
	if pool, ok := a.pageTable.(vm.PageTablePagePool); ok {
		// For a 4-level radix page table (9 bits per level, 8-byte PTE, 4KB pages),
		// each lowest-level page-table page covers 512 * 4KB = 2MB of data pages.
		//
		// We allocate a conservative number of page-table pages based on the device
		// memory size. This is small (a few MB) compared to typical device memory
		// sizes and avoids running out in common workloads.
		const bitsPerLevel = 9
		leafPTPages := int(state.getStorageSize() >> (a.log2PageSize + bitsPerLevel))
		if leafPTPages < 1 {
			leafPTPages = 1
		}
		// Extra pages cover upper levels and fragmentation.
		numPTPages := leafPTPages + leafPTPages/8 + 64
		if numPTPages < 256 {
			numPTPages = 256
		}

		pAddrs := device.allocateMultiplePages(numPTPages)
		pool.AddPageTablePages(uint64(device.ID), pAddrs)
	}
}

// maybeRefillPageTablePagePool tops up the page-table page pool for the radix
// page table when it is running low on a particular chiplet.
//
// Rationale:
//   - In MGVM, page-table pages are allocated on-demand from the same device
//     physical memory space, colocated with the mapped data page's chiplet.
//   - In our Akita v4 port, the radix page table consumes a pool of reserved
//     physical pages. If a workload/plan concentrates allocations on one
//     chiplet, that chiplet's reserved pool may run out even though the chiplet
//     still has free physical pages. That would be an artifact.
//
// This function implements a lightweight on-demand expansion strategy to avoid
// that artifact while keeping the driver/page-table interface simple and
// avoiding lock cycles.
func (a *memoryAllocatorImpl) maybeRefillPageTablePagePool(device *Device, dataPAddr uint64) {
	// Only applies to radix page tables that expose a pool size query.
	pt, ok := a.pageTable.(interface {
		NumFreePageTablePages(deviceID uint64, chiplet int) int
	})
	if !ok {
		return
	}

	pool, ok := a.pageTable.(vm.PageTablePagePool)
	if !ok {
		return
	}

	// Compute the data page's owning chiplet using the same MGVM-style mapping
	// used by the partitioned allocator.
	chiplet := 0
	if NumChipletsPerGPU > 1 && NumMemoryBanksPerChiplet > 0 {
		init := device.MemState.getInitialAddress()
		if dataPAddr >= init {
			pageIndex := (dataPAddr - init) >> a.log2PageSize
			totalBanks := uint64(NumChipletsPerGPU * NumMemoryBanksPerChiplet)
			if totalBanks != 0 {
				bankIndex := pageIndex % totalBanks
				chiplet = int(bankIndex / uint64(NumMemoryBanksPerChiplet))
			}
		}
	}

	const lowWatermark = 64
	const refillBatch = 256

	free := pt.NumFreePageTablePages(uint64(device.ID), chiplet)
	if free >= lowWatermark {
		return
	}

	// Refill by allocating additional physical pages on the same chiplet.
	pAddrs := make([]uint64, 0, refillBatch)
	for i := 0; i < refillBatch; i++ {
		pAddrs = append(pAddrs, device.allocatePageOnChiplet(chiplet))
	}

	pool.AddPageTablePages(uint64(device.ID), pAddrs)
}

// NextDeviceInitialAddress returns the physical address where the next
// device will be placed. This is useful for platform builders to align their
// memAddrOffset with the driver's memory allocator layout.
func (a *memoryAllocatorImpl) NextDeviceInitialAddress() uint64 {
	a.Lock()
	defer a.Unlock()

	return a.totalStorageByteSize
}

func (a *memoryAllocatorImpl) GetDeviceIDByPAddr(pAddr uint64) int {
	a.Lock()
	defer a.Unlock()

	return a.deviceIDByPAddr(pAddr)
}

func (a *memoryAllocatorImpl) deviceIDByPAddr(pAddr uint64) int {
	for id, dev := range a.devices {
		state := dev.MemState
		if isPAddrOnDevice(pAddr, state) {
			return id
		}
	}

	panic("device not found")
}

func isPAddrOnDevice(
	pAddr uint64,
	state DeviceMemoryState,
) bool {
	return pAddr >= state.getInitialAddress() &&
		pAddr < state.getInitialAddress()+state.getStorageSize()
}

func (a *memoryAllocatorImpl) Allocate(
	pid vm.PID,
	byteSize uint64,
	deviceID int,
) uint64 {
	if byteSize == 0 {
		panic("Allocating 0 bytes.")
	}

	a.Lock()
	defer a.Unlock()

	pageSize := uint64(1 << a.log2PageSize)
	numPages := (byteSize-1)/pageSize + 1
	return a.allocatePages(int(numPages), pid, deviceID, false)
}

// AllocateWithPagePlacement allocates memory on a device, optionally placing
// pages on specific chiplets.
func (a *memoryAllocatorImpl) AllocateWithPagePlacement(
	pid vm.PID,
	byteSize uint64,
	deviceID int,
	pageChiplets []int,
) uint64 {
	if byteSize == 0 {
		panic("Allocating 0 bytes.")
	}

	a.Lock()
	defer a.Unlock()

	pageSize := uint64(1 << a.log2PageSize)
	numPages := int((byteSize-1)/pageSize + 1)

	// Fast path: no placement requested.
	if len(pageChiplets) == 0 {
		return a.allocatePages(numPages, pid, deviceID, false)
	}

	return a.allocatePagesWithPlacement(numPages, pid, deviceID, false, pageChiplets)
}

func (a *memoryAllocatorImpl) AllocateUnified(
	pid vm.PID,
	byteSize uint64,
) uint64 {
	if byteSize == 0 {
		panic("Allocating 0 bytes.")
	}

	a.Lock()
	defer a.Unlock()

	pageSize := uint64(1 << a.log2PageSize)
	numPages := (byteSize-1)/pageSize + 1
	return a.allocatePages(int(numPages), pid, 1, true)
}

func (a *memoryAllocatorImpl) allocatePages(
	numPages int,
	pid vm.PID,
	deviceID int,
	unified bool,
) (firstPageVAddr uint64) {
	pState, found := a.processMemoryStates[pid]
	if !found {
		a.processMemoryStates[pid] = &processMemoryState{
			pid:       pid,
			nextVAddr: uint64(1 << a.log2PageSize),
		}
		pState = a.processMemoryStates[pid]
	}
	device := a.devices[deviceID]

	pageSize := uint64(1 << a.log2PageSize)
	nextVAddr := pState.nextVAddr

	for i := 0; i < numPages; i++ {
		pAddr := device.allocatePage()
		vAddr := nextVAddr + uint64(i)*pageSize

		page := vm.Page{
			PID:      pid,
			VAddr:    vAddr,
			PAddr:    pAddr,
			PageSize: pageSize,
			Valid:    true,
			Unified:  unified,
			DeviceID: uint64(a.deviceIDByPAddr(pAddr)),
		}

		// fmt.Printf("page.addr is %x piage Device ID is %d \n", page.PAddr, page.DeviceID)
		// debug.PrintStack()

		// Ensure the radix page table has enough page-table pages on the target chiplet.
		a.maybeRefillPageTablePagePool(device, pAddr)

		a.pageTable.Insert(page)
		a.vAddrToPageMapping[page.VAddr] = page
	}

	pState.nextVAddr += pageSize * uint64(numPages)
	a.allocationPageCounts[nextVAddr] = numPages

	return nextVAddr
}

func (a *memoryAllocatorImpl) allocatePagesWithPlacement(
	numPages int,
	pid vm.PID,
	deviceID int,
	unified bool,
	pageChiplets []int,
) (firstPageVAddr uint64) {
	pState, found := a.processMemoryStates[pid]
	if !found {
		a.processMemoryStates[pid] = &processMemoryState{
			pid:       pid,
			nextVAddr: uint64(1 << a.log2PageSize),
		}
		pState = a.processMemoryStates[pid]
	}

	device := a.devices[deviceID]
	pageSize := uint64(1 << a.log2PageSize)
	nextVAddr := pState.nextVAddr

	for i := 0; i < numPages; i++ {
		var pAddr uint64
		if pageChiplets != nil && i < len(pageChiplets) {
			pAddr = device.allocatePageOnChiplet(pageChiplets[i])
		} else {
			pAddr = device.allocatePage()
		}

		vAddr := nextVAddr + uint64(i)*pageSize

		page := vm.Page{
			PID:      pid,
			VAddr:    vAddr,
			PAddr:    pAddr,
			PageSize: pageSize,
			Valid:    true,
			Unified:  unified,
			DeviceID: uint64(a.deviceIDByPAddr(pAddr)),
		}

		// Ensure the radix page table has enough page-table pages on the target chiplet.
		a.maybeRefillPageTablePagePool(device, pAddr)

		a.pageTable.Insert(page)
		a.vAddrToPageMapping[page.VAddr] = page
	}

	pState.nextVAddr += pageSize * uint64(numPages)
	a.allocationPageCounts[nextVAddr] = numPages
	return nextVAddr
}

func (a *memoryAllocatorImpl) Remap(
	pid vm.PID,
	pageVAddr, byteSize uint64,
	deviceID int,
) {
	a.Lock()
	defer a.Unlock()

	pageSize := uint64(1 << a.log2PageSize)
	addr := pageVAddr
	vAddrs := make([]uint64, 0)
	for addr < pageVAddr+byteSize {
		vAddrs = append(vAddrs, addr)
		addr += pageSize
	}

	a.allocateMultiplePagesWithGivenVAddrs(pid, deviceID, vAddrs, false)
}

func (a *memoryAllocatorImpl) RemovePage(vAddr uint64) {
	a.Lock()
	defer a.Unlock()

	a.removePage(vAddr)
}

func (a *memoryAllocatorImpl) removePage(vAddr uint64) {
	page, ok := a.vAddrToPageMapping[vAddr]

	if !ok {
		panic("page not found")
	}

	deviceID := a.deviceIDByPAddr(page.PAddr)
	dState := a.devices[deviceID].MemState
	dState.addSinglePAddr(page.PAddr)

	a.pageTable.Remove(page.PID, page.VAddr)
	delete(a.vAddrToPageMapping, page.VAddr)
}

func (a *memoryAllocatorImpl) AllocatePageWithGivenVAddr(
	pid vm.PID,
	deviceID int,
	vAddr uint64,
	isUnified bool,
) vm.Page {
	a.Lock()
	defer a.Unlock()

	return a.allocatePageWithGivenVAddr(pid, deviceID, vAddr, isUnified)
}

func (a *memoryAllocatorImpl) allocatePageWithGivenVAddr(
	pid vm.PID,
	deviceID int,
	vAddr uint64,
	isUnified bool,
) vm.Page {
	pageSize := uint64(1 << a.log2PageSize)

	device := a.devices[deviceID]
	pAddr := device.allocatePage()

	page := vm.Page{
		PID:      pid,
		VAddr:    vAddr,
		PAddr:    pAddr,
		PageSize: pageSize,
		Valid:    true,
		DeviceID: uint64(deviceID),
		Unified:  isUnified,
	}
	a.vAddrToPageMapping[page.VAddr] = page
	a.pageTable.Update(page)

	return page
}

func (a *memoryAllocatorImpl) allocateMultiplePagesWithGivenVAddrs(
	pid vm.PID,
	deviceID int,
	vAddrs []uint64,
	isUnified bool,
) (pages []vm.Page) {
	pageSize := uint64(1 << a.log2PageSize)

	device := a.devices[deviceID]
	pAddrs := device.allocateMultiplePages(len(vAddrs))

	for i, vAddr := range vAddrs {
		page := vm.Page{
			PID:      pid,
			VAddr:    vAddr,
			PAddr:    pAddrs[i],
			PageSize: pageSize,
			Valid:    true,
			DeviceID: uint64(deviceID),
			Unified:  isUnified,
		}
		a.vAddrToPageMapping[page.VAddr] = page
		a.pageTable.Update(page)
		pages = append(pages, page)
	}

	return pages
}

func (a *memoryAllocatorImpl) Free(ptr uint64) {
	a.Lock()
	defer a.Unlock()

	if numPages, ok := a.allocationPageCounts[ptr]; ok {
		pageSize := uint64(1 << a.log2PageSize)
		for i := 0; i < numPages; i++ {
			a.removePage(ptr + uint64(i)*pageSize)
		}
		delete(a.allocationPageCounts, ptr)
		return
	}

	// Backward-compatible fallback for callers that free a single page without
	// going through the standard Allocate APIs that register an allocation.
	a.removePage(ptr)
}
