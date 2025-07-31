// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customtypes

import ethtypes "github.com/luxfi/geth/core/types"

// EmptyExtDataHash is the known hash of empty extdata bytes.
var EmptyExtDataHash = ethtypes.RLPHash([]byte(nil))
