// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/holiman/uint256"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/tracing"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/params"
	"github.com/luxfi/geth/precompile/contract"
)

// statefulPrecompileWrapper wraps a StatefulPrecompiledContract to implement PrecompiledContract
type statefulPrecompileWrapper struct {
	precompile contract.StatefulPrecompiledContract
	evm        *EVM
}

// RequiredGas calculates the gas required for the stateful precompile
func (s *statefulPrecompileWrapper) RequiredGas(input []byte) uint64 {
	// Default gas cost for stateful precompiles
	// This should be overridden by specific implementations
	return 10000
}

// Run executes the stateful precompile
func (s *statefulPrecompileWrapper) Run(input []byte) ([]byte, error) {
	if s.evm == nil {
		return nil, errors.New("EVM not set for stateful precompile")
	}

	// Create accessible state wrapper
	accessibleState := &accessibleStateImpl{
		evm:         s.evm,
		blockCtx:    s.evm.Context,
	}

	// Get caller from the EVM context
	caller := s.evm.TxContext.Origin

	// Run the stateful precompile with unlimited gas for now
	// In production, this should use proper gas metering
	ret, _, err := s.precompile.Run(
		accessibleState,
		caller,
		common.Address{}, // Contract address will be set by specific implementations
		input,
		^uint64(0), // Max gas
		false,      // Not read-only
	)

	return ret, err
}

// accessibleStateImpl provides state access to stateful precompiles
type accessibleStateImpl struct {
	evm      *EVM
	blockCtx BlockContext
}

func (a *accessibleStateImpl) GetStateDB() contract.StateDB {
	return &stateDBWrapper{stateDB: a.evm.StateDB}
}

func (a *accessibleStateImpl) GetBlockContext() contract.BlockContext {
	return &blockContextWrapper{ctx: a.blockCtx}
}

func (a *accessibleStateImpl) GetConsensusContext() context.Context {
	// Return a basic context for now
	return context.Background()
}

func (a *accessibleStateImpl) GetChainConfig() contract.ChainConfig {
	return &chainConfigWrapper{config: a.evm.chainConfig}
}

// stateDBWrapper wraps StateDB to implement contract.StateDB
type stateDBWrapper struct {
	stateDB StateDB
}

func (s *stateDBWrapper) GetState(addr common.Address, hash common.Hash) common.Hash {
	return s.stateDB.GetState(addr, hash)
}

func (s *stateDBWrapper) SetState(addr common.Address, key, value common.Hash) {
	s.stateDB.SetState(addr, key, value)
}

func (s *stateDBWrapper) SetNonce(addr common.Address, nonce uint64) {
	s.stateDB.SetNonce(addr, nonce, tracing.NonceChangeUnspecified)
}

func (s *stateDBWrapper) GetNonce(addr common.Address) uint64 {
	return s.stateDB.GetNonce(addr)
}

func (s *stateDBWrapper) GetBalance(addr common.Address) *uint256.Int {
	return s.stateDB.GetBalance(addr)
}

func (s *stateDBWrapper) AddBalance(addr common.Address, amount *uint256.Int) {
	s.stateDB.AddBalance(addr, amount, tracing.BalanceChangeUnspecified)
}

func (s *stateDBWrapper) CreateAccount(addr common.Address) {
	s.stateDB.CreateAccount(addr)
}

func (s *stateDBWrapper) Exist(addr common.Address) bool {
	return s.stateDB.Exist(addr)
}

func (s *stateDBWrapper) AddLog(log *types.Log) {
	s.stateDB.AddLog(log)
}

func (s *stateDBWrapper) GetPredicateStorageSlots(address common.Address, index int) ([]byte, bool) {
	// Not implemented in coreth yet
	return nil, false
}

func (s *stateDBWrapper) GetTxHash() common.Hash {
	// Return empty hash for now - should be set from transaction context
	return common.Hash{}
}

func (s *stateDBWrapper) Snapshot() int {
	return s.stateDB.Snapshot()
}

func (s *stateDBWrapper) RevertToSnapshot(id int) {
	s.stateDB.RevertToSnapshot(id)
}

// blockContextWrapper wraps BlockContext to implement contract.BlockContext
type blockContextWrapper struct {
	ctx BlockContext
}

func (b *blockContextWrapper) Number() *big.Int {
	return b.ctx.BlockNumber
}

func (b *blockContextWrapper) Timestamp() uint64 {
	return b.ctx.Time
}

func (b *blockContextWrapper) GetPredicateResults(txHash common.Hash, precompileAddress common.Address) []byte {
	// Not implemented in coreth yet
	return nil
}

// chainConfigWrapper wraps params.ChainConfig to implement contract.ChainConfig
type chainConfigWrapper struct {
	config *params.ChainConfig
}

func (c *chainConfigWrapper) GetFeeConfig() contract.FeeConfig {
	// Return default fee config for now
	// This should be properly implemented based on chain configuration
	return contract.FeeConfig{
		GasLimit:        big.NewInt(8000000),
		TargetBlockRate: 2,
		MinBaseFee:      big.NewInt(25000000000),
		TargetGas:       big.NewInt(15000000),
		BaseFeeChangeDenominator: big.NewInt(36),
		MinBlockGasCost: big.NewInt(0),
		MaxBlockGasCost: big.NewInt(1000000),
		BlockGasCostStep: big.NewInt(200000),
	}
}

func (c *chainConfigWrapper) AllowedFeeRecipients() bool {
	// Default to false - no fee recipient restrictions
	return false
}

// Stateful precompile implementations (placeholders for now)

// deployerAllowList controls who can deploy contracts
type deployerAllowList struct{}

func (d *deployerAllowList) RequiredGas(input []byte) uint64 {
	return 5000 // Base cost for reading/writing allowlist
}

func (d *deployerAllowList) Run(input []byte) ([]byte, error) {
	// TODO: Implement actual deployer allowlist logic
	return nil, fmt.Errorf("deployerAllowList not yet implemented")
}

// nativeMinter controls native token minting
type nativeMinter struct{}

func (n *nativeMinter) RequiredGas(input []byte) uint64 {
	return 10000 // Base cost for minting operations
}

func (n *nativeMinter) Run(input []byte) ([]byte, error) {
	// TODO: Implement actual native minter logic
	return nil, fmt.Errorf("nativeMinter not yet implemented")
}

// txAllowList controls who can submit transactions
type txAllowList struct{}

func (t *txAllowList) RequiredGas(input []byte) uint64 {
	return 5000 // Base cost for reading/writing allowlist
}

func (t *txAllowList) Run(input []byte) ([]byte, error) {
	// TODO: Implement actual tx allowlist logic
	return nil, fmt.Errorf("txAllowList not yet implemented")
}

// feeManager controls fee configuration
type feeManager struct{}

func (f *feeManager) RequiredGas(input []byte) uint64 {
	return 10000 // Base cost for fee management operations
}

func (f *feeManager) Run(input []byte) ([]byte, error) {
	// TODO: Implement actual fee manager logic
	return nil, fmt.Errorf("feeManager not yet implemented")
}

// rewardManager controls block rewards
type rewardManager struct{}

func (r *rewardManager) RequiredGas(input []byte) uint64 {
	return 10000 // Base cost for reward management operations
}

func (r *rewardManager) Run(input []byte) ([]byte, error) {
	// TODO: Implement actual reward manager logic
	return nil, fmt.Errorf("rewardManager not yet implemented")
}