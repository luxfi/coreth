// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package params

import (
	"maps"
	"math/big"
	"slices"

	"github.com/holiman/uint256"
	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/coreth/nativeasset"
	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/precompile/contract"
	"github.com/luxfi/coreth/precompile/modules"
	"github.com/luxfi/coreth/precompile/precompileconfig"
	"github.com/luxfi/coreth/predicate"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/tracing"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/core/vm"
)

type RulesExtra extras.Rules

func GetRulesExtra(r Rules) *extras.Rules {
	// If Payload is set and is a RulesExtra (which is an alias for extras.Rules), convert and return it
	if extra, ok := r.Payload.(RulesExtra); ok {
		return (*extras.Rules)(&extra)
	}
	// Also check for *extras.Rules for backwards compatibility
	if extra, ok := r.Payload.(*extras.Rules); ok && extra != nil {
		return extra
	}
	// Return an empty Rules struct to prevent nil pointer dereference.
	// This means Lux-specific features (Apricot phases, Durango, etc.) won't be enabled
	// unless properly configured via chain config.
	return &extras.Rules{
		Precompiles:         make(map[common.Address]precompileconfig.Config),
		Predicaters:         make(map[common.Address]precompileconfig.Predicater),
		AccepterPrecompiles: make(map[common.Address]precompileconfig.Accepter),
	}
}

// SetRulesExtra sets the extra rules data in the Payload field
func SetRulesExtra(r *Rules, extra *extras.Rules) {
	r.Payload = extra
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

// In Lux, native asset precompiles remain active after Banff (unlike Avalanche).
// Only the Genesis contract is deprecated.
var PrecompiledContractsBanff = map[common.Address]contract.StatefulPrecompiledContract{
	nativeasset.GenesisContractAddr:    &nativeasset.DeprecatedContract{},
	nativeasset.NativeAssetBalanceAddr: &nativeasset.NativeAssetBalance{GasCost: AssetBalanceApricot},
	nativeasset.NativeAssetCallAddr:    &nativeasset.NativeAssetCall{GasCost: AssetCallApricot, CallNewAccountGas: CallNewAccountGas},
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

// statefulPrecompileWrapper wraps a coreth StatefulPrecompiledContract to implement
// geth's vm.StatefulPrecompiledContract interface
type statefulPrecompileWrapper struct {
	contract contract.StatefulPrecompiledContract
}

// RequiredGas returns 0 because stateful precompiles handle gas internally
func (w *statefulPrecompileWrapper) RequiredGas(input []byte) uint64 {
	return 0
}

// Run is the standard precompile interface - should not be called for stateful precompiles
func (w *statefulPrecompileWrapper) Run(input []byte) ([]byte, error) {
	// This should not be called - RunStateful should be called instead
	return nil, vm.ErrExecutionReverted
}

// Name returns the name of the precompile for debugging
func (w *statefulPrecompileWrapper) Name() string {
	return "StatefulPrecompile"
}

// RunStateful implements vm.StatefulPrecompiledContract
func (w *statefulPrecompileWrapper) RunStateful(env vm.PrecompileEnvironment, input []byte, suppliedGas uint64) ([]byte, uint64, error) {
	// Create accessible state from the environment
	accessState := accessibleState{
		env: env,
		blockContext: &precompileBlockContext{
			number: env.BlockNumber(),
			time:   env.BlockTime(),
			// predicateResults will be nil for now - can be enhanced later
		},
	}

	addrs := env.Addresses()
	ret, returnedGas, err := w.contract.Run(accessState, addrs.Caller, addrs.Self, input, suppliedGas, env.ReadOnly())
	// Handle two different gas accounting patterns:
	// 1. Some precompiles (NativeAssetBalance) calculate and return remaining gas directly
	// 2. Other precompiles (NativeAssetCall) use env.UseGas() and return suppliedGas unchanged
	// Use min(returnedGas, env.Gas()) to correctly handle both patterns
	envGas := env.Gas()
	if returnedGas < envGas {
		return ret, returnedGas, err
	}
	return ret, envGas, err
}

func makePrecompile(c contract.StatefulPrecompiledContract) vm.PrecompiledContract {
	return &statefulPrecompileWrapper{contract: c}
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
	// Wrap the vm.StateDB with multicoin support
	if a.env.ReadOnly() {
		return &stateDBWrapper{state: a.env.ReadOnlyState()}
	}
	return &stateDBWrapper{state: a.env.StateDB()}
}

// stateDBWrapper wraps vm.StateDB to implement contract.StateDB with multicoin support
type stateDBWrapper struct {
	state vm.StateDB
}

func (w *stateDBWrapper) GetState(addr common.Address, hash common.Hash) common.Hash {
	return w.state.GetState(addr, hash)
}

func (w *stateDBWrapper) SetState(addr common.Address, key, value common.Hash) common.Hash {
	// vm.StateDB.SetState returns the previous value
	return w.state.SetState(addr, key, value)
}

func (w *stateDBWrapper) SetNonce(addr common.Address, nonce uint64, reason tracing.NonceChangeReason) {
	w.state.SetNonce(addr, nonce, reason)
}

func (w *stateDBWrapper) GetNonce(addr common.Address) uint64 {
	return w.state.GetNonce(addr)
}

func (w *stateDBWrapper) GetBalance(addr common.Address) *uint256.Int {
	return w.state.GetBalance(addr)
}

func (w *stateDBWrapper) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	return w.state.AddBalance(addr, amount, reason)
}

func (w *stateDBWrapper) SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	return w.state.SubBalance(addr, amount, reason)
}

// GetBalanceMultiCoin implements multicoin balance reading using state storage
func (w *stateDBWrapper) GetBalanceMultiCoin(addr common.Address, coinID common.Hash) *big.Int {
	// Normalize coinID (set 0th bit to 1 to partition multicoin from normal state)
	coinID[0] |= 0x01
	return w.state.GetState(addr, coinID).Big()
}

// AddBalanceMultiCoin adds multicoin balance using state storage
func (w *stateDBWrapper) AddBalanceMultiCoin(addr common.Address, coinID common.Hash, amount *big.Int) {
	if amount.Sign() == 0 {
		w.state.AddBalance(addr, new(uint256.Int), tracing.BalanceChangeUnspecified)
		return
	}
	newAmount := new(big.Int).Add(w.GetBalanceMultiCoin(addr, coinID), amount)
	coinID[0] |= 0x01
	w.state.SetState(addr, coinID, common.BigToHash(newAmount))
}

// SubBalanceMultiCoin subtracts multicoin balance using state storage
func (w *stateDBWrapper) SubBalanceMultiCoin(addr common.Address, coinID common.Hash, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	newAmount := new(big.Int).Sub(w.GetBalanceMultiCoin(addr, coinID), amount)
	coinID[0] |= 0x01
	w.state.SetState(addr, coinID, common.BigToHash(newAmount))
}

func (w *stateDBWrapper) CreateAccount(addr common.Address) {
	w.state.CreateAccount(addr)
}

func (w *stateDBWrapper) Exist(addr common.Address) bool {
	return w.state.Exist(addr)
}

func (w *stateDBWrapper) AddLog(log *types.Log) {
	w.state.AddLog(log)
}

func (w *stateDBWrapper) Logs() []*types.Log {
	return w.state.Logs()
}

func (w *stateDBWrapper) GetPredicateStorageSlots(address common.Address, index int) ([]byte, bool) {
	// Try to get predicate storage slots if the underlying state supports it
	if ps, ok := w.state.(interface {
		GetPredicateStorageSlots(common.Address, int) ([]byte, bool)
	}); ok {
		return ps.GetPredicateStorageSlots(address, index)
	}
	return nil, false
}

func (w *stateDBWrapper) TxHash() common.Hash {
	return w.state.TxHash()
}

func (w *stateDBWrapper) Snapshot() int {
	return w.state.Snapshot()
}

func (w *stateDBWrapper) RevertToSnapshot(id int) {
	w.state.RevertToSnapshot(id)
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
