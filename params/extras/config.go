// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extras

import (
	"encoding/json"
	"fmt"
	"math/big"

	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/coreth/utils"
	"github.com/luxfi/geth/common"
	ethparams "github.com/luxfi/geth/params"
)

var (
	// TestChainConfig is the default test configuration with all upgrades active.
	// This represents mainnet behavior where all legacy upgrades are active.
	TestChainConfig = &ChainConfig{
		NetworkUpgrades: NetworkUpgrades{
			BanffBlockTimestamp:   utils.NewUint64(0),
			CortinaBlockTimestamp: utils.NewUint64(0),
			DurangoBlockTimestamp: utils.NewUint64(0),
			EtnaTimestamp:         utils.NewUint64(0),
			FortunaTimestamp:      utils.NewUint64(0),
			GraniteTimestamp:      utils.NewUint64(0),
		},
	}

	// GenesisChainConfig is for testing genesis block behavior.
	// Only has Banff+ upgrades scheduled at future timestamps.
	GenesisChainConfig = &ChainConfig{
		NetworkUpgrades: NetworkUpgrades{
			BanffBlockTimestamp:   utils.NewUint64(1000),
			CortinaBlockTimestamp: utils.NewUint64(2000),
			DurangoBlockTimestamp: utils.NewUint64(3000),
			EtnaTimestamp:         utils.NewUint64(4000),
			FortunaTimestamp:      utils.NewUint64(5000),
			GraniteTimestamp:      utils.NewUint64(6000),
		},
	}

	// MainnetChainConfig represents Lux mainnet configuration.
	// All legacy upgrades are active, with modern upgrades scheduled.
	MainnetChainConfig = TestChainConfig

	// Legacy test configs - all point to TestChainConfig for backward compatibility
	// These are kept for tests that still reference them.
	TestBanffChainConfig   = TestChainConfig
	TestCortinaChainConfig = TestChainConfig
	TestDurangoChainConfig = TestChainConfig
	TestEtnaChainConfig    = TestChainConfig
	TestFortunaChainConfig = TestChainConfig
	TestGraniteChainConfig = TestChainConfig
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
	NetworkUpgrades // Config for timestamps that enable network upgrades.

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
	var banner string

	banner += "Lux Upgrades (timestamp based):\n"
	banner += c.NetworkUpgrades.Description()
	banner += "\n"

	upgradeConfigBytes, err := json.Marshal(c.UpgradeConfig)
	if err != nil {
		upgradeConfigBytes = []byte("cannot marshal UpgradeConfig")
	}
	banner += fmt.Sprintf("Upgrade Config: %s", string(upgradeConfigBytes))
	banner += "\n"
	return banner
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

type fork struct {
	name      string
	block     *big.Int // some go-ethereum forks use block numbers
	timestamp *uint64  // Lux forks use timestamps
	optional  bool     // if true, the fork may be nil and next fork is still allowed
}

func (c *ChainConfig) CheckConfigForkOrder() error {
	if c == nil {
		return nil
	}
	return checkForks(c.forkOrder(), false)
}

// checkForks checks that forks are enabled in order and returns an error if not.
func checkForks(forks []fork, blockFork bool) error {
	lastFork := fork{}
	for _, cur := range forks {
		if lastFork.name != "" {
			switch {
			case lastFork.block == nil && lastFork.timestamp == nil && (cur.block != nil || cur.timestamp != nil):
				if cur.block != nil {
					return fmt.Errorf("unsupported fork ordering: %v not enabled, but %v enabled at block %v",
						lastFork.name, cur.name, cur.block)
				} else {
					return fmt.Errorf("unsupported fork ordering: %v not enabled, but %v enabled at timestamp %v",
						lastFork.name, cur.name, cur.timestamp)
				}
			case (lastFork.block != nil && cur.block != nil) || (lastFork.timestamp != nil && cur.timestamp != nil):
				if lastFork.block != nil && lastFork.block.Cmp(cur.block) > 0 {
					return fmt.Errorf("unsupported fork ordering: %v enabled at block %v, but %v enabled at block %v",
						lastFork.name, lastFork.block, cur.name, cur.block)
				} else if lastFork.timestamp != nil && *lastFork.timestamp > *cur.timestamp {
					return fmt.Errorf("unsupported fork ordering: %v enabled at timestamp %v, but %v enabled at timestamp %v",
						lastFork.name, lastFork.timestamp, cur.name, cur.timestamp)
				}
				if lastFork.timestamp != nil && cur.block != nil {
					return fmt.Errorf("unsupported fork ordering: %v used timestamp ordering, but %v reverted to block ordering",
						lastFork.name, cur.name)
				}
			}
		}
		if !cur.optional || (cur.block != nil || cur.timestamp != nil) {
			lastFork = cur
		}
	}
	return nil
}

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
