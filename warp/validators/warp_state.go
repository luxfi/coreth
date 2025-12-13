// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"context"
	"fmt"

	"github.com/luxfi/crypto/bls"
	"github.com/luxfi/ids"
	"github.com/luxfi/warp"
)

// warpValidatorState wraps State to implement warp.ValidatorState
type warpValidatorState struct {
	*State
}

// GetValidatorSet implements warp.ValidatorState interface
func (w *warpValidatorState) GetValidatorSet(chainID ids.ID, height uint64) (map[ids.NodeID]*warp.Validator, error) {
	// Get validators from the underlying state
	ctx := context.Background()
	nodeValidators, err := w.State.GetValidatorSet(ctx, height, chainID)
	if err != nil {
		return nil, err
	}

	// Convert to warp.Validator format
	warpValidators := make(map[ids.NodeID]*warp.Validator, len(nodeValidators))
	for nodeID, validator := range nodeValidators {
		// validator.PublicKey is []byte, parse it to *bls.PublicKey
		publicKey, err := bls.PublicKeyFromCompressedBytes(validator.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key for node %s: %w", nodeID, err)
		}

		warpValidator := &warp.Validator{
			PublicKey:      publicKey,
			PublicKeyBytes: validator.PublicKey,
			Weight:         validator.Weight,
			NodeID:         nodeID,
		}
		warpValidators[nodeID] = warpValidator
	}

	return warpValidators, nil
}

// GetCurrentHeight implements warp.ValidatorState interface
func (w *warpValidatorState) GetCurrentHeight() (uint64, error) {
	return w.State.GetCurrentHeight(context.Background())
}

// AsWarpValidatorState returns a warp.ValidatorState wrapper
func (s *State) AsWarpValidatorState() warp.ValidatorState {
	return &warpValidatorState{State: s}
}
