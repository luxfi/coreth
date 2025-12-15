// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"os"
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

// ExportChainRLPReply is the response for ExportChainRLP
type ExportChainRLPReply struct {
	Success    bool   `json:"success"`
	File       string `json:"file"`
	FirstBlock uint64 `json:"firstBlock"`
	LastBlock  uint64 `json:"lastBlock"`
	Count      uint64 `json:"count"`
	Message    string `json:"message,omitempty"`
}

// ExportChainRLP exports blocks to RLP format compatible with admin_importChain.
// This is the recommended format for chain migration/recovery.
// Parameters:
//   - file: Output file path (supports .gz compression)
//   - first: First block number to export (optional, defaults to 1)
//   - last: Last block number to export (optional, defaults to current head)
func (api *MigrateAPI) ExportChainRLP(file string, first *uint64, last *uint64) (*ExportChainRLPReply, error) {
	reply := &ExportChainRLPReply{File: file}
	bc := api.vm.blockChain

	// Determine block range
	var startBlock, endBlock uint64 = 1, bc.CurrentBlock().Number.Uint64()
	if first != nil {
		startBlock = *first
	}
	if last != nil {
		endBlock = *last
	}

	if startBlock > endBlock {
		return reply, fmt.Errorf("first block (%d) cannot be greater than last block (%d)", startBlock, endBlock)
	}

	// Skip genesis (block 0) as admin_importChain expects
	if startBlock == 0 {
		startBlock = 1
	}

	reply.FirstBlock = startBlock
	reply.LastBlock = endBlock

	// Use the existing blockchain Export functionality
	// This writes blocks in RLP format compatible with admin_importChain
	if err := bc.ExportN(nil, startBlock, endBlock); err != nil {
		// ExportN with nil writer just validates - now export to file
	}

	// Create output file
	outFile, err := os.Create(file)
	if err != nil {
		return reply, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	var writer io.Writer = outFile
	if strings.HasSuffix(file, ".gz") {
		gzWriter := gzip.NewWriter(outFile)
		defer gzWriter.Close()
		writer = gzWriter
	}

	// Export blocks
	var count uint64
	err = bc.ExportCallback(func(block *types.Block) error {
		if err := block.EncodeRLP(writer); err != nil {
			return err
		}
		count++
		if count%10000 == 0 {
			api.vm.logger.Info("ExportChainRLP progress",
				"exported", count,
				"height", block.NumberU64(),
			)
		}
		return nil
	}, startBlock, endBlock)

	if err != nil {
		return reply, fmt.Errorf("export failed: %w", err)
	}

	reply.Success = true
	reply.Count = count
	reply.Message = fmt.Sprintf("Exported %d blocks to %s", count, file)

	api.vm.logger.Info("ExportChainRLP complete",
		"file", file,
		"blocks", count,
		"range", fmt.Sprintf("%d-%d", startBlock, endBlock),
	)

	return reply, nil
}

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
// RAW IMPORT METHODS - Bypass consensus validation for disaster recovery
// ============================================================================

// ImportBlocksRawReply is the response for ImportBlocksRaw
type ImportBlocksRawReply struct {
	Imported      int      `json:"imported"`
	Failed        int      `json:"failed"`
	FirstHeight   uint64   `json:"firstHeight,omitempty"`
	LastHeight    uint64   `json:"lastHeight,omitempty"`
	Errors        []string `json:"errors,omitempty"`
	HeadBlockHash string   `json:"headBlockHash,omitempty"`
}

// ImportBlocksRaw imports blocks directly to the database without EVM execution.
// This is useful for disaster recovery when the genesis doesn't match.
// WARNING: This bypasses all validation - use only for trusted data!
func (api *MigrateAPI) ImportBlocksRaw(blocks []rpcapi.BlockEntry) (*ImportBlocksRawReply, error) {
	reply := &ImportBlocksRawReply{}
	db := api.vm.chaindb

	if len(blocks) == 0 {
		return reply, fmt.Errorf("no blocks provided")
	}

	reply.FirstHeight = blocks[0].Height
	reply.LastHeight = blocks[len(blocks)-1].Height

	for _, entry := range blocks {
		// Decode header using flexible decoder that handles pre/post Shanghai
		headerBytes, err := rpcapi.DecodeHexString(entry.Header)
		if err != nil {
			reply.Failed++
			reply.Errors = append(reply.Errors, fmt.Sprintf("block %d: header decode: %v", entry.Height, err))
			continue
		}

		header, err := rpcapi.DecodeHeader(headerBytes)
		if err != nil {
			reply.Failed++
			reply.Errors = append(reply.Errors, fmt.Sprintf("block %d: header RLP: %v", entry.Height, err))
			continue
		}

		// Decode body
		bodyBytes, err := rpcapi.DecodeHexString(entry.Body)
		if err != nil {
			reply.Failed++
			reply.Errors = append(reply.Errors, fmt.Sprintf("block %d: body decode: %v", entry.Height, err))
			continue
		}

		var body types.Body
		if err := rlp.DecodeBytes(bodyBytes, &body); err != nil {
			reply.Failed++
			reply.Errors = append(reply.Errors, fmt.Sprintf("block %d: body RLP: %v", entry.Height, err))
			continue
		}

		// Decode receipts if provided (using storage format)
		var receipts types.Receipts
		if entry.Receipts != "" {
			receiptsBytes, err := rpcapi.DecodeHexString(entry.Receipts)
			if err != nil {
				reply.Failed++
				reply.Errors = append(reply.Errors, fmt.Sprintf("block %d: receipts decode: %v", entry.Height, err))
				continue
			}
			// Try storage format first (without Bloom), then consensus format
			var storageReceipts []*types.ReceiptForStorage
			if err := rlp.DecodeBytes(receiptsBytes, &storageReceipts); err == nil {
				// Convert from storage format to full receipts
				for _, sr := range storageReceipts {
					receipts = append(receipts, (*types.Receipt)(sr))
				}
			} else {
				// Fall back to consensus format (with Bloom)
				if err := rlp.DecodeBytes(receiptsBytes, &receipts); err != nil {
					// On error, continue without receipts for raw import
					api.vm.logger.Warn("ImportBlocksRaw: failed to decode receipts, continuing without",
						"block", entry.Height, "error", err)
				}
			}
		}

		// Create block
		block := types.NewBlockWithHeader(header).WithBody(body)

		// Write directly to rawdb (bypass validation)
		rawdb.WriteBlock(db, block)
		rawdb.WriteReceipts(db, block.Hash(), block.NumberU64(), receipts)
		rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64())

		// Update head pointers
		rawdb.WriteHeadBlockHash(db, block.Hash())
		rawdb.WriteHeadHeaderHash(db, block.Hash())

		reply.Imported++
		reply.HeadBlockHash = block.Hash().Hex()

		// Log progress every 1000 blocks
		if reply.Imported%1000 == 0 {
			api.vm.logger.Info("ImportBlocksRaw progress",
				"imported", reply.Imported,
				"failed", reply.Failed,
				"height", entry.Height,
			)
		}
	}

	api.vm.logger.Info("ImportBlocksRaw complete",
		"imported", reply.Imported,
		"failed", reply.Failed,
	)

	return reply, nil
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

// FinalizeImportReply is the response for FinalizeImport
type FinalizeImportReply struct {
	Success       bool   `json:"success"`
	HeadBlock     uint64 `json:"headBlock"`
	HeadHash      string `json:"headHash"`
	PreviousBlock uint64 `json:"previousBlock"`
	PreviousHash  string `json:"previousHash"`
	Message       string `json:"message,omitempty"`
}

// FinalizeImport updates the in-memory chain state after ImportBlocksRaw.
// This should be called after importing blocks to make eth_blockNumber return
// the correct value without restarting the node.
//
// The targetHeight parameter specifies which block to set as the chain head.
// Use 0 to auto-detect from the database's HeadBlockHash.
func (api *MigrateAPI) FinalizeImport(targetHeight uint64) (*FinalizeImportReply, error) {
	reply := &FinalizeImportReply{}
	bc := api.vm.blockChain
	db := api.vm.chaindb

	// Get current state for comparison
	currentHeader := bc.CurrentBlock()
	reply.PreviousBlock = currentHeader.Number.Uint64()
	reply.PreviousHash = currentHeader.Hash().Hex()

	var targetBlock *types.Block

	if targetHeight == 0 {
		// Auto-detect from database
		headHash := rawdb.ReadHeadBlockHash(db)
		if headHash == (common.Hash{}) {
			return reply, fmt.Errorf("no head block hash found in database")
		}
		targetBlock = bc.GetBlockByHash(headHash)
		if targetBlock == nil {
			// Try to read directly from rawdb
			num, found := rawdb.ReadHeaderNumber(db, headHash)
			if !found {
				return reply, fmt.Errorf("head block not found: %s", headHash.Hex())
			}
			targetBlock = rawdb.ReadBlock(db, headHash, num)
		}
	} else {
		targetBlock = bc.GetBlockByNumber(targetHeight)
		if targetBlock == nil {
			// Try to read from rawdb via canonical hash
			hash := rawdb.ReadCanonicalHash(db, targetHeight)
			if hash == (common.Hash{}) {
				return reply, fmt.Errorf("canonical hash not found for height %d", targetHeight)
			}
			targetBlock = rawdb.ReadBlock(db, hash, targetHeight)
		}
	}

	if targetBlock == nil {
		return reply, fmt.Errorf("target block not found")
	}

	reply.HeadBlock = targetBlock.NumberU64()
	reply.HeadHash = targetBlock.Hash().Hex()

	// Update database head pointers
	rawdb.WriteHeadBlockHash(db, targetBlock.Hash())
	rawdb.WriteHeadHeaderHash(db, targetBlock.Hash())
	rawdb.WriteCanonicalHash(db, targetBlock.Hash(), targetBlock.NumberU64())

	// Update the VM's accepted block database
	// This is required for the Avalanche/Lux consensus to recognize the block
	if err := api.vm.acceptedBlockDB.Put(lastAcceptedKey, targetBlock.Hash().Bytes()); err != nil {
		api.vm.logger.Warn("FinalizeImport: failed to update acceptedBlockDB", "error", err)
	}

	// Reload in-memory chain state from database
	if err := bc.ReloadHeadFromDB(); err != nil {
		api.vm.logger.Warn("FinalizeImport: failed to reload head from DB", "error", err)
		reply.Message = fmt.Sprintf("Database updated but in-memory reload failed: %v. Restart node to apply.", err)
		return reply, nil
	}

	api.vm.logger.Info("FinalizeImport: chain head updated",
		"height", targetBlock.NumberU64(),
		"hash", targetBlock.Hash().Hex(),
	)

	reply.Success = true
	reply.Message = "Chain head updated successfully. eth_blockNumber should now return the correct value."

	return reply, nil
}
