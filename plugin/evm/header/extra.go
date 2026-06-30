// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"errors"
	"fmt"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/upgrade/lp176"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/vm/components/gas"
)

var (
	errIncorrectFeeState  = errors.New("incorrect fee state")
	errInvalidExtraLength = errors.New("invalid header.Extra length")
)

// ExtraPrefix returns the LP-176 fee-state prefix for the header's Extra
// canonical for every header.
//
// If the `desiredTargetExcess` is nil, the parent's target excess is used.
func ExtraPrefix(
	config *extras.ChainConfig,
	parent *types.Header,
	header *types.Header,
	desiredTargetExcess *gas.Gas,
) ([]byte, error) {
	state, err := feeStateAfterBlock(
		config,
		parent,
		header,
		desiredTargetExcess,
	)
	if err != nil {
		return nil, fmt.Errorf("calculating fee state: %w", err)
	}
	return state.Bytes(), nil
}

// VerifyExtraPrefix verifies that the header's Extra field is correctly
// formatted under the canonical LP-176 layout.
func VerifyExtraPrefix(
	config *extras.ChainConfig,
	parent *types.Header,
	header *types.Header,
) error {
	remoteState, err := lp176.ParseState(header.Extra)
	if err != nil {
		return fmt.Errorf("parsing remote fee state: %w", err)
	}

	// By passing in the claimed target excess, we ensure that the expected
	// target excess is equal to the claimed target excess if it is possible
	// to have correctly set it to that value. Otherwise, the resulting
	// value will be as close to the claimed value as possible, but would
	// not be equal.
	expectedState, err := feeStateAfterBlock(
		config,
		parent,
		header,
		&remoteState.TargetExcess,
	)
	if err != nil {
		return fmt.Errorf("calculating expected fee state: %w", err)
	}

	if remoteState != expectedState {
		return fmt.Errorf("%w: expected %+v, found %+v",
			errIncorrectFeeState,
			expectedState,
			remoteState,
		)
	}
	return nil
}

// VerifyExtra verifies that the header's Extra field is correctly formatted.
// for every header.
func VerifyExtra(_ extras.LuxRules, extra []byte) error {
	if len(extra) < lp176.StateSize {
		return fmt.Errorf(
			"%w: expected >= %d but got %d",
			errInvalidExtraLength,
			lp176.StateSize,
			len(extra),
		)
	}
	return nil
}

// PredicateBytesFromExtra returns the predicate result bytes from the header's
// canonical for every header.
func PredicateBytesFromExtra(_ extras.LuxRules, extra []byte) []byte {
	offset := lp176.StateSize
	if len(extra) <= offset {
		return nil
	}
	return extra[offset:]
}

// SetPredicateBytesInExtra sets the predicate result bytes in the header's
// extra data. Under activate-all-implicitly the LP-176 offset is canonical.
func SetPredicateBytesInExtra(_ extras.LuxRules, extra []byte, predicateBytes []byte) []byte {
	offset := lp176.StateSize

	if len(extra) < offset {
		// pad extra with zeros
		extra = append(extra, make([]byte, offset-len(extra))...)
	} else {
		// truncate extra to the offset
		extra = extra[:offset]
	}
	extra = append(extra, predicateBytes...)
	return extra
}
