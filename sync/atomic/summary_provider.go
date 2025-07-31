// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package atomic

import (
	"fmt"

	"github.com/luxfi/coreth/plugin/evm/atomic/state"

	"github.com/luxfi/node/snow/engine/snowman/block"

	"github.com/luxfi/coreth/sync"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
)

var _ sync.SummaryProvider = (*SummaryProvider)(nil)

// SummaryProvider is the summary provider that provides the state summary for the atomic trie.
type SummaryProvider struct {
	trie *state.AtomicTrie
}

// Initialize initializes the summary provider with the atomic trie.
func (a *SummaryProvider) Initialize(trie *state.AtomicTrie) {
	a.trie = trie
}

// StateSummaryAtBlock returns the block state summary at [blk] if valid.
func (a *SummaryProvider) StateSummaryAtBlock(blk *types.Block) (block.StateSummary, error) {
	height := blk.NumberU64()
	atomicRoot, err := a.trie.Root(height)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve atomic trie root for height (%d): %w", height, err)
	}

	if atomicRoot == (common.Hash{}) {
		return nil, fmt.Errorf("atomic trie root not found for height (%d)", height)
	}

	summary, err := NewSummary(blk.Hash(), height, blk.Root(), atomicRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to construct syncable block at height %d: %w", height, err)
	}
	return summary, nil
}
