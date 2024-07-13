// (c) 2021-2024, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package handlers

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/luxfi/coreth/core/state/snapshot"
	"github.com/luxfi/coreth/core/types"
)

type BlockProvider interface {
	GetBlock(common.Hash, uint64) *types.Block
}

type SnapshotProvider interface {
	Snapshots() *snapshot.Tree
}

type SyncDataProvider interface {
	BlockProvider
	SnapshotProvider
}
