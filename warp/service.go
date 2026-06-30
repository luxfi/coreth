// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"context"
	"errors"
	"fmt"

	consensusctx "github.com/luxfi/consensus/context"
	validators "github.com/luxfi/consensus/validator"
	"github.com/luxfi/constants"
	warpprecompile "github.com/luxfi/coreth/precompile/contracts/warp"
	warpValidators "github.com/luxfi/coreth/warp/validators"
	"github.com/luxfi/geth/common/hexutil"
	"github.com/luxfi/ids"
	log "github.com/luxfi/log"
	"github.com/luxfi/warp"
	"github.com/luxfi/warp/payload"
)

var errNoValidators = errors.New("cannot aggregate signatures from chain with no validators")

// API introduces quasarman specific functionality to the evm
type API struct {
	chainContext                 *consensusctx.Context
	backend                      Backend
	signatureAggregator          *warp.SignatureAggregator
	requirePrimaryNetworkSigners func() bool
}

func NewAPI(chainCtx *consensusctx.Context, backend Backend, signatureAggregator *warp.SignatureAggregator, requirePrimaryNetworkSigners func() bool) *API {
	return &API{
		backend:                      backend,
		chainContext:                 chainCtx,
		signatureAggregator:          signatureAggregator,
		requirePrimaryNetworkSigners: requirePrimaryNetworkSigners,
	}
}

// GetMessage returns the Warp message associated with a messageID.
func (a *API) GetMessage(ctx context.Context, messageID ids.ID) (hexutil.Bytes, error) {
	message, err := a.backend.GetMessage(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message %s with error %w", messageID, err)
	}
	return hexutil.Bytes(message.Bytes()), nil
}

// GetMessageSignature returns the BLS signature associated with a messageID.
func (a *API) GetMessageSignature(ctx context.Context, messageID ids.ID) (hexutil.Bytes, error) {
	unsignedMessage, err := a.backend.GetMessage(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message %s with error %w", messageID, err)
	}
	signature, err := a.backend.GetMessageSignature(ctx, unsignedMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to get signature for message %s with error %w", messageID, err)
	}
	return signature[:], nil
}

// GetBlockSignature returns the BLS signature associated with a blockID.
func (a *API) GetBlockSignature(ctx context.Context, blockID ids.ID) (hexutil.Bytes, error) {
	signature, err := a.backend.GetBlockSignature(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get signature for block %s with error %w", blockID, err)
	}
	return signature[:], nil
}

// GetMessageAggregateSignature fetches the aggregate signature for the requested [messageID]
func (a *API) GetMessageAggregateSignature(ctx context.Context, messageID ids.ID, quorumNum uint64, chainIDStr string) (signedMessageBytes hexutil.Bytes, err error) {
	unsignedMessage, err := a.backend.GetMessage(messageID)
	if err != nil {
		return nil, err
	}
	return a.aggregateSignatures(ctx, unsignedMessage, quorumNum, chainIDStr)
}

// GetBlockAggregateSignature fetches the aggregate signature for the requested [blockID]
func (a *API) GetBlockAggregateSignature(ctx context.Context, blockID ids.ID, quorumNum uint64, chainIDStr string) (signedMessageBytes hexutil.Bytes, err error) {
	blockHashPayload, err := payload.NewHash(blockID[:])
	if err != nil {
		return nil, err
	}
	unsignedMessage, err := warp.NewUnsignedMessage(a.chainContext.NetworkID, a.chainContext.ChainID, blockHashPayload.Bytes())
	if err != nil {
		return nil, err
	}

	return a.aggregateSignatures(ctx, unsignedMessage, quorumNum, chainIDStr)
}

func (a *API) aggregateSignatures(ctx context.Context, unsignedMessage *warp.UnsignedMessage, quorumNum uint64, chainIDStr string) (hexutil.Bytes, error) {
	chainID := constants.PrimaryNetworkID
	if len(chainIDStr) > 0 {
		sid, err := ids.FromString(chainIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse chainID: %q", chainIDStr)
		}
		chainID = sid
	}

	// Get validator state from chain context
	validatorState, ok := a.chainContext.ValidatorState.(validators.State)
	if !ok {
		return nil, errors.New("validator state not available")
	}

	pChainHeight, err := validatorState.GetCurrentHeight(ctx)
	if err != nil {
		return nil, err
	}

	state := warpValidators.NewState(validatorState, constants.PrimaryNetworkID, a.chainContext.ChainID, a.requirePrimaryNetworkSigners())

	// Get validator set from the wrapped state
	validatorMap, err := state.GetValidatorSet(ctx, pChainHeight, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get validator set: %w", err)
	}
	if len(validatorMap) == 0 {
		return nil, fmt.Errorf("%w (ChainID: %s, Height: %d)", errNoValidators, chainID, pChainHeight)
	}

	// Convert to warp.Validator slice
	warpValidatorList := make([]*warp.Validator, 0, len(validatorMap))
	var totalWeight uint64
	for nodeID, v := range validatorMap {
		warpValidatorList = append(warpValidatorList, &warp.Validator{
			PublicKeyBytes: v.PublicKey,
			Weight:         v.Weight,
			NodeID:         nodeID,
		})
		totalWeight += v.Weight
	}

	log.Debug("Fetching signature",
		"sourceChainID", chainID,
		"height", pChainHeight,
		"numValidators", len(warpValidatorList),
		"totalWeight", totalWeight,
	)
	warpMessage := &warp.Message{
		UnsignedMessage: unsignedMessage,
		Signature:       &warp.BitSetSignature{},
	}
	signedMessage, _, _, err := a.signatureAggregator.AggregateSignatures(
		ctx,
		warpMessage,
		nil,
		warpValidatorList,
		quorumNum,
		warpprecompile.WarpQuorumDenominator,
	)
	if err != nil {
		return nil, err
	}
	// TODO: return the signature and total weight as well to the caller for more complete details
	// Need to decide on the best UI for this and write up documentation with the potential
	// gotchas that could impact signed messages becoming invalid.
	return hexutil.Bytes(signedMessage.Bytes()), nil
}
