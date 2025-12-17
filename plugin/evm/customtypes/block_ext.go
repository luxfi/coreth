// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customtypes

import (
	"math/big"
	"slices"

	"github.com/luxfi/geth/common"
	ethtypes "github.com/luxfi/geth/core/types"
)

// SetBlockExtra sets the [BlockBodyExtra] `extra` in the [Block] `b`.
func SetBlockExtra(b *ethtypes.Block, extra *BlockBodyExtra) {
	extras.Block.Set(b, extra)
}

// BlockBodyExtra is a struct containing extra fields used by Lux
// in the [Block] and [Body].
type BlockBodyExtra struct {
	Version uint32
	ExtData *[]byte
}

// Copy deep copies the [BlockBodyExtra] `b` and returns it.
// It is notably used in the following functions:
// - [ethtypes.Block.Body]
// - [ethtypes.Block.WithSeal]
// - [ethtypes.Block.WithBody]
// - [ethtypes.Block.WithWithdrawals]
func (b *BlockBodyExtra) Copy() *BlockBodyExtra {
	cpy := *b
	if b.ExtData != nil {
		data := slices.Clone(*b.ExtData)
		cpy.ExtData = &data
	}
	return &cpy
}

// BodyRLPFieldPointersForEncoding returns the fields that should be encoded
// for the [Body] and [BlockBodyExtra].
// Note the following fields are added (+) and removed (-) compared to geth:
// - (-) [ethtypes.Body] `Withdrawals` field
// - (+) [BlockBodyExtra] `Version` field
// - (+) [BlockBodyExtra] `ExtData` field
// TODO: Temporarily disabled - needs geth support
/*
func (b *BlockBodyExtra) BodyRLPFieldsForEncoding(body *ethtypes.Body) *rlp.Fields {
	return &rlp.Fields{
		Required: []any{
			body.Transactions,
			body.Uncles,
			b.Version,
			b.ExtData,
		},
	}
}
*/

// TODO: Temporarily disabled - needs geth support
/*
// BodyRLPFieldPointersForDecoding returns the fields that should be decoded to
// for the [Body] and [BlockBodyExtra].
func (b *BlockBodyExtra) BodyRLPFieldPointersForDecoding(body *ethtypes.Body) *rlp.Fields {
	return &rlp.Fields{
		Required: []any{
			&body.Transactions,
			&body.Uncles,
			&b.Version,
			&b.ExtData,
		},
	}
}

// BlockRLPFieldPointersForEncoding returns the fields that should be encoded
// for the [Block] and [BlockBodyExtra].
// Note the following fields are added (+) and removed (-) compared to geth:
// - (-) [ethtypes.Block] `Withdrawals` field
// - (+) [BlockBodyExtra] `Version` field
// - (+) [BlockBodyExtra] `ExtData` field
func (b *BlockBodyExtra) BlockRLPFieldsForEncoding(block *ethtypes.BlockRLPProxy) *rlp.Fields {
	return &rlp.Fields{
		Required: []any{
			block.Header,
			block.Txs,
			block.Uncles,
			b.Version,
			b.ExtData,
		},
	}
}

// BlockRLPFieldPointersForDecoding returns the fields that should be decoded to
// for the [Block] and [BlockBodyExtra].
func (b *BlockBodyExtra) BlockRLPFieldPointersForDecoding(block *ethtypes.BlockRLPProxy) *rlp.Fields {
	return &rlp.Fields{
		Required: []any{
			&block.Header,
			&block.Txs,
			&block.Uncles,
			&b.Version,
			&b.ExtData,
		},
	}
}
*/

func BlockExtData(b *ethtypes.Block) []byte {
	if data := extras.Block.Get(b).ExtData; data != nil {
		return *data
	}
	return nil
}

// GetBlockVersion returns the block version from the block body extra.
// Named GetBlockVersion to avoid collision with ethtypes.BlockVersion type.
func GetBlockVersion(b *ethtypes.Block) uint32 {
	return extras.Block.Get(b).Version
}

func BlockExtDataGasUsed(b *ethtypes.Block) *big.Int {
	used := GetHeaderExtra(b.Header()).ExtDataGasUsed
	if used == nil {
		return nil
	}
	return new(big.Int).Set(used)
}

func BlockGasCost(b *ethtypes.Block) *big.Int {
	cost := GetHeaderExtra(b.Header()).BlockGasCost
	if cost == nil {
		return nil
	}
	return new(big.Int).Set(cost)
}

func CalcExtDataHash(extdata []byte) common.Hash {
	if len(extdata) == 0 {
		return EmptyExtDataHash
	}
	return rlpHash(extdata)
}

func NewBlockWithExtData(
	header *ethtypes.Header, txs []*ethtypes.Transaction, uncles []*ethtypes.Header, receipts []*ethtypes.Receipt,
	hasher ethtypes.ListHasher, extdata []byte, recalc bool,
) *ethtypes.Block {
	// Get header extras from the original header (stored in sync.Map)
	origExtra := GetHeaderExtra(header)

	// Calculate ExtDataHash if recalc requested
	extDataHash := origExtra.ExtDataHash
	if recalc {
		extDataHash = CalcExtDataHash(extdata)
	}

	// Copy the Lux fields to the geth Header struct BEFORE creating the block
	// This ensures consistent hash computation after JSON roundtrip
	if extDataHash != (common.Hash{}) {
		header.ExtDataHash = &extDataHash
	}
	if origExtra.ExtDataGasUsed != nil {
		header.ExtDataGasUsed = new(big.Int).Set(origExtra.ExtDataGasUsed)
	}
	if origExtra.BlockGasCost != nil {
		header.BlockGasCost = new(big.Int).Set(origExtra.BlockGasCost)
	}

	body := &ethtypes.Body{
		Transactions: txs,
		Uncles:       uncles,
	}
	block := ethtypes.NewBlock(header, body, receipts, hasher)

	// Also store in HeaderExtra sync.Map for backward compatibility
	blockHeaderExtra := &HeaderExtra{
		ExtDataHash:    extDataHash,
		ExtDataGasUsed: origExtra.ExtDataGasUsed,
		BlockGasCost:   origExtra.BlockGasCost,
	}
	SetHeaderExtra(block.Header(), blockHeaderExtra)

	extdataCopy := make([]byte, len(extdata))
	copy(extdataCopy, extdata)
	extra := &BlockBodyExtra{
		ExtData: &extdataCopy,
	}
	extras.Block.Set(block, extra)
	return block
}
