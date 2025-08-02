// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossip

import (
	"context"
	"time"

	"github.com/luxfi/ids"
	"github.com/luxfi/log"
	"github.com/luxfi/node/consensus/engine/core"
	"github.com/luxfi/node/network/p2p"
	"github.com/luxfi/node/network/p2p/gossip"
)

var _ p2p.Handler = (*txGossipHandler)(nil)

func NewTxGossipHandler[T gossip.Gossipable](
	log log.Logger,
	marshaller gossip.Marshaller[T],
	mempool gossip.Set[T],
	metrics gossip.Metrics,
	maxMessageSize int,
	throttlingPeriod time.Duration,
	throttlingLimit int,
	validators p2p.ValidatorSet,
) *txGossipHandler {
	// push gossip messages can be handled from any peer
	handler := gossip.NewHandler(
		log,
		marshaller,
		mempool,
		metrics,
		maxMessageSize,
	)

	// pull gossip requests are filtered by validators and are throttled
	// to prevent spamming
	validatorHandler := p2p.NewValidatorHandler(
		p2p.NewThrottlerHandler(
			handler,
			p2p.NewSlidingWindowThrottler(throttlingPeriod, throttlingLimit),
			log,
		),
		validators,
		log,
	)

	return &txGossipHandler{
		appGossipHandler:  handler,
		appRequestHandler: validatorHandler,
	}
}

type txGossipHandler struct {
	appGossipHandler  p2p.Handler
	appRequestHandler p2p.Handler
}

func (t *txGossipHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, gossipBytes []byte) {
	t.appGossipHandler.AppGossip(ctx, nodeID, gossipBytes)
}

func (t *txGossipHandler) AppRequest(ctx context.Context, nodeID ids.NodeID, deadline time.Time, requestBytes []byte) ([]byte, *core.AppError) {
	return t.appRequestHandler.AppRequest(ctx, nodeID, deadline, requestBytes)
}

func (t *txGossipHandler) CrossChainAppRequest(ctx context.Context, chainID ids.ID, deadline time.Time, requestBytes []byte) ([]byte, error) {
	// We don't handle cross-chain requests in the gossip handler
	return nil, nil
}
