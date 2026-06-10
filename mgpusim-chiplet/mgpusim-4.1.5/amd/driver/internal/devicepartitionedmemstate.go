package internal

import "fmt"

// devicePartitionedMemoryState implements a chiplet-partitioned
// physical memory allocator.
//
// Physical pages are owned by chiplets according to an MGVM-style
// bank-interleaved layout at page granularity:
//
//   totalBanks = NumChipletsPerGPU * NumMemoryBanksPerChiplet
//   bankIndex  = pageIndex % totalBanks
//   chiplet    = bankIndex / NumMemoryBanksPerChiplet
//
// Baseline allocation policy (used when no explicit placement is requested) is
// **round-robin across chiplets per page**. This provides a simple RR data-page
// baseline for comparison against LASP/plan-based placement.
//
// Explicit per-page chiplet placement is supported via allocatePageOnChiplet.
type devicePartitionedMemoryState struct {
	log2PageSize       uint64
	initialAddress     uint64
	storageSize        uint64
	numChiplets        int
	numBanksPerChiplet int
		// nextPartitionToAlloc tracks which chiplet to allocate the next page
	// from under the baseline round-robin policy.
	nextPartitionToAlloc int
	availablePAddrs [][]uint64 // per-chiplet free-list
}

func newDevicePartitionedMemoryState(log2pagesize uint64) DeviceMemoryState {
	numChiplets := NumChipletsPerGPU
	if numChiplets <= 0 {
		numChiplets = 1
	}
	banksPerChiplet := NumMemoryBanksPerChiplet
	if banksPerChiplet <= 0 {
		banksPerChiplet = 1
	}

	return &devicePartitionedMemoryState{
		log2PageSize:       log2pagesize,
		numChiplets:        numChiplets,
		numBanksPerChiplet: banksPerChiplet,
		availablePAddrs:    make([][]uint64, numChiplets),
	}
}

func (dms *devicePartitionedMemoryState) setInitialAddress(addr uint64) {
	dms.initialAddress = addr
	dms.nextPartitionToAlloc = 0

	// Reset free lists (important for tests that reuse an instance).
	for i := range dms.availablePAddrs {
		dms.availablePAddrs[i] = dms.availablePAddrs[i][:0]
	}

	pageSize := uint64(1 << dms.log2PageSize)
	endAddr := dms.initialAddress + dms.storageSize

	cur := dms.initialAddress
	for cur < endAddr {
		for chiplet := 0; chiplet < dms.numChiplets && cur < endAddr; chiplet++ {
			for bank := 0; bank < dms.numBanksPerChiplet && cur < endAddr; bank++ {
				dms.availablePAddrs[chiplet] = append(dms.availablePAddrs[chiplet], cur)
				cur += pageSize
			}
		}
	}
}

func (dms *devicePartitionedMemoryState) getInitialAddress() uint64 {
	return dms.initialAddress
}

func (dms *devicePartitionedMemoryState) setStorageSize(size uint64) {
	dms.storageSize = size
}

func (dms *devicePartitionedMemoryState) getStorageSize() uint64 {
	return dms.storageSize
}

func (dms *devicePartitionedMemoryState) addSinglePAddr(addr uint64) {
	chiplet := dms.chipletOfPAddr(addr)
	dms.availablePAddrs[chiplet] = append(dms.availablePAddrs[chiplet], addr)
}

func (dms *devicePartitionedMemoryState) popNextAvailablePAddrs() uint64 {
	if dms.noAvailablePAddrs() {
		panic("out of memory")
	}

	// Baseline policy: allocate pages round-robin across chiplets, one page at a
	// time. This intentionally creates a "striped" data-page baseline for
	// comparison against plan/LASP placement.
	if dms.numChiplets <= 0 {
		for i := 0; i < len(dms.availablePAddrs); i++ {
			if len(dms.availablePAddrs[i]) > 0 {
				pAddr := dms.availablePAddrs[i][0]
				dms.availablePAddrs[i] = dms.availablePAddrs[i][1:]
				return pAddr
			}
		}
		panic("out of memory")
	}

	start := dms.nextPartitionToAlloc % dms.numChiplets
	for i := 0; i < dms.numChiplets; i++ {
		chiplet := (start + i) % dms.numChiplets
		if len(dms.availablePAddrs[chiplet]) == 0 {
			continue
		}
		pAddr := dms.availablePAddrs[chiplet][0]
		dms.availablePAddrs[chiplet] = dms.availablePAddrs[chiplet][1:]
		dms.nextPartitionToAlloc = (chiplet + 1) % dms.numChiplets
		return pAddr
	}

	panic("out of memory")
}

func (dms *devicePartitionedMemoryState) noAvailablePAddrs() bool {
	for i := 0; i < dms.numChiplets; i++ {
		if len(dms.availablePAddrs[i]) > 0 {
			return false
		}
	}
	return true
}

func (dms *devicePartitionedMemoryState) allocateMultiplePages(numPages int) (pAddrs []uint64) {
	pAddrs = make([]uint64, 0, numPages)
	for i := 0; i < numPages; i++ {
		pAddrs = append(pAddrs, dms.popNextAvailablePAddrs())
	}
	return pAddrs
}

func (dms *devicePartitionedMemoryState) allocatePageOnChiplet(chiplet int) uint64 {
	if chiplet < 0 || chiplet >= dms.numChiplets {
		panic(fmt.Sprintf("invalid chiplet id %d (numChiplets=%d)", chiplet, dms.numChiplets))
	}

	if len(dms.availablePAddrs[chiplet]) == 0 {
		panic(fmt.Sprintf("out of memory on chiplet %d", chiplet))
	}

	pAddr := dms.availablePAddrs[chiplet][0]
	dms.availablePAddrs[chiplet] = dms.availablePAddrs[chiplet][1:]
	return pAddr
}

func (dms *devicePartitionedMemoryState) chipletOfPAddr(pAddr uint64) int {
	if pAddr < dms.initialAddress {
		return 0
	}

	pageIndex := (pAddr - dms.initialAddress) >> dms.log2PageSize
	totalBanks := uint64(dms.numChiplets * dms.numBanksPerChiplet)
	if totalBanks == 0 {
		return 0
	}

	bankIndex := pageIndex % totalBanks
	chiplet := bankIndex / uint64(dms.numBanksPerChiplet)
	return int(chiplet)
}
