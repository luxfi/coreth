// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package txpool

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/event"
	"github.com/luxfi/geth/log"
	"github.com/luxfi/geth/params"
)

// TxStatus is the current status of a transaction as seen by the pool.
type TxStatus uint

const (
	TxStatusUnknown TxStatus = iota
	TxStatusQueued
	TxStatusPending
	TxStatusIncluded
)

// BlockChain defines the minimal set of methods needed to back a tx pool with
// a chain. Exists to allow mocking the live chain out of tests.
type BlockChain interface {
	// Config retrieves the chain's fork configuration.
	Config() *params.ChainConfig

	// CurrentBlock returns the current head of the chain.
	CurrentBlock() *types.Header

	// SubscribeChainHeadEvent subscribes to new blocks being added to the chain.
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription

	// StateAt returns a state database for a given root hash (generally the head).
	StateAt(root common.Hash) (*state.StateDB, error)
}

// TxPool is an aggregator for various transaction specific pools, collectively
// tracking all the transactions deemed interesting by the node. Transactions
// enter the pool when they are received from the network or submitted locally.
// They exit the pool when they are included in the blockchain or evicted due to
// resource constraints.
type TxPool struct {
	subpools []SubPool // List of subpools for specialized transaction handling
	chain    BlockChain
	signer   types.Signer

	stateLock sync.RWMutex   // The lock for protecting state instance
	state     *state.StateDB // Current state at the blockchain head

	subs event.SubscriptionScope // Subscription scope to unsubscribe all on shutdown
	quit chan chan error         // Quit channel to tear down the head updater
	term chan struct{}           // Termination channel to detect a closed pool

	sync chan chan error // Testing / simulator channel to block until internal reset is done
}

// New creates a new transaction pool to gather, sort and filter inbound
// transactions from the network.
func New(gasTip uint64, chain BlockChain, subpools []SubPool) (*TxPool, error) {
	// Retrieve the current head so that all subpools and this main coordinator
	// pool will have the same starting state, even if the chain moves forward
	// during initialization.
	head := chain.CurrentBlock()

	// Initialize the state with head block, or fallback to empty one in
	// case the head state is not available (might occur when node is not
	// fully synced).
	statedb, err := chain.StateAt(head.Root)
	if err != nil {
		statedb, err = chain.StateAt(types.EmptyRootHash)
	}
	if err != nil {
		return nil, err
	}
	pool := &TxPool{
		subpools: subpools,
		chain:    chain,
		signer:   types.LatestSigner(chain.Config()),
		state:    statedb,
		quit:     make(chan chan error),
		term:     make(chan struct{}),
		sync:     make(chan chan error),
	}
	reserver := NewReservationTracker()
	for i, subpool := range subpools {
		if err := subpool.Init(gasTip, head, reserver.NewHandle(i)); err != nil {
			for j := i - 1; j >= 0; j-- {
				subpools[j].Close()
			}
			return nil, err
		}
	}
	go pool.loop(head)
	return pool, nil
}

// Close terminates the transaction pool and all its subpools.
func (p *TxPool) Close() error {
	var errs []error

	// Terminate the reset loop and wait for it to finish
	errc := make(chan error)
	p.quit <- errc
	if err := <-errc; err != nil {
		errs = append(errs, err)
	}
	// Terminate each subpool
	for _, subpool := range p.subpools {
		if err := subpool.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	// Unsubscribe anyone still listening for tx events
	p.subs.Close()

	if len(errs) > 0 {
		return fmt.Errorf("subpool close errors: %v", errs)
	}
	return nil
}

// loop is the transaction pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events.
func (p *TxPool) loop(head *types.Header) {
	// Close the termination marker when the pool stops
	defer close(p.term)

	// Subscribe to chain head events to trigger subpool resets
	var (
		newHeadCh  = make(chan core.ChainHeadEvent)
		newHeadSub = p.chain.SubscribeChainHeadEvent(newHeadCh)
	)
	defer newHeadSub.Unsubscribe()

	// Track the previous and current head to feed to an idle reset
	var (
		oldHead = head
		newHead = oldHead
	)
	// Consume chain head events and start resets when none is running
	var (
		resetBusy = make(chan struct{}, 1) // Allow 1 reset to run concurrently
		resetDone = make(chan *types.Header)

		resetForced bool       // Whether a forced reset was requested, only used in simulator mode
		resetWaiter chan error // Channel waiting on a forced reset, only used in simulator mode
	)
	// Notify the live reset waiter to not block if the txpool is closed.
	defer func() {
		if resetWaiter != nil {
			resetWaiter <- errors.New("pool already terminated")
			resetWaiter = nil
		}
	}()
	var errc chan error
	for errc == nil {
		// Something interesting might have happened, run a reset if there is
		// one needed but none is running. The resetter will run on its own
		// goroutine to allow chain head events to be consumed contiguously.
		if newHead != oldHead || resetForced {
			// Try to inject a busy marker and start a reset if successful
			select {
			case resetBusy <- struct{}{}:
				// Updates the statedb with the new chain head. The head state may be
				// unavailable if the initial state sync has not yet completed.
				if statedb, err := p.chain.StateAt(newHead.Root); err != nil {
					log.Error("Failed to reset txpool state", "err", err)
				} else {
					p.stateLock.Lock()
					p.state = statedb
					p.stateLock.Unlock()
				}

				// Busy marker injected, start a new subpool reset
				go func(oldHead, newHead *types.Header) {
					for _, subpool := range p.subpools {
						subpool.Reset(oldHead, newHead)
					}
					select {
					case resetDone <- newHead:
					case <-p.term:
					}
				}(oldHead, newHead)

				// If the reset operation was explicitly requested, consider it
				// being fulfilled and drop the request marker. If it was not,
				// this is a noop.
				resetForced = false

			default:
				// Reset already running, wait until it finishes.
				//
				// Note, this will not drop any forced reset request. If a forced
				// reset was requested, but we were busy, then when the currently
				// running reset finishes, a new one will be spun up.
			}
		}
		// Wait for the next chain head event or a previous reset finish
		select {
		case event := <-newHeadCh:
			// Chain moved forward, store the head for later consumption
			newHead = event.Header

		case head := <-resetDone:
			// Previous reset finished, update the old head and allow a new reset
			oldHead = head
			<-resetBusy

			// If someone is waiting for a reset to finish, notify them, unless
			// the forced op is still pending. In that case, wait another round
			// of resets.
			if resetWaiter != nil && !resetForced {
				resetWaiter <- nil
				resetWaiter = nil
			}

		case errc = <-p.quit:
			// Termination requested, break out on the next loop round

		case syncc := <-p.sync:
			// Transaction pool is running inside a simulator, and we are about
			// to create a new block. Request a forced sync operation to ensure
			// that any running reset operation finishes to make block imports
			// deterministic. On top of that, run a new reset operation to make
			// transaction insertions deterministic instead of being stuck in a
			// queue waiting for a reset.
			resetForced = true
			resetWaiter = syncc
		}
	}
	// Notify the closer of termination (no error possible for now)
	errc <- nil
}

// SetGasTip updates the minimum gas tip required by the transaction pool for a
// new transaction, and drops all transactions below this threshold.
func (p *TxPool) SetGasTip(tip *big.Int) {
	for _, subpool := range p.subpools {
		subpool.SetGasTip(tip)
	}
}

// Has returns an indicator whether the pool has a transaction cached with the
// given hash.
func (p *TxPool) Has(hash common.Hash) bool {
	for _, subpool := range p.subpools {
		if subpool.Has(hash) {
			return true
		}
	}
	return false
}

// Get returns a transaction if it is contained in the pool, or nil otherwise.
func (p *TxPool) Get(hash common.Hash) *types.Transaction {
	for _, subpool := range p.subpools {
		if tx := subpool.Get(hash); tx != nil {
			return tx
		}
	}
	return nil
}

// GetRLP returns a RLP-encoded transaction if it is contained in the pool.
func (p *TxPool) GetRLP(hash common.Hash) []byte {
	for _, subpool := range p.subpools {
		encoded := subpool.GetRLP(hash)
		if len(encoded) != 0 {
			return encoded
		}
	}
	return nil
}

// GetMetadata returns the transaction type and transaction size with the given
// hash.
func (p *TxPool) GetMetadata(hash common.Hash) *TxMetadata {
	for _, subpool := range p.subpools {
		if meta := subpool.GetMetadata(hash); meta != nil {
			return meta
		}
	}
	return nil
}

// Add enqueues a batch of transactions into the pool if they are valid. Due
// to the large transaction churn, add may postpone fully integrating the tx
// to a later point to batch multiple ones together.
//
// Note, if sync is set the method will block until all internal maintenance
// related to the add is finished. Only use this during tests for determinism.
func (p *TxPool) Add(txs []*types.Transaction, sync bool) []error {
	// Split the input transactions between the subpools. It shouldn't really
	// happen that we receive merged batches, but better graceful than strange
	// errors.
	//
	// We also need to track how the transactions were split across the subpools,
	// so we can piece back the returned errors into the original order.
	txsets := make([][]*types.Transaction, len(p.subpools))
	splits := make([]int, len(txs))

	for i, tx := range txs {
		// Mark this transaction belonging to no-subpool
		splits[i] = -1

		// Try to find a subpool that accepts the transaction
		for j, subpool := range p.subpools {
			if subpool.Filter(tx) {
				txsets[j] = append(txsets[j], tx)
				splits[i] = j
				break
			}
		}
	}
	// Add the transactions split apart to the individual subpools and piece
	// back the errors into the original sort order.
	errsets := make([][]error, len(p.subpools))
	for i := 0; i < len(p.subpools); i++ {
		errsets[i] = p.subpools[i].Add(txsets[i], sync)
	}
	errs := make([]error, len(txs))
	for i, split := range splits {
		// If the transaction was rejected by all subpools, mark it unsupported
		if split == -1 {
			errs[i] = fmt.Errorf("%w: received type %d", core.ErrTxTypeNotSupported, txs[i].Type())
			continue
		}
		// Find which subpool handled it and pull in the corresponding error
		errs[i] = errsets[split][0]
		errsets[split] = errsets[split][1:]
	}
	return errs
}

// Pending retrieves all currently processable transactions, grouped by origin
// account and sorted by nonce.
//
// The transactions can also be pre-filtered by the dynamic fee components to
// reduce allocations and load on downstream subsystems.
func (p *TxPool) Pending(filter PendingFilter) map[common.Address][]*LazyTransaction {
	txs := make(map[common.Address][]*LazyTransaction)
	for _, subpool := range p.subpools {
		for addr, set := range subpool.Pending(filter) {
			txs[addr] = set
		}
	}
	return txs
}

// SubscribeTransactions registers a subscription for new transaction events,
// supporting feeding only newly seen or also resurrected transactions.
func (p *TxPool) SubscribeTransactions(ch chan<- core.NewTxsEvent, reorgs bool) event.Subscription {
	subs := make([]event.Subscription, len(p.subpools))
	for i, subpool := range p.subpools {
		subs[i] = subpool.SubscribeTransactions(ch, reorgs)
	}
	return p.subs.Track(event.JoinSubscriptions(subs...))
}

// PoolNonce returns the next nonce of an account, with all transactions executable
// by the pool already applied on top.
func (p *TxPool) PoolNonce(addr common.Address) uint64 {
	// Since (for now) accounts are unique to subpools, only one pool will have
	// (at max) a non-state nonce. To avoid stateful lookups, just return the
	// highest nonce for now.
	var nonce uint64
	for _, subpool := range p.subpools {
		if next := subpool.Nonce(addr); nonce < next {
			nonce = next
		}
	}
	return nonce
}

// Nonce returns the next nonce of an account at the current chain head. Unlike
// PoolNonce, this function does not account for pending executable transactions.
func (p *TxPool) Nonce(addr common.Address) uint64 {
	p.stateLock.RLock()
	defer p.stateLock.RUnlock()

	return p.state.GetNonce(addr)
}

// Stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (p *TxPool) Stats() (int, int) {
	var runnable, blocked int
	for _, subpool := range p.subpools {
		run, block := subpool.Stats()

		runnable += run
		blocked += block
	}
	return runnable, blocked
}

// Content retrieves the data content of the transaction pool, returning all the
// pending as well as queued transactions, grouped by account and sorted by nonce.
func (p *TxPool) Content() (map[common.Address][]*types.Transaction, map[common.Address][]*types.Transaction) {
	var (
		runnable = make(map[common.Address][]*types.Transaction)
		blocked  = make(map[common.Address][]*types.Transaction)
	)
	for _, subpool := range p.subpools {
		run, block := subpool.Content()

		for addr, txs := range run {
			runnable[addr] = txs
		}
		for addr, txs := range block {
			blocked[addr] = txs
		}
	}
	return runnable, blocked
}

// ContentFrom retrieves the data content of the transaction pool, returning the
// pending as well as queued transactions of this address, grouped by nonce.
func (p *TxPool) ContentFrom(addr common.Address) ([]*types.Transaction, []*types.Transaction) {
	for _, subpool := range p.subpools {
		run, block := subpool.ContentFrom(addr)
		if len(run) != 0 || len(block) != 0 {
			return run, block
		}
	}
	return []*types.Transaction{}, []*types.Transaction{}
}

// Status returns the known status (unknown/pending/queued) of a transaction
// identified by its hash.
func (p *TxPool) Status(hash common.Hash) TxStatus {
	for _, subpool := range p.subpools {
		if status := subpool.Status(hash); status != TxStatusUnknown {
			return status
		}
	}
	return TxStatusUnknown
}

// Sync is a helper method for unit tests or simulator runs where the chain events
// are arriving in quick succession, without any time in between them to run the
// internal background reset operations. This method will run an explicit reset
// operation to ensure the pool stabilises, thus avoiding flakey behavior.
//
// Note, this method is only used for testing and is susceptible to DoS vectors.
// In production code, the pool is meant to reset on a separate thread.
func (p *TxPool) Sync() error {
	sync := make(chan error)
	select {
	case p.sync <- sync:
		return <-sync
	case <-p.term:
		return errors.New("pool already terminated")
	}
}

// Clear removes all tracked txs from the subpools.
//
// Note, this method invokes Sync() and is only used for testing, because it is
// susceptible to DoS vectors. In production code, the pool is meant to reset on
// a separate thread.
func (p *TxPool) Clear() {
	// Invoke Sync to ensure that txs pending addition don't get added to the pool after
	// the subpools are subsequently cleared
	p.Sync()
	for _, subpool := range p.subpools {
		subpool.Clear()
	}
}
