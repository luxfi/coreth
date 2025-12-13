// Copyright 2025 Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package eth

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/coreth/core"
	"github.com/luxfi/coreth/core/types"
	"github.com/luxfi/coreth/log"
	"github.com/luxfi/geth/rlp"
)

// LuxAPI provides RPC methods for Lux-specific operations
type LuxAPI struct {
	eth *Ethereum
}

// NewLuxAPI creates a new instance of LuxAPI
func NewLuxAPI(eth *Ethereum) *LuxAPI {
	return &LuxAPI{eth: eth}
}

// JSONLBlock represents a block in JSONL format
type JSONLBlock struct {
	Number      uint64 `json:"Number"`
	Hash        string `json:"Hash"`
	ParentHash  string `json:"ParentHash"`
	Header      string `json:"Header"`   // RLP-encoded header (hex)
	Body        string `json:"Body"`     // RLP-encoded body (hex)
	Receipts    string `json:"Receipts"` // RLP-encoded receipts (hex)
}

// ImportJSONL imports blocks from a JSONL file into the blockchain
// Returns the number of blocks imported and any error
func (api *LuxAPI) ImportJSONL(file string) (int, error) {
	log.Info("Starting JSONL import", "file", file)

	// Open the file
	f, err := os.Open(file)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	blockchain := api.eth.BlockChain()
	scanner := bufio.NewScanner(f)
	// Increase buffer size for large lines (blocks can be big)
	buf := make([]byte, 0, 64*1024*1024) // 64MB buffer
	scanner.Buffer(buf, 64*1024*1024)

	imported := 0
	batchSize := 100
	batch := make([]*types.Block, 0, batchSize)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var jsonBlock JSONLBlock
		if err := json.Unmarshal(line, &jsonBlock); err != nil {
			log.Warn("Failed to parse JSONL line", "err", err)
			continue
		}

		// Skip genesis block
		if jsonBlock.Number == 0 {
			continue
		}

		// Decode the block from RLP
		block, err := decodeBlockFromJSONL(&jsonBlock)
		if err != nil {
			log.Warn("Failed to decode block", "number", jsonBlock.Number, "err", err)
			continue
		}

		// Check if block already exists
		if blockchain.HasBlock(block.Hash(), block.NumberU64()) {
			continue
		}

		batch = append(batch, block)

		// Flush batch when full
		if len(batch) >= batchSize {
			if _, err := blockchain.InsertChain(batch); err != nil {
				return imported, fmt.Errorf("failed to insert batch at block %d: %w", batch[0].NumberU64(), err)
			}
			imported += len(batch)
			log.Info("Imported batch", "blocks", len(batch), "total", imported, "latest", batch[len(batch)-1].NumberU64())
			batch = batch[:0]
		}
	}

	// Import remaining batch
	if len(batch) > 0 {
		if _, err := blockchain.InsertChain(batch); err != nil {
			return imported, fmt.Errorf("failed to insert final batch: %w", err)
		}
		imported += len(batch)
	}

	if err := scanner.Err(); err != nil {
		return imported, fmt.Errorf("scanner error: %w", err)
	}

	log.Info("JSONL import completed", "total_imported", imported)
	return imported, nil
}

// ImportBlock imports a single block from JSON data
func (api *LuxAPI) ImportBlock(blockJSON map[string]interface{}) (common.Hash, error) {
	// Extract RLP data
	headerHex, ok := blockJSON["Header"].(string)
	if !ok {
		return common.Hash{}, fmt.Errorf("missing or invalid Header field")
	}
	bodyHex, ok := blockJSON["Body"].(string)
	if !ok {
		return common.Hash{}, fmt.Errorf("missing or invalid Body field")
	}

	jsonBlock := &JSONLBlock{
		Header: headerHex,
		Body:   bodyHex,
	}

	block, err := decodeBlockFromJSONL(jsonBlock)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to decode block: %w", err)
	}

	// Skip genesis
	if block.NumberU64() == 0 {
		return common.Hash{}, fmt.Errorf("cannot import genesis block")
	}

	blockchain := api.eth.BlockChain()

	// Check if already exists
	if blockchain.HasBlock(block.Hash(), block.NumberU64()) {
		return block.Hash(), nil // Already imported
	}

	// Insert
	if _, err := blockchain.InsertChain([]*types.Block{block}); err != nil {
		return common.Hash{}, fmt.Errorf("failed to insert block: %w", err)
	}

	return block.Hash(), nil
}

// ImportBlocks imports multiple blocks from JSON array
func (api *LuxAPI) ImportBlocks(blocksJSON []map[string]interface{}) (int, error) {
	blockchain := api.eth.BlockChain()
	blocks := make([]*types.Block, 0, len(blocksJSON))

	for i, blockJSON := range blocksJSON {
		headerHex, ok := blockJSON["Header"].(string)
		if !ok {
			return 0, fmt.Errorf("block %d: missing Header", i)
		}
		bodyHex, ok := blockJSON["Body"].(string)
		if !ok {
			return 0, fmt.Errorf("block %d: missing Body", i)
		}

		jsonBlock := &JSONLBlock{
			Header: headerHex,
			Body:   bodyHex,
		}

		block, err := decodeBlockFromJSONL(jsonBlock)
		if err != nil {
			return 0, fmt.Errorf("block %d: decode failed: %w", i, err)
		}

		// Skip genesis
		if block.NumberU64() == 0 {
			continue
		}

		// Skip existing
		if blockchain.HasBlock(block.Hash(), block.NumberU64()) {
			continue
		}

		blocks = append(blocks, block)
	}

	if len(blocks) == 0 {
		return 0, nil
	}

	if _, err := blockchain.InsertChain(blocks); err != nil {
		return 0, fmt.Errorf("failed to insert blocks: %w", err)
	}

	return len(blocks), nil
}

// Status returns the current blockchain status
func (api *LuxAPI) Status() map[string]interface{} {
	blockchain := api.eth.BlockChain()
	current := blockchain.CurrentBlock()

	return map[string]interface{}{
		"currentBlock": current.Number.Uint64(),
		"currentHash":  current.Hash().Hex(),
		"chainId":      blockchain.Config().ChainID.Uint64(),
	}
}

// decodeBlockFromJSONL decodes a block from JSONL format
func decodeBlockFromJSONL(jsonBlock *JSONLBlock) (*types.Block, error) {
	// Decode header
	headerBytes, err := hexDecode(jsonBlock.Header)
	if err != nil {
		return nil, fmt.Errorf("failed to decode header hex: %w", err)
	}

	var header types.Header
	if err := rlp.DecodeBytes(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to decode header RLP: %w", err)
	}

	// Decode body
	bodyBytes, err := hexDecode(jsonBlock.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode body hex: %w", err)
	}

	var body types.Body
	if err := rlp.DecodeBytes(bodyBytes, &body); err != nil {
		return nil, fmt.Errorf("failed to decode body RLP: %w", err)
	}

	// Create block
	block := types.NewBlockWithHeader(&header).WithBody(body)

	return block, nil
}

// hexDecode decodes a hex string (with or without 0x prefix)
func hexDecode(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	return hex.DecodeString(s)
}

// hasAllBlocksLux checks if all blocks already exist
func hasAllBlocksLux(chain *core.BlockChain, bs []*types.Block) bool {
	for _, b := range bs {
		if !chain.HasBlock(b.Hash(), b.NumberU64()) {
			return false
		}
	}
	return true
}
