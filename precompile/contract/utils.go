// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package contract

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/luxfi/geth/crypto"
)

// ErrOutOfGas is returned when there is not enough gas to execute an operation
var ErrOutOfGas = errors.New("out of gas")

// Gas costs for stateful precompiles
const (
	WriteGasCostPerSlot = 20_000
	ReadGasCostPerSlot  = 5_000

	// Per LOG operation.
	LogGas uint64 = 375 // from params/protocol_params.go
	// Gas cost of single topic of the LOG. Should be multiplied by the number of topics.
	LogTopicGas uint64 = 375 // from params/protocol_params.go
	// Per byte cost in a LOG operation's data. Should be multiplied by the byte size of the data.
	LogDataGas uint64 = 8 // from params/protocol_params.go
)

var functionSignatureRegex = regexp.MustCompile(`\w+\((\w*|(\w+,)+\w+)\)`)

// CalculateFunctionSelector returns the 4 byte function selector that results from [functionSignature]
// Ex. the function setBalance(addr address, balance uint256) should be passed in as the string:
// "setBalance(address,uint256)"
func CalculateFunctionSelector(functionSignature string) []byte {
	if !functionSignatureRegex.MatchString(functionSignature) {
		panic(fmt.Errorf("invalid function signature: %q", functionSignature))
	}
	hash := crypto.Keccak256([]byte(functionSignature))
	return hash[:4]
}

// DeductGas checks if [suppliedGas] is sufficient against [requiredGas] and deducts [requiredGas] from [suppliedGas].
func DeductGas(suppliedGas uint64, requiredGas uint64) (uint64, error) {
	if suppliedGas < requiredGas {
		return 0, ErrOutOfGas
	}
	return suppliedGas - requiredGas, nil
}