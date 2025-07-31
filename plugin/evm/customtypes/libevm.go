// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customtypes

import (
	ethtypes "github.com/luxfi/geth/core/types"
)

var extras = ethtypes.RegisterExtras[
	HeaderExtra, *HeaderExtra,
	BlockBodyExtra, *BlockBodyExtra,
	isMultiCoin,
]()
