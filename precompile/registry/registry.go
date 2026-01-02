// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Module to facilitate the registration of precompiles and their configuration.
package registry

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/holiman/uint256"
	corethcontract "github.com/luxfi/coreth/precompile/contract"
	corethmods "github.com/luxfi/coreth/precompile/modules"
	corethconfig "github.com/luxfi/coreth/precompile/precompileconfig"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/tracing"
	ethtypes "github.com/luxfi/geth/core/types"

	// Chain-integrated precompiles (stay in coreth)
	_ "github.com/luxfi/coreth/precompile/contracts/warp"

	// External precompile packages
	precompilecontract "github.com/luxfi/precompile/contract"
	precompilemods "github.com/luxfi/precompile/modules"
	precompileconfig "github.com/luxfi/precompile/precompileconfig"

	// ============================================
	// Post-Quantum Cryptography (0x0600-0x06FF)
	// ============================================
	_ "github.com/luxfi/precompile/mldsa"    // ML-DSA signature verification (FIPS 204)
	_ "github.com/luxfi/precompile/mlkem"    // ML-KEM key encapsulation (FIPS 203)
	_ "github.com/luxfi/precompile/slhdsa"   // SLH-DSA stateless hash signatures (FIPS 205)
	_ "github.com/luxfi/precompile/pqcrypto" // Unified PQ crypto operations

	// ============================================
	// Privacy/Encryption (0x0700-0x07FF)
	// ============================================
	_ "github.com/luxfi/precompile/fhe"   // Fully Homomorphic Encryption
	_ "github.com/luxfi/precompile/ecies" // Elliptic Curve Integrated Encryption
	_ "github.com/luxfi/precompile/ring"  // Ring signatures (anonymity)
	_ "github.com/luxfi/precompile/hpke"  // Hybrid Public Key Encryption

	// ============================================
	// Threshold Signatures (0x0800-0x08FF)
	// ============================================
	_ "github.com/luxfi/precompile/frost"    // FROST threshold Schnorr
	_ "github.com/luxfi/precompile/cggmp21"  // CGGMP21 threshold ECDSA
	_ "github.com/luxfi/precompile/ringtail" // Threshold lattice (post-quantum)

	// ============================================
	// ZK Proofs (0x0900-0x09FF)
	// ============================================
	_ "github.com/luxfi/precompile/kzg4844" // KZG commitments (EIP-4844)

	// ============================================
	// Curves (0x0A00-0x0AFF)
	// ============================================
	_ "github.com/luxfi/precompile/secp256r1" // P-256/secp256r1 verification

	// ============================================
	// AI Mining (0x0300-0x03FF)
	// ============================================
	_ "github.com/luxfi/precompile/ai" // AI mining rewards, TEE verification

	// ============================================
	// DEX (0x0400-0x04FF)
	// ============================================
	_ "github.com/luxfi/precompile/dex" // Uniswap v4-style DEX

	// ============================================
	// Graph/Query Layer (0x0500-0x05FF)
	// ============================================
	_ "github.com/luxfi/precompile/graph" // GraphQL query interface
)

func init() {
	// Bridge external precompile modules to coreth's registry.
	bridgeExternalModules()
}

// bridgeExternalModules copies modules from the external precompile package's
// registry to coreth's registry, adapting types as needed.
func bridgeExternalModules() {
	for _, extModule := range precompilemods.RegisteredModules() {
		adaptedModule := corethmods.Module{
			ConfigKey: extModule.ConfigKey,
			Address:   extModule.Address,
			Contract:  &contractAdapter{external: extModule.Contract},
			Configurator: &configuratorAdapter{
				external: extModule.Configurator,
			},
		}
		// Ignore errors for already registered modules (e.g., warp)
		_ = corethmods.RegisterModule(adaptedModule)
	}
}

// contractAdapter wraps an external precompile contract to implement coreth's interface
type contractAdapter struct {
	external precompilecontract.StatefulPrecompiledContract
}

func (c *contractAdapter) Run(
	accessibleState corethcontract.AccessibleState,
	caller common.Address,
	addr common.Address,
	input []byte,
	suppliedGas uint64,
	readOnly bool,
) (ret []byte, remainingGas uint64, err error) {
	adapted := &accessibleStateAdapter{corethState: accessibleState}
	return c.external.Run(adapted, caller, addr, input, suppliedGas, readOnly)
}

// accessibleStateAdapter adapts coreth's AccessibleState to precompile's interface
type accessibleStateAdapter struct {
	corethState corethcontract.AccessibleState
}

func (a *accessibleStateAdapter) GetStateDB() precompilecontract.StateDB {
	return &stateDBAdapter{corethDB: a.corethState.GetStateDB()}
}

func (a *accessibleStateAdapter) GetBlockContext() precompilecontract.BlockContext {
	return &blockContextAdapter{corethCtx: a.corethState.GetBlockContext()}
}

func (a *accessibleStateAdapter) GetConsensusContext() context.Context {
	ctx := a.corethState.GetConsensusContext()
	if ctx == nil {
		return context.Background()
	}
	return context.Background()
}

func (a *accessibleStateAdapter) GetChainConfig() precompileconfig.ChainConfig {
	return &chainConfigAdapter{corethConfig: a.corethState.GetChainConfig()}
}

func (a *accessibleStateAdapter) GetPrecompileEnv() precompilecontract.PrecompileEnvironment {
	return &precompileEnvAdapter{corethEnv: a.corethState.GetPrecompileEnv()}
}

// stateDBAdapter adapts coreth's StateDB to precompile's interface
type stateDBAdapter struct {
	corethDB corethcontract.StateDB
}

func (s *stateDBAdapter) GetState(addr common.Address, key common.Hash) common.Hash {
	return s.corethDB.GetState(addr, key)
}

func (s *stateDBAdapter) SetState(addr common.Address, key common.Hash, value common.Hash) common.Hash {
	return s.corethDB.SetState(addr, key, value)
}

func (s *stateDBAdapter) SetNonce(addr common.Address, nonce uint64, reason tracing.NonceChangeReason) {
	s.corethDB.SetNonce(addr, nonce, reason)
}

func (s *stateDBAdapter) GetNonce(addr common.Address) uint64 {
	return s.corethDB.GetNonce(addr)
}

func (s *stateDBAdapter) GetBalance(addr common.Address) *uint256.Int {
	return s.corethDB.GetBalance(addr)
}

func (s *stateDBAdapter) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	return s.corethDB.AddBalance(addr, amount, reason)
}

func (s *stateDBAdapter) SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	return s.corethDB.SubBalance(addr, amount, reason)
}

func (s *stateDBAdapter) GetBalanceMultiCoin(addr common.Address, coinID common.Hash) *big.Int {
	return s.corethDB.GetBalanceMultiCoin(addr, coinID)
}

func (s *stateDBAdapter) AddBalanceMultiCoin(addr common.Address, coinID common.Hash, amount *big.Int) {
	s.corethDB.AddBalanceMultiCoin(addr, coinID, amount)
}

func (s *stateDBAdapter) SubBalanceMultiCoin(addr common.Address, coinID common.Hash, amount *big.Int) {
	s.corethDB.SubBalanceMultiCoin(addr, coinID, amount)
}

func (s *stateDBAdapter) CreateAccount(addr common.Address) {
	s.corethDB.CreateAccount(addr)
}

func (s *stateDBAdapter) Exist(addr common.Address) bool {
	return s.corethDB.Exist(addr)
}

func (s *stateDBAdapter) AddLog(log *ethtypes.Log) {
	s.corethDB.AddLog(log)
}

func (s *stateDBAdapter) Logs() []*ethtypes.Log {
	return s.corethDB.Logs()
}

func (s *stateDBAdapter) GetPredicateStorageSlots(addr common.Address, index int) ([]byte, bool) {
	return s.corethDB.GetPredicateStorageSlots(addr, index)
}

func (s *stateDBAdapter) TxHash() common.Hash {
	return s.corethDB.TxHash()
}

func (s *stateDBAdapter) Snapshot() int {
	return s.corethDB.Snapshot()
}

func (s *stateDBAdapter) RevertToSnapshot(id int) {
	s.corethDB.RevertToSnapshot(id)
}

// blockContextAdapter adapts coreth's BlockContext to precompile's interface
type blockContextAdapter struct {
	corethCtx corethcontract.BlockContext
}

func (b *blockContextAdapter) Number() *big.Int {
	return b.corethCtx.Number()
}

func (b *blockContextAdapter) Timestamp() uint64 {
	return b.corethCtx.Timestamp()
}

func (b *blockContextAdapter) GetPredicateResults(txHash common.Hash, precompileAddress common.Address) []byte {
	return b.corethCtx.GetPredicateResults(txHash, precompileAddress)
}

// chainConfigAdapter adapts coreth's ChainConfig to precompile's interface
type chainConfigAdapter struct {
	corethConfig corethconfig.ChainConfig
}

func (c *chainConfigAdapter) IsDurango(timestamp uint64) bool {
	return c.corethConfig.IsDurango(timestamp)
}

// precompileEnvAdapter adapts coreth's PrecompileEnvironment to precompile's interface
type precompileEnvAdapter struct {
	corethEnv interface{}
}

func (p *precompileEnvAdapter) ReadOnly() bool {
	if p.corethEnv == nil {
		return false
	}
	if env, ok := p.corethEnv.(interface{ ReadOnly() bool }); ok {
		return env.ReadOnly()
	}
	return false
}

// configuratorAdapter adapts external Configurator to coreth's interface
type configuratorAdapter struct {
	external precompilecontract.Configurator
}

func (c *configuratorAdapter) MakeConfig() corethconfig.Config {
	extConfig := c.external.MakeConfig()
	return &configAdapter{external: extConfig}
}

func (c *configuratorAdapter) Configure(
	chainConfig corethconfig.ChainConfig,
	cfg corethconfig.Config,
	state corethcontract.StateDB,
	blockContext corethcontract.ConfigurationBlockContext,
) error {
	adapter, ok := cfg.(*configAdapter)
	if !ok {
		return nil
	}
	adaptedChainConfig := &chainConfigAdapter{corethConfig: chainConfig}
	adaptedState := &stateDBAdapter{corethDB: state}
	adaptedBlockContext := &configBlockContextAdapter{corethCtx: blockContext}
	return c.external.Configure(adaptedChainConfig, adapter.external, adaptedState, adaptedBlockContext)
}

// configBlockContextAdapter adapts coreth's ConfigurationBlockContext
type configBlockContextAdapter struct {
	corethCtx corethcontract.ConfigurationBlockContext
}

func (c *configBlockContextAdapter) Number() *big.Int {
	return c.corethCtx.Number()
}

func (c *configBlockContextAdapter) Timestamp() uint64 {
	return c.corethCtx.Timestamp()
}

// configAdapter adapts external Config to coreth's interface
type configAdapter struct {
	external precompileconfig.Config
}

func (c *configAdapter) Key() string {
	return c.external.Key()
}

func (c *configAdapter) Timestamp() *uint64 {
	return c.external.Timestamp()
}

func (c *configAdapter) IsDisabled() bool {
	return c.external.IsDisabled()
}

func (c *configAdapter) Equal(other corethconfig.Config) bool {
	otherAdapter, ok := other.(*configAdapter)
	if !ok {
		return false
	}
	return c.external.Equal(otherAdapter.external)
}

func (c *configAdapter) Verify(chainConfig corethconfig.ChainConfig) error {
	return c.external.Verify(&chainConfigAdapter{corethConfig: chainConfig})
}

func (c *configAdapter) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, c.external)
}

func (c *configAdapter) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.external)
}
