// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package modules

import (
	"fmt"
	"sort"

	"github.com/luxfi/coreth/constants"
	"github.com/luxfi/coreth/utils"
	"github.com/luxfi/geth/common"
)

var (
	// registeredModules is a list of Module to preserve order
	// for deterministic iteration
	registeredModules = make([]Module, 0)

	// Reserved address ranges for stateful precompiles
	// 0x0100-0x01FF: Warp/Teleport messaging
	// 0x0200-0x02FF: Chain config (AllowLists, FeeManager, etc.)
	// 0x0300-0x03FF: AI Mining
	// 0x0400-0x04FF: DEX (Uniswap v4-style)
	// 0x0500-0x05FF: Graph/Query layer
	// 0x0600-0x06FF: Post-quantum crypto
	// 0x0700-0x07FF: Privacy/Encryption
	// 0x0800-0x08FF: Threshold signatures
	// 0x0900-0x09FF: ZK proofs
	// 0x0A00-0x0AFF: Curves (secp256r1, etc.)
	reservedRanges = []utils.AddressRange{
		// Warp/Teleport (0x0100-0x01FF)
		{
			Start: common.HexToAddress("0x0100000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x01000000000000000000000000000000000000ff"),
		},
		// Chain Config (0x0200-0x02FF)
		{
			Start: common.HexToAddress("0x0200000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x02000000000000000000000000000000000000ff"),
		},
		// AI Mining (0x0300-0x03FF)
		{
			Start: common.HexToAddress("0x0300000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x03000000000000000000000000000000000000ff"),
		},
		// DEX - Uniswap v4-style (0x0400-0x04FF)
		{
			Start: common.HexToAddress("0x0400000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x04000000000000000000000000000000000000ff"),
		},
		// Graph/Query Layer (0x0500-0x05FF)
		{
			Start: common.HexToAddress("0x0500000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x05000000000000000000000000000000000000ff"),
		},
		// Post-Quantum Crypto (0x0600-0x06FF)
		{
			Start: common.HexToAddress("0x0600000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x06000000000000000000000000000000000000ff"),
		},
		// Privacy/Encryption (0x0700-0x07FF)
		{
			Start: common.HexToAddress("0x0700000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x07000000000000000000000000000000000000ff"),
		},
		// Threshold Signatures (0x0800-0x08FF)
		{
			Start: common.HexToAddress("0x0800000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x08000000000000000000000000000000000000ff"),
		},
		// ZK Proofs (0x0900-0x09FF)
		{
			Start: common.HexToAddress("0x0900000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x09000000000000000000000000000000000000ff"),
		},
		// Curves - secp256r1, etc. (0x0A00-0x0AFF)
		{
			Start: common.HexToAddress("0x0A00000000000000000000000000000000000000"),
			End:   common.HexToAddress("0x0A000000000000000000000000000000000000ff"),
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
