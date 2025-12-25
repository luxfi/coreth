// Copyright (C) 2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package params

import (
	"github.com/luxfi/coreth/precompile/contract"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/precompiles/mldsa"
	"github.com/luxfi/precompiles/pqcrypto"
	"github.com/luxfi/precompiles/slhdsa"

	pqcontract "github.com/luxfi/precompiles/contract"
)

// pqPrecompileAdapter wraps a precompiles.StatefulPrecompiledContract to implement
// coreth's contract.StatefulPrecompiledContract interface.
type pqPrecompileAdapter struct {
	inner pqcontract.StatefulPrecompiledContract
}

// Run implements contract.StatefulPrecompiledContract.
// The PQ crypto precompiles don't use AccessibleState, so we can safely pass nil.
func (p *pqPrecompileAdapter) Run(
	_ contract.AccessibleState,
	caller common.Address,
	addr common.Address,
	input []byte,
	suppliedGas uint64,
	readOnly bool,
) (ret []byte, remainingGas uint64, err error) {
	return p.inner.Run(nil, caller, addr, input, suppliedGas, readOnly)
}

// PQCryptoPrecompiles returns the post-quantum cryptography precompiles.
// These provide ML-DSA, SLH-DSA, and ML-KEM signature verification and key encapsulation.
var PQCryptoPrecompiles = map[common.Address]contract.StatefulPrecompiledContract{
	// ML-DSA (FIPS 204) - Lattice-based signatures
	mldsa.ContractMLDSAVerifyAddress: &pqPrecompileAdapter{inner: mldsa.MLDSAVerifyPrecompile},

	// SLH-DSA (FIPS 205) - Hash-based signatures
	slhdsa.ContractSLHDSAVerifyAddress: &pqPrecompileAdapter{inner: slhdsa.SLHDSAVerifyPrecompile},

	// PQCrypto - General post-quantum operations including ML-KEM
	pqcrypto.ContractAddress: &pqPrecompileAdapter{inner: pqcrypto.PQCryptoPrecompile},
}

func init() {
	// Add PQ crypto precompiles to Banff and later upgrade phases
	for addr, precompile := range PQCryptoPrecompiles {
		PrecompiledContractsBanff[addr] = precompile
		PrecompiledContractsApricotPhase6[addr] = precompile
	}
}
