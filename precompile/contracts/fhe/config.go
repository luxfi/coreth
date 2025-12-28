// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fhe

import (
	"github.com/luxfi/coreth/precompile/precompileconfig"
	"github.com/luxfi/geth/common"
)

var _ precompileconfig.Config = (*Config)(nil)

// Config implements the precompileconfig.Config interface for the FHE precompile
type Config struct {
	precompileconfig.Upgrade

	// SecurityLevel determines the TFHE security level (128, 192, 256)
	SecurityLevel uint16 `json:"securityLevel,omitempty"`
	
	// CoprocessorAddress is the address of the FHE coprocessor (for async ops)
	CoprocessorAddress common.Address `json:"coprocessorAddress,omitempty"`
	
	// ThresholdChainID is the chain ID for threshold decryption
	ThresholdChainID uint32 `json:"thresholdChainID,omitempty"`
	
	// MaxCiphertextSize is the maximum allowed ciphertext size in bytes
	MaxCiphertextSize uint64 `json:"maxCiphertextSize,omitempty"`
	
	// GasPerGate is the gas cost per FHE gate operation
	GasPerGate uint64 `json:"gasPerGate,omitempty"`
}

// NewConfig returns a new FHE config with default values
func NewConfig(blockTimestamp *uint64) *Config {
	return &Config{
		Upgrade: precompileconfig.Upgrade{
			BlockTimestamp: blockTimestamp,
		},
		SecurityLevel:     128,
		MaxCiphertextSize: 32 * 1024, // 32KB
		GasPerGate:        10000,     // ~10k gas per gate
	}
}

// Key returns the key for the FHE precompile config
func (*Config) Key() string {
	return ConfigKey
}

// Verify checks that the config is valid
func (c *Config) Verify(chainConfig precompileconfig.ChainConfig) error {
	// Validate security level
	if c.SecurityLevel != 0 && c.SecurityLevel != 128 && c.SecurityLevel != 192 && c.SecurityLevel != 256 {
		return ErrInvalidSecurityLevel
	}
	
	return nil
}

// Equal returns true if the configs are equal
func (c *Config) Equal(other precompileconfig.Config) bool {
	otherConfig, ok := other.(*Config)
	if !ok {
		return false
	}
	
	return c.Upgrade.Equal(&otherConfig.Upgrade) &&
		c.SecurityLevel == otherConfig.SecurityLevel &&
		c.CoprocessorAddress == otherConfig.CoprocessorAddress &&
		c.ThresholdChainID == otherConfig.ThresholdChainID &&
		c.MaxCiphertextSize == otherConfig.MaxCiphertextSize &&
		c.GasPerGate == otherConfig.GasPerGate
}
