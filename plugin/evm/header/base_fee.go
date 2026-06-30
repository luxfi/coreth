// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"fmt"
	"math/big"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/geth/core/types"
)

// BaseFee takes the previous header and the timestamp of its child block and
// calculates the expected base fee for the child block. Under
// live.
func BaseFee(
	config *extras.ChainConfig,
	parent *types.Header,
	timestamp uint64,
) (*big.Int, error) {
	state, err := feeStateBeforeBlock(config, parent, timestamp)
	if err != nil {
		return nil, fmt.Errorf("calculating initial fee state: %w", err)
	}
	price := state.GasPrice()
	return new(big.Int).SetUint64(uint64(price)), nil
}

// EstimateNextBaseFee attempts to estimate the base fee of a block built at
// `timestamp` on top of `parent`.
//
// If timestamp is before parent.Time, then timestamp is set to parent.Time.
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
