// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Module to facilitate the registration of precompiles and their configuration.
package registry

// Force imports of each precompile to ensure each precompile's init function runs and registers itself
// with the registry.
import (
	// Chain-integrated precompiles (stay in coreth)
	_ "github.com/luxfi/coreth/precompile/contracts/warp"

	// Crypto precompiles from standalone precompiles package
	_ "github.com/luxfi/precompiles/fhe"
	_ "github.com/luxfi/precompiles/mldsa"
	_ "github.com/luxfi/precompiles/pqcrypto"
)

// This list is kept just for reference. The actual addresses defined in respective packages of precompiles.
// Note: it is important that none of these addresses conflict with each other or any other precompiles
// in /coreth/contracts/contracts/**.
//
// FHEPrecompileAddress = common.HexToAddress("0x0000000000000000000000000000000000000080") // 128

// WarpMessengerAddress = common.HexToAddress("0x0200000000000000000000000000000000000005")
// ADD PRECOMPILES BELOW
// NewPrecompileAddress = common.HexToAddress("0x02000000000000000000000000000000000000??")
