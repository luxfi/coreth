// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
//
// This file is a derived work, based on the go-ethereum library whose original
// notices appear below.
//
// It is distributed under a license compatible with the licensing terms of the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********
// Copyright 2015 The go-ethereum Authors
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

package core

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/luxfi/coreth/consensus/dummy"
	"github.com/luxfi/coreth/params"
	"github.com/luxfi/crypto"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/core/vm"
	"github.com/luxfi/geth/triedb"
)

func ExampleGenerateChain() {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		key3, _ = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
		addr1   = common.PubkeyToAddress(key1.PublicKey)
		addr2   = common.PubkeyToAddress(key2.PublicKey)
		addr3   = common.PubkeyToAddress(key3.PublicKey)
		db      = rawdb.NewMemoryDatabase()
		genDb   = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block. With all upgrades
	// active the dynamic fee is always on, so accounts must be funded at Ether
	// scale and transactions must pay base fee + tip.
	funds := new(big.Int).Mul(big.NewInt(1000000), big.NewInt(params.Ether))
	gspec := &Genesis{
		Config: params.TestChainConfig,
		Alloc: types.GenesisAlloc{
			addr1: {Balance: funds},
			addr2: {Balance: funds},
		},
	}
	genesis := gspec.MustCommit(genDb, triedb.NewDatabase(genDb, triedb.HashDefaults))

	// This call generates a chain of 3 blocks. The function runs for
	// each block and adds different features to gen based on the
	// block index.
	signer := types.LatestSigner(params.TestChainConfig)
	tip := big.NewInt(50000 * params.GWei)
	newTx := func(gen *BlockGen, key *ecdsa.PrivateKey, from, to common.Address, amount *big.Int) *types.Transaction {
		tx, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{
			ChainID:   params.TestChainConfig.ChainID,
			Nonce:     gen.TxNonce(from),
			To:        &to,
			Gas:       params.TxGas,
			Value:     amount,
			GasFeeCap: new(big.Int).Add(gen.BaseFee(), tip),
			GasTipCap: tip,
			Data:      []byte{},
		}), signer, key)
		if err != nil {
			panic(err)
		}
		return tx
	}
	chain, _, err := GenerateChain(gspec.Config, genesis, dummy.NewCoinbaseFaker(), genDb, 3, 10, func(i int, gen *BlockGen) {
		switch i {
		case 0:
			// In block 1, addr1 sends addr2 some ether.
			gen.AddTx(newTx(gen, key1, addr1, addr2, big.NewInt(10000)))
		case 1:
			// In block 2, addr1 sends some more ether to addr2.
			// addr2 passes it on to addr3.
			gen.AddTx(newTx(gen, key1, addr1, addr2, big.NewInt(1000)))
		case 2:
			gen.AddTx(newTx(gen, key2, addr2, addr3, big.NewInt(1000)))
		}
	})
	if err != nil {
		panic(err)
	}

	// Import the chain. This runs all block validation rules.
	blockchain, _ := NewBlockChain(db, DefaultCacheConfigWithScheme(rawdb.HashScheme), gspec, dummy.NewCoinbaseFaker(), vm.Config{}, common.Hash{}, false)
	defer blockchain.Stop()

	if i, err := blockchain.InsertChain(chain); err != nil {
		fmt.Printf("insert error (block %d): %v\n", chain[i].NumberU64(), err)
		return
	}

	state, _ := blockchain.State()
	fmt.Printf("last block: #%d\n", blockchain.CurrentBlock().Number)
	// Print the net change from the genesis balance for the senders, and the
	// absolute balance for addr3 (which only ever receives). With all upgrades
	// active each transaction pays base fee + tip, so the senders' net change is
	// the transferred value plus the fees they paid.
	addr1Spent := new(big.Int).Sub(funds, state.GetBalance(addr1).ToBig())
	addr2Spent := new(big.Int).Sub(funds, state.GetBalance(addr2).ToBig())
	fmt.Println("addr1 net spent:", addr1Spent)
	fmt.Println("addr2 net spent:", addr2Spent)
	fmt.Println("balance of addr3:", state.GetBalance(addr3))
	// Expected output has been modified since uncle blocks and block rewards have
	// been removed from the original test, and because all upgrades are active so
	// transactions pay base fee + tip.
	//
	// addr1 sends 10000 + 1000 (= 11000) and pays the fee for two txs.
	// addr2 receives 11000, sends 1000, and pays the fee for one tx, so its net
	// spent is the one fee + 1000 - 11000 received = fee - 10000.
	// addr3 receives 1000 and sends nothing.

	// Output:
	// last block: #3
	// addr1 net spent: 2100000000000053000
	// addr2 net spent: 1050000000000011000
	// balance of addr3: 1000
}
