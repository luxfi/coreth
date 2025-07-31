// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"github.com/luxfi/geth/common"

	"github.com/luxfi/node/snow/engine/snowman/block"
)

type Syncable interface {
	block.StateSummary
	GetBlockHash() common.Hash
	GetBlockRoot() common.Hash
}

type SyncableParser interface {
	Parse(summaryBytes []byte, acceptImpl AcceptImplFn) (Syncable, error)
}

type AcceptImplFn func(Syncable) (block.StateSyncMode, error)
