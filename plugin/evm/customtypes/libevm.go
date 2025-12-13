// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customtypes

import (
	"sync"

	ethtypes "github.com/luxfi/geth/core/types"
)

// Extra data storage using sync.Map for thread-safe access
var (
	headerExtras sync.Map // map[*ethtypes.Header]*HeaderExtra
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
			if v, ok := headerExtras.Load(h); ok {
				return v.(*HeaderExtra)
			}
			return &HeaderExtra{}
		},
		Set: func(h *ethtypes.Header, extra *HeaderExtra) {
			headerExtras.Store(h, extra)
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
