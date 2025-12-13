// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extstate

import (
	"math/big"

	corethparams "github.com/luxfi/coreth/params"
	"github.com/luxfi/coreth/predicate"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/core/stateconf"
	"github.com/luxfi/geth/core/tracing"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/params"
	"github.com/holiman/uint256"
)


type StateDB struct {
	*state.StateDB

	// Ordered storage slots to be used in predicate verification as set in the tx access list.
	// Only set in [StateDB.Prepare], and un-modified through execution.
	predicateStorageSlots map[common.Address][][]byte
}

// New creates a new [StateDB] with the given [state.StateDB], wrapping it with
// additional functionality.
func New(vm *state.StateDB) *StateDB {
	return &StateDB{
		StateDB:               vm,
		predicateStorageSlots: make(map[common.Address][][]byte),
	}
}

func (s *StateDB) Prepare(rules params.Rules, sender, coinbase common.Address, dst *common.Address, precompiles []common.Address, list types.AccessList) {
	rulesExtra := corethparams.GetRulesExtra(rules)
	s.predicateStorageSlots = predicate.PreparePredicateStorageSlots(rulesExtra, list)
	s.StateDB.Prepare(rules, sender, coinbase, dst, precompiles, list)
}

// GetPredicateStorageSlots returns the storage slots associated with the address, index pair.
// A list of access tuples can be included within transaction types post EIP-2930. The address
// is declared directly on the access tuple and the index is the i'th occurrence of an access
// tuple with the specified address.
//
// Ex. AccessList[[AddrA, Predicate1], [AddrB, Predicate2], [AddrA, Predicate3]]
// In this case, the caller could retrieve predicates 1-3 with the following calls:
// GetPredicateStorageSlots(AddrA, 0) -> Predicate1
// GetPredicateStorageSlots(AddrB, 0) -> Predicate2
// GetPredicateStorageSlots(AddrA, 1) -> Predicate3
func (s *StateDB) GetPredicateStorageSlots(address common.Address, index int) ([]byte, bool) {
	predicates, exists := s.predicateStorageSlots[address]
	if !exists || index >= len(predicates) {
		return nil, false
	}
	return predicates[index], true
}

// Retrieve the balance from the given address or 0 if object not found
func (s *StateDB) GetBalanceMultiCoin(addr common.Address, coinID common.Hash) *big.Int {
	normalizeCoinID(&coinID)
	return s.GetState(addr, coinID).Big()
}

// AddBalanceMultiCoin adds amount to the account associated with addr.
func (s *StateDB) AddBalanceMultiCoin(addr common.Address, coinID common.Hash, amount *big.Int) {
	if amount.Sign() == 0 {
		s.StateDB.AddBalance(addr, new(uint256.Int), tracing.BalanceChangeUnspecified) // used to cause touch
		return
	}

	// Multicoin functionality has been removed

	newAmount := new(big.Int).Add(s.GetBalanceMultiCoin(addr, coinID), amount)
	normalizeCoinID(&coinID)
	s.SetState(addr, coinID, common.BigToHash(newAmount))
}

// SubBalance subtracts amount from the account associated with addr.
func (s *StateDB) SubBalanceMultiCoin(addr common.Address, coinID common.Hash, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	// Note: It's not needed to set the IsMultiCoin (extras) flag here, as this
	// call would always be preceded by a call to AddBalanceMultiCoin, which would
	// set the extra flag. Seems we should remove the redundant code.
	// Multicoin functionality has been removed
	newAmount := new(big.Int).Sub(s.GetBalanceMultiCoin(addr, coinID), amount)
	normalizeCoinID(&coinID)
	s.SetState(addr, coinID, common.BigToHash(newAmount))
}

// normalizeStateKey sets the 0th bit of the first byte in `key` to 0.
// This partitions normal state storage from multicoin storage.
func normalizeStateKey(key *common.Hash) {
	key[0] &^= 0x01
}

// normalizeCoinID sets the 0th bit of the first byte in `coinID` to 1.
// This partitions multicoin storage from normal state storage.
func normalizeCoinID(coinID *common.Hash) {
	coinID[0] |= 0x01
}

// The following methods are forwarded to the embedded state.StateDB and are provided
// to satisfy vm.StateDB interface requirements.

// AddBalance wraps the underlying StateDB's AddBalance.
func (s *StateDB) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	return s.StateDB.AddBalance(addr, amount, reason)
}

// SubBalance wraps the underlying StateDB's SubBalance.
func (s *StateDB) SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	return s.StateDB.SubBalance(addr, amount, reason)
}

// SetNonce wraps the underlying StateDB's SetNonce.
func (s *StateDB) SetNonce(addr common.Address, nonce uint64, reason tracing.NonceChangeReason) {
	s.StateDB.SetNonce(addr, nonce, reason)
}

// SetState wraps the underlying StateDB's SetState.
func (s *StateDB) SetState(addr common.Address, key, value common.Hash) common.Hash {
	return s.StateDB.SetState(addr, key, value)
}

// SetCode wraps the underlying StateDB's SetCode.
func (s *StateDB) SetCode(addr common.Address, code []byte, reason tracing.CodeChangeReason) []byte {
	return s.StateDB.SetCode(addr, code, reason)
}

// The following methods provide simplified signatures for use by precompile contracts.
// These are used by contract.StateDB interface implementations.

// AddBalanceSimple adds balance without returning the result.
func (s *StateDB) AddBalanceSimple(addr common.Address, amount *uint256.Int) {
	s.StateDB.AddBalance(addr, amount, tracing.BalanceChangeUnspecified)
}

// SetNonceSimple sets nonce without specifying a reason.
func (s *StateDB) SetNonceSimple(addr common.Address, nonce uint64) {
	s.StateDB.SetNonce(addr, nonce, tracing.NonceChangeUnspecified)
}

// GetStateWithOptions wraps GetState accepting optional stateconf options
// for compatibility with the contract.StateDB interface.
func (s *StateDB) GetStateWithOptions(addr common.Address, hash common.Hash, _ ...stateconf.StateDBStateOption) common.Hash {
	return s.StateDB.GetState(addr, hash)
}

// SetStateWithOptions wraps SetState accepting optional stateconf options
// for compatibility with the contract.StateDB interface.
func (s *StateDB) SetStateWithOptions(addr common.Address, key, value common.Hash, _ ...stateconf.StateDBStateOption) {
	s.StateDB.SetState(addr, key, value)
}

// TxHash returns the current transaction hash.
// This is required by contract.StateDB interface.
func (s *StateDB) TxHash() common.Hash {
	// Use the transaction hash from the logs if available
	logs := s.StateDB.Logs()
	if len(logs) > 0 {
		return logs[0].TxHash
	}
	return common.Hash{}
}
