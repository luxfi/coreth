// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// warptest exposes common functionality for testing the warp package.
package warptest

import (
	"context"
	"slices"

	"github.com/luxfi/consensus/core/choices"
	"github.com/luxfi/consensus/engine/chain/block"
	"github.com/luxfi/consensus/engine/chain/block/blocktest"
	consensustest "github.com/luxfi/consensus/test/helpers"
	"github.com/luxfi/database"
	"github.com/luxfi/ids"
)

// EmptyBlockClient returns an error if a block is requested
var EmptyBlockClient BlockClient = MakeBlockClient()

type BlockClient func(ctx context.Context, blockID ids.ID) (block.Block, error)

func (f BlockClient) GetAcceptedBlock(ctx context.Context, blockID ids.ID) (block.Block, error) {
	return f(ctx, blockID)
}

// MakeBlockClient returns a new BlockClient that returns the provided blocks.
// If a block is requested that isn't part of the provided blocks, an error is
// returned.
func MakeBlockClient(blkIDs ...ids.ID) BlockClient {
	return func(_ context.Context, blkID ids.ID) (block.Block, error) {
		if !slices.Contains(blkIDs, blkID) {
			return nil, database.ErrNotFound
		}

		return &blocktest.Block{
			Decidable: consensustest.Decidable{
				IDV:     blkID,
				StatusV: choices.Accepted,
			},
		}, nil
	}
}
