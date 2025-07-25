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
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/consensus/beacon"
	"github.com/luxfi/geth/consensus/ethash"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/crypto"
	"github.com/luxfi/geth/params"
	"github.com/luxfi/geth/triedb"
)

func TestGeneratePOSChain(t *testing.T) {
	var (
		keyHex  = "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"
		key, _  = crypto.HexToECDSA(keyHex)
		address = crypto.PubkeyToAddress(key.PublicKey) // 658bdf435d810c91414ec09147daa6db62406379
		aa      = common.Address{0xaa}
		bb      = common.Address{0xbb}
		funds   = big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(params.Ether))
		config  = *params.MergedTestChainConfig
		gspec   = &Genesis{
			Config: &config,
			Alloc: types.GenesisAlloc{
				address:                   {Balance: funds},
				params.BeaconRootsAddress: {Code: params.BeaconRootsCode},
			},
			BaseFee:    big.NewInt(params.InitialBaseFee),
			Difficulty: common.Big1,
			GasLimit:   5_000_000,
		}
		gendb = rawdb.NewMemoryDatabase()
		db    = rawdb.NewMemoryDatabase()
	)

	// init 0xaa with some storage elements
	storage := make(map[common.Hash]common.Hash)
	storage[common.Hash{0x00}] = common.Hash{0x00}
	storage[common.Hash{0x01}] = common.Hash{0x01}
	storage[common.Hash{0x02}] = common.Hash{0x02}
	storage[common.Hash{0x03}] = common.HexToHash("0303")
	gspec.Alloc[aa] = types.Account{
		Balance: common.Big1,
		Nonce:   1,
		Storage: storage,
		Code:    common.Hex2Bytes("6042"),
	}
	gspec.Alloc[bb] = types.Account{
		Balance: common.Big2,
		Nonce:   1,
		Storage: storage,
		Code:    common.Hex2Bytes("600154600354"),
	}
	genesis := gspec.MustCommit(gendb, triedb.NewDatabase(gendb, triedb.HashDefaults))
	engine := beacon.New(ethash.NewFaker())

	genchain, genreceipts := GenerateChain(gspec.Config, genesis, engine, gendb, 4, func(i int, gen *BlockGen) {
		gen.SetParentBeaconRoot(common.Hash{byte(i + 1)})

		// Add value transfer tx.
		tx := types.MustSignNewTx(key, gen.Signer(), &types.LegacyTx{
			Nonce:    gen.TxNonce(address),
			To:       &address,
			Value:    big.NewInt(1000),
			Gas:      params.TxGas,
			GasPrice: new(big.Int).Add(gen.BaseFee(), common.Big1),
		})
		gen.AddTx(tx)

		// Add withdrawals.
		if i == 1 {
			gen.AddWithdrawal(&types.Withdrawal{
				Validator: 42,
				Address:   common.Address{0xee},
				Amount:    1337,
			})
			gen.AddWithdrawal(&types.Withdrawal{
				Validator: 13,
				Address:   common.Address{0xee},
				Amount:    1,
			})
		}
		if i == 3 {
			gen.AddWithdrawal(&types.Withdrawal{
				Validator: 42,
				Address:   common.Address{0xee},
				Amount:    1337,
			})
			gen.AddWithdrawal(&types.Withdrawal{
				Validator: 13,
				Address:   common.Address{0xee},
				Amount:    1,
			})
		}
	})

	// Import the chain. This runs all block validation rules.
	blockchain, _ := NewBlockChain(db, gspec, engine, nil)
	defer blockchain.Stop()

	if i, err := blockchain.InsertChain(genchain); err != nil {
		t.Fatalf("insert error (block %d): %v\n", genchain[i].NumberU64(), err)
	}

	// enforce that withdrawal indexes are monotonically increasing from 0
	var (
		withdrawalIndex uint64
	)
	for i := range genchain {
		blocknum := genchain[i].NumberU64()
		block := blockchain.GetBlockByNumber(blocknum)
		if block == nil {
			t.Fatalf("block %d not found", blocknum)
		}

		// Verify receipts.
		genBlockReceipts := genreceipts[i]
		for _, r := range genBlockReceipts {
			if r.BlockNumber.Cmp(block.Number()) != 0 {
				t.Errorf("receipt has wrong block number %d, want %d", r.BlockNumber, block.Number())
			}
			if r.BlockHash != block.Hash() {
				t.Errorf("receipt has wrong block hash %v, want %v", r.BlockHash, block.Hash())
			}

			// patch up empty logs list to make DeepEqual below work
			if r.Logs == nil {
				r.Logs = []*types.Log{}
			}
		}
		blockchainReceipts := blockchain.GetReceiptsByHash(block.Hash())
		if !reflect.DeepEqual(genBlockReceipts, blockchainReceipts) {
			t.Fatalf("receipts mismatch\ngenerated: %s\nblockchain: %s", spew.Sdump(genBlockReceipts), spew.Sdump(blockchainReceipts))
		}

		// Verify withdrawals.
		if len(block.Withdrawals()) == 0 {
			continue
		}
		for j := 0; j < len(block.Withdrawals()); j++ {
			if block.Withdrawals()[j].Index != withdrawalIndex {
				t.Fatalf("withdrawal index %d does not equal expected index %d", block.Withdrawals()[j].Index, withdrawalIndex)
			}
			withdrawalIndex += 1
		}

		// Verify parent beacon root.
		want := common.Hash{byte(blocknum)}
		if got := block.BeaconRoot(); *got != want {
			t.Fatalf("block %d, wrong parent beacon root: got %s, want %s", i, got, want)
		}
		state, _ := blockchain.State()
		idx := block.Time()%8191 + 8191
		got := state.GetState(params.BeaconRootsAddress, common.BigToHash(new(big.Int).SetUint64(idx)))
		if got != want {
			t.Fatalf("block %d, wrong parent beacon root in state: got %s, want %s", i, got, want)
		}
	}
}

func ExampleGenerateChain() {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		key3, _ = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		addr3   = crypto.PubkeyToAddress(key3.PublicKey)
		db      = rawdb.NewMemoryDatabase()
		genDb   = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc:  types.GenesisAlloc{addr1: {Balance: big.NewInt(1000000)}},
	}
	genesis := gspec.MustCommit(genDb, triedb.NewDatabase(genDb, triedb.HashDefaults))

	// This call generates a chain of 5 blocks. The function runs for
	// each block and adds different features to gen based on the
	// block index.
	signer := types.HomesteadSigner{}
	chain, _ := GenerateChain(gspec.Config, genesis, ethash.NewFaker(), genDb, 5, func(i int, gen *BlockGen) {
		switch i {
		case 0:
			// In block 1, addr1 sends addr2 some ether.
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(10000), params.TxGas, nil, nil), signer, key1)
			gen.AddTx(tx)
		case 1:
			// In block 2, addr1 sends some more ether to addr2.
			// addr2 passes it on to addr3.
			tx1, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(1000), params.TxGas, nil, nil), signer, key1)
			tx2, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr2), addr3, big.NewInt(1000), params.TxGas, nil, nil), signer, key2)
			gen.AddTx(tx1)
			gen.AddTx(tx2)
		case 2:
			// Block 3 is empty but was mined by addr3.
			gen.SetCoinbase(addr3)
			gen.SetExtra([]byte("yeehaw"))
		case 3:
			// Block 4 includes blocks 2 and 3 as uncle headers (with modified extra data).
			b2 := gen.PrevBlock(1).Header()
			b2.Extra = []byte("foo")
			gen.AddUncle(b2)
			b3 := gen.PrevBlock(2).Header()
			b3.Extra = []byte("foo")
			gen.AddUncle(b3)
		}
	})

	// Import the chain. This runs all block validation rules.
	blockchain, _ := NewBlockChain(db, gspec, ethash.NewFaker(), DefaultConfig().WithStateScheme(rawdb.HashScheme))
	defer blockchain.Stop()

	if i, err := blockchain.InsertChain(chain); err != nil {
		fmt.Printf("insert error (block %d): %v\n", chain[i].NumberU64(), err)
		return
	}

	state, _ := blockchain.State()
	fmt.Printf("last block: #%d\n", blockchain.CurrentBlock().Number)
	fmt.Println("balance of addr1:", state.GetBalance(addr1))
	fmt.Println("balance of addr2:", state.GetBalance(addr2))
	fmt.Println("balance of addr3:", state.GetBalance(addr3))
	// Output:
	// last block: #5
	// balance of addr1: 989000
	// balance of addr2: 10000
	// balance of addr3: 19687500000000001000
}
