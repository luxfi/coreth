// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/geth/core/types"
)

var errEstimateBaseFeeWithoutActivation = errors.New("cannot estimate base fee for chain without apricot phase 3 scheduled")

// isLuxChain returns true if this config represents a Lux chain (has any
// Lux-specific network upgrades configured). This distinguishes between
// minimal geth test configs and actual Lux chain configs.
func isLuxChain(config *extras.ChainConfig) bool {
	if config == nil {
		return false
	}
	// Check if any Lux-specific timestamp is set
	return config.BanffBlockTimestamp != nil ||
		config.CortinaBlockTimestamp != nil ||
		config.DurangoBlockTimestamp != nil ||
		config.EtnaTimestamp != nil ||
		config.FortunaTimestamp != nil ||
		config.GraniteTimestamp != nil
}

// BaseFee takes the previous header and the timestamp of its child block and
// calculates the expected base fee for the child block.
//
// Prior to AP3, the returned base fee will be nil.
// For non-Lux configs (minimal geth test configs), returns nil.
func BaseFee(
	config *extras.ChainConfig,
	parent *types.Header,
	timestamp uint64,
) (*big.Int, error) {
	// For non-Lux configs (minimal geth test configs), return nil base fee
	// unless the parent already has a base fee (EIP-1559 via geth's LondonBlock).
	if !isLuxChain(config) {
		if parent != nil && parent.BaseFee != nil && parent.BaseFee.Sign() > 0 {
			// Parent has base fee - this is an EIP-1559 chain, continue with geth-style
			return baseFeeFromWindow(config, parent, timestamp)
		}
		return nil, nil
	}

	switch {
	case config.IsFortuna(timestamp):
		state, err := feeStateBeforeBlock(config, parent, timestamp)
		if err != nil {
			return nil, fmt.Errorf("calculating initial fee state: %w", err)
		}
		price := state.GasPrice()
		return new(big.Int).SetUint64(uint64(price)), nil
	case config.IsApricotPhase3(timestamp):
		return baseFeeFromWindow(config, parent, timestamp)
	default:
		// Prior to AP3 the expected base fee is nil, unless we're dealing with
		// historic SubnetEVM blocks which always had dynamic fees.
		// SubnetEVM blocks have a base fee in the header, so check if parent has one.
		if parent != nil && parent.BaseFee != nil && parent.BaseFee.Sign() > 0 {
			// This is a SubnetEVM block - calculate base fee using the same
			// algorithm as AP3 (dynamic fee window).
			return baseFeeFromWindow(config, parent, timestamp)
		}
		return nil, nil
	}
}

// EstimateNextBaseFee attempts to estimate the base fee of a block built at
// `timestamp` on top of `parent`.
//
// If timestamp is before parent.Time, then timestamp is set to parent.Time.
// Apricot Phase 3 (dynamic fees) is always active on Lux mainnet.
//
// Warning: This function should only be used in estimation and should not be
// used when calculating the canonical base fee for a block.
func EstimateNextBaseFee(
	config *extras.ChainConfig,
	parent *types.Header,
	timestamp uint64,
) (*big.Int, error) {
	// Dynamic fees (AP3+) are always active on Lux mainnet
	timestamp = max(timestamp, parent.Time)
	return BaseFee(config, parent, timestamp)
}
