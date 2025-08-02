// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// warptest exposes common functionality for testing the warp package.
package warptest

import (
	"context"
	"slices"

	"github.com/luxfi/ids"
	"github.com/luxfi/node/consensus/choices"
	"github.com/luxfi/node/database"
	"github.com/luxfi/node/quasar/consensus/quasarman"
	"github.com/luxfi/node/quasar/consensus/quasarman/quasarmantest"
)

// EmptyBlockClient returns an error if a block is requested
var EmptyBlockClient BlockClient = MakeBlockClient()

type BlockClient func(ctx context.Context, blockID ids.ID) (quasarman.Block, error)

func (f BlockClient) GetAcceptedBlock(ctx context.Context, blockID ids.ID) (quasarman.Block, error) {
	return f(ctx, blockID)
}

// MakeBlockClient returns a new BlockClient that returns the provided blocks.
// If a block is requested that isn't part of the provided blocks, an error is
// returned.
func MakeBlockClient(blkIDs ...ids.ID) BlockClient {
	return func(_ context.Context, blkID ids.ID) (quasarman.Block, error) {
		if !slices.Contains(blkIDs, blkID) {
			return nil, database.ErrNotFound
		}

		return &quasarmantest.Block{
			TestDecidable: choices.TestDecidable{
				IDV:    blkID,
				StatusV: choices.Accepted,
			},
			IDV: blkID,
		}, nil
	}
}
