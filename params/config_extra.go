// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package params

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/luxfi/node/upgrade"
	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/utils"
	ethparams "github.com/luxfi/geth/params"
)

const (
	maxJSONLen = 64 * 1024 * 1024 // 64MB

	// TODO: Value to pass to geth's Rules by default where the appropriate
	// context is not available in the lux code. (similar to context.TODO())
	IsMergeTODO = true
)

var (
	initiallyActive       = uint64(upgrade.InitiallyActiveTime.Unix())
	unscheduledActivation = uint64(upgrade.UnscheduledActivationTime.Unix())
)

// SetEthUpgrades enables Ethereum network upgrades using the same time as
// the Lux network upgrade that enables them.
func SetEthUpgrades(c *ChainConfig) error {
	// Set Ethereum block upgrades to initially activated as they were already
	// activated on launch.
	c.HomesteadBlock = big.NewInt(0)
	c.DAOForkBlock = big.NewInt(0)
	c.DAOForkSupport = true
	c.EIP150Block = big.NewInt(0)
	c.EIP155Block = big.NewInt(0)
	c.EIP158Block = big.NewInt(0)
	c.ByzantiumBlock = big.NewInt(0)
	c.ConstantinopleBlock = big.NewInt(0)
	c.PetersburgBlock = big.NewInt(0)
	c.IstanbulBlock = big.NewInt(0)
	c.MuirGlacierBlock = big.NewInt(0)

	// For Lux mainnet (96369) and testnet (96370), and other networks,
	// Berlin and London are always active from genesis.
	c.BerlinBlock = big.NewInt(0)
	c.LondonBlock = big.NewInt(0)

	extra := GetExtra(c)

	// For Lux networks: respect the explicit shanghaiTime from genesis config.
	// If shanghaiTime is already set (e.g., to 0 for genesis activation), keep it.
	// Only use Durango to set Shanghai if shanghaiTime was not set in genesis.
	if c.ShanghaiTime == nil {
		if durango := extra.DurangoBlockTimestamp; durango != nil && *durango < unscheduledActivation {
			c.ShanghaiTime = utils.NewUint64(*durango)
		}
	}

	// Cancun activation is tied to Etna - the node's upgrade config is authoritative.
	if etna := extra.EtnaTimestamp; etna != nil && *etna < unscheduledActivation {
		c.CancunTime = utils.NewUint64(*etna)
	} else {
		// If Etna is unscheduled, Cancun should also be unscheduled
		c.CancunTime = nil
	}
	return nil
}

func GetExtra(c *ChainConfig) *extras.ChainConfig {
	ex := payloads.ChainConfig.Get(c)
	if ex == nil {
		ex = &extras.ChainConfig{}
		payloads.ChainConfig.Set(c, ex)
	}
	return ex
}

func Copy(c *ChainConfig) *ChainConfig {
	cpy := *c
	cpyPtr := &cpy
	extraCpy := *GetExtra(c)
	// Note: We need to heap-allocate the copy so the pointer remains valid
	// after this function returns, as extras are keyed by pointer.
	heapCpy := new(ChainConfig)
	*heapCpy = *cpyPtr
	heapExtraCpy := new(extras.ChainConfig)
	*heapExtraCpy = extraCpy
	return WithExtra(heapCpy, heapExtraCpy)
}

// WithExtra sets the extra payload on `c` and returns the modified argument.
func WithExtra(c *ChainConfig, extra *extras.ChainConfig) *ChainConfig {
	payloads.ChainConfig.Set(c, extra)
	return c
}

type ChainConfigWithUpgradesJSON struct {
	ChainConfig
	UpgradeConfig extras.UpgradeConfig `json:"upgrades,omitempty"`
}

// MarshalJSON implements json.Marshaler. This is a workaround for the fact that
// the embedded ChainConfig struct has a MarshalJSON method, which prevents
// the default JSON marshalling from working for UpgradeConfig.
// TODO: consider removing this method by allowing external tag for the embedded
// ChainConfig struct.
func (cu ChainConfigWithUpgradesJSON) MarshalJSON() ([]byte, error) {
	// embed the ChainConfig struct into the response
	chainConfigJSON, err := json.Marshal(&cu.ChainConfig)
	if err != nil {
		return nil, err
	}
	if len(chainConfigJSON) > maxJSONLen {
		return nil, errors.New("value too large")
	}

	type upgrades struct {
		UpgradeConfig extras.UpgradeConfig `json:"upgrades"`
	}

	upgradeJSON, err := json.Marshal(upgrades{cu.UpgradeConfig})
	if err != nil {
		return nil, err
	}
	if len(upgradeJSON) > maxJSONLen {
		return nil, errors.New("value too large")
	}

	// merge the two JSON objects
	mergedJSON := make([]byte, 0, len(chainConfigJSON)+len(upgradeJSON)+1)
	mergedJSON = append(mergedJSON, chainConfigJSON[:len(chainConfigJSON)-1]...)
	mergedJSON = append(mergedJSON, ',')
	mergedJSON = append(mergedJSON, upgradeJSON[1:]...)
	return mergedJSON, nil
}

func (cu *ChainConfigWithUpgradesJSON) UnmarshalJSON(input []byte) error {
	var cc ChainConfig
	if err := json.Unmarshal(input, &cc); err != nil {
		return err
	}

	type upgrades struct {
		UpgradeConfig extras.UpgradeConfig `json:"upgrades"`
	}

	var u upgrades
	if err := json.Unmarshal(input, &u); err != nil {
		return err
	}
	cu.ChainConfig = cc
	cu.UpgradeConfig = u.UpgradeConfig
	return nil
}

// ToWithUpgradesJSON converts the ChainConfig to ChainConfigWithUpgradesJSON with upgrades explicitly displayed.
// ChainConfig does not include upgrades in its JSON output.
// This is a workaround for showing upgrades in the JSON output.
func ToWithUpgradesJSON(c *ChainConfig) *ChainConfigWithUpgradesJSON {
	return &ChainConfigWithUpgradesJSON{
		ChainConfig:   *c,
		UpgradeConfig: GetExtra(c).UpgradeConfig,
	}
}

// CheckCompatible checks whether scheduled fork transitions have been imported
// with a mismatching chain configuration. This includes both geth's Ethereum
// forks and Lux-specific network upgrades.
func CheckCompatible(stored, newcfg *ChainConfig, headBlock uint64, headTimestamp uint64) *ethparams.ConfigCompatError {
	// First check geth's Ethereum fork compatibility
	if err := stored.CheckCompatible(newcfg, headBlock, headTimestamp); err != nil {
		return err
	}

	// Then check Lux-specific network upgrades compatibility
	storedExtra := GetExtra(stored)
	newExtra := GetExtra(newcfg)

	return storedExtra.NetworkUpgrades.CheckNetworkUpgradesCompatible(&newExtra.NetworkUpgrades, headTimestamp)
}
