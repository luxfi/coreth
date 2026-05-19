// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extras

import (
	"reflect"

	ethparams "github.com/luxfi/geth/params"
	"github.com/luxfi/upgrade"
)

// NetworkUpgrades is the canonical Lux C-Chain upgrade view under
// activate-all-implicitly: every network upgrade is live from genesis. The
// struct is retained as an empty type so that callers can keep passing it
// around (and JSON consumers keep getting the same `{}` payload) without
// surfacing per-upgrade timestamp knobs that no longer exist.
type NetworkUpgrades struct{}

func (NetworkUpgrades) Equal(other *NetworkUpgrades) bool {
	return reflect.DeepEqual(NetworkUpgrades{}, *other)
}

// CheckNetworkUpgradesCompatible always succeeds: there is no upgrade ordering
// to enforce, so no compat error can be raised.
func (NetworkUpgrades) CheckNetworkUpgradesCompatible(*NetworkUpgrades, uint64) *ethparams.ConfigCompatError {
	return nil
}

func (NetworkUpgrades) forkOrder() []fork { return nil }

// All upgrade predicates collapse to always-true. The methods are kept so the
// large callsite surface in plugin/evm keeps compiling without touching every
// header / state-transition seam in the same patch.
func (NetworkUpgrades) IsApricotPhase1(uint64) bool     { return true }
func (NetworkUpgrades) IsApricotPhase2(uint64) bool     { return true }
func (NetworkUpgrades) IsApricotPhase3(uint64) bool     { return true }
func (NetworkUpgrades) IsApricotPhase4(uint64) bool     { return true }
func (NetworkUpgrades) IsApricotPhase5(uint64) bool     { return true }
func (NetworkUpgrades) IsApricotPhasePre6(uint64) bool  { return true }
func (NetworkUpgrades) IsApricotPhase6(uint64) bool     { return true }
func (NetworkUpgrades) IsApricotPhasePost6(uint64) bool { return true }
func (NetworkUpgrades) IsBanff(uint64) bool             { return true }
func (NetworkUpgrades) IsCortina(uint64) bool           { return true }
func (NetworkUpgrades) IsDurango(uint64) bool           { return true }
func (NetworkUpgrades) IsEtna(uint64) bool              { return true }
func (NetworkUpgrades) IsFortuna(uint64) bool           { return true }
func (NetworkUpgrades) IsGranite(uint64) bool           { return true }

// Description returns a constant string under activate-all-implicitly.
func (NetworkUpgrades) Description() string {
	return " - All upgrades:                     active from genesis\n"
}

// GetNetworkUpgrades is a no-op shim: any input upgrade.Config is ignored
// because every upgrade is live from genesis.
func GetNetworkUpgrades(upgrade.Config) NetworkUpgrades {
	return NetworkUpgrades{}
}

// LuxRules collapses to the single field that still carries runtime semantics
// (IsGenesis is set dynamically by header processing for pre-EVM-upgrade Lux
// mainnet blocks). All other upgrade predicates are unconditionally true.
type LuxRules struct {
	IsGenesis bool
}

// GetLuxRules returns the canonical activate-all-implicitly chain rules.
func (NetworkUpgrades) GetLuxRules(uint64) LuxRules {
	return LuxRules{}
}
