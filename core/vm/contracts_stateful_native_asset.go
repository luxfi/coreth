// (c) 2019-2020, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"math/big"

	"github.com/luxfi/geth/precompile/contract"
	"github.com/luxfi/geth/vmerrs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

// PrecompiledContractsApricot contains the default set of pre-compiled Ethereum
// contracts used in the Istanbul release and the stateful precompiled contracts
// added for the Lux Apricot release.
// Apricot is incompatible with the YoloV3 Release since it does not include the
// BLS12-381 Curve Operations added to the set of precompiled contracts

var (
	genesisContractAddr    = common.HexToAddress("0x0100000000000000000000000000000000000000")
	NativeAssetBalanceAddr = common.HexToAddress("0x0100000000000000000000000000000000000001")
	NativeAssetCallAddr    = common.HexToAddress("0x0100000000000000000000000000000000000002")
)

// nativeAssetBalance is a precompiled contract used to retrieve the native asset balance
type nativeAssetBalance struct {
	gasCost uint64
}

// PackNativeAssetBalanceInput packs the arguments into the required input data for a transaction to be passed into
// the native asset balance contract.
func PackNativeAssetBalanceInput(address common.Address, assetID common.Hash) []byte {
	input := make([]byte, 52)
	copy(input, address.Bytes())
	copy(input[20:], assetID.Bytes())
	return input
}

// UnpackNativeAssetBalanceInput attempts to unpack [input] into the arguments to the native asset balance precompile
func UnpackNativeAssetBalanceInput(input []byte) (common.Address, common.Hash, error) {
	if len(input) != 52 {
		return common.Address{}, common.Hash{}, fmt.Errorf("native asset balance input had unexpcted length %d", len(input))
	}
	address := common.BytesToAddress(input[:20])
	assetID := common.Hash{}
	assetID.SetBytes(input[20:52])
	return address, assetID, nil
}

// Run implements StatefulPrecompiledContract
func (b *nativeAssetBalance) Run(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	// input: encodePacked(address 20 bytes, assetID 32 bytes)
	if suppliedGas < b.gasCost {
		return nil, 0, vmerrs.ErrOutOfGas
	}
	remainingGas = suppliedGas - b.gasCost

	address, assetID, err := UnpackNativeAssetBalanceInput(input)
	if err != nil {
		return nil, remainingGas, vmerrs.ErrExecutionReverted
	}

	res, overflow := uint256.FromBig(accessibleState.GetStateDB().GetBalanceMultiCoin(address, assetID))
	if overflow {
		return nil, remainingGas, vmerrs.ErrExecutionReverted
	}
	return common.LeftPadBytes(res.Bytes(), 32), remainingGas, nil
}

// nativeAssetCall atomically transfers a native asset to a recipient address as well as calling that
// address
type nativeAssetCall struct {
	gasCost uint64
}

// PackNativeAssetCallInput packs the arguments into the required input data for a transaction to be passed into
// the native asset contract.
// Assumes that [assetAmount] is non-nil.
func PackNativeAssetCallInput(address common.Address, assetID common.Hash, assetAmount *big.Int, callData []byte) []byte {
	input := make([]byte, 84+len(callData))
	copy(input[0:20], address.Bytes())
	copy(input[20:52], assetID.Bytes())
	assetAmount.FillBytes(input[52:84])
	copy(input[84:], callData)
	return input
}

// UnpackNativeAssetCallInput attempts to unpack [input] into the arguments to the native asset call precompile
func UnpackNativeAssetCallInput(input []byte) (common.Address, common.Hash, *big.Int, []byte, error) {
	if len(input) < 84 {
		return common.Address{}, common.Hash{}, nil, nil, fmt.Errorf("native asset call input had unexpected length %d", len(input))
	}
	to := common.BytesToAddress(input[:20])
	assetID := common.BytesToHash(input[20:52])
	assetAmount := new(big.Int).SetBytes(input[52:84])
	callData := input[84:]
	return to, assetID, assetAmount, callData, nil
}

// Run implements StatefulPrecompiledContract
func (c *nativeAssetCall) Run(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	// input: encodePacked(address 20 bytes, assetID 32 bytes, assetAmount 32 bytes, callData variable length bytes)
	return accessibleState.NativeAssetCall(caller, input, suppliedGas, c.gasCost, readOnly)
}

type deprecatedContract struct{}

func (*deprecatedContract) Run(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	return nil, suppliedGas, vmerrs.ErrExecutionReverted
}
