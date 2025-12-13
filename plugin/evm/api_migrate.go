// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
	"github.com/luxfi/migrate/rpcapi"
)

// MigrateAPI provides RPC methods for blockchain migration.
// This includes both export (from source chain) and import (to target chain).
// It uses the shared migrate/rpcapi package for consistent behavior with SubnetEVM.
type MigrateAPI struct {
	vm *VM
}

// NewMigrateAPI creates a new MigrateAPI instance
func NewMigrateAPI(vm *VM) *MigrateAPI {
	return &MigrateAPI{vm: vm}
}

// vmLogAdapter adapts the VM logger to the rpcapi.Logger interface
type vmLogAdapter struct {
	vm *VM
}

func (l vmLogAdapter) Info(msg string, ctx ...interface{}) {
	l.vm.logger.Info(msg, ctx...)
}

// ============================================================================
// IMPORT METHODS - Uses InsertChain with full EVM execution
// ============================================================================

// ImportBlocks imports blocks using full EVM execution via InsertChain.
// This requires a matching genesis configuration on the target chain.
// Each block's transactions are executed to rebuild state.
func (api *MigrateAPI) ImportBlocks(blocks []rpcapi.BlockEntry) (*rpcapi.ImportBlocksReply, error) {
	reply := &rpcapi.ImportBlocksReply{}
	bc := api.vm.blockChain
	logger := vmLogAdapter{vm: api.vm}

	if err := rpcapi.ImportBlocks(bc, logger, blocks, reply); err != nil {
		return reply, err
	}
	return reply, nil
}

// ============================================================================
// EXPORT METHODS - Export blocks for migration
// ============================================================================

// ExportBlock exports a single block by number or hash.
// Returns the block data in JSONL-compatible format.
func (api *MigrateAPI) ExportBlock(blockNumOrHash string) (*rpcapi.BlockEntry, error) {
	bc := api.vm.blockChain
	db := api.vm.chaindb

	var block *types.Block

	// Try to parse as number first
	if strings.HasPrefix(blockNumOrHash, "0x") && len(blockNumOrHash) == 66 {
		// It's a hash
		hash := common.HexToHash(blockNumOrHash)
		block = bc.GetBlockByHash(hash)
	} else {
		// Try as number
		var num uint64
		if strings.HasPrefix(blockNumOrHash, "0x") {
			fmt.Sscanf(blockNumOrHash, "0x%x", &num)
		} else {
			fmt.Sscanf(blockNumOrHash, "%d", &num)
		}
		block = bc.GetBlockByNumber(num)
	}

	if block == nil {
		return nil, fmt.Errorf("block not found: %s", blockNumOrHash)
	}

	return api.blockToEntry(block, db)
}

// ExportBlockRange exports a range of blocks.
// Returns blocks from startHeight to endHeight (inclusive).
func (api *MigrateAPI) ExportBlockRange(startHeight, endHeight uint64) ([]rpcapi.BlockEntry, error) {
	bc := api.vm.blockChain
	db := api.vm.chaindb

	if endHeight < startHeight {
		return nil, fmt.Errorf("endHeight (%d) must be >= startHeight (%d)", endHeight, startHeight)
	}

	currentHeight := bc.CurrentBlock().Number.Uint64()
	if endHeight > currentHeight {
		endHeight = currentHeight
	}

	entries := make([]rpcapi.BlockEntry, 0, endHeight-startHeight+1)

	for height := startHeight; height <= endHeight; height++ {
		block := bc.GetBlockByNumber(height)
		if block == nil {
			return nil, fmt.Errorf("block %d not found", height)
		}

		entry, err := api.blockToEntry(block, db)
		if err != nil {
			return nil, fmt.Errorf("block %d export failed: %w", height, err)
		}
		entries = append(entries, *entry)
	}

	return entries, nil
}

// blockToEntry converts a block to a BlockEntry for export
func (api *MigrateAPI) blockToEntry(block *types.Block, db interface{}) (*rpcapi.BlockEntry, error) {
	// Encode header
	headerBytes, err := rlp.EncodeToBytes(block.Header())
	if err != nil {
		return nil, fmt.Errorf("header encode: %w", err)
	}

	// Encode body
	body := &types.Body{
		Transactions: block.Transactions(),
		Uncles:       block.Uncles(),
	}
	bodyBytes, err := rlp.EncodeToBytes(body)
	if err != nil {
		return nil, fmt.Errorf("body encode: %w", err)
	}

	// Get receipts
	var receiptsHex string
	if ethdb, ok := db.(interface{ GetKey([]byte) []byte }); ok {
		_ = ethdb // Receipts lookup if needed
	}
	// Read receipts from rawdb
	receipts := rawdb.ReadReceipts(api.vm.chaindb, block.Hash(), block.NumberU64(), block.Time(), api.vm.blockChain.Config())
	if receipts != nil {
		receiptsBytes, err := rlp.EncodeToBytes(receipts)
		if err == nil {
			receiptsHex = "0x" + hex.EncodeToString(receiptsBytes)
		}
	}

	entry := &rpcapi.BlockEntry{
		Height:   block.NumberU64(),
		Hash:     block.Hash().Hex(),
		Header:   "0x" + hex.EncodeToString(headerBytes),
		Body:     "0x" + hex.EncodeToString(bodyBytes),
		Receipts: receiptsHex,
		Meta: &rpcapi.BlockMeta{
			ChainID:     api.vm.blockChain.Config().ChainID.Uint64(),
			GenesisHash: api.vm.blockChain.Genesis().Hash().Hex(),
		},
	}

	return entry, nil
}

// ============================================================================
// INFO METHODS
// ============================================================================

// GetChainInfo returns information about the current chain state.
func (api *MigrateAPI) GetChainInfo() (*rpcapi.ChainInfo, error) {
	bc := api.vm.blockChain
	current := bc.CurrentBlock()

	info := &rpcapi.ChainInfo{
		ChainID:       bc.Config().ChainID.Uint64(),
		CurrentHeight: current.Number.Uint64(),
		CurrentHash:   current.Hash().Hex(),
		StateRoot:     current.Root.Hex(),
		GenesisHash:   bc.Genesis().Hash().Hex(),
	}

	return info, nil
}
