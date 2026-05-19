// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extras

import (
	"encoding/json"
	"fmt"
	"math/big"

	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/geth/common"
	ethparams "github.com/luxfi/geth/params"
)

var (
	// TestChainConfig is the canonical Lux C-Chain config with every upgrade
	// active from genesis. Under activate-all-implicitly there is no difference
	// between a "test" and a "main" config: callers that need a chain config
	// get this.
	TestChainConfig = &ChainConfig{}

	// MainnetChainConfig is an alias for TestChainConfig; the production
	// chain runs the same activate-all-implicitly rule set.
	MainnetChainConfig = TestChainConfig

	// GenesisChainConfig is retained as the same canonical config; the
	// activate-all-implicitly directive deletes the "blocks before fork X"
	// genesis flavor entirely.
	GenesisChainConfig = TestChainConfig
)

// UpgradeConfig includes the following configs that may be specified in upgradeBytes:
// - Timestamps that enable lux network upgrades,
// - Enabling or disabling precompiles as network upgrades.
type UpgradeConfig struct {
	// Config for enabling and disabling precompiles as network upgrades.
	PrecompileUpgrades []PrecompileUpgrade `json:"precompileUpgrades,omitempty"`
}

// LuxContext provides Lux specific context directly into the EVM.
type LuxContext struct {
	ConsensusCtx *consensusctx.Context
}

type ChainConfig struct {
	LuxContext `json:"-"` // Lux specific context set during VM initialization. Not serialized.

	UpgradeConfig `json:"-"` // Config specified in upgradeBytes (lux network upgrades or enable/disabling precompiles). Not serialized.
}

func (c *ChainConfig) CheckConfigCompatible(newcfg_ *ethparams.ChainConfig, headNumber *big.Int, headTimestamp uint64) *ethparams.ConfigCompatError {
	if c == nil {
		return nil
	}
	return nil
}

func (c *ChainConfig) Description() string {
	if c == nil {
		return ""
	}
	upgradeConfigBytes, err := json.Marshal(c.UpgradeConfig)
	if err != nil {
		upgradeConfigBytes = []byte("cannot marshal UpgradeConfig")
	}
	return fmt.Sprintf("Lux Upgrades: all active from genesis.\nUpgrade Config: %s\n", string(upgradeConfigBytes))
}

// isForkTimestampIncompatible returns true if a fork scheduled at timestamp s1
// cannot be rescheduled to timestamp s2 because head is already past the fork.
func isForkTimestampIncompatible(s1, s2 *uint64, head uint64) bool {
	return (isTimestampForked(s1, head) || isTimestampForked(s2, head)) && !configTimestampEqual(s1, s2)
}

// isTimestampForked returns whether a fork scheduled at timestamp s is active
// at the given head timestamp.
func isTimestampForked(s *uint64, head uint64) bool {
	if s == nil {
		return false
	}
	return *s <= head
}

func configTimestampEqual(x, y *uint64) bool {
	if x == nil {
		return y == nil
	}
	if y == nil {
		return x == nil
	}
	return *x == *y
}

// UnmarshalJSON parses the JSON-encoded data and stores the result in the
// object pointed to by c.
func (c *ChainConfig) UnmarshalJSON(data []byte) error {
	type _ChainConfigExtra ChainConfig
	tmp := _ChainConfigExtra{}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*c = ChainConfig(tmp)
	return nil
}

// MarshalJSON returns the JSON encoding of c.
func (c *ChainConfig) MarshalJSON() ([]byte, error) {
	type _ChainConfigExtra ChainConfig
	return json.Marshal(_ChainConfigExtra(*c))
}

// CheckConfigForkOrder is a no-op under activate-all-implicitly: there is no
// ordering to enforce because every upgrade is live from genesis.
func (c *ChainConfig) CheckConfigForkOrder() error { return nil }

// Verify verifies chain config.
func (c *ChainConfig) Verify() error {
	if err := c.verifyPrecompileUpgrades(); err != nil {
		return fmt.Errorf("invalid precompile upgrades: %w", err)
	}
	return nil
}

// IsPrecompileEnabled returns whether precompile with `address` is enabled at `timestamp`.
func (c *ChainConfig) IsPrecompileEnabled(address common.Address, timestamp uint64) bool {
	config := c.GetActivePrecompileConfig(address, timestamp)
	return config != nil && !config.IsDisabled()
}

// IsForkTransition returns true if `fork` activates during the transition from
// `parent` to `current`.
func IsForkTransition(fork *uint64, parent *uint64, current uint64) bool {
	var parentForked bool
	if parent != nil {
		parentForked = isTimestampForked(fork, *parent)
	}
	currentForked := isTimestampForked(fork, current)
	return !parentForked && currentForked
}
