// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomic

import "github.com/luxfi/constants"

// Canonical atomic-tx accounting constants. Under activate-all-implicitly there
// ones (no per-upgrade variants).
const (
	// LuxAtomicTxFee is the LUX amount burned per atomic tx.
	LuxAtomicTxFee = constants.MilliLux

	// AtomicTxBaseCost is the base intrinsic gas charged per atomic tx.
	AtomicTxBaseCost uint64 = 10_000

	// AtomicGasLimit caps the cumulative atomic gas consumed in a block.
	// A block may include any set of atomic txs whose total atomic gas is <=
	// this limit (analogous to the block gas limit for EVM txs).
	AtomicGasLimit uint64 = 100_000
)
