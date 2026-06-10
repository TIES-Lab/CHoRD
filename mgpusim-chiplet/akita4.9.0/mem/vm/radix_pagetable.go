package vm

import (
	"encoding/binary"
	"fmt"
	"sync"
)

// PTEStorage is an optional backing storage interface for a page table.
//
// It intentionally lives in the vm package (instead of depending on the mem
// package) to avoid import cycles, because the mem protocol definitions depend
// on vm types.
//
// In MGPUSim, the global physical storage (mem.Storage) satisfies this
// interface.
type PTEStorage interface {
	Write(address uint64, data []byte) error
}

// RadixPageTable is a simplified radix-tree page table that is compatible with
// the MGVM-style page-walk model.
//
// It supports a 4-level radix tree with 9 bits per level (512 entries per page)
// and uses 8-byte PTEs, so a page-table page naturally fits into a 4KB page.
//
// The page table itself does not allocate physical memory. Instead, it
// consumes a pool of pre-allocated physical pages (page-table pages) provided
// by the driver via the PageTablePagePool interface.
//
// This implementation is intended for timing simulation. It is thread-safe.
type RadixPageTable struct {
	mu sync.RWMutex

	log2PageSize uint64

	// Optional backing storage. If provided, the page table will materialize
	// page-table entries into the storage so that a page-table walker can fetch
	// PTE values through the normal cache/memory hierarchy.
	storage PTEStorage

	// Free page-table pages per device.
	ptPagePool map[uint64][]uint64

	// Optional device memory layout information, used to place page-table pages
	// on a specific chiplet.
	deviceLayouts map[uint64]radixDeviceLayout

	// When device layout is known, ptPagePoolByChiplet stores free page-table
	// pages grouped by chiplet ID.
	ptPagePoolByChiplet map[uint64][][]uint64

	// Per-process page tables.
	procTables map[PID]*radixProcTable
}

type radixDeviceLayout struct {
	initialAddress     uint64
	numChiplets        int
	numBanksPerChiplet int
}

type radixProcTable struct {
	pid PID

	// Root page-table node (level 0).
	root *ptNode

	// A fast path for Find(). Key is aligned virtual address.
	pages map[uint64]Page
}

// ptNode represents a page-table page. It has 512 entries.
//
// For levels 0..2, entries point to the physical address of the next-level
// page-table page.
// For level 3, entries point to the physical address of the data page.
type ptNode struct {
	level    int
	pAddr    uint64
	entries  [512]uint64
	children [512]*ptNode
}

const (
	radixNumLevels  = 4
	radixBitsPerLvl = 9
	radixEntries    = 1 << radixBitsPerLvl // 512
	radixPTEBytes   = 8
	radixLvlMask    = uint64(radixEntries - 1)
)

// NewRadixPageTable creates a radix-tree page table.
func NewRadixPageTable(log2PageSize uint64) *RadixPageTable {
	return &RadixPageTable{
		log2PageSize:        log2PageSize,
		ptPagePool:          make(map[uint64][]uint64),
		deviceLayouts:       make(map[uint64]radixDeviceLayout),
		ptPagePoolByChiplet: make(map[uint64][][]uint64),
		procTables:          make(map[PID]*radixProcTable),
	}
}

// SetDeviceMemoryLayout implements PageTableDeviceMemoryLayout.
//
// The driver calls this when registering a device. When set, the radix page
// table will attempt to allocate page-table pages from the same chiplet as the
// mapped data page (MGVM-style colocation).
func (pt *RadixPageTable) SetDeviceMemoryLayout(
	deviceID uint64,
	initialAddress uint64,
	numChiplets int,
	numBanksPerChiplet int,
) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if numChiplets <= 0 {
		numChiplets = 1
	}
	if numBanksPerChiplet <= 0 {
		numBanksPerChiplet = 1
	}

	pt.deviceLayouts[deviceID] = radixDeviceLayout{
		initialAddress:     initialAddress,
		numChiplets:        numChiplets,
		numBanksPerChiplet: numBanksPerChiplet,
	}

	// Initialize per-chiplet pools if not exists.
	if _, ok := pt.ptPagePoolByChiplet[deviceID]; !ok {
		pt.ptPagePoolByChiplet[deviceID] = make([][]uint64, numChiplets)
	} else if len(pt.ptPagePoolByChiplet[deviceID]) != numChiplets {
		// Resize while keeping existing entries.
		old := pt.ptPagePoolByChiplet[deviceID]
		neu := make([][]uint64, numChiplets)
		copy(neu, old)
		pt.ptPagePoolByChiplet[deviceID] = neu
	}
}

// WithStorage attaches a backing storage to the page table. When set, the page
// table will write updated PTE values into the storage so that PTW reads can
// obtain real PTE bytes.
func (pt *RadixPageTable) WithStorage(storage PTEStorage) *RadixPageTable {
	pt.storage = storage
	return pt
}

// SetStorage attaches a backing storage to the page table.
func (pt *RadixPageTable) SetStorage(storage PTEStorage) {
	pt.storage = storage
}

func (pt *RadixPageTable) storePTE(pteAddr uint64, val uint64) {
	if pt.storage == nil {
		return
	}

	buf := make([]byte, radixPTEBytes)
	binary.LittleEndian.PutUint64(buf, val)
	if err := pt.storage.Write(pteAddr, buf); err != nil {
		panic(fmt.Sprintf(
			"radix page table: cannot write PTE to storage at 0x%x: %v",
			pteAddr, err))
	}
}

// GetLog2PageSize returns log2(pageSize).
func (pt *RadixPageTable) GetLog2PageSize() uint64 {
	return pt.log2PageSize
}

// AddPageTablePages implements PageTablePagePool.
func (pt *RadixPageTable) AddPageTablePages(deviceID uint64, pAddrs []uint64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if len(pAddrs) == 0 {
		return
	}

	// If we know the device layout, distribute pages into chiplet-specific pools
	// so that we can allocate page-table pages on the same chiplet as the mapped
	// data page.
	if layout, ok := pt.deviceLayouts[deviceID]; ok {
		if layout.numChiplets <= 0 {
			layout.numChiplets = 1
		}
		pool, ok2 := pt.ptPagePoolByChiplet[deviceID]
		if !ok2 || len(pool) != layout.numChiplets {
			pool = make([][]uint64, layout.numChiplets)
			pt.ptPagePoolByChiplet[deviceID] = pool
		}

		for _, p := range pAddrs {
			chiplet := pt.chipletOfPAddrLocked(layout, p)
			if chiplet < 0 || chiplet >= layout.numChiplets {
				chiplet = 0
			}
			pt.ptPagePoolByChiplet[deviceID][chiplet] = append(pt.ptPagePoolByChiplet[deviceID][chiplet], p)
		}
		return
	}

	// Fallback: no layout info, use a single pool.
	pt.ptPagePool[deviceID] = append(pt.ptPagePool[deviceID], pAddrs...)
}

// NumFreePageTablePages returns the number of free page-table pages currently
// available for a given device and chiplet.
//
// This is an optional helper method used by the driver to implement
// MGVM-style on-demand expansion of the page-table page pool. It is NOT part of
// the vm.PageTable interface.
func (pt *RadixPageTable) NumFreePageTablePages(deviceID uint64, chiplet int) int {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pools, ok := pt.ptPagePoolByChiplet[deviceID]; ok {
		if chiplet >= 0 && chiplet < len(pools) {
			return len(pools[chiplet])
		}
		// Chiplet out of range: return total across chiplets.
		total := 0
		for i := 0; i < len(pools); i++ {
			total += len(pools[i])
		}
		return total
	}

	return len(pt.ptPagePool[deviceID])
}

// Insert inserts a page mapping.
func (pt *RadixPageTable) Insert(page Page) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if page.PageSize != 0 {
		expected := uint64(1) << pt.log2PageSize
		if page.PageSize != expected {
			panic(fmt.Sprintf("radix page table only supports fixed page size %d, got %d", expected, page.PageSize))
		}
	}

	proc := pt.getOrCreateProcTable(page.PID)
	if proc.root == nil {
		proc.root = pt.newPTNode(int(page.DeviceID), 0, 0)
	}

	alignedVAddr := pt.alignToPage(page.VAddr)
	page.VAddr = alignedVAddr
	if page.PageSize == 0 {
		page.PageSize = uint64(1) << pt.log2PageSize
	}
	page.Valid = true

	// Update the leaf mapping.
	pt.insertMapping(proc, page)

	// Update the fast path.
	proc.pages[alignedVAddr] = page
}

// Remove removes a page mapping.
func (pt *RadixPageTable) Remove(pid PID, vAddr uint64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	proc, ok := pt.procTables[pid]
	if !ok || proc.root == nil {
		return
	}

	alignedVAddr := pt.alignToPage(vAddr)

	// Best-effort removal from the radix tree.
	pt.removeMapping(proc, alignedVAddr)

	delete(proc.pages, alignedVAddr)
}

// Find looks up a mapping.
func (pt *RadixPageTable) Find(pid PID, vAddr uint64) (Page, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	proc, ok := pt.procTables[pid]
	if !ok {
		return Page{}, false
	}

	alignedVAddr := pt.alignToPage(vAddr)
	page, found := proc.pages[alignedVAddr]
	return page, found && page.Valid
}

// ReverseLookup finds a page by its physical address across all processes.
//
// This method is required by the vm.PageTable interface and is used by
// components such as the MMU during page migration.
func (pt *RadixPageTable) ReverseLookup(pAddr uint64) (Page, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	for _, proc := range pt.procTables {
		for _, page := range proc.pages {
			if page.Valid && page.PAddr == pAddr {
				return page, true
			}
		}
	}

	return Page{}, false
}

// Update updates an existing mapping.
func (pt *RadixPageTable) Update(page Page) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	proc := pt.getOrCreateProcTable(page.PID)
	if proc.root == nil {
		proc.root = pt.newPTNode(int(page.DeviceID), 0, 0)
	}

	alignedVAddr := pt.alignToPage(page.VAddr)
	page.VAddr = alignedVAddr
	if page.PageSize == 0 {
		page.PageSize = uint64(1) << pt.log2PageSize
	}
	page.Valid = true

	pt.insertMapping(proc, page)
	proc.pages[alignedVAddr] = page
}

// Rearrange converts a virtual address into a rearranged VPN representation.
//
// This matches MGVM's implementation, allowing the page walker to repeatedly
// consume the low radixBitsPerLvl bits to obtain the page-table indices from
// the top level to the leaf.
func (pt *RadixPageTable) Rearrange(vAddr uint64) uint64 {
	vpn := vAddr >> pt.log2PageSize
	var rearranged uint64
	for i := 0; i < radixNumLevels; i++ {
		idx := vpn & radixLvlMask
		rearranged = (rearranged << radixBitsPerLvl) | idx
		vpn >>= radixBitsPerLvl
	}
	return rearranged
}

// NextLevel shifts the rearranged VPN to the next level.
func (pt *RadixPageTable) NextLevel(rearrangedVPN uint64) uint64 {
	return rearrangedVPN >> radixBitsPerLvl
}

// MoveToLevel moves the rearranged VPN to the specified level.
func (pt *RadixPageTable) MoveToLevel(rearrangedVPN uint64, level int) uint64 {
	if level <= 0 {
		return rearrangedVPN
	}
	if level >= radixNumLevels {
		return 0
	}
	return rearrangedVPN >> (uint(level) * radixBitsPerLvl)
}

// GetRoot returns the physical address of the root page-table page for the
// given process.
func (pt *RadixPageTable) GetRoot(pid PID) uint64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	proc, ok := pt.procTables[pid]
	if !ok || proc.root == nil {
		return 0
	}
	return proc.root.pAddr
}

// AddOffset returns the physical address of the PTE within a page-table page.
//
// base is the physical base address (PPN) of the current page-table page.
func (pt *RadixPageTable) AddOffset(base uint64, rearrangedVPN uint64) uint64 {
	idx := rearrangedVPN & radixLvlMask
	return base + idx*radixPTEBytes
}

// FindPTEValue returns the value of the PTE at the given level.
//
// Level is 0..3 for a 4-level page table.
// For levels 0..2, the value is the physical address of the next-level page
// table page.
// For level 3, the value is the physical address of the data page.
func (pt *RadixPageTable) FindPTEValue(pid PID, vAddr uint64, level int) (uint64, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if level < 0 || level >= radixNumLevels {
		return 0, false
	}

	proc, ok := pt.procTables[pid]
	if !ok || proc.root == nil {
		return 0, false
	}

	n := proc.root
	rearranged := pt.Rearrange(vAddr)

	for l := 0; l <= level; l++ {
		idx := int(rearranged & radixLvlMask)
		if l == level {
			val := n.entries[idx]
			if val == 0 {
				return 0, false
			}
			return val, true
		}

		// Go to the next-level node.
		child := n.children[idx]
		if child == nil {
			return 0, false
		}
		n = child
		rearranged = rearranged >> radixBitsPerLvl
	}

	return 0, false
}

// ---- internal helpers ----

func (pt *RadixPageTable) getOrCreateProcTable(pid PID) *radixProcTable {
	proc, ok := pt.procTables[pid]
	if ok {
		return proc
	}

	proc = &radixProcTable{
		pid:   pid,
		pages: make(map[uint64]Page),
	}
	pt.procTables[pid] = proc
	return proc
}

func (pt *RadixPageTable) popPTPage(deviceID int) uint64 {
	id := uint64(deviceID)
	pool := pt.ptPagePool[id]
	if len(pool) == 0 {
		panic(fmt.Sprintf(
			"radix page table ran out of page-table pages for device %d; "+
				"please increase the reserved page-table page pool",
			deviceID,
		))
	}
	pAddr := pool[len(pool)-1]
	pt.ptPagePool[id] = pool[:len(pool)-1]
	return pAddr
}

func (pt *RadixPageTable) popPTPageOnChiplet(deviceID int, chiplet int, fallbackAllowed bool) uint64 {
	id := uint64(deviceID)

	// Fast path: chiplet-specific pool.
	if pools, ok := pt.ptPagePoolByChiplet[id]; ok && chiplet >= 0 && chiplet < len(pools) {
		pool := pools[chiplet]
		if len(pool) > 0 {
			pAddr := pool[len(pool)-1]
			pools[chiplet] = pool[:len(pool)-1]
			pt.ptPagePoolByChiplet[id] = pools
			return pAddr
		}
	}

	if fallbackAllowed {
		return pt.popPTPage(deviceID)
	}

	panic(fmt.Sprintf(
		"radix page table ran out of page-table pages on device %d chiplet %d; "+
			"please increase the reserved page-table page pool",
		deviceID, chiplet,
	))
}

func (pt *RadixPageTable) newPTNode(deviceID int, level int, chiplet int) *ptNode {
	// If we have chiplet pools, allocate from the requested chiplet. Otherwise,
	// fall back to the legacy single pool.
	useFallback := true
	if _, ok := pt.ptPagePoolByChiplet[uint64(deviceID)]; ok {
		useFallback = false
	}

	n := &ptNode{
		level: level,
		pAddr: pt.popPTPageOnChiplet(deviceID, chiplet, useFallback),
	}
	return n
}

func (pt *RadixPageTable) insertMapping(proc *radixProcTable, page Page) {
	rearranged := pt.Rearrange(page.VAddr)
	n := proc.root

	// MGVM-style page-table page placement: allocate page-table pages on the same
	// chiplet as the mapped data page.
	//
	// Root is created with chiplet=0 (consistent with MGVM's first allocation).
	chipletHint := 0
	if layout, ok := pt.deviceLayouts[uint64(page.DeviceID)]; ok {
		chipletHint = pt.chipletOfPAddrLocked(layout, page.PAddr)
		if chipletHint < 0 || chipletHint >= layout.numChiplets {
			chipletHint = 0
		}
	}

	for level := 0; level < radixNumLevels; level++ {
		idx := int(rearranged & radixLvlMask)

		if level == radixNumLevels-1 {
			// Leaf PTE.
			n.entries[idx] = page.PAddr
			pt.storePTE(n.pAddr+uint64(idx)*radixPTEBytes, page.PAddr)
			return
		}

		if n.children[idx] == nil {
			child := pt.newPTNode(int(page.DeviceID), level+1, chipletHint)
			n.children[idx] = child
			n.entries[idx] = child.pAddr
			pt.storePTE(n.pAddr+uint64(idx)*radixPTEBytes, child.pAddr)
		}

		n = n.children[idx]
		rearranged = rearranged >> radixBitsPerLvl
	}
}

func (pt *RadixPageTable) chipletOfPAddrLocked(layout radixDeviceLayout, pAddr uint64) int {
	if layout.numChiplets <= 0 || layout.numBanksPerChiplet <= 0 {
		return 0
	}
	if pAddr < layout.initialAddress {
		return 0
	}

	pageIndex := (pAddr - layout.initialAddress) >> pt.log2PageSize
	totalBanks := uint64(layout.numChiplets * layout.numBanksPerChiplet)
	if totalBanks == 0 {
		return 0
	}
	bankIndex := pageIndex % totalBanks
	chiplet := bankIndex / uint64(layout.numBanksPerChiplet)
	return int(chiplet)
}

func (pt *RadixPageTable) removeMapping(proc *radixProcTable, alignedVAddr uint64) {
	rearranged := pt.Rearrange(alignedVAddr)
	n := proc.root
	if n == nil {
		return
	}

	for level := 0; level < radixNumLevels; level++ {
		idx := int(rearranged & radixLvlMask)

		if level == radixNumLevels-1 {
			n.entries[idx] = 0
			pt.storePTE(n.pAddr+uint64(idx)*radixPTEBytes, 0)
			return
		}

		child := n.children[idx]
		if child == nil {
			return
		}

		n = child
		rearranged = rearranged >> radixBitsPerLvl
	}
}

func (pt *RadixPageTable) alignToPage(vAddr uint64) uint64 {
	mask := (uint64(1) << pt.log2PageSize) - 1
	return vAddr &^ mask
}
