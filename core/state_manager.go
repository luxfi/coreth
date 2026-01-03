// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
//
// This file is a derived work, based on the go-ethereum library whose original
// notices appear below.
//
// It is distributed under a license compatible with the licensing terms of the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********
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

package core

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/luxfi/coreth/plugin/evm/customrawdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/ethdb"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// flushWindow is the distance to the [commitInterval] when we start
// optimistically flushing trie nodes to disk (only applicable in [pruning]
// mode).
//
// We perform this optimistic flushing to reduce synchronized database IO at the
// [commitInterval].
const flushWindow = 768

type TrieWriter interface {
	InsertTrie(block *types.Block) error // Handle inserted trie reference of [root]
	AcceptTrie(block *types.Block) error // Mark [root] as part of an accepted block
	RejectTrie(block *types.Block) error // Notify TrieWriter that the block containing [root] has been rejected
	CommitAccepted() error               // Commit pending accepted roots (for path scheme)
	Shutdown() error
}

type TrieDB interface {
	Dereference(root common.Hash) error
	Commit(root common.Hash, report bool) error
	Size() (common.StorageSize, common.StorageSize, common.StorageSize)
	Cap(limit common.StorageSize) error
}

func NewTrieWriter(db TrieDB, config *CacheConfig) TrieWriter {
	// Database should only be used in pruning mode, but we shouldn't explicitly manage this.
	if config.Pruning && config.StateScheme != customrawdb.DatabaseScheme {
		cm := &cappedMemoryTrieWriter{
			TrieDB:           db,
			memoryCap:        common.StorageSize(config.TrieDirtyLimit) * 1024 * 1024,
			targetCommitSize: common.StorageSize(config.TrieDirtyCommitTarget) * 1024 * 1024,
			imageCap:         4 * 1024 * 1024,
			commitInterval:   config.CommitInterval,
			tipBuffer:        NewBoundedBuffer(int(config.StateHistory), db.Dereference),
		}
		cm.flushStepSize = (cm.memoryCap - cm.targetCommitSize) / common.StorageSize(flushWindow)
		return cm
	} else {
		return &noPruningTrieWriter{
			TrieDB:      db,
			stateScheme: config.StateScheme,
		}
	}
}

type noPruningTrieWriter struct {
	TrieDB
	stateScheme      string
	lastAcceptedRoot common.Hash // Track last accepted root for path scheme deferred commit
}

func (np *noPruningTrieWriter) InsertTrie(block *types.Block) error {
	// We don't attempt to [Cap] here because we should never have
	// a significant amount of [TrieDB.Dirties] (we commit each block).
	return nil
}

func (np *noPruningTrieWriter) AcceptTrie(block *types.Block) error {
	root := block.Root()

	// For path scheme, we should NOT call Commit for every block because pathdb's
	// Commit(root, false) wipes ALL other layers in the tree (via tree.cap(root, 0)).
	// This breaks when accepting blocks in order because block N's commit removes
	// the layers for blocks N+1, N+2, etc.
	//
	// Instead, we defer the commit until all pending accepted blocks are processed.
	// The commit is triggered by calling CommitAccepted() after DrainAcceptorQueue.
	// This ensures that:
	// 1. All accepted blocks' layers are processed before any are committed
	// 2. Only the last accepted root is committed (persists all accepted state)
	// 3. Layers for non-accepted blocks (which are children) remain available
	if np.stateScheme == rawdb.PathScheme {
		np.lastAcceptedRoot = root
		return nil
	}
	// For hash scheme and database scheme, we commit each block's trie.
	// We don't need to call [Dereference] on the block root at the end of this
	// function because it is removed from the [TrieDB.Dirties] map in [Commit].
	return np.TrieDB.Commit(root, false)
}

// CommitAccepted commits the last accepted root for path scheme.
// This should be called after DrainAcceptorQueue to persist the accepted state.
func (np *noPruningTrieWriter) CommitAccepted() error {
	if np.stateScheme == rawdb.PathScheme && np.lastAcceptedRoot != (common.Hash{}) {
		if err := np.TrieDB.Commit(np.lastAcceptedRoot, false); err != nil {
			return fmt.Errorf("failed to commit accepted trie: %w", err)
		}
	}
	return nil
}

func (np *noPruningTrieWriter) RejectTrie(block *types.Block) error {
	// In archive mode (no pruning), we don't dereference rejected roots because:
	// 1. The state is either already committed to disk (for accepted blocks), or
	// 2. The nodes might be shared with other blocks that haven't been committed yet.
	//
	// For case 2, dereferencing could remove nodes from the dirty cache that are
	// still needed by other blocks with the same state root. This is particularly
	// problematic with blocks that have identical state roots.
	//
	// Since we're in archive mode and not trying to save memory, skipping
	// dereference is safe. The dirty cache will be cleared when nodes are
	// committed or when the blockchain shuts down.
	return nil
}

func (np *noPruningTrieWriter) Shutdown() error {
	// Commit any pending accepted root on shutdown
	return np.CommitAccepted()
}

type cappedMemoryTrieWriter struct {
	TrieDB
	memoryCap        common.StorageSize
	targetCommitSize common.StorageSize
	flushStepSize    common.StorageSize
	imageCap         common.StorageSize
	commitInterval   uint64

	tipBuffer *BoundedBuffer[common.Hash]
}

func (cm *cappedMemoryTrieWriter) InsertTrie(block *types.Block) error {
	// The use of [Cap] in [InsertTrie] prevents exceeding the configured memory
	// limit (and OOM) in case there is a large backlog of processing (unaccepted) blocks.
	_, nodes, imgs := cm.TrieDB.Size() // all memory is contained within the nodes return for hashdb
	if nodes <= cm.memoryCap && imgs <= cm.imageCap {
		return nil
	}
	if err := cm.TrieDB.Cap(cm.memoryCap - ethdb.IdealBatchSize); err != nil {
		return fmt.Errorf("failed to cap trie for block %s: %w", block.Hash().Hex(), err)
	}

	return nil
}

func (cm *cappedMemoryTrieWriter) AcceptTrie(block *types.Block) error {
	root := block.Root()

	// Attempt to dereference roots at least [tipBufferSize] old (so queries at tip
	// can still be completed).
	//
	// Note: It is safe to dereference roots that have been committed to disk
	// (they are no-ops).
	if err := cm.tipBuffer.Insert(root); err != nil {
		return err
	}

	// Commit this root if we have reached the [commitInterval].
	modCommitInterval := block.NumberU64() % cm.commitInterval
	if modCommitInterval == 0 {
		if err := cm.TrieDB.Commit(root, true); err != nil {
			return fmt.Errorf("failed to commit trie for block %s: %w", block.Hash().Hex(), err)
		}
		return nil
	}

	// Write at least [flushStepSize] of the oldest nodes in the trie database
	// dirty cache to disk as we approach the [commitInterval] to reduce the number of trie nodes
	// that will need to be written at once on [Commit] (to roughly [targetCommitSize]).
	//
	// To reduce the number of useless trie nodes that are committed during this
	// capping, we only optimistically flush within the [flushWindow]. During
	// this period, the [targetMemory] decreases stepwise by [flushStepSize]
	// as we get closer to the commit boundary.
	//
	// Most trie nodes are 300B, so we will write at least ~1000 trie nodes in
	// a single optimistic flush (with the default [flushStepSize]=312KB).
	distanceFromCommit := cm.commitInterval - modCommitInterval // this cannot be 0
	if distanceFromCommit > flushWindow {
		return nil
	}
	targetMemory := cm.targetCommitSize + cm.flushStepSize*common.StorageSize(distanceFromCommit)
	_, nodes, _ := cm.TrieDB.Size()
	if nodes <= targetMemory {
		return nil
	}
	targetCap := targetMemory - ethdb.IdealBatchSize
	if err := cm.TrieDB.Cap(targetCap); err != nil {
		return fmt.Errorf("failed to cap trie for block %s (target=%s): %w", block.Hash().Hex(), targetCap, err)
	}
	return nil
}

func (cm *cappedMemoryTrieWriter) RejectTrie(block *types.Block) error {
	// We don't dereference immediately because the rejected block's root might
	// be shared with other blocks that haven't been accepted yet. This is
	// particularly problematic with blocks that have identical state roots.
	//
	// Instead, we let the tipBuffer handle dereferencing for accepted blocks,
	// and rely on the Cap mechanism to limit memory usage. The memory impact
	// of not dereferencing rejected blocks is minimal since rejections are rare.
	//
	// Note: The original behavior was to call cm.TrieDB.Dereference(block.Root())
	// but this caused issues when two blocks had the same root.
	return nil
}

// CommitAccepted is a no-op for cappedMemoryTrieWriter since it handles
// commits at commitInterval rather than after DrainAcceptorQueue.
func (cm *cappedMemoryTrieWriter) CommitAccepted() error {
	return nil
}

func (cm *cappedMemoryTrieWriter) Shutdown() error {
	// If [tipBuffer] entry is empty, no need to do any cleanup on
	// shutdown.
	last, exists := cm.tipBuffer.Last()
	if !exists {
		return nil
	}

	// Attempt to commit last item added to [dereferenceQueue] on shutdown to avoid
	// re-processing the state on the next startup.
	return cm.TrieDB.Commit(last, true)
}
