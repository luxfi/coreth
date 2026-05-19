// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gaslimit_initial

import "github.com/luxfi/coreth/utils"

const (
	// Phase 1 upgrade.
	//
	MinGasPrice = 225 * utils.GWei

	// GasLimit is the target amount of gas that can be included in a single
	//
	// This value encodes the default parameterization of the initial gas
	// targeting mechanism.
	GasLimit = 8_000_000
)
