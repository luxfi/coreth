// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"math/big"

	"github.com/holiman/uint256"
	"github.com/luxfi/coreth/core/extstate"
	"github.com/luxfi/coreth/plugin/evm/atomic"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/tracing"
)

// stateDBWrapper adapts extstate.StateDB to the atomic.StateDB interface
type stateDBWrapper struct {
	*extstate.StateDB
}

// AddBalance implements the atomic.StateDB interface
func (w *stateDBWrapper) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	return w.StateDB.AddBalance(addr, amount, reason)
}

// SubBalance implements the atomic.StateDB interface
func (w *stateDBWrapper) SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	return w.StateDB.SubBalance(addr, amount, reason)
}

// GetBalance implements the atomic.StateDB interface
func (w *stateDBWrapper) GetBalance(addr common.Address) *uint256.Int {
	return w.StateDB.GetBalance(addr)
}

// GetBalanceMultiCoin implements the atomic.StateDB interface
func (w *stateDBWrapper) GetBalanceMultiCoin(addr common.Address, coinID common.Hash) *big.Int {
	return w.StateDB.GetBalanceMultiCoin(addr, coinID)
}

// GetNonce implements the atomic.StateDB interface
func (w *stateDBWrapper) GetNonce(addr common.Address) uint64 {
	return w.StateDB.GetNonce(addr)
}

// SetNonce implements the atomic.StateDB interface
func (w *stateDBWrapper) SetNonce(addr common.Address, nonce uint64, reason tracing.NonceChangeReason) {
	w.StateDB.SetNonce(addr, nonce, reason)
}

// newStateDBWrapper creates a new stateDBWrapper
func newStateDBWrapper(extStateDB *extstate.StateDB) atomic.StateDB {
	return &stateDBWrapper{extStateDB}
}
