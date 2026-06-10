package ptw

import (
	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/mem/vm"
	"github.com/sarchlab/akita/v4/sim"
)

// Builder builds a page table walker component.
type Builder struct {
	engine sim.Engine
	freq   sim.Freq

	pageTable     vm.PageTable
	addressMapper mem.AddressToPortMapper

	numWalkers       int
	pwcEntries       int
	pwcLookupLatency int

	// Optional chiplet layout information (for local/remote PTE read stats).
	chipletID           int
	memAddrOffset       uint64
	numChiplets         int
	numBanksPerChiplet  int
	bankInterleavingSize uint64
}

// MakeBuilder creates a builder with reasonable defaults.
func MakeBuilder() Builder {
	return Builder{
		numWalkers:       16,
		pwcEntries:       32,
		pwcLookupLatency: 10,
	}
}

func (b Builder) WithEngine(engine sim.Engine) Builder {
	b.engine = engine
	return b
}

func (b Builder) WithFreq(freq sim.Freq) Builder {
	b.freq = freq
	return b
}

func (b Builder) WithPageTable(pageTable vm.PageTable) Builder {
	b.pageTable = pageTable
	return b
}

func (b Builder) WithAddressMapper(mapper mem.AddressToPortMapper) Builder {
	b.addressMapper = mapper
	return b
}

func (b Builder) WithNumWalkers(numWalkers int) Builder {
	b.numWalkers = numWalkers
	return b
}

func (b Builder) WithPWCEntries(entries int) Builder {
	b.pwcEntries = entries
	return b
}

func (b Builder) WithPWCLookupLatency(latency int) Builder {
	b.pwcLookupLatency = latency
	return b
}

// WithChipletLayout configures the memory interleaving layout for attributing
// PTE reads as local vs remote.
//
// The PTW logic itself does not depend on this information for correctness.
func (b Builder) WithChipletLayout(
	chipletID int,
	memAddrOffset uint64,
	numChiplets int,
	numBanksPerChiplet int,
	bankInterleavingSize uint64,
) Builder {
	b.chipletID = chipletID
	b.memAddrOffset = memAddrOffset
	b.numChiplets = numChiplets
	b.numBanksPerChiplet = numBanksPerChiplet
	b.bankInterleavingSize = bankInterleavingSize
	return b
}

// Build creates a new PTW component.
func (b Builder) Build(name string) *Comp {
	c := NewComp(name, b.engine, b.freq)

	c.numWalkers = b.numWalkers
	c.pwcLookupLatency = b.pwcLookupLatency
	c.chipletID = b.chipletID
	c.memAddrOffset = b.memAddrOffset
	c.numChiplets = b.numChiplets
	c.numBanksPerChiplet = b.numBanksPerChiplet
	c.bankInterleavingSize = b.bankInterleavingSize
	c.addrMapper = b.addressMapper

	// Optional chiplet layout (for stats only).
	c.chipletID = b.chipletID
	c.memAddrOffset = b.memAddrOffset
	c.numChiplets = b.numChiplets
	c.numBanksPerChiplet = b.numBanksPerChiplet
	c.bankInterleavingSize = b.bankInterleavingSize

	// The PTW requires a radix-like page table that can provide
	// MGVM-style walk helpers.
	pt, ok := b.pageTable.(PageTable)
	if !ok {
		panic("PTW requires a radix-like PageTable implementation")
	}
	c.pageTable = pt

	c.pwc = newPWC(b.pwcEntries, pt.GetLog2PageSize(), c.bitsPerLevel, c.numLevels)
	return c
}
