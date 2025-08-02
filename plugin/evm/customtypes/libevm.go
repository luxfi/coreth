// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customtypes

import (
	ethtypes "github.com/luxfi/geth/core/types"
)

// TODO: Temporarily disabled - needs geth support
// var extras = ethtypes.RegisterExtras[
// 	HeaderExtra, *HeaderExtra,
// 	BlockBodyExtra, *BlockBodyExtra,
// ]()

// Temporary workaround to provide extras functionality
type extrasType struct {
	Block struct {
		Get func(*ethtypes.Block) *BlockBodyExtra
		Set func(*ethtypes.Block, *BlockBodyExtra)
	}
	Header struct {
		Get func(*ethtypes.Header) *HeaderExtra
		Set func(*ethtypes.Header, *HeaderExtra)
	}
}

var extras = extrasType{
	Block: struct {
		Get func(*ethtypes.Block) *BlockBodyExtra
		Set func(*ethtypes.Block, *BlockBodyExtra)
	}{
		Get: func(b *ethtypes.Block) *BlockBodyExtra {
			// Return default empty extra for now
			return &BlockBodyExtra{}
		},
		Set: func(b *ethtypes.Block, extra *BlockBodyExtra) {
			// No-op for now
		},
	},
	Header: struct {
		Get func(*ethtypes.Header) *HeaderExtra
		Set func(*ethtypes.Header, *HeaderExtra)
	}{
		Get: func(h *ethtypes.Header) *HeaderExtra {
			// Return default empty extra for now
			return &HeaderExtra{}
		},
		Set: func(h *ethtypes.Header, extra *HeaderExtra) {
			// No-op for now
		},
	},
}
