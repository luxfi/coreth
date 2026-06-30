// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"errors"
	"fmt"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/customtypes"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/math"
)

var (
	errInvalidExtraDataGasUsed = errors.New("invalid extra data gas used")
	errInvalidGasUsed          = errors.New("invalid gas used")
	errInvalidGasLimit         = errors.New("invalid gas limit")
)

// GasLimit takes the previous header and the timestamp of its child block and
// calculates the gas limit for the child block. Under activate-all-implicitly
// the dynamic-fee state machine is always active.
func GasLimit(
	config *extras.ChainConfig,
	parent *types.Header,
	timestamp uint64,
) (uint64, error) {
	state, err := feeStateBeforeBlock(config, parent, timestamp)
	if err != nil {
		return 0, fmt.Errorf("calculating initial fee state: %w", err)
	}
	// The gas limit is set to the maximum capacity, rather than the current
	// capacity, to minimize the differences with upstream geth. During
	// block building and gas usage calculations, the gas limit is checked
	// against the current capacity.
	return uint64(state.MaxCapacity()), nil
}

// VerifyGasUsed verifies that the gas used is less than or equal to the gas
// always live.
func VerifyGasUsed(
	config *extras.ChainConfig,
	parent *types.Header,
	header *types.Header,
) error {
	gasUsed := header.GasUsed
	extDataGasUsed := customtypes.GetHeaderExtra(header).ExtDataGasUsed
	if extDataGasUsed != nil {
		if !extDataGasUsed.IsUint64() {
			return fmt.Errorf("%w: %d is not a uint64",
				errInvalidExtraDataGasUsed,
				extDataGasUsed,
			)
		}
		var err error
		gasUsed, err = math.Add64(gasUsed, extDataGasUsed.Uint64())
		if err != nil {
			return fmt.Errorf("%w while calculating gas used", err)
		}
	}

	capacity, err := GasCapacity(config, parent, header.Time)
	if err != nil {
		return fmt.Errorf("calculating gas capacity: %w", err)
	}
	if gasUsed > capacity {
		return fmt.Errorf("%w: have %d, capacity %d",
			errInvalidGasUsed,
			gasUsed,
			capacity,
		)
	}
	return nil
}

// VerifyGasLimit verifies that the gas limit for the header is valid under
// activate-all-implicitly (dynamic-fee state always live).
func VerifyGasLimit(
	config *extras.ChainConfig,
	parent *types.Header,
	header *types.Header,
) error {
	state, err := feeStateBeforeBlock(config, parent, header.Time)
	if err != nil {
		return fmt.Errorf("calculating initial fee state: %w", err)
	}
	maxCapacity := state.MaxCapacity()
	if header.GasLimit != uint64(maxCapacity) {
		return fmt.Errorf("%w: have %d, want %d",
			errInvalidGasLimit,
			header.GasLimit,
			maxCapacity,
		)
	}
	return nil
}

// GasCapacity takes the previous header and the timestamp of its child block
// and calculates the available gas that can be consumed in the child block.
// Under activate-all-implicitly the dynamic-fee state machine is always live.
func GasCapacity(
	config *extras.ChainConfig,
	parent *types.Header,
	timestamp uint64,
) (uint64, error) {
	state, err := feeStateBeforeBlock(config, parent, timestamp)
	if err != nil {
		return 0, fmt.Errorf("calculating initial fee state: %w", err)
	}
	return uint64(state.Gas.Capacity), nil
}

// RemainingAtomicGasCapacity returns the maximum amount ExtDataGasUsed could be
// on `header` while still being valid based on the initial capacity and
// consumed gas. Under activate-all-implicitly the dynamic-fee state machine
// is always live.
func RemainingAtomicGasCapacity(
	config *extras.ChainConfig,
	parent *types.Header,
	header *types.Header,
) (uint64, error) {
	state, err := feeStateBeforeBlock(config, parent, header.Time)
	if err != nil {
		return 0, fmt.Errorf("calculating initial fee state: %w", err)
	}
	if err := state.ConsumeGas(header.GasUsed, nil); err != nil {
		return 0, fmt.Errorf("%w while calculating available gas", err)
	}
	return uint64(state.Gas.Capacity), nil
}
