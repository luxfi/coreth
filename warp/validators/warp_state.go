// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"context"
	"fmt"

	"github.com/luxfi/ids"
	"github.com/luxfi/warp"
	"github.com/luxfi/warp/bls"
	cryptobls "github.com/luxfi/crypto/bls"
)

// warpValidatorState wraps State to implement warp.ValidatorState
type warpValidatorState struct {
	*State
}

// GetValidatorSet implements warp.ValidatorState interface
func (w *warpValidatorState) GetValidatorSet(chainID []byte, height uint64) (map[string]*warp.Validator, error) {
	// Convert chainID bytes to subnetID
	if len(chainID) != 32 {
		return nil, fmt.Errorf("invalid chainID length: expected 32, got %d", len(chainID))
	}
	
	var subnetID ids.ID
	copy(subnetID[:], chainID)
	
	// Get validators from the underlying state
	ctx := context.Background()
	nodeValidators, err := w.State.GetValidatorSet(ctx, height, subnetID)
	if err != nil {
		return nil, err
	}
	
	// Convert to warp.Validator format
	warpValidators := make(map[string]*warp.Validator, len(nodeValidators))
	for nodeID, validator := range nodeValidators {
		// validator.PublicKey is from luxfi/crypto/bls, we need to convert to warp/bls
		// Serialize the public key to bytes
		publicKeyBytes := cryptobls.PublicKeyToCompressedBytes(validator.PublicKey)
		warpPK, err := bls.PublicKeyFromBytes(publicKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to convert public key for node %s: %w", nodeID, err)
		}
		
		warpValidator := &warp.Validator{
			PublicKey:      warpPK,
			PublicKeyBytes: publicKeyBytes,
			Weight:         validator.Weight,
			NodeID:         nodeID[:],
		}
		warpValidators[string(nodeID[:])] = warpValidator
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