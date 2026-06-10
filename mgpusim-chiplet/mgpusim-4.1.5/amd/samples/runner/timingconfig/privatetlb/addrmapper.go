package privatetlb

import (
	"fmt"

	"github.com/sarchlab/akita/v4/sim"
)

// StripedAddressPortMapper maps an address to a port based on striped
// interleaving across NumBanks banks. The mapping is computed after subtracting
// MemAddrOffset.
//
// bankIndex = floor(((addr - MemAddrOffset) mod (NumBanks*Striping)) / Striping)
//
// If UseAddressSpaceLimitation is enabled, addresses outside [LowAddress,
// HighAddress) are routed to ModuleForOtherAddresses.
type StripedAddressPortMapper struct {
	MemAddrOffset uint64
	NumBanks      uint64
	Striping      uint64
	LowModules    []sim.RemotePort

	UseAddressSpaceLimitation bool
	LowAddress                uint64
	HighAddress               uint64
	ModuleForOtherAddresses   sim.RemotePort
}

func (m *StripedAddressPortMapper) Find(address uint64) sim.RemotePort {
	if m.UseAddressSpaceLimitation {
		if address < m.LowAddress || address >= m.HighAddress {
			return m.ModuleForOtherAddresses
		}
	}

	if m.NumBanks == 0 || m.Striping == 0 {
		panic("StripedAddressPortMapper: NumBanks and Striping must be non-zero")
	}
	if len(m.LowModules) != int(m.NumBanks) {
		panic(fmt.Sprintf(
			"StripedAddressPortMapper: LowModules length (%d) != NumBanks (%d)",
			len(m.LowModules), m.NumBanks,
		))
	}
	if address < m.MemAddrOffset {
		return m.ModuleForOtherAddresses
	}

	rel := address - m.MemAddrOffset
	idx := (rel % (m.NumBanks * m.Striping)) / m.Striping
	return m.LowModules[idx]
}

// StripedLocalVRemoteAddressPortMapper maps an address to:
//   - a local bank module (LocalModules) if the bank belongs to this chiplet
//   - ModuleForRemoteBanks otherwise
//
// It is used for L1-to-L2 routing inside a chiplet: local banks go to local L2
// caches, remote banks go to the chiplet RDMA engine.
//
// If UseAddressSpaceLimitation is enabled and address is outside [LowAddress,
// HighAddress), the mapper returns ModuleForOutsideRange.
type StripedLocalVRemoteAddressPortMapper struct {
	MemAddrOffset uint64
	NumBanks      uint64
	Striping      uint64

	LocalBankStart uint64
	LocalModules   []sim.RemotePort

	ModuleForRemoteBanks  sim.RemotePort
	ModuleForOutsideRange sim.RemotePort

	UseAddressSpaceLimitation bool
	LowAddress                uint64
	HighAddress               uint64
}

func (m *StripedLocalVRemoteAddressPortMapper) Find(address uint64) sim.RemotePort {
	if m.UseAddressSpaceLimitation {
		if address < m.LowAddress || address >= m.HighAddress {
			return m.ModuleForOutsideRange
		}
	}

	if m.NumBanks == 0 || m.Striping == 0 {
		panic("StripedLocalVRemoteAddressPortMapper: NumBanks and Striping must be non-zero")
	}
	if address < m.MemAddrOffset {
		return m.ModuleForOutsideRange
	}

	rel := address - m.MemAddrOffset
	idx := (rel % (m.NumBanks * m.Striping)) / m.Striping

	localIdx := int(idx - m.LocalBankStart)
	if localIdx >= 0 && localIdx < len(m.LocalModules) {
		return m.LocalModules[localIdx]
	}
	return m.ModuleForRemoteBanks
}

// StripedLocalBankAddressPortMapper maps an address to a local-bank module.
// It assumes the incoming address belongs to the local bank range.
//
// This is used by a chiplet's RDMA engine to forward remote requests received
// from other chiplets to the correct local L2 cache.
type StripedLocalBankAddressPortMapper struct {
	MemAddrOffset uint64
	NumBanks      uint64
	Striping      uint64

	LocalBankStart uint64
	LocalModules   []sim.RemotePort
}

func (m *StripedLocalBankAddressPortMapper) Find(address uint64) sim.RemotePort {
	if m.NumBanks == 0 || m.Striping == 0 {
		panic("StripedLocalBankAddressPortMapper: NumBanks and Striping must be non-zero")
	}
	if address < m.MemAddrOffset {
		panic("StripedLocalBankAddressPortMapper: address < MemAddrOffset")
	}

	rel := address - m.MemAddrOffset
	idx := (rel % (m.NumBanks * m.Striping)) / m.Striping
	localIdx := idx - m.LocalBankStart

	if localIdx >= uint64(len(m.LocalModules)) {
		panic(fmt.Sprintf(
			"StripedLocalBankAddressPortMapper: bank %d out of local range start=%d len=%d",
			idx, m.LocalBankStart, len(m.LocalModules),
		))
	}

	return m.LocalModules[localIdx]
}

// StripedChipletAddressPortMapper maps an address to a chiplet module.
//
// It groups B banks per chiplet, where StripingPerChiplet = B * BankStriping.
// chipletIndex = floor(((addr - MemAddrOffset) mod (NumChiplets*StripingPerChiplet)) / StripingPerChiplet)
//
// This is used as the remote-module table of chiplet RDMA engines.
type StripedChipletAddressPortMapper struct {
	MemAddrOffset             uint64
	NumChiplets               uint64
	StripingPerChiplet        uint64
	LowModules                []sim.RemotePort
	UseAddressSpaceLimitation bool
	LowAddress                uint64
	HighAddress               uint64
	ModuleForOtherAddresses   sim.RemotePort
}

func (m *StripedChipletAddressPortMapper) Find(address uint64) sim.RemotePort {
	if m.UseAddressSpaceLimitation {
		if address < m.LowAddress || address >= m.HighAddress {
			return m.ModuleForOtherAddresses
		}
	}

	if m.NumChiplets == 0 || m.StripingPerChiplet == 0 {
		panic("StripedChipletAddressPortMapper: NumChiplets and StripingPerChiplet must be non-zero")
	}
	if len(m.LowModules) != int(m.NumChiplets) {
		panic(fmt.Sprintf(
			"StripedChipletAddressPortMapper: LowModules length (%d) != NumChiplets (%d)",
			len(m.LowModules), m.NumChiplets,
		))
	}
	if address < m.MemAddrOffset {
		return m.ModuleForOtherAddresses
	}

	rel := address - m.MemAddrOffset
	idx := (rel % (m.NumChiplets * m.StripingPerChiplet)) / m.StripingPerChiplet
	return m.LowModules[idx]
}
