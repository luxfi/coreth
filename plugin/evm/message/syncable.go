// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"github.com/luxfi/geth/common"
)

// TODO: Temporarily disabled - needs block.StateSummary from node
/*
type Syncable interface {
	block.StateSummary
	GetBlockHash() common.Hash
	GetBlockRoot() common.Hash
}

type SyncableParser interface {
	Parse(summaryBytes []byte, acceptImpl AcceptImplFn) (Syncable, error)
}

type AcceptImplFn func(Syncable) (block.StateSyncMode, error)
*/

// Temporary stubs
type Syncable interface {
	GetBlockHash() common.Hash
	GetBlockRoot() common.Hash
}

type SyncableParser interface {
	Parse(summaryBytes []byte, acceptImpl AcceptImplFn) (Syncable, error)
}

type AcceptImplFn func(Syncable) (int, error)
