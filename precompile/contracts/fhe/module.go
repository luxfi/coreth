// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fhe

import (
	"fmt"

	"github.com/luxfi/coreth/precompile/contract"
	"github.com/luxfi/coreth/precompile/modules"
	"github.com/luxfi/coreth/precompile/precompileconfig"

	"github.com/luxfi/geth/common"
)

var _ contract.Configurator = (*configurator)(nil)

// ConfigKey is the key used in json config files to specify this precompile config.
// must be unique across all precompiles.
const ConfigKey = "fheConfig"

// ContractAddress is the address of the FHE precompile contract
// Using reserved range 0x01... for custom precompiles, address 0x80 within range
var ContractAddress = common.HexToAddress("0x0100000000000000000000000000000000000080")

// Module is the precompile module. It is used to register the precompile contract.
var Module = modules.Module{
	ConfigKey:    ConfigKey,
	Address:      ContractAddress,
	Contract:     FHEPrecompile,
	Configurator: &configurator{},
}

type configurator struct{}

func init() {
	// Register the precompile module.
	// Each precompile contract registers itself through [RegisterModule] function.
	if err := modules.RegisterModule(Module); err != nil {
		panic(err)
	}
}

// MakeConfig returns a new precompile config instance.
// This is required to Marshal/Unmarshal the precompile config.
func (*configurator) MakeConfig() precompileconfig.Config {
	return new(Config)
}

// Configure initializes the FHE precompile state
func (*configurator) Configure(chainConfig precompileconfig.ChainConfig, cfg precompileconfig.Config, state contract.StateDB, _ contract.ConfigurationBlockContext) error {
	config, ok := cfg.(*Config)
	if !ok {
		return fmt.Errorf("expected config type %T, got %T: %v", &Config{}, cfg, cfg)
	}
	
	// Store FHE configuration in state if needed
	_ = config
	return nil
}
