// (c) 2025, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossip

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/network/p2p"
	"github.com/luxfi/node/snow/engine/common"
	"github.com/luxfi/node/utils/logging"
)

var _ p2p.Handler = (*txGossipHandler)(nil)

type txGossipHandler struct {
	log logging.Logger
}

func NewTxGossipHandler(log logging.Logger) *txGossipHandler {
	return &txGossipHandler{
		log: log,
	}
}

func (t *txGossipHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, gossipBytes []byte) {
	// TODO: Implement gossip handling
	t.log.Debug("received app gossip",
		zap.Stringer("nodeID", nodeID),
		zap.Int("size", len(gossipBytes)),
	)
}

func (t *txGossipHandler) AppRequest(ctx context.Context, nodeID ids.NodeID, deadline time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	// TODO: Implement request handling
	t.log.Debug("received app request",
		zap.Stringer("nodeID", nodeID),
		zap.Time("deadline", deadline),
		zap.Int("size", len(requestBytes)),
	)
	return nil, nil
}

func (t *txGossipHandler) CrossChainAppRequest(ctx context.Context, chainID ids.ID, deadline time.Time, requestBytes []byte) ([]byte, error) {
	// TODO: Implement cross-chain request handling
	t.log.Debug("received cross-chain app request",
		zap.Stringer("chainID", chainID),
		zap.Time("deadline", deadline),
		zap.Int("size", len(requestBytes)),
	)
	return nil, nil
}