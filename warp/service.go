// (c) 2023, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"context"
	"errors"
	"fmt"

	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/vms/platformvm/warp"
	"github.com/luxfi/node/vms/platformvm/warp/payload"
	"github.com/luxfi/coreth/peer"
	"github.com/luxfi/coreth/warp/aggregator"
	"github.com/luxfi/coreth/warp/validators"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
)

var errNoValidators = errors.New("cannot aggregate signatures from subnet with no validators")

// API introduces snowman specific functionality to the evm
type API struct {
	networkID                     uint32
	sourceSubnetID, sourceChainID ids.ID
	backend                       Backend
	state                         *validators.State
	client                        peer.NetworkClient
}

func NewAPI(networkID uint32, sourceSubnetID ids.ID, sourceChainID ids.ID, state *validators.State, backend Backend, client peer.NetworkClient) *API {
	return &API{
		networkID:      networkID,
		sourceSubnetID: sourceSubnetID,
		sourceChainID:  sourceChainID,
		backend:        backend,
		state:          state,
		client:         client,
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
	signature, err := a.backend.GetMessageSignature(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get signature for message %s with error %w", messageID, err)
	}
	return signature[:], nil
}

// GetBlockSignature returns the BLS signature associated with a blockID.
func (a *API) GetBlockSignature(ctx context.Context, blockID ids.ID) (hexutil.Bytes, error) {
	signature, err := a.backend.GetBlockSignature(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get signature for block %s with error %w", blockID, err)
	}
	return signature[:], nil
}

// GetMessageAggregateSignature fetches the aggregate signature for the requested [messageID]
func (a *API) GetMessageAggregateSignature(ctx context.Context, messageID ids.ID, quorumNum uint64, subnetIDStr string) (signedMessageBytes hexutil.Bytes, err error) {
	unsignedMessage, err := a.backend.GetMessage(messageID)
	if err != nil {
		return nil, err
	}
	return a.aggregateSignatures(ctx, unsignedMessage, quorumNum, subnetIDStr)
}

// GetBlockAggregateSignature fetches the aggregate signature for the requested [blockID]
func (a *API) GetBlockAggregateSignature(ctx context.Context, blockID ids.ID, quorumNum uint64, subnetIDStr string) (signedMessageBytes hexutil.Bytes, err error) {
	blockHashPayload, err := payload.NewHash(blockID)
	if err != nil {
		return nil, err
	}
	unsignedMessage, err := warp.NewUnsignedMessage(a.networkID, a.sourceChainID, blockHashPayload.Bytes())
	if err != nil {
		return nil, err
	}

	return a.aggregateSignatures(ctx, unsignedMessage, quorumNum, subnetIDStr)
}

func (a *API) aggregateSignatures(ctx context.Context, unsignedMessage *warp.UnsignedMessage, quorumNum uint64, subnetIDStr string) (hexutil.Bytes, error) {
	subnetID := a.sourceSubnetID
	if len(subnetIDStr) > 0 {
		sid, err := ids.FromString(subnetIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subnetID: %q", subnetIDStr)
		}
		subnetID = sid
	}
	pChainHeight, err := a.state.GetCurrentHeight(ctx)
	if err != nil {
		return nil, err
	}

	validators, totalWeight, err := warp.GetCanonicalValidatorSet(ctx, a.state, pChainHeight, subnetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get validator set: %w", err)
	}
	if len(validators) == 0 {
		return nil, fmt.Errorf("%w (SubnetID: %s, Height: %d)", errNoValidators, subnetID, pChainHeight)
	}

	log.Debug("Fetching signature",
		"sourceSubnetID", subnetID,
		"height", pChainHeight,
		"numValidators", len(validators),
		"totalWeight", totalWeight,
	)

	agg := aggregator.New(aggregator.NewSignatureGetter(a.client), validators, totalWeight)
	signatureResult, err := agg.AggregateSignatures(ctx, unsignedMessage, quorumNum)
	if err != nil {
		return nil, err
	}
	// TODO: return the signature and total weight as well to the caller for more complete details
	// Need to decide on the best UI for this and write up documentation with the potential
	// gotchas that could impact signed messages becoming invalid.
	return hexutil.Bytes(signatureResult.Message.Bytes()), nil
}
