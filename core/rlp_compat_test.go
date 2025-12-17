// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package core

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
)

// TestHeaderRLPRoundtrip verifies that header RLP encoding/decoding works correctly
// for different field counts (16, 17, 18, 19 field formats).
func TestHeaderRLPRoundtrip(t *testing.T) {
	tests := []struct {
		name   string
		header *types.Header
	}{
		{
			name: "16-field genesis (pre-ExtDataHash)",
			header: &types.Header{
				ParentHash:  common.Hash{},
				UncleHash:   types.EmptyUncleHash,
				Coinbase:    common.Address{},
				Root:        types.EmptyRootHash,
				TxHash:      types.EmptyTxsHash,
				ReceiptHash: types.EmptyReceiptsHash,
				Bloom:       types.Bloom{},
				Difficulty:  big.NewInt(131072),
				Number:      big.NewInt(0), // Genesis
				GasLimit:    4712388,
				GasUsed:     0,
				Time:        0,
				Extra:       []byte{},
				MixDigest:   common.Hash{},
				Nonce:       types.BlockNonce{},
				BaseFee:     big.NewInt(225000000000),
			},
		},
		{
			name: "19-field Lux block",
			header: &types.Header{
				ParentHash:  common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
				UncleHash:   types.EmptyUncleHash,
				Coinbase:    common.HexToAddress("0x1234567890123456789012345678901234567890"),
				Root:        common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
				TxHash:      types.EmptyTxsHash,
				ReceiptHash: types.EmptyReceiptsHash,
				Bloom:       types.Bloom{},
				Difficulty:  big.NewInt(1),
				Number:      big.NewInt(100),
				GasLimit:    8000000,
				GasUsed:     21000,
				Time:        1699000000,
				Extra:       []byte{0x01, 0x02, 0x03},
				MixDigest:   common.Hash{},
				Nonce:       types.BlockNonce{},
				BaseFee:     big.NewInt(25000000000),
				ExtDataHash: &common.Hash{},
				ExtDataGasUsed: big.NewInt(0),
				BlockGasCost:   big.NewInt(0),
			},
		},
		{
			name: "19-field Lux block with non-zero ExtDataHash",
			header: &types.Header{
				ParentHash:  common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
				UncleHash:   types.EmptyUncleHash,
				Coinbase:    common.HexToAddress("0x1234567890123456789012345678901234567890"),
				Root:        common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
				TxHash:      types.EmptyTxsHash,
				ReceiptHash: types.EmptyReceiptsHash,
				Bloom:       types.Bloom{},
				Difficulty:  big.NewInt(1),
				Number:      big.NewInt(100),
				GasLimit:    8000000,
				GasUsed:     21000,
				Time:        1699000000,
				Extra:       []byte{0x01, 0x02, 0x03},
				MixDigest:   common.Hash{},
				Nonce:       types.BlockNonce{},
				BaseFee:     big.NewInt(25000000000),
				ExtDataHash: func() *common.Hash {
					h := common.HexToHash("0xfedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321")
					return &h
				}(),
				ExtDataGasUsed: big.NewInt(50000),
				BlockGasCost:   big.NewInt(100000),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded, err := rlp.EncodeToBytes(tt.header)
			if err != nil {
				t.Fatalf("Failed to encode header: %v", err)
			}

			// Decode
			var decoded types.Header
			if err := rlp.DecodeBytes(encoded, &decoded); err != nil {
				t.Fatalf("Failed to decode header: %v", err)
			}

			// Re-encode
			reencoded, err := rlp.EncodeToBytes(&decoded)
			if err != nil {
				t.Fatalf("Failed to re-encode header: %v", err)
			}

			// Verify roundtrip
			if !bytes.Equal(encoded, reencoded) {
				t.Errorf("RLP bytes mismatch after roundtrip:\nOriginal: %s\nDecoded:  %s",
					hex.EncodeToString(encoded), hex.EncodeToString(reencoded))
			}

			// Verify hash consistency
			origHash := tt.header.Hash()
			decodedHash := decoded.Hash()
			if origHash != decodedHash {
				t.Errorf("Hash mismatch after roundtrip:\nOriginal: %s\nDecoded:  %s",
					origHash.Hex(), decodedHash.Hex())
			}

			// Log success
			t.Logf("RLP roundtrip successful, hash: %s, encoded bytes: %d",
				origHash.Hex(), len(encoded))
		})
	}
}

// TestGenesisHashConsistency verifies that genesis block hash computation is consistent.
func TestGenesisHashConsistency(t *testing.T) {
	// Create a genesis-like header (16-field format)
	genesis := &types.Header{
		ParentHash:  common.Hash{},
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    common.Address{},
		Root:        types.EmptyRootHash,
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		Bloom:       types.Bloom{},
		Difficulty:  big.NewInt(131072),
		Number:      big.NewInt(0),
		GasLimit:    4712388,
		GasUsed:     0,
		Time:        0,
		Extra:       []byte{},
		MixDigest:   common.Hash{},
		Nonce:       types.BlockNonce{},
		BaseFee:     nil,
	}

	// Compute hash (should use Hash16 for genesis)
	hash := genesis.Hash()
	t.Logf("Genesis hash: %s", hash.Hex())

	// Compute Hash16 explicitly
	hash16 := genesis.Hash16()
	t.Logf("Hash16: %s", hash16.Hex())

	// They should be the same for genesis
	if hash != hash16 {
		t.Errorf("Genesis Hash() != Hash16(): %s != %s", hash.Hex(), hash16.Hex())
	}

	// Encode and decode
	encoded, err := rlp.EncodeToBytes(genesis)
	if err != nil {
		t.Fatalf("Failed to encode genesis: %v", err)
	}

	var decoded types.Header
	if err := rlp.DecodeBytes(encoded, &decoded); err != nil {
		t.Fatalf("Failed to decode genesis: %v", err)
	}

	// Hash should be consistent after decode
	decodedHash := decoded.Hash()
	if hash != decodedHash {
		t.Errorf("Genesis hash changed after roundtrip: %s != %s", hash.Hex(), decodedHash.Hex())
	}

	t.Logf("Genesis hash consistent: %s", hash.Hex())
}

// TestLuxBlockHashConsistency verifies that Lux block hash computation is consistent.
func TestLuxBlockHashConsistency(t *testing.T) {
	// Create a Lux-style header (19-field format)
	header := &types.Header{
		ParentHash:  common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Root:        common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		Bloom:       types.Bloom{},
		Difficulty:  big.NewInt(1),
		Number:      big.NewInt(100),
		GasLimit:    8000000,
		GasUsed:     21000,
		Time:        1699000000,
		Extra:       []byte{0x01, 0x02, 0x03},
		MixDigest:   common.Hash{},
		Nonce:       types.BlockNonce{},
		BaseFee:     big.NewInt(25000000000),
		ExtDataHash: func() *common.Hash {
			h := types.EmptyRootHash
			return &h
		}(),
		ExtDataGasUsed: big.NewInt(0),
		BlockGasCost:   big.NewInt(0),
	}

	// Compute hash (should use Hash19 for Lux blocks)
	hash := header.Hash()
	t.Logf("Lux block hash: %s", hash.Hex())

	// Encode and decode
	encoded, err := rlp.EncodeToBytes(header)
	if err != nil {
		t.Fatalf("Failed to encode header: %v", err)
	}
	t.Logf("Encoded %d bytes", len(encoded))

	var decoded types.Header
	if err := rlp.DecodeBytes(encoded, &decoded); err != nil {
		t.Fatalf("Failed to decode header: %v", err)
	}

	// Hash should be consistent after decode
	decodedHash := decoded.Hash()
	if hash != decodedHash {
		t.Errorf("Hash changed after roundtrip: %s != %s", hash.Hex(), decodedHash.Hex())
	}

	t.Logf("Lux block hash consistent: %s", hash.Hex())
}
