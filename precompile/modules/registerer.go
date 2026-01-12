// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package modules

import (
	"fmt"
	"sort"

	"github.com/luxfi/constants"
	"github.com/luxfi/coreth/utils"
	"github.com/luxfi/geth/common"
)

var (
	// registeredModules is a list of Module to preserve order
	// for deterministic iteration
	registeredModules = make([]Module, 0)

	// Reserved address ranges for stateful precompiles
	//
	// HIGH-BYTE RANGES (canonical/historic addresses):
	// 0x0100-0x01FF: Warp/Teleport messaging (unused, kept for compatibility)
	// 0x0200-0x02FF: Chain config (AllowLists, FeeManager, Warp at 0x0200...0005)
	// 0x0300-0x03FF: AI Mining
	//
	// LP-ALIGNED ADDRESSING (LP-9015):
	// BASE = 0x10000, address = BASE + (P << 12) | (C << 8) | II
	// P = Family (LP range first digit), C = Chain slot, II = Item index
	//
	// Family Pages (P nibble aligns with LP-xxxx first digit):
	// P=0: 0x10000-0x10FFF - Core/Network (AllowLists, Minting, Rewards)
	// P=2: 0x12000-0x12FFF - LP-2xxx (Q-Chain, PQ Identity)
	// P=3: 0x13000-0x13FFF - LP-3xxx (C-Chain, EVM/Crypto)
	// P=4: 0x14000-0x14FFF - LP-4xxx (Z-Chain, Privacy/ZK)
	// P=5: 0x15000-0x15FFF - LP-5xxx (T-Chain, Threshold/MPC)
	// P=6: 0x16000-0x16FFF - LP-6xxx (B-Chain, Bridges)
	// P=7: 0x17000-0x17FFF - LP-7xxx (A-Chain, AI)
	// P=9: 0x19000-0x19FFF - LP-9xxx (DEX/Markets)
	//
	// Chain Slots (C nibble):
	// 0=P-Chain, 1=X-Chain, 2=C-Chain, 3=Q-Chain, 4=A-Chain
	// 5=B-Chain, 6=Z-Chain, 7=M-Chain, 8=Zoo, 9=Hanzo, A=SPC
	reservedRanges = []utils.AddressRange{
		// HIGH-BYTE RANGES (canonical addresses for backward compatibility)
		// Warp/Teleport range (0x0100-0x01FF)
		{
			Start: common.HexToAddress("0x0100000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x01000000000000000000000000000000000000ff"),
		},
		// Chain Config range (0x0200-0x02FF) - includes canonical Warp at 0x0200...0005
		{
			Start: common.HexToAddress("0x0200000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x02000000000000000000000000000000000000ff"),
		},
		// AI Mining range (0x0300-0x03FF)
		{
			Start: common.HexToAddress("0x0300000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x03000000000000000000000000000000000000ff"),
		},
		// LP-aligned precompile range: 0x10000-0x1FFFF
		{
			Start: common.HexToAddress("0x0000000000000000000000000000000000010000"),
			End:   common.HexToAddress("0x000000000000000000000000000000000001ffff"),
		},
	}
)

// ReservedAddress returns true if [addr] is in a reserved range for custom precompiles
func ReservedAddress(addr common.Address) bool {
	for _, reservedRange := range reservedRanges {
		if reservedRange.Contains(addr) {
			return true
		}
	}

	return false
}

// RegisterModule registers a stateful precompile module
func RegisterModule(stm Module) error {
	address := stm.Address
	key := stm.ConfigKey

	if address == constants.BlackholeAddr {
		return fmt.Errorf("address %s overlaps with blackhole address", address)
	}
	if !ReservedAddress(address) {
		return fmt.Errorf("address %s not in a reserved range", address)
	}

	for _, registeredModule := range registeredModules {
		if registeredModule.ConfigKey == key {
			return fmt.Errorf("name %s already used by a stateful precompile", key)
		}
		if registeredModule.Address == address {
			return fmt.Errorf("address %s already used by a stateful precompile", address)
		}
	}
	// sort by address to ensure deterministic iteration
	registeredModules = insertSortedByAddress(registeredModules, stm)
	return nil
}

func GetPrecompileModuleByAddress(address common.Address) (Module, bool) {
	for _, stm := range registeredModules {
		if stm.Address == address {
			return stm, true
		}
	}
	return Module{}, false
}

func GetPrecompileModule(key string) (Module, bool) {
	for _, stm := range registeredModules {
		if stm.ConfigKey == key {
			return stm, true
		}
	}
	return Module{}, false
}

func RegisteredModules() []Module {
	return registeredModules
}

func insertSortedByAddress(data []Module, stm Module) []Module {
	data = append(data, stm)
	sort.Sort(moduleArray(data))
	return data
}
