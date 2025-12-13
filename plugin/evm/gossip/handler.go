// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossip

import (
	"context"
	"time"

	"github.com/luxfi/ids"
	"github.com/luxfi/log"
	"github.com/luxfi/p2p"
	"github.com/luxfi/p2p/gossip"
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
	bloomChecker gossip.BloomChecker,
) *txGossipHandler {
	// push gossip messages can be handled from any peer
	handler := gossip.NewHandler(
		log,
		marshaller,
		mempool,
		metrics,
		maxMessageSize,
		bloomChecker,
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
		gossipHandler:  handler,
		requestHandler: validatorHandler,
	}
}

type txGossipHandler struct {
	gossipHandler  p2p.Handler
	requestHandler p2p.Handler
}

func (t *txGossipHandler) Gossip(ctx context.Context, nodeID ids.NodeID, gossipBytes []byte) {
	t.gossipHandler.Gossip(ctx, nodeID, gossipBytes)
}

func (t *txGossipHandler) Request(ctx context.Context, nodeID ids.NodeID, deadline time.Time, requestBytes []byte) ([]byte, *p2p.Error) {
	return t.requestHandler.Request(ctx, nodeID, deadline, requestBytes)
}
