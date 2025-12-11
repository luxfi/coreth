// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Defines the interface for the configuration and execution of a precompile contract
package contract

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/holiman/uint256"
	"github.com/luxfi/geth/accounts/abi"
	"github.com/luxfi/geth/common"
	ethtypes "github.com/luxfi/geth/core/types"
)

// ErrExecutionReverted is returned when precompile execution is reverted
var ErrExecutionReverted = errors.New("execution reverted")

// StatefulPrecompiledContract is the interface for executing a precompiled contract with state access
type StatefulPrecompiledContract interface {
	// Run executes the precompiled contract with state access.
	Run(accessibleState AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error)
}

// RunStatefulPrecompileFunc is the function signature for a stateful precompile
type RunStatefulPrecompileFunc func(accessibleState AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error)

// StatefulPrecompileFunction defines a single function of a stateful precompile contract
type StatefulPrecompileFunction struct {
	selector  []byte
	execute   RunStatefulPrecompileFunc
	activator func(AccessibleState) bool
}

// NewStatefulPrecompileFunction creates a new stateful precompile function
func NewStatefulPrecompileFunction(selector []byte, execute RunStatefulPrecompileFunc) *StatefulPrecompileFunction {
	return &StatefulPrecompileFunction{
		selector: selector,
		execute:  execute,
	}
}

// NewStatefulPrecompileFunctionWithActivator creates a new stateful precompile function with an activator
func NewStatefulPrecompileFunctionWithActivator(selector []byte, execute RunStatefulPrecompileFunc, activator func(AccessibleState) bool) *StatefulPrecompileFunction {
	return &StatefulPrecompileFunction{
		selector:  selector,
		execute:   execute,
		activator: activator,
	}
}

// StatefulPrecompileWithFunctionSelectors wraps a StatefulPrecompileContract with function selector logic
type StatefulPrecompileWithFunctionSelectors struct {
	fallback  RunStatefulPrecompileFunc
	functions map[string]*StatefulPrecompileFunction
}

// NewStatefulPrecompileContract creates a new stateful precompile with fallback and functions
func NewStatefulPrecompileContract(fallback RunStatefulPrecompileFunc, functions []*StatefulPrecompileFunction) (StatefulPrecompiledContract, error) {
	functionMap := make(map[string]*StatefulPrecompileFunction)
	for _, fn := range functions {
		key := string(fn.selector)
		if _, exists := functionMap[key]; exists {
			return nil, ErrExecutionReverted
		}
		functionMap[key] = fn
	}
	return &StatefulPrecompileWithFunctionSelectors{
		fallback:  fallback,
		functions: functionMap,
	}, nil
}

// Run implements StatefulPrecompiledContract
func (s *StatefulPrecompileWithFunctionSelectors) Run(accessibleState AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	if len(input) < 4 {
		if s.fallback != nil {
			return s.fallback(accessibleState, caller, addr, input, suppliedGas, readOnly)
		}
		return nil, suppliedGas, ErrExecutionReverted
	}

	selector := string(input[:4])
	fn, exists := s.functions[selector]
	if !exists {
		if s.fallback != nil {
			return s.fallback(accessibleState, caller, addr, input, suppliedGas, readOnly)
		}
		return nil, suppliedGas, ErrExecutionReverted
	}

	if fn.activator != nil && !fn.activator(accessibleState) {
		return nil, suppliedGas, ErrExecutionReverted
	}

	return fn.execute(accessibleState, caller, addr, input[4:], suppliedGas, readOnly)
}

// StateReader provides read access to EVM state
type StateReader interface {
	GetState(common.Address, common.Hash) common.Hash
}

// StateDB is the interface for accessing EVM state
type StateDB interface {
	StateReader
	SetState(common.Address, common.Hash, common.Hash)

	SetNonce(common.Address, uint64)
	GetNonce(common.Address) uint64

	GetBalance(common.Address) *uint256.Int
	AddBalance(common.Address, *uint256.Int)

	CreateAccount(common.Address)
	Exist(common.Address) bool

	AddLog(*ethtypes.Log)
	GetPredicateStorageSlots(address common.Address, index int) ([]byte, bool)

	GetTxHash() common.Hash

	Snapshot() int
	RevertToSnapshot(int)
}

// AccessibleState defines the interface exposed to stateful precompile contracts
type AccessibleState interface {
	GetStateDB() StateDB
	GetBlockContext() BlockContext
	GetConsensusContext() context.Context
	GetChainConfig() ChainConfig
}

// ChainConfig provides chain configuration to precompiles
type ChainConfig interface {
	// GetFeeConfig returns the fee configuration at the given block number
	GetFeeConfig() FeeConfig
	// AllowedFeeRecipients returns true if fee recipients are restricted
	AllowedFeeRecipients() bool
}

// ConfigurationBlockContext defines the interface required to configure a precompile
type ConfigurationBlockContext interface {
	Number() *big.Int
	Timestamp() uint64
}

// BlockContext provides the block context to precompiles
type BlockContext interface {
	ConfigurationBlockContext
	// GetPredicateResults returns the result of verifying the predicates of the
	// given transaction, precompile address pair as a byte array.
	GetPredicateResults(txHash common.Hash, precompileAddress common.Address) []byte
}

// PrecompileConfig is the configuration for a precompile
type PrecompileConfig interface {
	// Key returns the unique key for this precompile config
	Key() string
	// Verify validates the precompile config
	Verify() error
	// Equal returns true if the given config is equal to this one
	Equal(PrecompileConfig) bool
}

// Configurator handles the configuration of a precompile
type Configurator interface {
	MakeConfig() PrecompileConfig
	Configure(
		chainConfig ChainConfig,
		precompileConfig PrecompileConfig,
		state StateDB,
		blockContext ConfigurationBlockContext,
	) error
}

// FeeConfig specifies the fee parameters for a chain
type FeeConfig struct {
	GasLimit        *big.Int `json:"gasLimit"`
	TargetBlockRate uint64   `json:"targetBlockRate"`
	MinBaseFee      *big.Int `json:"minBaseFee"`
	TargetGas       *big.Int `json:"targetGas"`
	BaseFeeChangeDenominator *big.Int `json:"baseFeeChangeDenominator"`
	MinBlockGasCost *big.Int `json:"minBlockGasCost"`
	MaxBlockGasCost *big.Int `json:"maxBlockGasCost"`
	BlockGasCostStep *big.Int `json:"blockGasCostStep"`
}

// ParseABI parses the given ABI string and returns the ABI object
func ParseABI(rawABI string) abi.ABI {
	parsedABI, err := abi.JSON(strings.NewReader(rawABI))
	if err != nil {
		panic(err)
	}
	return parsedABI
}

// IsDurangoActivated returns true if Durango is activated
func IsDurangoActivated(accessibleState AccessibleState) bool {
	// For now, we'll return true as Durango is active in production
	return true
}