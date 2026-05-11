// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// TODO: move to network

package evm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	consensusconfig "github.com/luxfi/consensus/config"
	ethcommon "github.com/luxfi/geth/common"
	log "github.com/luxfi/log"
	"github.com/luxfi/metric"

	"github.com/luxfi/ids"
	"github.com/luxfi/p2p/gossip"

	"github.com/luxfi/coreth/core"
	"github.com/luxfi/coreth/core/txpool"
	"github.com/luxfi/coreth/eth"
	"github.com/luxfi/coreth/plugin/evm/config"
	"github.com/luxfi/geth/core/types"
)

// ErrClassicalTxForbidden is returned by GossipEthTxPool.Add when the
// chain runs under a ChainSecurityProfile with ForbidECDSAWallets=true
// and a classical ECDSA (v,r,s)-signed Ethereum transaction is offered
// for admission. The C-chain refuses every legacy tx type
// (Legacy/AccessList/DynamicFee/Blob/SetCode) under strict-PQ until a
// dedicated PQAuthTx type lands. Closes red-team finding F118.
var ErrClassicalTxForbidden = errors.New("coreth: classical ECDSA-signed tx refused under strict-PQ ForbidECDSAWallets")

const pendingTxsBuffer = 10

var (
	_ gossip.Gossipable               = (*GossipEthTx)(nil)
	_ gossip.Marshaller[*GossipEthTx] = (*GossipEthTxMarshaller)(nil)
	_ gossip.Set[*GossipEthTx]        = (*GossipEthTxPool)(nil)

	_ eth.PushGossiper = (*EthPushGossiper)(nil)
)

func NewGossipEthTxPool(mempool *txpool.TxPool, registerer metric.Registerer) (*GossipEthTxPool, error) {
	return NewGossipEthTxPoolWithProfile(mempool, registerer, nil)
}

// NewGossipEthTxPoolWithProfile is the F118 entry point. profile is the
// resolved chain-wide ChainSecurityProfile; when its ForbidECDSAWallets
// bit is true, Add refuses every classical Ethereum tx type. Pass nil
// to preserve classical-compat behaviour (legacy chains, dev mode).
func NewGossipEthTxPoolWithProfile(
	mempool *txpool.TxPool,
	registerer metric.Registerer,
	profile *consensusconfig.ChainSecurityProfile,
) (*GossipEthTxPool, error) {
	bloom, err := gossip.NewBloomFilter(
		registerer,
		"eth_tx_bloom_filter",
		config.TxGossipBloomMinTargetElements,
		config.TxGossipBloomTargetFalsePositiveRate,
		config.TxGossipBloomResetFalsePositiveRate,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize bloom filter: %w", err)
	}

	return &GossipEthTxPool{
		mempool:         mempool,
		pendingTxs:      make(chan core.NewTxsEvent, pendingTxsBuffer),
		bloom:           bloom,
		securityProfile: profile,
	}, nil
}

type GossipEthTxPool struct {
	mempool    *txpool.TxPool
	pendingTxs chan core.NewTxsEvent

	bloom *gossip.BloomFilter
	lock  sync.RWMutex

	// subscribed is set to true when the gossip subscription is active
	// mostly used for testing
	subscribed atomic.Bool

	// securityProfile, when non-nil with ForbidECDSAWallets=true, makes
	// Add refuse every classical Ethereum tx type (legacy / EIP-2930 /
	// EIP-1559 / EIP-4844 / EIP-7702). All of those are secp256k1-signed
	// and there is no PQAuthTx type yet, so this is a blanket gate.
	// Closes red-team finding F118 at the mempool admission boundary.
	securityProfile *consensusconfig.ChainSecurityProfile
}

// IsSubscribed returns whether or not the gossip subscription is active.
func (g *GossipEthTxPool) IsSubscribed() bool {
	return g.subscribed.Load()
}

func (g *GossipEthTxPool) Subscribe(ctx context.Context) {
	sub := g.mempool.SubscribeTransactions(g.pendingTxs, false)
	if sub == nil {
		log.Warn("failed to subscribe to new txs event")
		return
	}
	g.subscribed.CompareAndSwap(false, true)
	defer func() {
		sub.Unsubscribe()
		g.subscribed.CompareAndSwap(true, false)
	}()

	for {
		select {
		case <-ctx.Done():
			log.Debug("shutting down subscription")
			return
		case pendingTxs := <-g.pendingTxs:
			g.lock.Lock()
			optimalElements := (g.mempool.PendingSize(txpool.PendingFilter{}) + len(pendingTxs.Txs)) * config.TxGossipBloomChurnMultiplier
			for _, pendingTx := range pendingTxs.Txs {
				tx := &GossipEthTx{Tx: pendingTx}
				g.bloom.Add(tx)
				reset, err := gossip.ResetBloomFilterIfNeeded(g.bloom, optimalElements)
				if err != nil {
					log.Error("failed to reset bloom filter", "err", err)
					continue
				}

				if reset {
					log.Debug("resetting bloom filter", "reason", "reached max filled ratio")

					g.mempool.IteratePending(func(tx *types.Transaction) bool {
						g.bloom.Add(&GossipEthTx{Tx: tx})
						return true
					})
				}
			}
			g.lock.Unlock()
		}
	}
}

// Add enqueues the transaction to the mempool. Subscribe should be called
// to receive an event if tx is actually added to the mempool or not.
//
// Under a ChainSecurityProfile with ForbidECDSAWallets=true, every
// classical Ethereum tx type is refused at this boundary with
// ErrClassicalTxForbidden — there is no PQAuthTx type yet, so the gate
// is a blanket refusal. Closes red-team finding F118.
func (g *GossipEthTxPool) Add(tx *GossipEthTx) error {
	if g.securityProfile != nil && g.securityProfile.ForbidECDSAWallets {
		// Every Ethereum tx type produced today is (v,r,s)-secp256k1
		// signed. There is no PQ-signed Ethereum tx type. Refuse.
		return ErrClassicalTxForbidden
	}
	return g.mempool.Add([]*types.Transaction{tx.Tx}, false, false)[0]
}

// Has should just return whether or not the [txID] is still in the mempool,
// not whether it is in the mempool AND pending.
func (g *GossipEthTxPool) Has(txID ids.ID) bool {
	return g.mempool.Has(ethcommon.Hash(txID))
}

func (g *GossipEthTxPool) Iterate(f func(tx *GossipEthTx) bool) {
	g.mempool.IteratePending(func(tx *types.Transaction) bool {
		return f(&GossipEthTx{Tx: tx})
	})
}

func (g *GossipEthTxPool) GetFilter() ([]byte, []byte) {
	g.lock.RLock()
	defer g.lock.RUnlock()

	return g.bloom.Marshal()
}

type GossipEthTxMarshaller struct{}

func (g GossipEthTxMarshaller) MarshalGossip(tx *GossipEthTx) ([]byte, error) {
	return tx.Tx.MarshalBinary()
}

func (g GossipEthTxMarshaller) UnmarshalGossip(bytes []byte) (*GossipEthTx, error) {
	tx := &GossipEthTx{
		Tx: &types.Transaction{},
	}

	return tx, tx.Tx.UnmarshalBinary(bytes)
}

type GossipEthTx struct {
	Tx *types.Transaction
}

func (tx *GossipEthTx) GossipID() ids.ID {
	return ids.ID(tx.Tx.Hash())
}

// EthPushGossiper is used by the ETH backend to push transactions issued over
// the RPC and added to the mempool to peers.
type EthPushGossiper struct {
	vm *VM
}

func (e *EthPushGossiper) Add(tx *types.Transaction) {
	// eth.Backend is initialized before the [ethTxPushGossiper] is created, so
	// we just ignore any gossip requests until it is set.
	ethTxPushGossiper := e.vm.ethTxPushGossiper.Get()
	if ethTxPushGossiper == nil {
		return
	}
	ethTxPushGossiper.Add(&GossipEthTx{tx})
}
