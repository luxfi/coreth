// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"
	"sync"
	"time"

	"github.com/holiman/uint256"

	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/consensus/engine/chain/block"
	"github.com/luxfi/coreth/core"
	"github.com/luxfi/coreth/core/txpool"
	"github.com/luxfi/coreth/plugin/evm/extension"
	log "github.com/luxfi/log"
	"github.com/luxfi/concurrent/lock"
)

const (
	// Minimum amount of time to wait after building a block before attempting to build a block
	// a second time without changing the contents of the mempool.
	minBlockBuildingRetryDelay = 500 * time.Millisecond
)

type blockBuilder struct {
	ctx *consensusctx.Context

	txPool       *txpool.TxPool
	extraMempool extension.BuilderMempool

	shutdownChan <-chan struct{}
	shutdownWg   *sync.WaitGroup

	pendingSignal *lock.Cond

	buildBlockLock sync.Mutex
	// lastBuildTime is the time when the last block was built.
	// This is used to ensure that we don't build blocks too frequently,
	// but at least after a minimum delay of minBlockBuildingRetryDelay.
	lastBuildTime time.Time

	// toEngine is used to notify the consensus engine about pending transactions
	// for multi-validator consensus block production
	toEngine chan<- block.Message
}

// Logger returns the logger from the context
func (b *blockBuilder) Logger() log.Logger {
	return b.ctx.Log.(log.Logger)
}

// NewBlockBuilder creates a new block builder. extraMempool is an optional mempool (can be nil) that
// can be used to add transactions to the block builder, in addition to the txPool.
func (vm *VM) NewBlockBuilder(extraMempool extension.BuilderMempool) *blockBuilder {
	b := &blockBuilder{
		ctx:          vm.ctx,
		txPool:       vm.txPool,
		extraMempool: extraMempool,
		shutdownChan: vm.shutdownChan,
		shutdownWg:   &vm.shutdownWg,
		toEngine:     vm.toEngine, // Pass toEngine channel for consensus notification
	}
	b.pendingSignal = lock.NewCond(&b.buildBlockLock)
	return b
}

// handleGenerateBlock is called from the VM immediately after BuildBlock.
func (b *blockBuilder) handleGenerateBlock() {
	b.buildBlockLock.Lock()
	defer b.buildBlockLock.Unlock()
	b.lastBuildTime = time.Now()
}

// needToBuild returns true if there are outstanding transactions to be issued
// into a block.
func (b *blockBuilder) needToBuild() bool {
	size := b.txPool.PendingSize(txpool.PendingFilter{
		MinTip: uint256.MustFromBig(b.txPool.GasTip()),
	})
	return size > 0 || (b.extraMempool != nil && b.extraMempool.PendingLen() > 0)
}

// signalCanBuild notifies a block is expected to be built.
func (b *blockBuilder) signalCanBuild() {
	b.buildBlockLock.Lock()
	defer b.buildBlockLock.Unlock()
	b.pendingSignal.Broadcast()
}

// awaitSubmittedTxs waits for new transactions to be submitted
// and notifies the VM when the tx pool has transactions to be
// put into a new block.
func (b *blockBuilder) awaitSubmittedTxs() {
	// txSubmitChan is invoked when new transactions are issued as well as on re-orgs which
	// may orphan transactions that were previously in a preferred block.
	txSubmitChan := make(chan core.NewTxsEvent)
	b.txPool.SubscribeTransactions(txSubmitChan, true)

	var extraChan <-chan struct{}
	if b.extraMempool != nil {
		extraChan = b.extraMempool.SubscribePendingTxs()
	}

	b.shutdownWg.Add(1)
	go b.Logger().RecoverAndPanic(func() {
		defer b.shutdownWg.Done()

		for {
			select {
			case <-txSubmitChan:
				log.Trace("New tx detected, trying to generate a block")
				b.signalCanBuild()
				// Notify consensus engine about pending transactions for multi-validator consensus
				b.notifyConsensusEngine()
			case <-extraChan:
				log.Trace("New extra Tx detected, trying to generate a block")
				b.signalCanBuild()
				// Notify consensus engine about pending transactions for multi-validator consensus
				b.notifyConsensusEngine()
			case <-b.shutdownChan:
				return
			}
		}
	})
}

// notifyConsensusEngine sends a PendingTxs message to the consensus engine via toEngine channel.
// This is required for multi-validator consensus where the node's chain manager reads from
// toEngine to trigger block building and gossiping.
func (b *blockBuilder) notifyConsensusEngine() {
	if b.toEngine == nil {
		return
	}
	select {
	case b.toEngine <- block.Message{Type: block.PendingTxs}:
		log.Debug("Notified consensus engine about pending transactions")
	default:
		log.Trace("toEngine channel full, notification dropped")
	}
}

// waitForEvent waits until a block needs to be built.
// It returns only after at least [minBlockBuildingRetryDelay] passed from the last time a block was built.
func (b *blockBuilder) waitForEvent(ctx context.Context) (block.Message, error) {
	lastBuildTime, err := b.waitForNeedToBuild(ctx)
	if err != nil {
		return block.Message{}, err
	}
	timeSinceLastBuildTime := time.Since(lastBuildTime)
	if b.lastBuildTime.IsZero() || timeSinceLastBuildTime >= minBlockBuildingRetryDelay {
		log.Debug("Last time we built a block was long enough ago, no need to wait",
			"timeSinceLastBuildTime", timeSinceLastBuildTime,
		)
		return block.Message{Type: block.PendingTxs}, nil
	}
	timeUntilNextBuild := minBlockBuildingRetryDelay - timeSinceLastBuildTime
	log.Debug("Last time we built a block was too recent, waiting",
		"timeUntilNextBuild", timeUntilNextBuild,
	)
	select {
	case <-ctx.Done():
		return block.Message{}, ctx.Err()
	case <-time.After(timeUntilNextBuild):
		return block.Message{Type: block.PendingTxs}, nil
	}
}

// waitForNeedToBuild waits until needToBuild returns true.
// It returns the last time a block was built.
func (b *blockBuilder) waitForNeedToBuild(ctx context.Context) (time.Time, error) {
	b.buildBlockLock.Lock()
	defer b.buildBlockLock.Unlock()
	for !b.needToBuild() {
		if err := b.pendingSignal.Wait(ctx); err != nil {
			return time.Time{}, err
		}
	}
	return b.lastBuildTime, nil
}

// AutominingConfig contains configuration for automining.
type AutominingConfig struct {
	// BuildBlock builds a new block and returns it wrapped for consensus.
	// The block must implement both Verify() and Accept() methods.
	BuildBlock func(ctx context.Context) (interface {
		Verify(context.Context) error
		Accept(context.Context) error
	}, error)
	// Interval is the minimum time between block builds.
	Interval time.Duration
}

// startAutomining starts a goroutine that automatically builds and accepts
// blocks when there are pending transactions. This is used for dev mode
// (luxd --dev) to provide anvil-like behavior where blocks are mined
// immediately when transactions are submitted.
func (b *blockBuilder) startAutomining(config AutominingConfig) {
	// Subscribe to new transactions
	txSubmitChan := make(chan core.NewTxsEvent)
	b.txPool.SubscribeTransactions(txSubmitChan, true)

	var extraChan <-chan struct{}
	if b.extraMempool != nil {
		extraChan = b.extraMempool.SubscribePendingTxs()
	}

	interval := config.Interval
	if interval == 0 {
		interval = 100 * time.Millisecond // Default interval for dev mode
	}

	b.shutdownWg.Add(1)
	go b.Logger().RecoverAndPanic(func() {
		defer b.shutdownWg.Done()

		log.Info("Automining started - blocks will be produced when transactions are pending")

		for {
			select {
			case <-txSubmitChan:
				b.automineBlock(config, interval)
			case <-extraChan:
				b.automineBlock(config, interval)
			case <-b.shutdownChan:
				log.Info("Automining stopped")
				return
			}
		}
	})
}

// automineBlock builds and accepts a block if there are pending transactions.
func (b *blockBuilder) automineBlock(config AutominingConfig, interval time.Duration) {
	// Wait minimum interval between blocks
	b.buildBlockLock.Lock()
	timeSinceLastBuild := time.Since(b.lastBuildTime)
	b.buildBlockLock.Unlock()

	if timeSinceLastBuild < interval {
		time.Sleep(interval - timeSinceLastBuild)
	}

	// Check if we still need to build
	if !b.needToBuild() {
		return
	}

	ctx := context.Background()

	// Build the block
	blk, err := config.BuildBlock(ctx)
	if err != nil {
		log.Error("Automining: failed to build block", "err", err)
		return
	}

	// Verify the block before accepting
	if err := blk.Verify(ctx); err != nil {
		log.Error("Automining: failed to verify block", "err", err)
		return
	}

	// Accept the block (after verification)
	if err := blk.Accept(ctx); err != nil {
		log.Error("Automining: failed to accept block", "err", err)
		return
	}

	log.Info("Automining: block built, verified, and accepted")

	// Update last build time
	b.buildBlockLock.Lock()
	b.lastBuildTime = time.Now()
	b.buildBlockLock.Unlock()
}
