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
// Copyright 2014 The go-ethereum Authors
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

package customtypes_test

import (
	"bytes"
	"math/big"
	"reflect"
	"testing"

	"github.com/luxfi/coreth/internal/blocktest"
	"github.com/luxfi/coreth/params"
	"github.com/luxfi/crypto"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
	"github.com/luxfi/math"

	// This test file has to be in package types_test to avoid a circular
	// dependency when importing `params`. We dot-import the package to mimic
	// regular same-package behaviour.
	. "github.com/luxfi/coreth/plugin/evm/customtypes"
)

// TestBlockEncoding tests that blocks can be RLP encoded and decoded
// using the standard geth block format.
func TestBlockEncoding(t *testing.T) {
	// Create a block with known values using geth's NewBlock
	header := &types.Header{
		ParentHash:  common.HexToHash("4504ee98a94d16dbd70a35370501a3cb00c2965b012672085fbd328a72962902"),
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    common.HexToAddress("0100000000000000000000000000000000000000"),
		Root:        common.HexToHash("0202e12a30c13562445052414c24dce5f1c530bb164e2a50897f0a6a1f78f158"),
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		Difficulty:  big.NewInt(1),
		Number:      big.NewInt(3),
		GasLimit:    8000000,
		GasUsed:     0,
		Time:        1617383050,
		Extra:       []byte{},
	}

	body := &types.Body{
		Transactions: []*types.Transaction{},
		Uncles:       []*types.Header{},
	}
	block := types.NewBlock(header, body, nil, blocktest.NewHasher())

	// Encode the block
	blockEnc, err := rlp.EncodeToBytes(block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}

	// Decode the block
	var decoded types.Block
	if err := rlp.DecodeBytes(blockEnc, &decoded); err != nil {
		t.Fatal("decode error: ", err)
	}

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}
	check("ParentHash", decoded.ParentHash(), header.ParentHash)
	check("Coinbase", decoded.Coinbase(), header.Coinbase)
	check("Root", decoded.Root(), header.Root)
	check("Difficulty", decoded.Difficulty(), header.Difficulty)
	check("BlockNumber", decoded.NumberU64(), header.Number.Uint64())
	check("GasLimit", decoded.GasLimit(), header.GasLimit)
	check("Time", decoded.Time(), header.Time)

	// Re-encode and verify it matches
	reEncoded, err := rlp.EncodeToBytes(&decoded)
	if err != nil {
		t.Fatal("re-encode error: ", err)
	}
	if !bytes.Equal(reEncoded, blockEnc) {
		t.Errorf("encoded block mismatch:\ngot:  %x\nwant: %x", reEncoded, blockEnc)
	}
}

// TestEIP1559BlockEncoding tests EIP-1559 block with BaseFee using geth format
func TestEIP1559BlockEncoding(t *testing.T) {
	// Create a block with EIP-1559 BaseFee
	header := &types.Header{
		ParentHash:  common.HexToHash("4504ee98a94d16dbd70a35370501a3cb00c2965b012672085fbd328a72962902"),
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"),
		Root:        common.HexToHash("ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017"),
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		Difficulty:  big.NewInt(131072),
		Number:      big.NewInt(3),
		GasLimit:    3141592,
		GasUsed:     21000,
		Time:        1426516743,
		Extra:       []byte{},
		MixDigest:   common.HexToHash("bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff498"),
		Nonce:       types.EncodeNonce(0xa13a5a8c8f2bb1c4),
		BaseFee:     big.NewInt(1_000_000_000),
	}

	body := &types.Body{
		Transactions: []*types.Transaction{},
		Uncles:       []*types.Header{},
	}
	block := types.NewBlock(header, body, nil, blocktest.NewHasher())

	// Encode the block
	blockEnc, err := rlp.EncodeToBytes(block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}

	// Decode the block
	var decoded types.Block
	if err := rlp.DecodeBytes(blockEnc, &decoded); err != nil {
		t.Fatal("decode error: ", err)
	}

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}

	check("Difficulty", decoded.Difficulty(), header.Difficulty)
	check("GasLimit", decoded.GasLimit(), header.GasLimit)
	check("GasUsed", decoded.GasUsed(), header.GasUsed)
	check("Coinbase", decoded.Coinbase(), header.Coinbase)
	check("MixDigest", decoded.MixDigest(), header.MixDigest)
	check("Root", decoded.Root(), header.Root)
	check("BaseFee", decoded.BaseFee(), header.BaseFee)

	// Re-encode and verify it matches
	reEncoded, err := rlp.EncodeToBytes(&decoded)
	if err != nil {
		t.Fatal("re-encode error: ", err)
	}
	if !bytes.Equal(reEncoded, blockEnc) {
		t.Errorf("encoded block mismatch:\ngot:  %x\nwant: %x", reEncoded, blockEnc)
	}
}

// TestEIP2718BlockEncoding tests EIP-2718 typed transactions (legacy + AccessListTx)
func TestEIP2718BlockEncoding(t *testing.T) {
	// Create legacy tx.
	to := common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	tx1 := types.NewTx(&types.LegacyTx{
		Nonce:    0,
		To:       &to,
		Value:    big.NewInt(10),
		Gas:      50000,
		GasPrice: big.NewInt(10),
	})
	sig := common.Hex2Bytes("9bea4c4daac7c7c52e093e6a4c35dbbcf8856f1af7b059ba20253e70848d094f8a8fae537ce25ed8cb5af9adac3f141af69bd515bd2ba031522df09b97dd72b100")
	tx1, _ = tx1.WithSignature(types.HomesteadSigner{}, sig)

	// Create ACL tx.
	addr := common.HexToAddress("0x0000000000000000000000000000000000000001")
	tx2 := types.NewTx(&types.AccessListTx{
		ChainID:    big.NewInt(1),
		Nonce:      0,
		To:         &to,
		Gas:        123457,
		GasPrice:   big.NewInt(10),
		AccessList: types.AccessList{{Address: addr, StorageKeys: []common.Hash{{0}}}},
	})
	sig2 := common.Hex2Bytes("3dbacc8d0259f2508625e97fdfc57cd85fdd16e5821bc2c10bdd1a52649e8335476e10695b183a87b0aa292a7f4b78ef0c3fbe62aa2c42c84e1d9c3da159ef1401")
	tx2, _ = tx2.WithSignature(types.NewEIP2930Signer(big.NewInt(1)), sig2)

	// Create header
	header := &types.Header{
		ParentHash: common.Hash{},
		UncleHash:  types.EmptyUncleHash,
		Coinbase:   common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"),
		Root:       common.HexToHash("ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017"),
		Difficulty: big.NewInt(131072),
		Number:     big.NewInt(512),
		GasLimit:   3141592,
		GasUsed:    42000,
		Time:       1426516743,
		Extra:      []byte("coolest block on chain"),
		MixDigest:  common.HexToHash("bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff498"),
		Nonce:      types.EncodeNonce(0xa13a5a8c8f2bb1c4),
	}

	// Create block with transactions
	body := &types.Body{
		Transactions: []*types.Transaction{tx1, tx2},
		Uncles:       []*types.Header{},
	}
	block := types.NewBlock(header, body, nil, blocktest.NewHasher())

	// Attach header extra (empty ExtDataHash) - must be done AFTER NewBlock
	// because NewBlock creates an internal copy of the header
	SetHeaderExtra(block.Header(), &HeaderExtra{
		ExtDataHash: EmptyExtDataHash,
	})

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}

	check("Difficulty", block.Difficulty(), big.NewInt(131072))
	check("GasLimit", block.GasLimit(), uint64(3141592))
	check("GasUsed", block.GasUsed(), uint64(42000))
	check("Coinbase", block.Coinbase(), common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"))
	check("MixDigest", block.MixDigest(), common.HexToHash("bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff498"))
	check("Root", block.Root(), common.HexToHash("ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017"))
	check("Nonce", block.Nonce(), uint64(0xa13a5a8c8f2bb1c4))
	check("Time", block.Time(), uint64(1426516743))
	check("ExtDataHash", GetHeaderExtra(block.Header()).ExtDataHash, EmptyExtDataHash)
	check("BaseFee", block.BaseFee(), (*big.Int)(nil))
	check("ExtDataGasUsed", BlockExtDataGasUsed(block), (*big.Int)(nil))
	check("BlockGasCost", BlockGasCost(block), (*big.Int)(nil))

	check("len(Transactions)", len(block.Transactions()), 2)
	check("Transactions[0].Hash", block.Transactions()[0].Hash(), tx1.Hash())
	check("Transactions[1].Hash", block.Transactions()[1].Hash(), tx2.Hash())
	check("Transactions[1].Type()", block.Transactions()[1].Type(), uint8(types.AccessListTxType))

	// Test encode/decode roundtrip
	blockEnc, err := rlp.EncodeToBytes(block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}

	var decoded types.Block
	if err := rlp.DecodeBytes(blockEnc, &decoded); err != nil {
		t.Fatal("decode error: ", err)
	}

	check("Decoded.Difficulty", decoded.Difficulty(), block.Difficulty())
	check("Decoded.GasLimit", decoded.GasLimit(), block.GasLimit())
	check("Decoded.GasUsed", decoded.GasUsed(), block.GasUsed())
	check("Decoded.Coinbase", decoded.Coinbase(), block.Coinbase())
	check("Decoded.len(Transactions)", len(decoded.Transactions()), len(block.Transactions()))

	// Re-encode and verify it matches
	reEncoded, err := rlp.EncodeToBytes(&decoded)
	if err != nil {
		t.Fatal("re-encode error: ", err)
	}
	if !bytes.Equal(reEncoded, blockEnc) {
		t.Errorf("encoded block mismatch:\ngot:  %x\nwant: %x", reEncoded, blockEnc)
	}
}

// TestBlockEncodingWithExtraData tests blocks with ExtData (atomic transaction data)
// attached via the extras storage system.
func TestBlockEncodingWithExtraData(t *testing.T) {
	// Sample atomic transaction data
	extData := common.FromHex("00000000000000003039c85fc1980a77c5da78fe5486233fc09a769bb812bcb2cc548cf9495d046b3f1bd891ad56056d9c01f18f43f58b5c784ad07a4a49cf3d1f11623804b5cba2c6bf000000028a0f7c3e4d840143671a4c4ecacccb4d60fb97dce97a7aa5d60dfd072a7509cf00000001dbcf890f77f49b96857648b72b77f9f82937f28a68704af05da0dc12ba53f2db0000000500002d79883d20000000000100000000e0d5c4edc78f594b79025a56c44933c28e8ba3e51e6e23318727eeaac10eb27d00000001dbcf890f77f49b96857648b72b77f9f82937f28a68704af05da0dc12ba53f2db0000000500002d79883d20000000000100000000000000016dc8ea73dd39ab12fa2ecbc3427abaeb87d56fd800005af3107a4000dbcf890f77f49b96857648b72b77f9f82937f28a68704af05da0dc12ba53f2db0000000200000009000000010d9f115cd63c3ab78b5b82cfbe4339cd6be87f21cda14cf192b269c7a6cb2d03666aa8f8b23ca0a2ceee4050e75c9b05525a17aa1dd0e9ea391a185ce395943f0000000009000000010d9f115cd63c3ab78b5b82cfbe4339cd6be87f21cda14cf192b269c7a6cb2d03666aa8f8b23ca0a2ceee4050e75c9b05525a17aa1dd0e9ea391a185ce395943f00")

	// Create header with ExtDataHash
	header := &types.Header{
		ParentHash:  common.HexToHash("2a0d1d68d26eb213cf1c6c1e6abbaf374f0ee9a5428558df334c36d380c6a080"),
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    common.HexToAddress("0100000000000000000000000000000000000000"),
		Root:        common.HexToHash("c0caa90fe3722cb2e288f7998d54a855a6d40f67e0e77a695d0d65dad22c6290"),
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		Difficulty:  big.NewInt(1),
		Number:      big.NewInt(2),
		GasLimit:    8000000,
		GasUsed:     0,
		Time:        1617382963,
		Extra:       []byte{},
	}

	// Calculate extDataHash for verification
	extDataHash := CalcExtDataHash(extData)

	// Create block using NewBlockWithExtData which attaches the ExtData
	// Note: recalc=true will compute and set ExtDataHash on the block's internal header
	block := NewBlockWithExtData(header, nil, nil, nil, blocktest.NewHasher(), extData, true)

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}

	check("ParentHash", block.ParentHash(), header.ParentHash)
	check("UncleHash", block.UncleHash(), types.EmptyUncleHash)
	check("Coinbase", block.Coinbase(), header.Coinbase)
	check("Root", block.Root(), header.Root)
	check("TxHash", block.TxHash(), types.EmptyTxsHash)
	check("ReceiptHash", block.ReceiptHash(), types.EmptyReceiptsHash)
	check("Difficulty", block.Difficulty(), big.NewInt(1))
	check("BlockNumber", block.NumberU64(), uint64(2))
	check("GasLimit", block.GasLimit(), uint64(8000000))
	check("GasUsed", block.GasUsed(), uint64(0))
	check("Time", block.Time(), uint64(1617382963))
	check("Extra", block.Extra(), []byte{})
	check("ExtDataHash", GetHeaderExtra(block.Header()).ExtDataHash, extDataHash)
	check("BaseFee", block.BaseFee(), (*big.Int)(nil))
	check("ExtDataGasUsed", BlockExtDataGasUsed(block), (*big.Int)(nil))
	check("BlockGasCost", BlockGasCost(block), (*big.Int)(nil))

	check("len(Transactions)", len(block.Transactions()), 0)

	// Verify ExtData is retrievable
	if !bytes.Equal(BlockExtData(block), extData) {
		t.Errorf("Block ExtData field mismatch:\nexpected: 0x%x\ngot: 0x%x", extData, BlockExtData(block))
	}

	// Test basic RLP encode/decode (ExtData stored separately, not in RLP)
	blockEnc, err := rlp.EncodeToBytes(block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}

	var decoded types.Block
	if err := rlp.DecodeBytes(blockEnc, &decoded); err != nil {
		t.Fatal("decode error: ", err)
	}

	check("Decoded.ParentHash", decoded.ParentHash(), block.ParentHash())
	check("Decoded.Root", decoded.Root(), block.Root())
	check("Decoded.Number", decoded.NumberU64(), block.NumberU64())

	// Re-encode and verify it matches
	reEncoded, err := rlp.EncodeToBytes(&decoded)
	if err != nil {
		t.Fatal("re-encode error: ", err)
	}
	if !bytes.Equal(reEncoded, blockEnc) {
		t.Errorf("encoded block mismatch:\ngot:  %x\nwant: %x", reEncoded, blockEnc)
	}
}

func TestUncleHash(t *testing.T) {
	uncles := make([]*types.Header, 0)
	h := types.CalcUncleHash(uncles)
	exp := common.HexToHash("1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347")
	if h != exp {
		t.Fatalf("empty uncle hash is wrong, got %x != %x", h, exp)
	}
}

var benchBuffer = bytes.NewBuffer(make([]byte, 0, 32000))

func BenchmarkEncodeBlock(b *testing.B) {
	block := makeBenchBlock()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		benchBuffer.Reset()
		if err := rlp.Encode(benchBuffer, block); err != nil {
			b.Fatal(err)
		}
	}
}

func makeBenchBlock() *types.Block {
	var (
		key, _   = crypto.GenerateKey()
		txs      = make([]*types.Transaction, 70)
		receipts = make([]*types.Receipt, len(txs))
		signer   = types.LatestSigner(params.TestChainConfig)
		uncles   = make([]*types.Header, 3)
	)
	header := &types.Header{
		Difficulty: math.BigPow(11, 11),
		Number:     math.BigPow(2, 9),
		GasLimit:   12345678,
		GasUsed:    1476322,
		Time:       9876543,
		Extra:      []byte("coolest block on chain"),
	}
	for i := range txs {
		amount := math.BigPow(2, int64(i))
		price := big.NewInt(300000)
		data := make([]byte, 100)
		tx := types.NewTransaction(uint64(i), common.Address{}, amount, 123457, price, data)
		signedTx, err := types.SignTx(tx, signer, key)
		if err != nil {
			panic(err)
		}
		txs[i] = signedTx
		receipts[i] = types.NewReceipt(make([]byte, 32), false, tx.Gas())
	}
	for i := range uncles {
		uncles[i] = &types.Header{
			Difficulty: math.BigPow(11, 11),
			Number:     math.BigPow(2, 9),
			GasLimit:   12345678,
			GasUsed:    1476322,
			Time:       9876543,
			Extra:      []byte("benchmark uncle"),
		}
	}
	return types.NewBlock(header, &types.Body{Transactions: txs, Uncles: uncles}, receipts, blocktest.NewHasher())
}

// TestAP4BlockEncoding tests Apricot Phase 4 blocks with ExtDataGasUsed and BlockGasCost
func TestAP4BlockEncoding(t *testing.T) {
	// Create transactions
	tx1 := types.NewTransaction(0, common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87"), big.NewInt(10), 50000, big.NewInt(10), nil)
	tx1, _ = tx1.WithSignature(types.HomesteadSigner{}, common.Hex2Bytes("9bea4c4daac7c7c52e093e6a4c35dbbcf8856f1af7b059ba20253e70848d094f8a8fae537ce25ed8cb5af9adac3f141af69bd515bd2ba031522df09b97dd72b100"))

	addr := common.HexToAddress("0x0000000000000000000000000000000000000001")
	accesses := types.AccessList{types.AccessTuple{
		Address: addr,
		StorageKeys: []common.Hash{
			{0},
		},
	}}
	to := common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	txdata := &types.DynamicFeeTx{
		ChainID:    big.NewInt(1),
		Nonce:      0,
		To:         &to,
		Gas:        123457,
		GasFeeCap:  big.NewInt(1_000_000_000),
		GasTipCap:  big.NewInt(0),
		AccessList: accesses,
		Data:       []byte{},
	}
	tx2 := types.NewTx(txdata)
	tx2, _ = tx2.WithSignature(types.LatestSignerForChainID(big.NewInt(1)), common.Hex2Bytes("fe38ca4e44a30002ac54af7cf922a6ac2ba11b7d22f548e8ecb3f51f41cb31b06de6a5cbae13c0c856e33acf021b51819636cfc009d39eafb9f606d546e305a800"))

	// Create header with EIP-1559 fields
	header := &types.Header{
		ParentHash:  common.HexToHash("4504ee98a94d16dbd70a35370501a3cb00c2965b012672085fbd328a72962902"),
		UncleHash:   common.Hash{},
		Coinbase:    common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"),
		Root:        common.HexToHash("ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017"),
		TxHash:      common.Hash{},
		ReceiptHash: common.Hash{},
		Difficulty:  big.NewInt(131072),
		Number:      big.NewInt(3),
		GasLimit:    3141592,
		GasUsed:     21000,
		Time:        1426516743,
		Extra:       []byte{},
		MixDigest:   common.HexToHash("bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff498"),
		Nonce:       types.EncodeNonce(0xa13a5a8c8f2bb1c4),
		BaseFee:     big.NewInt(1_000_000_000),
	}

	// Create block
	body := &types.Body{
		Transactions: []*types.Transaction{tx1, tx2},
		Uncles:       []*types.Header{},
	}
	block := types.NewBlock(header, body, nil, blocktest.NewHasher())

	// Attach header extra with ExtDataGasUsed and BlockGasCost (AP4 fields)
	// Must be done AFTER NewBlock because NewBlock creates an internal copy of the header
	SetHeaderExtra(block.Header(), &HeaderExtra{
		ExtDataHash:    EmptyExtDataHash,
		ExtDataGasUsed: big.NewInt(25_000),
		BlockGasCost:   big.NewInt(1_000_000),
	})

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}

	check("Difficulty", block.Difficulty(), big.NewInt(131072))
	check("GasLimit", block.GasLimit(), uint64(3141592))
	check("GasUsed", block.GasUsed(), uint64(21000))
	check("Coinbase", block.Coinbase(), common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"))
	check("MixDigest", block.MixDigest(), common.HexToHash("bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff498"))
	check("Root", block.Root(), common.HexToHash("ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017"))
	check("Nonce", block.Nonce(), uint64(0xa13a5a8c8f2bb1c4))
	check("Time", block.Time(), uint64(1426516743))
	check("BaseFee", block.BaseFee(), big.NewInt(1_000_000_000))
	check("ExtDataGasUsed", BlockExtDataGasUsed(block), big.NewInt(25_000))
	check("BlockGasCost", BlockGasCost(block), big.NewInt(1_000_000))

	check("len(Transactions)", len(block.Transactions()), 2)
	check("Transactions[0].Hash", block.Transactions()[0].Hash(), tx1.Hash())
	check("Transactions[1].Hash", block.Transactions()[1].Hash(), tx2.Hash())
	check("Transactions[1].Type", block.Transactions()[1].Type(), tx2.Type())

	// Test encode/decode roundtrip
	blockEnc, err := rlp.EncodeToBytes(block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}

	var decoded types.Block
	if err := rlp.DecodeBytes(blockEnc, &decoded); err != nil {
		t.Fatal("decode error: ", err)
	}

	check("Decoded.Difficulty", decoded.Difficulty(), block.Difficulty())
	check("Decoded.GasLimit", decoded.GasLimit(), block.GasLimit())
	check("Decoded.BaseFee", decoded.BaseFee(), block.BaseFee())
	check("Decoded.len(Transactions)", len(decoded.Transactions()), len(block.Transactions()))

	// Re-encode and verify it matches
	reEncoded, err := rlp.EncodeToBytes(&decoded)
	if err != nil {
		t.Fatal("re-encode error: ", err)
	}
	if !bytes.Equal(reEncoded, blockEnc) {
		t.Errorf("encoded block mismatch:\ngot:  %x\nwant: %x", reEncoded, blockEnc)
	}
}
