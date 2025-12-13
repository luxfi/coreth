// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Defines the interface for the configuration and execution of a precompile contract
package contract

import (
	"math/big"

	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/coreth/precompile/precompileconfig"
	"github.com/luxfi/geth/common"
	ethtypes "github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/core/vm"
	"github.com/luxfi/geth/core/stateconf"
	"github.com/holiman/uint256"
)

// StatefulPrecompiledContract is the interface for executing a precompiled contract
type StatefulPrecompiledContract interface {
	// Run executes the precompiled contract.
	Run(accessibleState AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error)
}

// StateDB is the interface for accessing EVM state
type StateDB interface {
	GetState(common.Address, common.Hash, ...stateconf.StateDBStateOption) common.Hash
	SetState(common.Address, common.Hash, common.Hash, ...stateconf.StateDBStateOption)

	SetNonce(common.Address, uint64)
	GetNonce(common.Address) uint64

	GetBalance(common.Address) *uint256.Int
	AddBalance(common.Address, *uint256.Int)
	GetBalanceMultiCoin(common.Address, common.Hash) *big.Int
	AddBalanceMultiCoin(common.Address, common.Hash, *big.Int)
	SubBalanceMultiCoin(common.Address, common.Hash, *big.Int)

	CreateAccount(common.Address)
	Exist(common.Address) bool

	AddLog(*ethtypes.Log)
	Logs() []*ethtypes.Log
	GetPredicateStorageSlots(address common.Address, index int) ([]byte, bool)

	TxHash() common.Hash

	Snapshot() int
	RevertToSnapshot(int)
}

// AccessibleState defines the interface exposed to stateful precompile contracts
type AccessibleState interface {
	GetStateDB() StateDB
	GetBlockContext() BlockContext
	GetConsensusContext() *consensusctx.Context
	GetChainConfig() precompileconfig.ChainConfig
	GetPrecompileEnv() vm.PrecompileEnvironment
}

// ConfigurationBlockContext defines the interface required to configure a precompile.
type ConfigurationBlockContext interface {
	Number() *big.Int
	Timestamp() uint64
}

type BlockContext interface {
	ConfigurationBlockContext
	// GetPredicateResults returns the result of verifying the predicates of the
	// given transaction, precompile address pair as a byte array.
	GetPredicateResults(txHash common.Hash, precompileAddress common.Address) []byte
}

type Configurator interface {
	MakeConfig() precompileconfig.Config
	Configure(
		chainConfig precompileconfig.ChainConfig,
		precompileconfig precompileconfig.Config,
		state StateDB,
		blockContext ConfigurationBlockContext,
	) error
}
