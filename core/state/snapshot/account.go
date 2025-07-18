// (c) 2019-2025, Lux Industries Inc.
//
// This file is a derived work, based on the go-ethereum library whose original
// notices appear below.
//
// It is distributed under a license compatible with the licensing terms of the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********
// Copyright 2019 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package snapshot

import (
	"bytes"
	"math/big"

	"github.com/luxfi/geth/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// Account is a modified version of a state.Account, where the root is replaced
// with a byte slice. This format can be used to represent full-consensus format
// or slim-snapshot format which replaces the empty root and code hash as nil
// byte slice.
type Account struct {
	Nonce       uint64
	Balance     *big.Int
	Root        []byte
	CodeHash    []byte
	IsMultiCoin bool
}

// SlimAccount converts a state.Account content into a slim snapshot account
func SlimAccount(nonce uint64, balance *big.Int, root common.Hash, codehash []byte, isMultiCoin bool) Account {
	slim := Account{
		Nonce:       nonce,
		Balance:     balance,
		IsMultiCoin: isMultiCoin,
	}
	if root != types.EmptyRootHash {
		slim.Root = root[:]
	}
	if !bytes.Equal(codehash, types.EmptyCodeHash[:]) {
		slim.CodeHash = codehash
	}
	return slim
}

// SlimAccountRLP converts a state.Account content into a slim snapshot
// version RLP encoded.
func SlimAccountRLP(nonce uint64, balance *big.Int, root common.Hash, codehash []byte, isMultiCoin bool) []byte {
	data, err := rlp.EncodeToBytes(SlimAccount(nonce, balance, root, codehash, isMultiCoin))
	if err != nil {
		panic(err)
	}
	return data
}

// FullAccount decodes the data on the 'slim RLP' format and return
// the consensus format account.
func FullAccount(data []byte) (Account, error) {
	var account Account
	if err := rlp.DecodeBytes(data, &account); err != nil {
		return Account{}, err
	}
	if len(account.Root) == 0 {
		account.Root = types.EmptyRootHash[:]
	}
	if len(account.CodeHash) == 0 {
		account.CodeHash = types.EmptyCodeHash[:]
	}
	return account, nil
}

// FullAccountRLP converts data on the 'slim RLP' format into the full RLP-format.
func FullAccountRLP(data []byte) ([]byte, error) {
	account, err := FullAccount(data)
	if err != nil {
		return nil, err
	}
	return rlp.EncodeToBytes(account)
}
