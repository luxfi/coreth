// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customtypes

import (
	"sync"

	"github.com/luxfi/geth/common"
	ethtypes "github.com/luxfi/geth/core/types"
)

// Extra data storage using sync.Map for thread-safe access
// Note: Header extras are keyed by header hash because geth's Block.Header()
// returns a copy, so pointer-based keys don't work.
var (
	headerExtras sync.Map // map[common.Hash]*HeaderExtra (keyed by header hash)
	blockExtras  sync.Map // map[*ethtypes.Block]*BlockBodyExtra
	bodyExtras   sync.Map // map[*ethtypes.Body]*BlockBodyExtra
)

// extrasType provides access methods for attaching extra data to geth types
type extrasType struct {
	Block struct {
		Get func(*ethtypes.Block) *BlockBodyExtra
		Set func(*ethtypes.Block, *BlockBodyExtra)
	}
	Header struct {
		Get func(*ethtypes.Header) *HeaderExtra
		Set func(*ethtypes.Header, *HeaderExtra)
	}
	Body struct {
		Get func(*ethtypes.Body) *BlockBodyExtra
		Set func(*ethtypes.Body, *BlockBodyExtra)
	}
}

// headerKey computes the key for storing header extras.
// Uses the header hash for lookup since geth's Block.Header() returns copies.
func headerKey(h *ethtypes.Header) common.Hash {
	return h.Hash()
}

var extras = extrasType{
	Block: struct {
		Get func(*ethtypes.Block) *BlockBodyExtra
		Set func(*ethtypes.Block, *BlockBodyExtra)
	}{
		Get: func(b *ethtypes.Block) *BlockBodyExtra {
			if v, ok := blockExtras.Load(b); ok {
				return v.(*BlockBodyExtra)
			}
			return &BlockBodyExtra{}
		},
		Set: func(b *ethtypes.Block, extra *BlockBodyExtra) {
			blockExtras.Store(b, extra)
		},
	},
	Header: struct {
		Get func(*ethtypes.Header) *HeaderExtra
		Set func(*ethtypes.Header, *HeaderExtra)
	}{
		Get: func(h *ethtypes.Header) *HeaderExtra {
			key := headerKey(h)
			if v, ok := headerExtras.Load(key); ok {
				return v.(*HeaderExtra)
			}
			return &HeaderExtra{}
		},
		Set: func(h *ethtypes.Header, extra *HeaderExtra) {
			key := headerKey(h)
			headerExtras.Store(key, extra)
		},
	},
	Body: struct {
		Get func(*ethtypes.Body) *BlockBodyExtra
		Set func(*ethtypes.Body, *BlockBodyExtra)
	}{
		Get: func(b *ethtypes.Body) *BlockBodyExtra {
			if v, ok := bodyExtras.Load(b); ok {
				return v.(*BlockBodyExtra)
			}
			return &BlockBodyExtra{}
		},
		Set: func(b *ethtypes.Body, extra *BlockBodyExtra) {
			bodyExtras.Store(b, extra)
		},
	},
}
