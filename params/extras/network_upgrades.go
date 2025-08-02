// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extras

import (
	"fmt"
	"reflect"

	"github.com/luxfi/node/upgrade"
	ethparams "github.com/luxfi/geth/params"
)

// NetworkUpgrades tracks the timestamps of all the Lux upgrades.
//
// For each upgrade, a nil value means the fork hasn't happened and is not
// scheduled. A pointer to 0 means the fork has already activated.
type NetworkUpgrades struct {
	ApricotPhase1BlockTimestamp *uint64 `json:"apricotPhase1BlockTimestamp,omitempty"` // Apricot Phase 1 Block Timestamp
	// Apricot Phase 2 Block Timestamp includes a modified version of the Berlin
	// Hard Fork.
	ApricotPhase2BlockTimestamp *uint64 `json:"apricotPhase2BlockTimestamp,omitempty"`
	// Apricot Phase 3 introduces dynamic fees and a modified version of the
	// London Hard Fork.
	ApricotPhase3BlockTimestamp *uint64 `json:"apricotPhase3BlockTimestamp,omitempty"`
	// Apricot Phase 4 introduces the notion of a block fee to the dynamic fee
	// algorithm.
	ApricotPhase4BlockTimestamp *uint64 `json:"apricotPhase4BlockTimestamp,omitempty"`
	// Apricot Phase 5 introduces a batch of atomic transactions with a maximum
	// atomic gas limit per block.
	ApricotPhase5BlockTimestamp *uint64 `json:"apricotPhase5BlockTimestamp,omitempty"`
	// Apricot Phase Pre-6 deprecates the NativeAssetCall precompile (soft).
	ApricotPhasePre6BlockTimestamp *uint64 `json:"apricotPhasePre6BlockTimestamp,omitempty"`
	// Apricot Phase 6 deprecates the NativeAssetBalance and NativeAssetCall
	// precompiles.
	ApricotPhase6BlockTimestamp *uint64 `json:"apricotPhase6BlockTimestamp,omitempty"`
	// Apricot Phase Post-6 deprecates the NativeAssetCall precompile (soft).
	ApricotPhasePost6BlockTimestamp *uint64 `json:"apricotPhasePost6BlockTimestamp,omitempty"`
	// Banff restricts import/export transactions to LUX.
	BanffBlockTimestamp *uint64 `json:"banffBlockTimestamp,omitempty"`
	// Cortina increases the block gas limit to 15M.
	CortinaBlockTimestamp *uint64 `json:"cortinaBlockTimestamp,omitempty"`
	// Durango activates Lux Warp Messaging and the Shanghai Execution
	// Spec Upgrade (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/shanghai.md#included-eips).
	//
	// Note: EIP-4895 is excluded since withdrawals are not relevant to the
	// Lux C-Chain or Subnets running the EVM.
	DurangoBlockTimestamp *uint64 `json:"durangoBlockTimestamp,omitempty"`
	// Etna activates Cancun (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/cancun.md#included-eips)
	// and reduces the min base fee.
	// Note: EIP-4844 BlobTxs are not enabled in the mempool and blocks are not
	// allowed to contain them. For details see https://github.com/lux-foundation/ACPs/pull/131
	EtnaTimestamp *uint64 `json:"etnaTimestamp,omitempty"`
	// Fortuna modifies the gas price mechanism based on ACP-176
	FortunaTimestamp *uint64 `json:"fortunaTimestamp,omitempty"`
	// Granite is a placeholder for the next upgrade.
	GraniteTimestamp *uint64 `json:"graniteTimestamp,omitempty"`
}

func (n *NetworkUpgrades) Equal(other *NetworkUpgrades) bool {
	return reflect.DeepEqual(n, other)
}

// newTimestampCompatError creates a timestamp compatibility error
func newTimestampCompatError(what string, storedtime, newtime *uint64) *ethparams.ConfigCompatError {
	return &ethparams.ConfigCompatError{
		What: what,
		// Since all upgrades are always activated, we don't need to check compatibility
		// Just return a minimal error for the interface
	}
}

func (n *NetworkUpgrades) checkNetworkUpgradesCompatible(newcfg *NetworkUpgrades, time uint64) *ethparams.ConfigCompatError {
	// For now, all upgrades are considered compatible
	// Keep this structure as a template for future network upgrades
	
	// Example of how to check for a future upgrade:
	// if isForkTimestampIncompatible(n.FutureUpgradeTimestamp, newcfg.FutureUpgradeTimestamp, time) {
	//     return newTimestampCompatError("Future upgrade fork block timestamp", n.FutureUpgradeTimestamp, newcfg.FutureUpgradeTimestamp)
	// }
	
	return nil
}

func (n *NetworkUpgrades) forkOrder() []fork {
	return []fork{
		{name: "apricotPhase1BlockTimestamp", timestamp: n.ApricotPhase1BlockTimestamp},
		{name: "apricotPhase2BlockTimestamp", timestamp: n.ApricotPhase2BlockTimestamp},
		{name: "apricotPhase3BlockTimestamp", timestamp: n.ApricotPhase3BlockTimestamp},
		{name: "apricotPhase4BlockTimestamp", timestamp: n.ApricotPhase4BlockTimestamp},
		{name: "apricotPhase5BlockTimestamp", timestamp: n.ApricotPhase5BlockTimestamp},
		{name: "apricotPhasePre6BlockTimestamp", timestamp: n.ApricotPhasePre6BlockTimestamp},
		{name: "apricotPhase6BlockTimestamp", timestamp: n.ApricotPhase6BlockTimestamp},
		{name: "apricotPhasePost6BlockTimestamp", timestamp: n.ApricotPhasePost6BlockTimestamp},
		{name: "banffBlockTimestamp", timestamp: n.BanffBlockTimestamp},
		{name: "cortinaBlockTimestamp", timestamp: n.CortinaBlockTimestamp},
		{name: "durangoBlockTimestamp", timestamp: n.DurangoBlockTimestamp},
		{name: "etnaTimestamp", timestamp: n.EtnaTimestamp},
		{name: "fortunaTimestamp", timestamp: n.FortunaTimestamp},
		{name: "graniteTimestamp", timestamp: n.GraniteTimestamp},
	}
}

// IsApricotPhase1 returns whether [time] represents a block
// with a timestamp after the Apricot Phase 1 upgrade time.
func (n NetworkUpgrades) IsApricotPhase1(time uint64) bool {
	return isTimestampForked(n.ApricotPhase1BlockTimestamp, time)
}

// IsApricotPhase2 returns whether [time] represents a block
// with a timestamp after the Apricot Phase 2 upgrade time.
func (n NetworkUpgrades) IsApricotPhase2(time uint64) bool {
	return isTimestampForked(n.ApricotPhase2BlockTimestamp, time)
}

// IsApricotPhase3 returns whether [time] represents a block
// with a timestamp after the Apricot Phase 3 upgrade time.
func (n *NetworkUpgrades) IsApricotPhase3(time uint64) bool {
	return isTimestampForked(n.ApricotPhase3BlockTimestamp, time)
}

// IsApricotPhase4 returns whether [time] represents a block
// with a timestamp after the Apricot Phase 4 upgrade time.
func (n NetworkUpgrades) IsApricotPhase4(time uint64) bool {
	return isTimestampForked(n.ApricotPhase4BlockTimestamp, time)
}

// IsApricotPhase5 returns whether [time] represents a block
// with a timestamp after the Apricot Phase 5 upgrade time.
func (n NetworkUpgrades) IsApricotPhase5(time uint64) bool {
	return isTimestampForked(n.ApricotPhase5BlockTimestamp, time)
}

// IsApricotPhasePre6 returns whether [time] represents a block
// with a timestamp after the Apricot Phase Pre 6 upgrade time.
func (n NetworkUpgrades) IsApricotPhasePre6(time uint64) bool {
	return isTimestampForked(n.ApricotPhasePre6BlockTimestamp, time)
}

// IsApricotPhase6 returns whether [time] represents a block
// with a timestamp after the Apricot Phase 6 upgrade time.
func (n NetworkUpgrades) IsApricotPhase6(time uint64) bool {
	return isTimestampForked(n.ApricotPhase6BlockTimestamp, time)
}

// IsApricotPhasePost6 returns whether [time] represents a block
// with a timestamp after the Apricot Phase 6 Post upgrade time.
func (n NetworkUpgrades) IsApricotPhasePost6(time uint64) bool {
	return isTimestampForked(n.ApricotPhasePost6BlockTimestamp, time)
}

// IsBanff returns whether [time] represents a block
// with a timestamp after the Banff upgrade time.
func (n NetworkUpgrades) IsBanff(time uint64) bool {
	return isTimestampForked(n.BanffBlockTimestamp, time)
}

// IsCortina returns whether [time] represents a block
// with a timestamp after the Cortina upgrade time.
func (n NetworkUpgrades) IsCortina(time uint64) bool {
	return isTimestampForked(n.CortinaBlockTimestamp, time)
}

// IsDurango returns whether [time] represents a block
// with a timestamp after the Durango upgrade time.
func (n NetworkUpgrades) IsDurango(time uint64) bool {
	return isTimestampForked(n.DurangoBlockTimestamp, time)
}

// IsEtna returns whether [time] represents a block
// with a timestamp after the Etna upgrade time.
func (n NetworkUpgrades) IsEtna(time uint64) bool {
	return isTimestampForked(n.EtnaTimestamp, time)
}

// IsFortuna returns whether [time] represents a block
// with a timestamp after the Fortuna upgrade time.
func (n *NetworkUpgrades) IsFortuna(time uint64) bool {
	return isTimestampForked(n.FortunaTimestamp, time)
}

// IsGranite returns whether [time] represents a block
// with a timestamp after the Granite upgrade time.
func (n *NetworkUpgrades) IsGranite(time uint64) bool {
	return isTimestampForked(n.GraniteTimestamp, time)
}

func (n NetworkUpgrades) Description() string {
	var banner string
	banner += fmt.Sprintf(" - Apricot Phase 1 Timestamp:        @%-10v (https://github.com/luxfi/node/releases/tag/v1.3.0)\n", ptrToString(n.ApricotPhase1BlockTimestamp))
	banner += fmt.Sprintf(" - Apricot Phase 2 Timestamp:        @%-10v (https://github.com/luxfi/node/releases/tag/v1.4.0)\n", ptrToString(n.ApricotPhase2BlockTimestamp))
	banner += fmt.Sprintf(" - Apricot Phase 3 Timestamp:        @%-10v (https://github.com/luxfi/node/releases/tag/v1.5.0)\n", ptrToString(n.ApricotPhase3BlockTimestamp))
	banner += fmt.Sprintf(" - Apricot Phase 4 Timestamp:        @%-10v (https://github.com/luxfi/node/releases/tag/v1.6.0)\n", ptrToString(n.ApricotPhase4BlockTimestamp))
	banner += fmt.Sprintf(" - Apricot Phase 5 Timestamp:        @%-10v (https://github.com/luxfi/node/releases/tag/v1.7.0)\n", ptrToString(n.ApricotPhase5BlockTimestamp))
	banner += fmt.Sprintf(" - Apricot Phase P6 Timestamp        @%-10v (https://github.com/luxfi/node/releases/tag/v1.8.0)\n", ptrToString(n.ApricotPhasePre6BlockTimestamp))
	banner += fmt.Sprintf(" - Apricot Phase 6 Timestamp:        @%-10v (https://github.com/luxfi/node/releases/tag/v1.8.0)\n", ptrToString(n.ApricotPhase6BlockTimestamp))
	banner += fmt.Sprintf(" - Apricot Phase Post-6 Timestamp:   @%-10v (https://github.com/luxfi/node/releases/tag/v1.8.0\n", ptrToString(n.ApricotPhasePost6BlockTimestamp))
	banner += fmt.Sprintf(" - Banff Timestamp:                  @%-10v (https://github.com/luxfi/node/releases/tag/v1.9.0)\n", ptrToString(n.BanffBlockTimestamp))
	banner += fmt.Sprintf(" - Cortina Timestamp:                @%-10v (https://github.com/luxfi/node/releases/tag/v1.10.0)\n", ptrToString(n.CortinaBlockTimestamp))
	banner += fmt.Sprintf(" - Durango Timestamp:                @%-10v (https://github.com/luxfi/node/releases/tag/v1.11.0)\n", ptrToString(n.DurangoBlockTimestamp))
	banner += fmt.Sprintf(" - Etna Timestamp:                   @%-10v (https://github.com/luxfi/node/releases/tag/v1.12.0)\n", ptrToString(n.EtnaTimestamp))
	banner += fmt.Sprintf(" - Fortuna Timestamp:                @%-10v (https://github.com/luxfi/node/releases/tag/v1.13.0)\n", ptrToString(n.FortunaTimestamp))
	banner += fmt.Sprintf(" - Granite Timestamp:                @%-10v (Unscheduled)\n", ptrToString(n.GraniteTimestamp))
	return banner
}

// GetNetworkUpgrades is temporarily disabled due to upgrade.Config API changes
// TODO: Update this function when the new upgrade.Config structure is finalized
func GetNetworkUpgrades(agoUpgrade upgrade.Config) NetworkUpgrades {
	// The upgrade.Config now only has ActivationTime field
	// Previous fields like ApricotPhase1Time, etc. have been removed
	return NetworkUpgrades{}
	
	// Original implementation for reference:
	// return NetworkUpgrades{
	// 	ApricotPhase1BlockTimestamp:     utils.TimeToNewUint64(agoUpgrade.ApricotPhase1Time),
	// 	ApricotPhase2BlockTimestamp:     utils.TimeToNewUint64(agoUpgrade.ApricotPhase2Time),
	// 	ApricotPhase3BlockTimestamp:     utils.TimeToNewUint64(agoUpgrade.ApricotPhase3Time),
	// 	ApricotPhase4BlockTimestamp:     utils.TimeToNewUint64(agoUpgrade.ApricotPhase4Time),
	// 	ApricotPhase5BlockTimestamp:     utils.TimeToNewUint64(agoUpgrade.ApricotPhase5Time),
	// 	ApricotPhasePre6BlockTimestamp:  utils.TimeToNewUint64(agoUpgrade.ApricotPhasePre6Time),
	// 	ApricotPhase6BlockTimestamp:     utils.TimeToNewUint64(agoUpgrade.ApricotPhase6Time),
	// 	ApricotPhasePost6BlockTimestamp: utils.TimeToNewUint64(agoUpgrade.ApricotPhasePost6Time),
	// 	BanffBlockTimestamp:             utils.TimeToNewUint64(agoUpgrade.BanffTime),
	// 	CortinaBlockTimestamp:           utils.TimeToNewUint64(agoUpgrade.CortinaTime),
	// 	DurangoBlockTimestamp:           utils.TimeToNewUint64(agoUpgrade.DurangoTime),
	// 	EtnaTimestamp:                   utils.TimeToNewUint64(agoUpgrade.EtnaTime),
	// 	FortunaTimestamp:                utils.TimeToNewUint64(agoUpgrade.FortunaTime),
	// 	GraniteTimestamp:                utils.TimeToNewUint64(agoUpgrade.GraniteTime),
	// }
}

type LuxRules struct {
	IsApricotPhase1, IsApricotPhase2, IsApricotPhase3, IsApricotPhase4, IsApricotPhase5 bool
	IsApricotPhasePre6, IsApricotPhase6, IsApricotPhasePost6                            bool
	IsBanff                                                                             bool
	IsCortina                                                                           bool
	IsDurango                                                                           bool
	IsEtna                                                                              bool
	IsFortuna                                                                           bool
	IsGranite                                                                           bool
}

func (n *NetworkUpgrades) GetLuxRules(timestamp uint64) LuxRules {
	return LuxRules{
		IsApricotPhase1:     n.IsApricotPhase1(timestamp),
		IsApricotPhase2:     n.IsApricotPhase2(timestamp),
		IsApricotPhase3:     n.IsApricotPhase3(timestamp),
		IsApricotPhase4:     n.IsApricotPhase4(timestamp),
		IsApricotPhase5:     n.IsApricotPhase5(timestamp),
		IsApricotPhasePre6:  n.IsApricotPhasePre6(timestamp),
		IsApricotPhase6:     n.IsApricotPhase6(timestamp),
		IsApricotPhasePost6: n.IsApricotPhasePost6(timestamp),
		IsBanff:             n.IsBanff(timestamp),
		IsCortina:           n.IsCortina(timestamp),
		IsDurango:           n.IsDurango(timestamp),
		IsEtna:              n.IsEtna(timestamp),
		IsFortuna:           n.IsFortuna(timestamp),
		IsGranite:           n.IsGranite(timestamp),
	}
}

func ptrToString(val *uint64) string {
	if val == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *val)
}
