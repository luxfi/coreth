// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extras

import (
	"fmt"
	"reflect"

	"github.com/luxfi/coreth/utils"
	ethparams "github.com/luxfi/geth/params"
	"github.com/luxfi/node/upgrade"
)

// NetworkUpgrades tracks the timestamps of Lux network upgrades.
//
// For Lux mainnet, all upgrades through Durango are already active.
// Only Etna, Fortuna, and Granite may be scheduled for future activation.
//
// Legacy Apricot phases (1-4) are always considered active.
// For each upgrade, a nil value means the fork hasn't happened and is not
// scheduled. A pointer to 0 means the fork has already activated.
type NetworkUpgrades struct {
	// Banff restricts import/export transactions to LUX.
	BanffBlockTimestamp *uint64 `json:"banffBlockTimestamp,omitempty"`
	// Cortina increases the block gas limit to 15M.
	CortinaBlockTimestamp *uint64 `json:"cortinaBlockTimestamp,omitempty"`
	// Durango activates Lux Warp Messaging and the Shanghai Execution
	// Spec Upgrade.
	DurangoBlockTimestamp *uint64 `json:"durangoBlockTimestamp,omitempty"`
	// Etna activates Cancun and reduces the min base fee.
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
	var rewindTo uint64
	if storedtime != nil {
		rewindTo = *storedtime
	}
	return &ethparams.ConfigCompatError{
		What:         what,
		StoredTime:   storedtime,
		NewTime:      newtime,
		RewindToTime: rewindTo,
	}
}

func (n *NetworkUpgrades) CheckNetworkUpgradesCompatible(newcfg *NetworkUpgrades, time uint64) *ethparams.ConfigCompatError {
	if isForkTimestampIncompatible(n.BanffBlockTimestamp, newcfg.BanffBlockTimestamp, time) {
		return newTimestampCompatError("Banff fork block timestamp", n.BanffBlockTimestamp, newcfg.BanffBlockTimestamp)
	}
	if isForkTimestampIncompatible(n.CortinaBlockTimestamp, newcfg.CortinaBlockTimestamp, time) {
		return newTimestampCompatError("Cortina fork block timestamp", n.CortinaBlockTimestamp, newcfg.CortinaBlockTimestamp)
	}
	if isForkTimestampIncompatible(n.DurangoBlockTimestamp, newcfg.DurangoBlockTimestamp, time) {
		return newTimestampCompatError("Durango fork block timestamp", n.DurangoBlockTimestamp, newcfg.DurangoBlockTimestamp)
	}
	if isForkTimestampIncompatible(n.EtnaTimestamp, newcfg.EtnaTimestamp, time) {
		return newTimestampCompatError("Etna fork timestamp", n.EtnaTimestamp, newcfg.EtnaTimestamp)
	}
	if isForkTimestampIncompatible(n.FortunaTimestamp, newcfg.FortunaTimestamp, time) {
		return newTimestampCompatError("Fortuna fork timestamp", n.FortunaTimestamp, newcfg.FortunaTimestamp)
	}
	if isForkTimestampIncompatible(n.GraniteTimestamp, newcfg.GraniteTimestamp, time) {
		return newTimestampCompatError("Granite fork timestamp", n.GraniteTimestamp, newcfg.GraniteTimestamp)
	}
	return nil
}

func (n *NetworkUpgrades) forkOrder() []fork {
	return []fork{
		{name: "banffBlockTimestamp", timestamp: n.BanffBlockTimestamp},
		{name: "cortinaBlockTimestamp", timestamp: n.CortinaBlockTimestamp},
		{name: "durangoBlockTimestamp", timestamp: n.DurangoBlockTimestamp},
		{name: "etnaTimestamp", timestamp: n.EtnaTimestamp},
		{name: "fortunaTimestamp", timestamp: n.FortunaTimestamp},
		{name: "graniteTimestamp", timestamp: n.GraniteTimestamp},
	}
}

// Legacy Apricot phase checks - always return true for mainnet compatibility
// These phases are all active on Lux mainnet.

func (n NetworkUpgrades) IsApricotPhase1(time uint64) bool     { return true }
func (n NetworkUpgrades) IsApricotPhase2(time uint64) bool     { return true }
func (n *NetworkUpgrades) IsApricotPhase3(time uint64) bool    { return true }
func (n NetworkUpgrades) IsApricotPhase4(time uint64) bool     { return true }
func (n NetworkUpgrades) IsApricotPhase5(time uint64) bool     { return true }
func (n NetworkUpgrades) IsApricotPhasePre6(time uint64) bool  { return true }
func (n NetworkUpgrades) IsApricotPhase6(time uint64) bool     { return true }
func (n NetworkUpgrades) IsApricotPhasePost6(time uint64) bool { return true }

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
	banner += " - Apricot Phases 1-6:               Always active (mainnet)\n"
	banner += fmt.Sprintf(" - Banff Timestamp:                  @%-10v\n", ptrToString(n.BanffBlockTimestamp))
	banner += fmt.Sprintf(" - Cortina Timestamp:                @%-10v\n", ptrToString(n.CortinaBlockTimestamp))
	banner += fmt.Sprintf(" - Durango Timestamp:                @%-10v\n", ptrToString(n.DurangoBlockTimestamp))
	banner += fmt.Sprintf(" - Etna Timestamp:                   @%-10v\n", ptrToString(n.EtnaTimestamp))
	banner += fmt.Sprintf(" - Fortuna Timestamp:                @%-10v\n", ptrToString(n.FortunaTimestamp))
	banner += fmt.Sprintf(" - Granite Timestamp:                @%-10v\n", ptrToString(n.GraniteTimestamp))
	return banner
}

// GetNetworkUpgrades converts an upgrade.Config to NetworkUpgrades.
func GetNetworkUpgrades(agoUpgrade upgrade.Config) NetworkUpgrades {
	return NetworkUpgrades{
		BanffBlockTimestamp:   utils.TimeToNewUint64(agoUpgrade.BanffTime),
		CortinaBlockTimestamp: utils.TimeToNewUint64(agoUpgrade.CortinaTime),
		DurangoBlockTimestamp: utils.TimeToNewUint64(agoUpgrade.DurangoTime),
		EtnaTimestamp:         utils.TimeToNewUint64(agoUpgrade.EtnaTime),
		FortunaTimestamp:      utils.TimeToNewUint64(agoUpgrade.FortunaTime),
		GraniteTimestamp:      utils.TimeToNewUint64(agoUpgrade.GraniteTime),
	}
}

// LuxRules contains the chain rules for a given block.
type LuxRules struct {
	// Legacy flags - always true for mainnet
	IsApricotPhase1, IsApricotPhase2, IsApricotPhase3, IsApricotPhase4, IsApricotPhase5 bool
	IsApricotPhasePre6, IsApricotPhase6, IsApricotPhasePost6                            bool

	// Active upgrade flags
	IsBanff   bool
	IsCortina bool
	IsDurango bool
	IsEtna    bool
	IsFortuna bool
	IsGranite bool

	// IsGenesis is set to true when processing historic Lux mainnet blocks
	// that were created with the original EVM (before network upgrades).
	// Genesis blocks have dynamic fees (base fee in header) but special handling.
	// This flag is set dynamically based on block context, not from config.
	IsGenesis bool
}

func (n *NetworkUpgrades) GetLuxRules(timestamp uint64) LuxRules {
	return LuxRules{
		// Legacy phases always active
		IsApricotPhase1:     true,
		IsApricotPhase2:     true,
		IsApricotPhase3:     true,
		IsApricotPhase4:     true,
		IsApricotPhase5:     true,
		IsApricotPhasePre6:  true,
		IsApricotPhase6:     true,
		IsApricotPhasePost6: true,
		// Scheduled upgrades
		IsBanff:   n.IsBanff(timestamp),
		IsCortina: n.IsCortina(timestamp),
		IsDurango: n.IsDurango(timestamp),
		IsEtna:    n.IsEtna(timestamp),
		IsFortuna: n.IsFortuna(timestamp),
		IsGranite: n.IsGranite(timestamp),
	}
}

func ptrToString(val *uint64) string {
	if val == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *val)
}
