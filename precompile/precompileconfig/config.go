// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Defines the stateless interface for unmarshalling an arbitrary config of a precompile
package precompileconfig

import (
	"math/big"

	"github.com/luxfi/geth/common"
)

// StatefulPrecompileConfig defines the interface for a stateful precompile to
// be enabled via a network upgrade.
type Config interface {
	// Key returns the unique key for the stateful precompile.
	Key() string
	// Timestamp returns the timestamp at which this stateful precompile should be enabled.
	// 1) 0 indicates that the precompile should be enabled from genesis.
	// 2) n indicates that the precompile should be enabled in the first block with timestamp >= [n].
	// 3) nil indicates that the precompile is never enabled.
	Timestamp() *uint64
	// IsDisabled returns true if this network upgrade should disable the precompile.
	IsDisabled() bool
	// Equal returns true if the provided argument configures the same precompile with the same parameters.
	Equal(Config) bool
	// Verify is called on startup and an error is treated as fatal. Configure can assume the Config has passed verification.
	Verify(ChainConfig) error
}

// ChainContext defines an interface that provides information to a stateful precompile
// about the chain configuration. The precompile can access this information to initialize
// its state.
type ChainConfig interface {
	// GetFeeConfig returns the original FeeConfig that was set in the genesis.
	GetFeeConfig() FeeConfig
	// AllowedFeeRecipients returns true if fee recipients are allowed in the genesis.
	AllowedFeeRecipients() bool
	// IsDurango returns true if the time is after Durango.
	IsDurango(time uint64) bool
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

// Accepter is an optional interface for StatefulPrecompiledContracts to implement.
// If implemented, Accept will be called for every log with the address of the precompile when the block is accepted.
type Accepter interface {
	Accept(blockHash common.Hash, blockNumber uint64, txHash common.Hash, logIndex int, topics []common.Hash, logData []byte) error
}