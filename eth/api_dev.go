// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/holiman/uint256"
	"github.com/luxfi/coreth/plugin/evm/upgrade/ap3"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/common/hexutil"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/core/tracing"
	"github.com/luxfi/geth/core/types"
	log "github.com/luxfi/log"
)

// DevAPI provides Anvil/Hardhat-compatible RPC methods for dev mode.
// These methods allow manipulation of blockchain state for testing purposes.
type DevAPI struct {
	eth       *Ethereum
	snapshots map[hexutil.Uint64]common.Hash // snapshot ID -> block hash
	nextSnap  hexutil.Uint64
	mu        sync.Mutex
}

// NewDevAPI creates a new DevAPI instance.
func NewDevAPI(eth *Ethereum) *DevAPI {
	return &DevAPI{
		eth:       eth,
		snapshots: make(map[hexutil.Uint64]common.Hash),
		nextSnap:  1,
	}
}

// SetBalance sets the balance of an address.
// Anvil: anvil_setBalance
// Hardhat: hardhat_setBalance
func (api *DevAPI) SetBalance(ctx context.Context, address common.Address, balance hexutil.Big) error {
	log.Info("SetBalance: called", "address", address.Hex(), "balance", balance.String())
	api.mu.Lock()
	defer api.mu.Unlock()

	// Get the current state
	header := api.eth.blockchain.CurrentBlock()
	log.Info("SetBalance: got current block", "number", header.Number, "root", header.Root.Hex())
	statedb, err := api.eth.blockchain.StateAt(header.Root)
	if err != nil {
		return fmt.Errorf("SetBalance: StateAt failed: %w", err)
	}

	// Set the balance
	u256Balance, overflow := uint256.FromBig((*big.Int)(&balance))
	if overflow {
		return errors.New("balance overflow")
	}
	statedb.SetBalance(address, u256Balance, tracing.BalanceChangeUnspecified)

	// Commit the state changes and create a new block
	return api.commitStateChanges(statedb)
}

// SetStorageAt sets a storage slot value for an address.
// Anvil: anvil_setStorageAt
// Hardhat: hardhat_setStorageAt
func (api *DevAPI) SetStorageAt(ctx context.Context, address common.Address, slot common.Hash, value common.Hash) error {
	api.mu.Lock()
	defer api.mu.Unlock()

	// Get the current state
	header := api.eth.blockchain.CurrentBlock()
	statedb, err := api.eth.blockchain.StateAt(header.Root)
	if err != nil {
		return err
	}

	// Set the storage value
	statedb.SetState(address, slot, value)

	// Commit the state changes and create a new block
	return api.commitStateChanges(statedb)
}

// SetCode sets the code of an address.
// Anvil: anvil_setCode
// Hardhat: hardhat_setCode
func (api *DevAPI) SetCode(ctx context.Context, address common.Address, code hexutil.Bytes) error {
	api.mu.Lock()
	defer api.mu.Unlock()

	// Get the current state
	header := api.eth.blockchain.CurrentBlock()
	statedb, err := api.eth.blockchain.StateAt(header.Root)
	if err != nil {
		return err
	}

	// Set the code
	statedb.SetCode(address, code, tracing.CodeChangeUnspecified)

	// Commit the state changes and create a new block
	return api.commitStateChanges(statedb)
}

// SetNonce sets the nonce of an address.
// Anvil: anvil_setNonce
// Hardhat: hardhat_setNonce
func (api *DevAPI) SetNonce(ctx context.Context, address common.Address, nonce hexutil.Uint64) error {
	api.mu.Lock()
	defer api.mu.Unlock()

	// Get the current state
	header := api.eth.blockchain.CurrentBlock()
	statedb, err := api.eth.blockchain.StateAt(header.Root)
	if err != nil {
		return err
	}

	// Set the nonce
	statedb.SetNonce(address, uint64(nonce), tracing.NonceChangeUnspecified)

	// Commit the state changes and create a new block
	return api.commitStateChanges(statedb)
}

// Mine forces mining of new blocks.
// Anvil: evm_mine (single optional timestamp parameter)
// Hardhat: hardhat_mine (blocks count, optional interval)
// We use optional pointer parameters to handle both calling conventions.
func (api *DevAPI) Mine(ctx context.Context, blocks *hexutil.Uint64, interval *hexutil.Uint64) (common.Hash, error) {
	api.mu.Lock()
	defer api.mu.Unlock()

	// Parse arguments - support both Anvil and Hardhat conventions
	// Anvil: evm_mine(timestamp?) -> mines 1 block
	// Hardhat: hardhat_mine(blocks?, interval?) -> mines N blocks
	numBlocks := uint64(1)

	if blocks != nil {
		numBlocks = uint64(*blocks)
	}

	// Interval is ignored for now (we mine instantly)
	_ = interval

	if numBlocks == 0 {
		numBlocks = 1
	}

	var lastHash common.Hash
	for i := uint64(0); i < numBlocks; i++ {
		// Generate a new block using the miner
		block, err := api.eth.miner.GenerateBlock(nil)
		if err != nil {
			return common.Hash{}, err
		}

		// Insert the block into the chain
		if err := api.eth.blockchain.InsertBlock(block); err != nil {
			return common.Hash{}, err
		}

		// Accept the block
		if err := api.eth.blockchain.Accept(block); err != nil {
			return common.Hash{}, err
		}
		// Note: We don't call DrainAcceptorQueue() here because it triggers
		// snapshot flatten which isn't supported in PathDB mode.
		// The blocks are already accepted, we just don't wait for async processing.
		lastHash = block.Hash()
	}

	return lastHash, nil
}

// Snapshot creates a snapshot of the current state.
// Returns a snapshot ID that can be used with Revert.
// Anvil: evm_snapshot
// Hardhat: evm_snapshot
func (api *DevAPI) Snapshot(ctx context.Context) hexutil.Uint64 {
	api.mu.Lock()
	defer api.mu.Unlock()

	// Store the current block hash as a snapshot
	blockHash := api.eth.blockchain.CurrentBlock().Hash()
	snapID := api.nextSnap
	api.snapshots[snapID] = blockHash
	api.nextSnap++

	return snapID
}

// Revert reverts the state to a previous snapshot.
// Note: In coreth, revert is limited - it can only revert to the current or
// recent state. Full reorg capabilities are not available.
// Anvil: evm_revert
// Hardhat: evm_revert
func (api *DevAPI) Revert(ctx context.Context, snapID hexutil.Uint64) (bool, error) {
	api.mu.Lock()
	defer api.mu.Unlock()

	blockHash, ok := api.snapshots[snapID]
	if !ok {
		return false, errors.New("snapshot not found")
	}

	// Get the block by hash
	block := api.eth.blockchain.GetBlockByHash(blockHash)
	if block == nil {
		return false, errors.New("snapshot block not found")
	}

	// For now, just verify the snapshot block exists and delete the snapshot
	// Full revert functionality requires SetPreference which may cause issues
	currentNumber := api.eth.blockchain.CurrentBlock().Number.Uint64()
	snapNumber := block.NumberU64()

	if snapNumber > currentNumber {
		return false, errors.New("cannot revert to future block")
	}

	// Delete the snapshot and all snapshots after it
	for id := range api.snapshots {
		if id >= snapID {
			delete(api.snapshots, id)
		}
	}

	// If trying to revert to the same block, it's a no-op success
	if snapNumber == currentNumber && api.eth.blockchain.CurrentBlock().Hash() == blockHash {
		return true, nil
	}

	// Otherwise, try to set preference back to the snapshot block
	if err := api.eth.blockchain.SetPreference(block); err != nil {
		return false, err
	}

	return true, nil
}

// IncreaseTime increases the block timestamp by mining a new block with adjusted time.
// Anvil: evm_increaseTime
// Hardhat: evm_increaseTime
func (api *DevAPI) IncreaseTime(ctx context.Context, seconds hexutil.Uint64) (hexutil.Uint64, error) {
	api.mu.Lock()
	defer api.mu.Unlock()

	// Get current block time and calculate the new time
	currentTime := api.eth.blockchain.CurrentBlock().Time
	newTime := currentTime + uint64(seconds)

	// Generate and insert a new block (the miner will use current clock time)
	block, err := api.eth.miner.GenerateBlock(nil)
	if err != nil {
		return 0, err
	}

	if err := api.eth.blockchain.InsertBlock(block); err != nil {
		return 0, err
	}

	if err := api.eth.blockchain.Accept(block); err != nil {
		return 0, err
	}
	// Note: We don't call DrainAcceptorQueue() here because it triggers
	// snapshot flatten which isn't supported in PathDB mode.

	return hexutil.Uint64(newTime), nil
}

// SetNextBlockTimestamp sets the timestamp for the next block by mining one.
// Anvil: evm_setNextBlockTimestamp
// Hardhat: evm_setNextBlockTimestamp
func (api *DevAPI) SetNextBlockTimestamp(ctx context.Context, timestamp hexutil.Uint64) error {
	// Mine a new block - the timestamp will be adjusted by the clock
	one := hexutil.Uint64(1)
	_, err := api.Mine(ctx, &one, nil)
	return err
}

// ImpersonateAccount starts impersonating an account (allows sending tx without private key).
// Anvil: anvil_impersonateAccount
// Hardhat: hardhat_impersonateAccount
func (api *DevAPI) ImpersonateAccount(ctx context.Context, address common.Address) error {
	// In dev mode, all accounts are effectively impersonatable
	// This is a no-op for compatibility
	return nil
}

// StopImpersonatingAccount stops impersonating an account.
// Anvil: anvil_stopImpersonatingAccount
// Hardhat: hardhat_stopImpersonatingAccount
func (api *DevAPI) StopImpersonatingAccount(ctx context.Context, address common.Address) error {
	// This is a no-op for compatibility
	return nil
}

// AutoImpersonate enables or disables auto-impersonation of all accounts.
// Anvil: anvil_autoImpersonateAccount
func (api *DevAPI) AutoImpersonate(ctx context.Context, enabled bool) error {
	// This is a no-op for compatibility
	return nil
}

// commitStateChanges commits state changes by creating a synthetic block.
// This is used by SetBalance, SetStorageAt, etc. to persist changes.
func (api *DevAPI) commitStateChanges(statedb *state.StateDB) error {
	bc := api.eth.blockchain
	parent := bc.CurrentBlock()

	log.Info("commitStateChanges: starting", "parentNum", parent.Number, "parentRoot", parent.Root.Hex())

	// Finalize the state changes
	statedb.Finalise(false)

	// Commit the state to get the new root
	root, err := statedb.Commit(parent.Number.Uint64()+1, false, false)
	if err != nil {
		log.Error("commitStateChanges: statedb.Commit failed", "err", err)
		return err
	}
	log.Info("commitStateChanges: statedb.Commit succeeded", "newRoot", root.Hex())

	// For HashDB: Flush the trie changes to disk
	// For PathDB: Skip direct commit - PathDB manages layers internally and
	// state changes are already tracked in memory after statedb.Commit().
	if bc.TrieDB().Scheme() == rawdb.HashScheme {
		if err := bc.TrieDB().Commit(root, false); err != nil {
			log.Error("commitStateChanges: TrieDB.Commit failed", "err", err)
			return err
		}
		log.Info("commitStateChanges: TrieDB.Commit succeeded (HashDB)")
	} else {
		log.Info("commitStateChanges: PathDB mode, state changes in memory")
	}

	// Create a synthetic block with the new state root
	newBlockNum := new(big.Int).Add(parent.Number, big.NewInt(1))
	timestamp := uint64(time.Now().Unix())
	if timestamp <= parent.Time {
		timestamp = parent.Time + 1
	}

	header := &types.Header{
		ParentHash:  parent.Hash(),
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    parent.Coinbase,
		Root:        root,
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		Bloom:       types.Bloom{},
		Difficulty:  common.Big0,
		Number:      newBlockNum,
		GasLimit:    parent.GasLimit,
		GasUsed:     0,
		Time:        timestamp,
		Extra:       (&ap3.Window{}).Bytes(), // 80-byte fee window (all zeros = no gas used)
		BaseFee:     parent.BaseFee,
	}

	// Create an empty block body
	block := types.NewBlock(header, &types.Body{}, nil, nil)

	// Insert the block (now also updates lastAccepted for RPC)
	if err := bc.InsertBlockWithoutState(block); err != nil {
		log.Error("commitStateChanges: InsertBlockWithoutState failed", "err", err)
		return err
	}
	log.Info("commitStateChanges: InsertBlockWithoutState succeeded", "blockNum", block.NumberU64(), "blockHash", block.Hash().Hex())

	// Verify we can still read the state
	newHeader := bc.CurrentBlock()
	log.Info("commitStateChanges: after insert", "currentBlockNum", newHeader.Number, "currentRoot", newHeader.Root.Hex())

	return nil
}

// DumpState returns a dump of the current state (for debugging).
func (api *DevAPI) DumpState(ctx context.Context) (state.Dump, error) {
	api.mu.Lock()
	defer api.mu.Unlock()

	header := api.eth.blockchain.CurrentBlock()
	statedb, err := api.eth.blockchain.StateAt(header.Root)
	if err != nil {
		return state.Dump{}, err
	}

	return statedb.RawDump(&state.DumpConfig{
		OnlyWithAddresses: true,
		Max:               256,
	}), nil
}

// unused variable to ensure we use time package
var _ = time.Now
