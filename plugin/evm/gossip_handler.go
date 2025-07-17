// (c) 2025, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/utils/logging"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/plugin/evm/message"
	"github.com/ava-labs/libevm/rlp"
)

// GossipHandler handles incoming gossip messages
type GossipHandler struct {
	log logging.Logger
	vm  *VM
}

func NewGossipHandler(vm *VM, log logging.Logger) *GossipHandler {
	return &GossipHandler{
		log: log,
		vm:  vm,
	}
}

func (h *GossipHandler) HandleAtomicTx(nodeID ids.NodeID, msg message.AtomicTxGossip) error {
	h.log.Debug("Received AtomicTx gossiped from peer")
	
	if len(msg.Tx) == 0 {
		h.log.Debug("Dropping AtomicTx message with empty tx")
		return nil
	}

	// TODO: Add to mempool when interface is fixed
	return nil
}

func (h *GossipHandler) HandleEthTxs(nodeID ids.NodeID, msg message.EthTxsGossip) error {
	h.log.Debug("Received EthTxs gossiped from peer")
	
	// Decode the transactions from RLP
	var txs []*types.Transaction
	if err := rlp.DecodeBytes(msg.Txs, &txs); err != nil {
		h.log.Debug("Failed to decode transactions from gossip")
		return nil
	}
	
	// Add transactions to the mempool
	errs := h.vm.txPool.AddRemotesSync(txs)
	for _, err := range errs {
		if err != nil {
			h.log.Debug("AppGossip: failed to add remote tx")
		}
	}
	return nil
}