// (c) 2021-2024, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/luxfi/coreth/precompile/contract"
)

// wrappedPrecompiledContract implements StatefulPrecompiledContract by wrapping stateless native precompiled contracts
// in Ethereum.
type wrappedPrecompiledContract struct {
	p PrecompiledContract
}

func newWrappedPrecompiledContract(p PrecompiledContract) contract.StatefulPrecompiledContract {
	return &wrappedPrecompiledContract{p: p}
}

// Run implements the StatefulPrecompiledContract interface
func (w *wrappedPrecompiledContract) Run(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	return RunPrecompiledContract(w.p, input, suppliedGas)
}

// RunStatefulPrecompiledContract confirms runs [precompile] with the specified parameters.
func RunStatefulPrecompiledContract(precompile contract.StatefulPrecompiledContract, accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	return precompile.Run(accessibleState, caller, addr, input, suppliedGas, readOnly)
}
