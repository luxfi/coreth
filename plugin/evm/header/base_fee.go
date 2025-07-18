// (c) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/params"
)

var errEstimateBaseFeeWithoutActivation = errors.New("cannot estimate base fee for chain without apricot phase 3 scheduled")

// BaseFee takes the previous header and the timestamp of its child block and
// calculates the expected base fee for the child block.
//
// Prior to AP3, the returned base fee will be nil.
func BaseFee(
	config *params.ChainConfig,
	parent *types.Header,
	timestamp uint64,
) (*big.Int, error) {
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
		// Prior to AP3 the expected base fee is nil.
		return nil, nil
	}
}

// EstimateNextBaseFee attempts to estimate the base fee of a block built at
// `timestamp` on top of `parent`.
//
// If timestamp is before parent.Time or the AP3 activation time, then timestamp
// is set to the maximum of parent.Time and the AP3 activation time.
//
// Warning: This function should only be used in estimation and should not be
// used when calculating the canonical base fee for a block.
func EstimateNextBaseFee(
	config *params.ChainConfig,
	parent *types.Header,
	timestamp uint64,
) (*big.Int, error) {
	if config.ApricotPhase3BlockTimestamp == nil {
		return nil, errEstimateBaseFeeWithoutActivation
	}

	timestamp = max(timestamp, parent.Time, *config.ApricotPhase3BlockTimestamp)
	return BaseFee(config, parent, timestamp)
}
