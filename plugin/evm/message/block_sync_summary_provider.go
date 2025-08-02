// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package message

import (
	"github.com/luxfi/geth/core/types"
)

type BlockSyncSummaryProvider struct{}

// StateSummaryAtBlock returns the block state summary at [block] if valid.
// TODO: Temporarily disabled - needs block.StateSummary from node
func (a *BlockSyncSummaryProvider) StateSummaryAtBlock(blk *types.Block) (Syncable, error) {
	return NewBlockSyncSummary(blk.Hash(), blk.NumberU64(), blk.Root())
}
