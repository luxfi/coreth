// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package params

import (
	"maps"
	"math/big"
	"slices"

	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/coreth/nativeasset"
	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/precompile/contract"
	"github.com/luxfi/coreth/precompile/modules"
	"github.com/luxfi/coreth/precompile/precompileconfig"
	"github.com/luxfi/coreth/predicate"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/vm"
)

type RulesExtra extras.Rules

func GetRulesExtra(r Rules) *extras.Rules {
	// Return an empty Rules struct to prevent nil pointer dereference.
	// This means Lux-specific features (Apricot phases, Durango, etc.) won't be enabled
	// unless properly configured via chain config.
	// TODO: Implement proper rules payload mechanism to store/retrieve extra rules data
	return &extras.Rules{
		Precompiles:         make(map[common.Address]precompileconfig.Config),
		Predicaters:         make(map[common.Address]precompileconfig.Predicater),
		AccepterPrecompiles: make(map[common.Address]precompileconfig.Accepter),
	}
}

// Temporarily commented out geth hooks
// func (r RulesExtra) CanCreateContract(ac *geth.AddressContext, gas uint64, state geth.StateReader) (uint64, error) {
// 	return gas, nil
// }

// func (r RulesExtra) CanExecuteTransaction(_ common.Address, _ *common.Address, _ geth.StateReader) error {
// 	return nil
// }

// MinimumGasConsumption is a no-op.
func (r RulesExtra) MinimumGasConsumption(x uint64) uint64 {
	return x // Simply return the input for now
}

var PrecompiledContractsApricotPhase2 = map[common.Address]contract.StatefulPrecompiledContract{
	nativeasset.GenesisContractAddr:    &nativeasset.DeprecatedContract{},
	nativeasset.NativeAssetBalanceAddr: &nativeasset.NativeAssetBalance{GasCost: AssetBalanceApricot},
	nativeasset.NativeAssetCallAddr:    &nativeasset.NativeAssetCall{GasCost: AssetCallApricot, CallNewAccountGas: CallNewAccountGas},
}

var PrecompiledContractsApricotPhasePre6 = map[common.Address]contract.StatefulPrecompiledContract{
	nativeasset.GenesisContractAddr:    &nativeasset.DeprecatedContract{},
	nativeasset.NativeAssetBalanceAddr: &nativeasset.DeprecatedContract{},
	nativeasset.NativeAssetCallAddr:    &nativeasset.DeprecatedContract{},
}

var PrecompiledContractsApricotPhase6 = map[common.Address]contract.StatefulPrecompiledContract{
	nativeasset.GenesisContractAddr:    &nativeasset.DeprecatedContract{},
	nativeasset.NativeAssetBalanceAddr: &nativeasset.NativeAssetBalance{GasCost: AssetBalanceApricot},
	nativeasset.NativeAssetCallAddr:    &nativeasset.NativeAssetCall{GasCost: AssetCallApricot, CallNewAccountGas: CallNewAccountGas},
}

var PrecompiledContractsBanff = map[common.Address]contract.StatefulPrecompiledContract{
	nativeasset.GenesisContractAddr:    &nativeasset.DeprecatedContract{},
	nativeasset.NativeAssetBalanceAddr: &nativeasset.DeprecatedContract{},
	nativeasset.NativeAssetCallAddr:    &nativeasset.DeprecatedContract{},
}

func (r RulesExtra) ActivePrecompiles(existing []common.Address) []common.Address {
	var precompiles map[common.Address]contract.StatefulPrecompiledContract
	switch {
	case r.IsBanff:
		precompiles = PrecompiledContractsBanff
	case r.IsApricotPhase6:
		precompiles = PrecompiledContractsApricotPhase6
	case r.IsApricotPhasePre6:
		precompiles = PrecompiledContractsApricotPhasePre6
	case r.IsApricotPhase2:
		precompiles = PrecompiledContractsApricotPhase2
	}

	var addresses []common.Address
	addresses = slices.AppendSeq(addresses, maps.Keys(precompiles))
	addresses = append(addresses, existing...)
	return addresses
}

// precompileOverrideBuiltin specifies precompiles that were activated prior to the
// dynamic precompile activation registry.
// These were only active historically and are not active in the current network.
func (r RulesExtra) precompileOverrideBuiltin(addr common.Address) (vm.PrecompiledContract, bool) {
	var precompiles map[common.Address]contract.StatefulPrecompiledContract
	switch {
	case r.IsBanff:
		precompiles = PrecompiledContractsBanff
	case r.IsApricotPhase6:
		precompiles = PrecompiledContractsApricotPhase6
	case r.IsApricotPhasePre6:
		precompiles = PrecompiledContractsApricotPhasePre6
	case r.IsApricotPhase2:
		precompiles = PrecompiledContractsApricotPhase2
	}

	precompile, ok := precompiles[addr]
	if !ok {
		return nil, false
	}

	return makePrecompile(precompile), true
}

func makePrecompile(contract contract.StatefulPrecompiledContract) vm.PrecompiledContract {
	// TODO: Implement proper precompile wrapping for libevm
	// The run function would be:
	// run := func(env vm.PrecompileEnvironment, input []byte, suppliedGas uint64) ([]byte, uint64, error) {
	//     ... get header, predicateResults, accessibleState ...
	//     return contract.Run(accessibleState, env.Addresses().Caller, env.Addresses().Self, input, suppliedGas, env.ReadOnly())
	// }
	_ = contract // silence unused variable warning
	return nil
}

func (r RulesExtra) PrecompileOverride(addr common.Address) (vm.PrecompiledContract, bool) {
	if p, ok := r.precompileOverrideBuiltin(addr); ok {
		return p, true
	}
	if _, ok := r.Precompiles[addr]; !ok {
		return nil, false
	}
	module, ok := modules.GetPrecompileModuleByAddress(addr)
	if !ok {
		return nil, false
	}

	return makePrecompile(module.Contract), true
}

type accessibleState struct {
	env          vm.PrecompileEnvironment
	blockContext *precompileBlockContext
}

func (a accessibleState) GetStateDB() contract.StateDB {
	// TODO the contracts should be refactored to call `env.ReadOnlyState`
	// or `env.StateDB` based on the env.ReadOnly() flag
	// For now, return nil until we implement proper state handling
	return nil
}

func (a accessibleState) GetBlockContext() contract.BlockContext {
	return a.blockContext
}

func (a accessibleState) GetChainConfig() precompileconfig.ChainConfig {
	return GetExtra(a.env.ChainConfig())
}

func (a accessibleState) GetConsensusContext() *consensusctx.Context {
	return GetExtra(a.env.ChainConfig()).ConsensusCtx
}

func (a accessibleState) GetPrecompileEnv() vm.PrecompileEnvironment {
	return a.env
}

type precompileBlockContext struct {
	number           *big.Int
	time             uint64
	predicateResults *predicate.Results
}

func (p *precompileBlockContext) Number() *big.Int {
	return p.number
}

func (p *precompileBlockContext) Timestamp() uint64 {
	return p.time
}

func (p *precompileBlockContext) GetPredicateResults(txHash common.Hash, precompileAddress common.Address) []byte {
	if p.predicateResults == nil {
		return nil
	}
	return p.predicateResults.GetPredicateResults(txHash, precompileAddress)
}
