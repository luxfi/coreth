// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"errors"
	"fmt"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/customtypes"
	"github.com/luxfi/coreth/plugin/evm/upgrade/lp176"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/vm/components/gas"
)

var errInvalidTimestamp = errors.New("invalid timestamp")

// feeStateBeforeBlock takes the previous header and the timestamp of its child
// block and calculates the fee state before the child block is executed. Under
// activate-all-implicitly the LP-176 capacity-based fee state machine is the
// only one.
func feeStateBeforeBlock(
	_ *extras.ChainConfig,
	parent *types.Header,
	timestamp uint64,
) (lp176.State, error) {
	if timestamp < parent.Time {
		return lp176.State{}, fmt.Errorf("%w: timestamp %d prior to parent timestamp %d",
			errInvalidTimestamp,
			timestamp,
			parent.Time,
		)
	}

	var state lp176.State
	if parent.Number.Cmp(common.Big0) != 0 {
		// The parent's claimed fee state IS the actual fee state — it has
		// already been verified. The genesis block has no encoded fee state.
		var err error
		state, err = lp176.ParseState(parent.Extra)
		if err != nil {
			return lp176.State{}, fmt.Errorf("parsing parent fee state: %w", err)
		}
	}

	state.AdvanceTime(timestamp - parent.Time)
	return state, nil
}

// feeStateAfterBlock takes the previous header and returns the fee state after
// the execution of the provided child.
func feeStateAfterBlock(
	config *extras.ChainConfig,
	parent *types.Header,
	header *types.Header,
	desiredTargetExcess *gas.Gas,
) (lp176.State, error) {
	// Calculate the gas state after the parent block
	state, err := feeStateBeforeBlock(config, parent, header.Time)
	if err != nil {
		return lp176.State{}, fmt.Errorf("calculating initial fee state: %w", err)
	}

	// Consume the gas used by the block
	extDataGasUsed := customtypes.GetHeaderExtra(header).ExtDataGasUsed
	if err := state.ConsumeGas(header.GasUsed, extDataGasUsed); err != nil {
		return lp176.State{}, fmt.Errorf("advancing the fee state: %w", err)
	}

	// If the desired target excess is specified, move the target excess as much
	// as possible toward that desired value.
	if desiredTargetExcess != nil {
		state.UpdateTargetExcess(*desiredTargetExcess)
	}
	return state, nil
}
