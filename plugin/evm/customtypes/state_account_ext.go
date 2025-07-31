// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customtypes

import (
	ethtypes "github.com/luxfi/geth/core/types"
)

type isMultiCoin bool

var IsMultiCoinPayloads = extras.StateAccount

func IsMultiCoin(s ethtypes.StateOrSlimAccount) bool {
	return bool(IsMultiCoinPayloads.Get(s))
}
