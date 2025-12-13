// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"github.com/luxfi/consensus/engine/chain/block"
	"github.com/luxfi/geth/common"
)

// Syncable extends block.StateSummary with additional getters for coreth.
type Syncable interface {
	block.StateSummary
	GetBlockHash() common.Hash
	GetBlockRoot() common.Hash
}

// SyncableParser parses byte-encoded summaries.
type SyncableParser interface {
	Parse(summaryBytes []byte, acceptImpl AcceptImplFn) (Syncable, error)
}

// AcceptImplFn is the accept implementation callback.
type AcceptImplFn func(Syncable) (block.StateSyncMode, error)
