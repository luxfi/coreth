// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package params

// LuxGenesisBaseFee is the base fee set on the genesis block. Historically this
// was the EIP-1559 "initial" base fee installed at the Apricot Phase 3 upgrade
// boundary; under activate-all-implicitly every chain starts with this value
// from block 0.
const LuxGenesisBaseFee = 25 * GWei
