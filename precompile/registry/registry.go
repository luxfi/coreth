// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Module to facilitate the registration of precompiles and their configuration.
package registry

// Force imports of each precompile to ensure each precompile's init function runs and registers itself
// with the registry.
import (
	// Chain-integrated precompiles (stay in coreth)
	_ "github.com/luxfi/coreth/precompile/contracts/warp"

	// ============================================
	// Post-Quantum Cryptography (0x0600-0x06FF)
	// ============================================
	_ "github.com/luxfi/precompile/mldsa"    // ML-DSA signature verification (FIPS 204)
	_ "github.com/luxfi/precompile/mlkem"    // ML-KEM key encapsulation (FIPS 203)
	_ "github.com/luxfi/precompile/slhdsa"   // SLH-DSA stateless hash signatures (FIPS 205)
	_ "github.com/luxfi/precompile/pqcrypto" // Unified PQ crypto operations

	// ============================================
	// Privacy/Encryption (0x0700-0x07FF)
	// ============================================
	_ "github.com/luxfi/precompile/fhe"   // Fully Homomorphic Encryption
	_ "github.com/luxfi/precompile/ecies" // Elliptic Curve Integrated Encryption
	_ "github.com/luxfi/precompile/ring"  // Ring signatures (anonymity)
	_ "github.com/luxfi/precompile/hpke"  // Hybrid Public Key Encryption

	// ============================================
	// Threshold Signatures (0x0800-0x08FF)
	// ============================================
	_ "github.com/luxfi/precompile/frost"    // FROST threshold Schnorr
	_ "github.com/luxfi/precompile/cggmp21"  // CGGMP21 threshold ECDSA
	_ "github.com/luxfi/precompile/ringtail" // Threshold lattice (post-quantum)

	// ============================================
	// ZK Proofs (0x0900-0x09FF)
	// ============================================
	_ "github.com/luxfi/precompile/kzg4844" // KZG commitments (EIP-4844)

	// ============================================
	// Curves (0x0A00-0x0AFF)
	// ============================================
	_ "github.com/luxfi/precompile/secp256r1" // P-256/secp256r1 verification

	// ============================================
	// AI Mining (0x0300-0x03FF)
	// ============================================
	_ "github.com/luxfi/precompile/ai" // AI mining rewards, TEE verification

	// ============================================
	// DEX (0x0400-0x04FF)
	// ============================================
	_ "github.com/luxfi/precompile/dex" // Uniswap v4-style DEX

	// ============================================
	// Graph/Query Layer (0x0500-0x05FF)
	// ============================================
	_ "github.com/luxfi/precompile/graph" // GraphQL query interface
)

// This list is kept just for reference. The actual addresses defined in respective packages of precompiles.
// Note: it is important that none of these addresses conflict with each other or any other precompiles
// in /coreth/contracts/contracts/**.
//
// FHEPrecompileAddress = common.HexToAddress("0x0000000000000000000000000000000000000080") // 128

// WarpMessengerAddress = common.HexToAddress("0x0200000000000000000000000000000000000005")
// ADD PRECOMPILES BELOW
// NewPrecompileAddress = common.HexToAddress("0x02000000000000000000000000000000000000??")
