// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fhe

import (
	"errors"
	"fmt"

	"github.com/luxfi/coreth/precompile/contract"
	"github.com/luxfi/geth/common"
)

// Function selectors (first 4 bytes of keccak256(signature))
var (
	// Core operations
	addSelector    = [4]byte{0x00, 0x00, 0x00, 0x01} // add(uint8,bytes,bytes)
	subSelector    = [4]byte{0x00, 0x00, 0x00, 0x02} // sub(uint8,bytes,bytes)
	mulSelector    = [4]byte{0x00, 0x00, 0x00, 0x03} // mul(uint8,bytes,bytes)
	
	// Comparison
	ltSelector     = [4]byte{0x00, 0x00, 0x00, 0x10} // lt(uint8,bytes,bytes)
	gtSelector     = [4]byte{0x00, 0x00, 0x00, 0x11} // gt(uint8,bytes,bytes)
	eqSelector     = [4]byte{0x00, 0x00, 0x00, 0x12} // eq(uint8,bytes,bytes)
	
	// Boolean gates (TFHE)
	andSelector    = [4]byte{0x00, 0x00, 0x00, 0x20} // and(uint8,bytes,bytes)
	orSelector     = [4]byte{0x00, 0x00, 0x00, 0x21} // or(uint8,bytes,bytes)
	xorSelector    = [4]byte{0x00, 0x00, 0x00, 0x22} // xor(uint8,bytes,bytes)
	notSelector    = [4]byte{0x00, 0x00, 0x00, 0x23} // not(uint8,bytes)
	
	// Control flow
	selectSelector = [4]byte{0x00, 0x00, 0x00, 0x30} // select(uint8,bytes,bytes,bytes)
	
	// Encryption/Decryption
	verifySelector = [4]byte{0x00, 0x00, 0x00, 0x40} // verify(uint8,bytes,int32)
	decryptSelector= [4]byte{0x00, 0x00, 0x00, 0x41} // decrypt(uint8,bytes,uint256)
)

// Errors
var (
	ErrInvalidSecurityLevel = errors.New("invalid security level")
	ErrInvalidInput         = errors.New("invalid input")
	ErrOperationFailed      = errors.New("FHE operation failed")
	ErrUnknownSelector      = errors.New("unknown function selector")
	ErrInsufficientGas      = errors.New("insufficient gas for FHE operation")
)

// Gas costs
const (
	GasVerify    = 5000
	GasAdd       = 10000
	GasSub       = 10000
	GasMul       = 15000
	GasCompare   = 12000
	GasAnd       = 50000  // TFHE gate with bootstrapping
	GasOr        = 50000
	GasXor       = 50000
	GasNot       = 1000   // No bootstrapping
	GasSelect    = 100000 // MUX operation
	GasDecrypt   = 200000 // Threshold decryption
)

// FHEPrecompile implements the FHE stateful precompile contract
var FHEPrecompile contract.StatefulPrecompiledContract = createFHEPrecompile()

func createFHEPrecompile() contract.StatefulPrecompiledContract {
	functions := []*contract.StatefulPrecompileFunction{
		contract.NewStatefulPrecompileFunction(addSelector[:], add),
		contract.NewStatefulPrecompileFunction(subSelector[:], sub),
		contract.NewStatefulPrecompileFunction(mulSelector[:], mul),
		contract.NewStatefulPrecompileFunction(ltSelector[:], lt),
		contract.NewStatefulPrecompileFunction(gtSelector[:], gt),
		contract.NewStatefulPrecompileFunction(eqSelector[:], eq),
		contract.NewStatefulPrecompileFunction(andSelector[:], and),
		contract.NewStatefulPrecompileFunction(orSelector[:], or),
		contract.NewStatefulPrecompileFunction(xorSelector[:], xor),
		contract.NewStatefulPrecompileFunction(notSelector[:], not),
		contract.NewStatefulPrecompileFunction(selectSelector[:], selectOp),
		contract.NewStatefulPrecompileFunction(verifySelector[:], verify),
		contract.NewStatefulPrecompileFunction(decryptSelector[:], decrypt),
	}

	// Create the stateful precompile with no fallback function
	statefulContract, err := contract.NewStatefulPrecompileContract(nil, functions)
	if err != nil {
		panic(err)
	}
	return statefulContract
}

// parseOperands extracts two ciphertext handles from input
func parseOperands(input []byte) (handle1, handle2 [32]byte, err error) {
	if len(input) < 65 { // 1 byte type + 32 bytes handle1 + 32 bytes handle2
		return handle1, handle2, ErrInvalidInput
	}
	copy(handle1[:], input[1:33])
	copy(handle2[:], input[33:65])
	return handle1, handle2, nil
}

// add performs homomorphic addition
func add(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasAdd {
		return nil, 0, ErrInsufficientGas
	}
	
	// Parse input
	if len(input) < 65 {
		return nil, suppliedGas - GasAdd, ErrInvalidInput
	}
	
	// TODO: Get ciphertexts from storage, perform addition, store result
	// For now, return a placeholder handle
	result := make([]byte, 32)
	return result, suppliedGas - GasAdd, nil
}

// sub performs homomorphic subtraction
func sub(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasSub {
		return nil, 0, ErrInsufficientGas
	}
	
	if len(input) < 65 {
		return nil, suppliedGas - GasSub, ErrInvalidInput
	}
	
	result := make([]byte, 32)
	return result, suppliedGas - GasSub, nil
}

// mul performs homomorphic multiplication
func mul(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasMul {
		return nil, 0, ErrInsufficientGas
	}
	
	if len(input) < 65 {
		return nil, suppliedGas - GasMul, ErrInvalidInput
	}
	
	result := make([]byte, 32)
	return result, suppliedGas - GasMul, nil
}

// lt performs homomorphic less-than comparison
func lt(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasCompare {
		return nil, 0, ErrInsufficientGas
	}
	
	if len(input) < 65 {
		return nil, suppliedGas - GasCompare, ErrInvalidInput
	}
	
	result := make([]byte, 32)
	return result, suppliedGas - GasCompare, nil
}

// gt performs homomorphic greater-than comparison
func gt(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasCompare {
		return nil, 0, ErrInsufficientGas
	}
	
	if len(input) < 65 {
		return nil, suppliedGas - GasCompare, ErrInvalidInput
	}
	
	result := make([]byte, 32)
	return result, suppliedGas - GasCompare, nil
}

// eq performs homomorphic equality comparison
func eq(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasCompare {
		return nil, 0, ErrInsufficientGas
	}
	
	if len(input) < 65 {
		return nil, suppliedGas - GasCompare, ErrInvalidInput
	}
	
	result := make([]byte, 32)
	return result, suppliedGas - GasCompare, nil
}

// and performs TFHE AND gate (with bootstrapping)
func and(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasAnd {
		return nil, 0, ErrInsufficientGas
	}
	
	if len(input) < 65 {
		return nil, suppliedGas - GasAnd, ErrInvalidInput
	}
	
	// TODO: Call luxfi/tfhe.AND()
	result := make([]byte, 32)
	return result, suppliedGas - GasAnd, nil
}

// or performs TFHE OR gate (with bootstrapping)
func or(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasOr {
		return nil, 0, ErrInsufficientGas
	}
	
	if len(input) < 65 {
		return nil, suppliedGas - GasOr, ErrInvalidInput
	}
	
	// TODO: Call luxfi/tfhe.OR()
	result := make([]byte, 32)
	return result, suppliedGas - GasOr, nil
}

// xor performs TFHE XOR gate (with bootstrapping)
func xor(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasXor {
		return nil, 0, ErrInsufficientGas
	}
	
	if len(input) < 65 {
		return nil, suppliedGas - GasXor, ErrInvalidInput
	}
	
	// TODO: Call luxfi/tfhe.XOR()
	result := make([]byte, 32)
	return result, suppliedGas - GasXor, nil
}

// not performs TFHE NOT gate (no bootstrapping needed)
func not(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasNot {
		return nil, 0, ErrInsufficientGas
	}
	
	if len(input) < 33 {
		return nil, suppliedGas - GasNot, ErrInvalidInput
	}
	
	// TODO: Call luxfi/tfhe.NOT()
	result := make([]byte, 32)
	return result, suppliedGas - GasNot, nil
}

// selectOp performs TFHE MUX: sel ? a : b
func selectOp(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasSelect {
		return nil, 0, ErrInsufficientGas
	}
	
	if len(input) < 97 { // 1 byte type + 3 * 32 bytes handles
		return nil, suppliedGas - GasSelect, ErrInvalidInput
	}
	
	// TODO: Call luxfi/tfhe.MUX()
	result := make([]byte, 32)
	return result, suppliedGas - GasSelect, nil
}

// verify imports and verifies an encrypted input
func verify(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasVerify {
		return nil, 0, ErrInsufficientGas
	}
	
	// TODO: Verify ciphertext format and store
	result := make([]byte, 32)
	return result, suppliedGas - GasVerify, nil
}

// decrypt initiates threshold decryption
func decrypt(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) ([]byte, uint64, error) {
	if suppliedGas < GasDecrypt {
		return nil, 0, ErrInsufficientGas
	}
	
	if readOnly {
		return nil, suppliedGas, fmt.Errorf("decrypt not allowed in read-only mode")
	}
	
	// TODO: Submit to threshold chain for decryption
	result := make([]byte, 32)
	return result, suppliedGas - GasDecrypt, nil
}
