// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"errors"
	"math/big"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/customtypes"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
)

var (
	errBaseFeeNil = errors.New("base fee is nil")
	errNoGasUsed  = errors.New("no gas used")
)

// BlockGasCost returns the required block gas cost. Under
// replaces the legacy AP4/AP5 block gas cost mechanism, so the value is
// always zero.
func BlockGasCost(
	_ *extras.ChainConfig,
	_ *types.Header,
	_ uint64,
) *big.Int {
	return common.Big0
}

// BlockGasCostWithStep returns the required block gas cost given a parent cost,
// step, and time elapsed. Under activate-all-implicitly the LP-176 capacity
// model is the only fee model, so the legacy step-based cost is zero.
func BlockGasCostWithStep(_ *big.Int, _ uint64, _ uint64) uint64 {
	return 0
}

// EstimateRequiredTip estimates the tip a transaction would have needed to
// pay to be included in a given block. Under activate-all-implicitly the
// block gas cost is always zero, so the required tip is sourced from the
// header's base fee directly: there is no longer a per-block tip surcharge.
func EstimateRequiredTip(
	_ *extras.ChainConfig,
	header *types.Header,
) (*big.Int, error) {
	if header.BaseFee == nil {
		return nil, errBaseFeeNil
	}

	extra := customtypes.GetHeaderExtra(header)

	// totalGasUsed = GasUsed + ExtDataGasUsed
	totalGasUsed := new(big.Int).SetUint64(header.GasUsed)
	if extra.ExtDataGasUsed != nil {
		totalGasUsed.Add(totalGasUsed, extra.ExtDataGasUsed)
	}
	if totalGasUsed.Sign() == 0 {
		return nil, errNoGasUsed
	}

	// Under LP-176 the per-block fee is the base fee. There is no extra
	// block-gas-cost tip required beyond the base fee.
	return new(big.Int).Set(common.Big0), nil
}
