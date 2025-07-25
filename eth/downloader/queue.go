// Copyright 2015 The go-ethereum Authors
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

// Contains the block download scheduler to collect download tasks and schedule
// them in an ordered, and throttled way.

package downloader

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/common/prque"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/crypto/kzg4844"
	"github.com/luxfi/geth/eth/ethconfig"
	"github.com/luxfi/geth/log"
	"github.com/luxfi/geth/metrics"
	"github.com/luxfi/geth/params"
	"github.com/luxfi/geth/rlp"
)

const (
	bodyType    = uint(0)
	receiptType = uint(1)
)

var (
	blockCacheMaxItems     = 8192              // Maximum number of blocks to cache before throttling the download
	blockCacheInitialItems = 2048              // Initial number of blocks to start fetching, before we know the sizes of the blocks
	blockCacheMemory       = 256 * 1024 * 1024 // Maximum amount of memory to use for block caching
	blockCacheSizeWeight   = 0.1               // Multiplier to approximate the average block size based on past ones
)

var (
	errNoFetchesPending = errors.New("no fetches pending")
	errStaleDelivery    = errors.New("stale delivery")
)

// fetchRequest is a currently running data retrieval operation.
type fetchRequest struct {
	Peer    *peerConnection // Peer to which the request was sent
	From    uint64          // Requested chain element index (used for skeleton fills only)
	Headers []*types.Header // Requested headers, sorted by request order
	Time    time.Time       // Time when the request was made
}

// fetchResult is a struct collecting partial results from data fetchers until
// all outstanding pieces complete and the result as a whole can be processed.
type fetchResult struct {
	pending atomic.Int32 // Flag telling what deliveries are outstanding

	Header       *types.Header
	Uncles       []*types.Header
	Transactions types.Transactions
	Receipts     rlp.RawValue
	Withdrawals  types.Withdrawals
}

func newFetchResult(header *types.Header, snapSync bool) *fetchResult {
	item := &fetchResult{
		Header: header,
	}
	if !header.EmptyBody() {
		item.pending.Store(item.pending.Load() | (1 << bodyType))
	} else if header.WithdrawalsHash != nil {
		item.Withdrawals = make(types.Withdrawals, 0)
	}
	if snapSync {
		if header.EmptyReceipts() {
			// Ensure the receipts list is valid even if it isn't actively fetched.
			item.Receipts = rlp.EmptyList
		} else {
			item.pending.Store(item.pending.Load() | (1 << receiptType))
		}
	}
	return item
}

// body returns a representation of the fetch result as a types.Body object.
func (f *fetchResult) body() types.Body {
	return types.Body{
		Transactions: f.Transactions,
		Uncles:       f.Uncles,
		Withdrawals:  f.Withdrawals,
	}
}

// SetBodyDone flags the body as finished.
func (f *fetchResult) SetBodyDone() {
	if v := f.pending.Load(); (v & (1 << bodyType)) != 0 {
		f.pending.Add(-1)
	}
}

// AllDone checks if item is done.
func (f *fetchResult) AllDone() bool {
	return f.pending.Load() == 0
}

// SetReceiptsDone flags the receipts as finished.
func (f *fetchResult) SetReceiptsDone() {
	if v := f.pending.Load(); (v & (1 << receiptType)) != 0 {
		f.pending.Add(-2)
	}
}

// Done checks if the given type is done already
func (f *fetchResult) Done(kind uint) bool {
	v := f.pending.Load()
	return v&(1<<kind) == 0
}

// queue represents hashes that are either need fetching or are being fetched
type queue struct {
	mode       SyncMode    // Synchronisation mode to decide on the block parts to schedule for fetching
	headerHead common.Hash // Hash of the last queued header to verify order

	// All data retrievals below are based on an already assembles header chain
	blockTaskPool  map[common.Hash]*types.Header      // Pending block (body) retrieval tasks, mapping hashes to headers
	blockTaskQueue *prque.Prque[int64, *types.Header] // Priority queue of the headers to fetch the blocks (bodies) for
	blockPendPool  map[string]*fetchRequest           // Currently pending block (body) retrieval operations
	blockWakeCh    chan bool                          // Channel to notify the block fetcher of new tasks

	receiptTaskPool  map[common.Hash]*types.Header      // Pending receipt retrieval tasks, mapping hashes to headers
	receiptTaskQueue *prque.Prque[int64, *types.Header] // Priority queue of the headers to fetch the receipts for
	receiptPendPool  map[string]*fetchRequest           // Currently pending receipt retrieval operations
	receiptWakeCh    chan bool                          // Channel to notify when receipt fetcher of new tasks

	resultCache *resultStore       // Downloaded but not yet delivered fetch results
	resultSize  common.StorageSize // Approximate size of a block (exponential moving average)

	lock   *sync.RWMutex
	active *sync.Cond
	closed bool

	logTime time.Time // Time instance when status was last reported
}

// newQueue creates a new download queue for scheduling block retrieval.
func newQueue(blockCacheLimit int, thresholdInitialSize int) *queue {
	lock := new(sync.RWMutex)
	q := &queue{
		blockTaskQueue:   prque.New[int64, *types.Header](nil),
		blockWakeCh:      make(chan bool, 1),
		receiptTaskQueue: prque.New[int64, *types.Header](nil),
		receiptWakeCh:    make(chan bool, 1),
		active:           sync.NewCond(lock),
		lock:             lock,
	}
	q.Reset(blockCacheLimit, thresholdInitialSize)
	return q
}

// Reset clears out the queue contents.
func (q *queue) Reset(blockCacheLimit int, thresholdInitialSize int) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.closed = false
	q.mode = ethconfig.FullSync
	q.headerHead = common.Hash{}

	q.blockTaskPool = make(map[common.Hash]*types.Header)
	q.blockTaskQueue.Reset()
	q.blockPendPool = make(map[string]*fetchRequest)

	q.receiptTaskPool = make(map[common.Hash]*types.Header)
	q.receiptTaskQueue.Reset()
	q.receiptPendPool = make(map[string]*fetchRequest)

	q.resultCache = newResultStore(blockCacheLimit)
	q.resultCache.SetThrottleThreshold(uint64(thresholdInitialSize))
}

// Close marks the end of the sync, unblocking Results.
// It may be called even if the queue is already closed.
func (q *queue) Close() {
	q.lock.Lock()
	q.closed = true
	q.active.Signal()
	q.lock.Unlock()
}

// PendingBodies retrieves the number of block body requests pending for retrieval.
func (q *queue) PendingBodies() int {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.blockTaskQueue.Size()
}

// PendingReceipts retrieves the number of block receipts pending for retrieval.
func (q *queue) PendingReceipts() int {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.receiptTaskQueue.Size()
}

// InFlightBlocks retrieves whether there are block fetch requests currently in
// flight.
func (q *queue) InFlightBlocks() bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	return len(q.blockPendPool) > 0
}

// InFlightReceipts retrieves whether there are receipt fetch requests currently
// in flight.
func (q *queue) InFlightReceipts() bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	return len(q.receiptPendPool) > 0
}

// Idle returns if the queue is fully idle or has some data still inside.
func (q *queue) Idle() bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	queued := q.blockTaskQueue.Size() + q.receiptTaskQueue.Size()
	pending := len(q.blockPendPool) + len(q.receiptPendPool)

	return (queued + pending) == 0
}

// Schedule adds a set of headers for the download queue for scheduling, returning
// the new headers encountered.
func (q *queue) Schedule(headers []*types.Header, hashes []common.Hash, from uint64) int {
	q.lock.Lock()
	defer q.lock.Unlock()

	// Insert all the headers prioritised by the contained block number
	var inserts int
	for i, header := range headers {
		// Make sure chain order is honoured and preserved throughout
		hash := hashes[i]
		if header.Number == nil || header.Number.Uint64() != from {
			log.Warn("Header broke chain ordering", "number", header.Number, "hash", hash, "expected", from)
			break
		}
		if q.headerHead != (common.Hash{}) && q.headerHead != header.ParentHash {
			log.Warn("Header broke chain ancestry", "number", header.Number, "hash", hash)
			break
		}
		// Make sure no duplicate requests are executed
		// We cannot skip this, even if the block is empty, since this is
		// what triggers the fetchResult creation.
		if _, ok := q.blockTaskPool[hash]; ok {
			log.Warn("Header already scheduled for block fetch", "number", header.Number, "hash", hash)
		} else {
			q.blockTaskPool[hash] = header
			q.blockTaskQueue.Push(header, -int64(header.Number.Uint64()))
		}
		// Queue for receipt retrieval
		if q.mode == ethconfig.SnapSync && !header.EmptyReceipts() {
			if _, ok := q.receiptTaskPool[hash]; ok {
				log.Warn("Header already scheduled for receipt fetch", "number", header.Number, "hash", hash)
			} else {
				q.receiptTaskPool[hash] = header
				q.receiptTaskQueue.Push(header, -int64(header.Number.Uint64()))
			}
		}
		inserts++
		q.headerHead = hash
		from++
	}
	return inserts
}

// Results retrieves and permanently removes a batch of fetch results from
// the cache. the result slice will be empty if the queue has been closed.
// Results can be called concurrently with Deliver and Schedule,
// but assumes that there are not two simultaneous callers to Results
func (q *queue) Results(block bool) []*fetchResult {
	// Abort early if there are no items and non-blocking requested
	if !block && !q.resultCache.HasCompletedItems() {
		return nil
	}
	closed := false
	for !closed && !q.resultCache.HasCompletedItems() {
		// In order to wait on 'active', we need to obtain the lock.
		// That may take a while, if someone is delivering at the same
		// time, so after obtaining the lock, we check again if there
		// are any results to fetch.
		// Also, in-between we ask for the lock and the lock is obtained,
		// someone can have closed the queue. In that case, we should
		// return the available results and stop blocking
		q.lock.Lock()
		if q.resultCache.HasCompletedItems() || q.closed {
			q.lock.Unlock()
			break
		}
		// No items available, and not closed
		q.active.Wait()
		closed = q.closed
		q.lock.Unlock()
	}
	// Regardless if closed or not, we can still deliver whatever we have
	results := q.resultCache.GetCompleted(maxResultsProcess)
	for _, result := range results {
		// Recalculate the result item weights to prevent memory exhaustion
		size := result.Header.Size()
		for _, uncle := range result.Uncles {
			size += uncle.Size()
		}
		size += common.StorageSize(len(result.Receipts))
		for _, tx := range result.Transactions {
			size += common.StorageSize(tx.Size())
		}
		size += common.StorageSize(result.Withdrawals.Size())
		q.resultSize = common.StorageSize(blockCacheSizeWeight)*size +
			(1-common.StorageSize(blockCacheSizeWeight))*q.resultSize
	}
	// Using the newly calibrated result size, figure out the new throttle limit
	// on the result cache
	throttleThreshold := uint64((common.StorageSize(blockCacheMemory) + q.resultSize - 1) / q.resultSize)
	throttleThreshold = q.resultCache.SetThrottleThreshold(throttleThreshold)

	// With results removed from the cache, wake throttled fetchers
	for _, ch := range []chan bool{q.blockWakeCh, q.receiptWakeCh} {
		select {
		case ch <- true:
		default:
		}
	}
	// Log some info at certain times
	if time.Since(q.logTime) >= 60*time.Second {
		q.logTime = time.Now()

		info := q.Stats()
		info = append(info, "throttle", throttleThreshold)
		log.Debug("Downloader queue stats", info...)
	}
	return results
}

func (q *queue) Stats() []interface{} {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return q.stats()
}

func (q *queue) stats() []interface{} {
	return []interface{}{
		"receiptTasks", q.receiptTaskQueue.Size(),
		"blockTasks", q.blockTaskQueue.Size(),
		"itemSize", q.resultSize,
	}
}

// ReserveBodies reserves a set of body fetches for the given peer, skipping any
// previously failed downloads. Beside the next batch of needed fetches, it also
// returns a flag whether empty blocks were queued requiring processing.
func (q *queue) ReserveBodies(p *peerConnection, count int) (*fetchRequest, bool, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.reserveHeaders(p, count, q.blockTaskPool, q.blockTaskQueue, q.blockPendPool, bodyType)
}

// ReserveReceipts reserves a set of receipt fetches for the given peer, skipping
// any previously failed downloads. Beside the next batch of needed fetches, it
// also returns a flag whether empty receipts were queued requiring importing.
func (q *queue) ReserveReceipts(p *peerConnection, count int) (*fetchRequest, bool, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.reserveHeaders(p, count, q.receiptTaskPool, q.receiptTaskQueue, q.receiptPendPool, receiptType)
}

// reserveHeaders reserves a set of data download operations for a given peer,
// skipping any previously failed ones. This method is a generic version used
// by the individual special reservation functions.
//
// Note, this method expects the queue lock to be already held for writing. The
// reason the lock is not obtained in here is because the parameters already need
// to access the queue, so they already need a lock anyway.
//
// Returns:
//
//	item     - the fetchRequest
//	progress - whether any progress was made
//	throttle - if the caller should throttle for a while
func (q *queue) reserveHeaders(p *peerConnection, count int, taskPool map[common.Hash]*types.Header, taskQueue *prque.Prque[int64, *types.Header],
	pendPool map[string]*fetchRequest, kind uint) (*fetchRequest, bool, bool) {
	// Short circuit if the pool has been depleted, or if the peer's already
	// downloading something (sanity check not to corrupt state)
	if taskQueue.Empty() {
		return nil, false, true
	}
	if _, ok := pendPool[p.id]; ok {
		return nil, false, false
	}
	// Retrieve a batch of tasks, skipping previously failed ones
	send := make([]*types.Header, 0, count)
	skip := make([]*types.Header, 0)
	progress := false
	throttled := false
	for proc := 0; len(send) < count && !taskQueue.Empty(); proc++ {
		// the task queue will pop items in order, so the highest prio block
		// is also the lowest block number.
		header, _ := taskQueue.Peek()

		// we can ask the resultcache if this header is within the
		// "prioritized" segment of blocks. If it is not, we need to throttle

		stale, throttle, item, err := q.resultCache.AddFetch(header, q.mode == ethconfig.SnapSync)
		if stale {
			// Don't put back in the task queue, this item has already been
			// delivered upstream
			taskQueue.PopItem()
			progress = true
			delete(taskPool, header.Hash())
			proc = proc - 1
			log.Error("Fetch reservation already delivered", "number", header.Number.Uint64())
			continue
		}
		if throttle {
			// There are no resultslots available. Leave it in the task queue
			// However, if there are any left as 'skipped', we should not tell
			// the caller to throttle, since we still want some other
			// peer to fetch those for us
			throttled = len(skip) == 0
			break
		}
		if err != nil {
			// this most definitely should _not_ happen
			log.Warn("Failed to reserve headers", "err", err)
			// There are no resultslots available. Leave it in the task queue
			break
		}
		if item.Done(kind) {
			// If it's a noop, we can skip this task
			delete(taskPool, header.Hash())
			taskQueue.PopItem()
			proc = proc - 1
			progress = true
			continue
		}
		// Remove it from the task queue
		taskQueue.PopItem()
		// Otherwise unless the peer is known not to have the data, add to the retrieve list
		if p.Lacks(header.Hash()) {
			skip = append(skip, header)
		} else {
			send = append(send, header)
		}
	}
	// Merge all the skipped headers back
	for _, header := range skip {
		taskQueue.Push(header, -int64(header.Number.Uint64()))
	}
	if q.resultCache.HasCompletedItems() {
		// Wake Results, resultCache was modified
		q.active.Signal()
	}
	// Assemble and return the block download request
	if len(send) == 0 {
		return nil, progress, throttled
	}
	request := &fetchRequest{
		Peer:    p,
		Headers: send,
		Time:    time.Now(),
	}
	pendPool[p.id] = request
	return request, progress, throttled
}

// Revoke cancels all pending requests belonging to a given peer. This method is
// meant to be called during a peer drop to quickly reassign owned data fetches
// to remaining nodes.
func (q *queue) Revoke(peerID string) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if request, ok := q.blockPendPool[peerID]; ok {
		for _, header := range request.Headers {
			q.blockTaskQueue.Push(header, -int64(header.Number.Uint64()))
		}
		delete(q.blockPendPool, peerID)
	}
	if request, ok := q.receiptPendPool[peerID]; ok {
		for _, header := range request.Headers {
			q.receiptTaskQueue.Push(header, -int64(header.Number.Uint64()))
		}
		delete(q.receiptPendPool, peerID)
	}
}

// ExpireBodies checks for in flight block body requests that exceeded a timeout
// allowance, canceling them and returning the responsible peers for penalisation.
func (q *queue) ExpireBodies(peer string) int {
	q.lock.Lock()
	defer q.lock.Unlock()

	bodyTimeoutMeter.Mark(1)
	return q.expire(peer, q.blockPendPool, q.blockTaskQueue)
}

// ExpireReceipts checks for in flight receipt requests that exceeded a timeout
// allowance, canceling them and returning the responsible peers for penalisation.
func (q *queue) ExpireReceipts(peer string) int {
	q.lock.Lock()
	defer q.lock.Unlock()

	receiptTimeoutMeter.Mark(1)
	return q.expire(peer, q.receiptPendPool, q.receiptTaskQueue)
}

// expire is the generic check that moves a specific expired task from a pending
// pool back into a task pool. The syntax on the passed taskQueue is a bit weird
// as we would need a generic expire method to handle both types, but that is not
// supported at the moment at least (Go 1.19).
//
// Note, this method expects the queue lock to be already held. The reason the
// lock is not obtained in here is that the parameters already need to access
// the queue, so they already need a lock anyway.
func (q *queue) expire(peer string, pendPool map[string]*fetchRequest, taskQueue interface{}) int {
	// Retrieve the request being expired and log an error if it's non-existent,
	// as there's no order of events that should lead to such expirations.
	req := pendPool[peer]
	if req == nil {
		log.Error("Expired request does not exist", "peer", peer)
		return 0
	}
	delete(pendPool, peer)

	// Return any non-satisfied requests to the pool
	if req.From > 0 {
		taskQueue.(*prque.Prque[int64, uint64]).Push(req.From, -int64(req.From))
	}
	for _, header := range req.Headers {
		taskQueue.(*prque.Prque[int64, *types.Header]).Push(header, -int64(header.Number.Uint64()))
	}
	return len(req.Headers)
}

// DeliverBodies injects a block body retrieval response into the results queue.
// The method returns the number of blocks bodies accepted from the delivery and
// also wakes any threads waiting for data delivery.
func (q *queue) DeliverBodies(id string, txLists [][]*types.Transaction, txListHashes []common.Hash,
	uncleLists [][]*types.Header, uncleListHashes []common.Hash,
	withdrawalLists [][]*types.Withdrawal, withdrawalListHashes []common.Hash,
) (int, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	validate := func(index int, header *types.Header) error {
		if txListHashes[index] != header.TxHash {
			return errInvalidBody
		}
		if uncleListHashes[index] != header.UncleHash {
			return errInvalidBody
		}
		if header.WithdrawalsHash == nil {
			// nil hash means that withdrawals should not be present in body
			if withdrawalLists[index] != nil {
				return errInvalidBody
			}
		} else { // non-nil hash: body must have withdrawals
			if withdrawalLists[index] == nil {
				return errInvalidBody
			}
			if withdrawalListHashes[index] != *header.WithdrawalsHash {
				return errInvalidBody
			}
		}
		// Blocks must have a number of blobs corresponding to the header gas usage,
		// and zero before the Cancun hardfork.
		var blobs int
		for _, tx := range txLists[index] {
			// Count the number of blobs to validate against the header's blobGasUsed
			blobs += len(tx.BlobHashes())

			// Validate the data blobs individually too
			if tx.Type() == types.BlobTxType {
				if len(tx.BlobHashes()) == 0 {
					return errInvalidBody
				}
				for _, hash := range tx.BlobHashes() {
					if !kzg4844.IsValidVersionedHash(hash[:]) {
						return errInvalidBody
					}
				}
				if tx.BlobTxSidecar() != nil {
					return errInvalidBody
				}
			}
		}
		if header.BlobGasUsed != nil {
			if want := *header.BlobGasUsed / params.BlobTxBlobGasPerBlob; uint64(blobs) != want { // div because the header is surely good vs the body might be bloated
				return errInvalidBody
			}
		} else {
			if blobs != 0 {
				return errInvalidBody
			}
		}
		return nil
	}

	reconstruct := func(index int, result *fetchResult) {
		result.Transactions = txLists[index]
		result.Uncles = uncleLists[index]
		result.Withdrawals = withdrawalLists[index]
		result.SetBodyDone()
	}
	return q.deliver(id, q.blockTaskPool, q.blockTaskQueue, q.blockPendPool,
		bodyReqTimer, bodyInMeter, bodyDropMeter, len(txLists), validate, reconstruct)
}

// DeliverReceipts injects a receipt retrieval response into the results queue.
// The method returns the number of transaction receipts accepted from the delivery
// and also wakes any threads waiting for data delivery.
func (q *queue) DeliverReceipts(id string, receiptList []rlp.RawValue, receiptListHashes []common.Hash) (int, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	validate := func(index int, header *types.Header) error {
		if receiptListHashes[index] != header.ReceiptHash {
			return errInvalidReceipt
		}
		return nil
	}
	reconstruct := func(index int, result *fetchResult) {
		result.Receipts = receiptList[index]
		result.SetReceiptsDone()
	}
	return q.deliver(id, q.receiptTaskPool, q.receiptTaskQueue, q.receiptPendPool,
		receiptReqTimer, receiptInMeter, receiptDropMeter, len(receiptList), validate, reconstruct)
}

// deliver injects a data retrieval response into the results queue.
//
// Note, this method expects the queue lock to be already held for writing. The
// reason this lock is not obtained in here is because the parameters already need
// to access the queue, so they already need a lock anyway.
func (q *queue) deliver(id string, taskPool map[common.Hash]*types.Header,
	taskQueue *prque.Prque[int64, *types.Header], pendPool map[string]*fetchRequest,
	reqTimer *metrics.Timer, resInMeter, resDropMeter *metrics.Meter,
	results int, validate func(index int, header *types.Header) error,
	reconstruct func(index int, result *fetchResult)) (int, error) {
	// Short circuit if the data was never requested
	request := pendPool[id]
	if request == nil {
		resDropMeter.Mark(int64(results))
		return 0, errNoFetchesPending
	}
	delete(pendPool, id)

	reqTimer.UpdateSince(request.Time)
	resInMeter.Mark(int64(results))

	// If no data items were retrieved, mark them as unavailable for the origin peer
	if results == 0 {
		for _, header := range request.Headers {
			request.Peer.MarkLacking(header.Hash())
		}
	}
	// Assemble each of the results with their headers and retrieved data parts
	var (
		accepted int
		failure  error
		i        int
		hashes   []common.Hash
	)
	for _, header := range request.Headers {
		// Short circuit assembly if no more fetch results are found
		if i >= results {
			break
		}
		// Validate the fields
		if err := validate(i, header); err != nil {
			failure = err
			break
		}
		hashes = append(hashes, header.Hash())
		i++
	}

	for _, header := range request.Headers[:i] {
		if res, stale, err := q.resultCache.GetDeliverySlot(header.Number.Uint64()); err == nil && !stale {
			reconstruct(accepted, res)
		} else {
			// else: between here and above, some other peer filled this result,
			// or it was indeed a no-op. This should not happen, but if it does it's
			// not something to panic about
			log.Error("Delivery stale", "stale", stale, "number", header.Number.Uint64(), "err", err)
			failure = errStaleDelivery
		}
		// Clean up a successful fetch
		delete(taskPool, hashes[accepted])
		accepted++
	}
	resDropMeter.Mark(int64(results - accepted))

	// Return all failed or missing fetches to the queue
	for _, header := range request.Headers[accepted:] {
		taskQueue.Push(header, -int64(header.Number.Uint64()))
	}
	// Wake up Results
	if accepted > 0 {
		q.active.Signal()
	}
	if failure == nil {
		return accepted, nil
	}
	// If none of the data was good, it's a stale delivery
	if accepted > 0 {
		return accepted, fmt.Errorf("partial failure: %v", failure)
	}
	return accepted, fmt.Errorf("%w: %v", failure, errStaleDelivery)
}

// Prepare configures the result cache to allow accepting and caching inbound
// fetch results.
func (q *queue) Prepare(offset uint64, mode SyncMode) {
	q.lock.Lock()
	defer q.lock.Unlock()

	// Prepare the queue for sync results
	q.resultCache.Prepare(offset)
	q.mode = mode
}
