// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomic

import (
	"context"
	"errors"

	"github.com/luxfi/coreth/plugin/evm/message"
	"github.com/luxfi/coreth/sync/handlers"
	"github.com/luxfi/coreth/sync/handlers/stats"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/codec"

	"github.com/luxfi/geth/metrics"
	"github.com/luxfi/geth/triedb"
)

var (
	_ handlers.LeafRequestHandler = (*uninitializedHandler)(nil)

	errUninitialized = errors.New("uninitialized handler")
)

type uninitializedHandler struct{}

func (h *uninitializedHandler) OnLeafsRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, leafsRequest message.LeafsRequest) ([]byte, error) {
	return nil, errUninitialized
}

// atomicLeafHandler is a wrapper around handlers.LeafRequestHandler that allows for initialization after creation
type leafHandler struct {
	handlers.LeafRequestHandler
}

// NewAtomicLeafHandler returns a new uninitialized leafHandler that can be later initialized
func NewLeafHandler() *leafHandler {
	return &leafHandler{
		LeafRequestHandler: &uninitializedHandler{},
	}
}

// Initialize initializes the atomicLeafHandler with the provided atomicTrieDB, trieKeyLength, and networkCodec
func (a *leafHandler) Initialize(atomicTrieDB *triedb.Database, trieKeyLength int, networkCodec codec.Manager) {
	handlerStats := stats.GetOrRegisterHandlerStats(metrics.Enabled())
	a.LeafRequestHandler = handlers.NewLeafsRequestHandler(atomicTrieDB, trieKeyLength, nil, networkCodec, handlerStats)
}
