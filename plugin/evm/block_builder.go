// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"
	"sync"
	"time"

	"github.com/holiman/uint256"

	commonEng "github.com/luxfi/consensus/core"
	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/node/utils/lock"
	"github.com/luxfi/coreth/core"
	"github.com/luxfi/coreth/core/txpool"
	"github.com/luxfi/coreth/plugin/evm/extension"
	"github.com/luxfi/log"
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
	// Type assert Log to get RecoverAndPanic
	logger, ok := b.ctx.Log.(log.Logger)
	if !ok {
		// Fallback to running without panic recovery
		go func() {
			defer b.shutdownWg.Done()
			for {
				select {
				case <-txSubmitChan:
					log.Trace("New tx detected, trying to generate a block")
					b.signalCanBuild()
				case <-extraChan:
					log.Trace("New extra Tx detected, trying to generate a block")
					b.signalCanBuild()
				case <-b.shutdownChan:
					return
				}
			}
		}()
		return
	}
	go logger.RecoverAndPanic(func() {
		defer b.shutdownWg.Done()

		for {
			select {
			case <-txSubmitChan:
				log.Trace("New tx detected, trying to generate a block")
				b.signalCanBuild()
			case <-extraChan:
				log.Trace("New extra Tx detected, trying to generate a block")
				b.signalCanBuild()
			case <-b.shutdownChan:
				return
			}
		}
	})
}

// waitForEvent waits until a block needs to be built.
// It returns only after at least [minBlockBuildingRetryDelay] passed from the last time a block was built.
func (b *blockBuilder) waitForEvent(ctx context.Context) (commonEng.Message, error) {
	lastBuildTime, err := b.waitForNeedToBuild(ctx)
	if err != nil {
		return commonEng.Message{}, err
	}
	timeSinceLastBuildTime := time.Since(lastBuildTime)
	if b.lastBuildTime.IsZero() || timeSinceLastBuildTime >= minBlockBuildingRetryDelay {
		log.Debug("Last time we built a block was long enough ago, no need to wait",
			"timeSinceLastBuildTime", timeSinceLastBuildTime,
		)
		return commonEng.Message{Type: commonEng.PendingTxs}, nil
	}
	timeUntilNextBuild := minBlockBuildingRetryDelay - timeSinceLastBuildTime
	log.Debug("Last time we built a block was too recent, waiting",
		"timeUntilNextBuild", timeUntilNextBuild,
	)
	select {
	case <-ctx.Done():
		return commonEng.Message{}, ctx.Err()
	case <-time.After(timeUntilNextBuild):
		return commonEng.Message{Type: commonEng.PendingTxs}, nil
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
