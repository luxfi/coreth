// (c) 2020-2021, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package core

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/luxfi/geth/consensus/dummy"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/params"
	"github.com/luxfi/geth/plugin/evm/upgrade/ap4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var TestCallbacks = dummy.ConsensusCallbacks{
	OnExtraStateChange: func(block *types.Block, _ *types.Header, sdb *state.StateDB) (*big.Int, *big.Int, error) {
		sdb.SetBalanceMultiCoin(common.HexToAddress("0xdeadbeef"), common.HexToHash("0xdeadbeef"), big.NewInt(block.Number().Int64()))
		return nil, nil, nil
	},
	OnFinalizeAndAssemble: func(
		header *types.Header,
		_ *types.Header,
		sdb *state.StateDB,
		_ []*types.Transaction,
	) ([]byte, *big.Int, *big.Int, error) {
		sdb.SetBalanceMultiCoin(common.HexToAddress("0xdeadbeef"), common.HexToHash("0xdeadbeef"), big.NewInt(header.Number.Int64()))
		return nil, nil, nil, nil
	},
}

type ChainTest struct {
	Name     string
	testFunc func(
		t *testing.T,
		create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error),
	)
}

var tests = []ChainTest{
	{
		"InsertChainAcceptSingleBlock",
		TestInsertChainAcceptSingleBlock,
	},
	{
		"InsertForkedChain",
		TestInsertLongForkedChain,
	},
	{
		"AcceptNonCanonicalBlock",
		TestAcceptNonCanonicalBlock,
	},
	{
		"SetPreferenceRewind",
		TestSetPreferenceRewind,
	},
	{
		"BuildOnVariousStages",
		TestBuildOnVariousStages,
	},
	{
		"EmptyBlocks",
		TestEmptyBlocks,
	},
	{
		"ReorgReInsert",
		TestReorgReInsert,
	},
	{
		"AcceptBlockIdenticalStateRoot",
		TestAcceptBlockIdenticalStateRoot,
	},
	{
		"ReprocessAcceptBlockIdenticalStateRoot",
		TestReprocessAcceptBlockIdenticalStateRoot,
	},
	{
		"GenerateChainInvalidBlockFee",
		TestGenerateChainInvalidBlockFee,
	},
	{
		"InsertChainInvalidBlockFee",
		TestInsertChainInvalidBlockFee,
	},
	{
		"InsertChainValidBlockFee",
		TestInsertChainValidBlockFee,
	},
}

func copyMemDB(db ethdb.Database) (ethdb.Database, error) {
	newDB := rawdb.NewMemoryDatabase()
	iter := db.NewIterator(nil, nil)
	defer iter.Release()
	for iter.Next() {
		if err := newDB.Put(iter.Key(), iter.Value()); err != nil {
			return nil, err
		}
	}

	return newDB, nil
}

// checkBlockChainState creates a new BlockChain instance and checks that exporting each block from
// genesis to last accepted from the original instance yields the same last accepted block and state
// root.
// Additionally, create another BlockChain instance from [originalDB] to ensure that BlockChain is
// persisted correctly through a restart.
func checkBlockChainState(
	t *testing.T,
	bc *BlockChain,
	gspec *Genesis,
	originalDB ethdb.Database,
	create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error),
	checkState func(sdb *state.StateDB) error,
) (*BlockChain, *BlockChain, *BlockChain) {
	var (
		lastAcceptedBlock = bc.LastConsensusAcceptedBlock()
		newDB             = rawdb.NewMemoryDatabase()
	)

	acceptedState, err := bc.StateAt(lastAcceptedBlock.Root())
	if err != nil {
		t.Fatal(err)
	}
	if err := checkState(acceptedState); err != nil {
		t.Fatalf("Check state failed for original blockchain due to: %s", err)
	}

	newBlockChain, err := create(newDB, gspec, common.Hash{})
	if err != nil {
		t.Fatalf("Failed to create new blockchain instance: %s", err)
	}
	defer newBlockChain.Stop()

	for i := uint64(1); i <= lastAcceptedBlock.NumberU64(); i++ {
		block := bc.GetBlockByNumber(i)
		if block == nil {
			t.Fatalf("Failed to retrieve block by number %d from original chain", i)
		}
		if err := newBlockChain.InsertBlock(block); err != nil {
			t.Fatalf("Failed to insert block %s:%d due to %s", block.Hash().Hex(), block.NumberU64(), err)
		}
		if err := newBlockChain.Accept(block); err != nil {
			t.Fatalf("Failed to accept block %s:%d due to %s", block.Hash().Hex(), block.NumberU64(), err)
		}
	}
	newBlockChain.DrainAcceptorQueue()

	newLastAcceptedBlock := newBlockChain.LastConsensusAcceptedBlock()
	if newLastAcceptedBlock.Hash() != lastAcceptedBlock.Hash() {
		t.Fatalf("Expected new blockchain to have last accepted block %s:%d, but found %s:%d", lastAcceptedBlock.Hash().Hex(), lastAcceptedBlock.NumberU64(), newLastAcceptedBlock.Hash().Hex(), newLastAcceptedBlock.NumberU64())
	}

	// Check that the state of [newBlockChain] passes the check
	acceptedState, err = newBlockChain.StateAt(lastAcceptedBlock.Root())
	if err != nil {
		t.Fatal(err)
	}
	if err := checkState(acceptedState); err != nil {
		t.Fatalf("Check state failed for newly generated blockchain due to: %s", err)
	}

	// Copy the database over to prevent any issues when re-using [originalDB] after this call.
	originalDB, err = copyMemDB(originalDB)
	if err != nil {
		t.Fatal(err)
	}
	restartedChain, err := create(originalDB, gspec, lastAcceptedBlock.Hash())
	if err != nil {
		t.Fatal(err)
	}
	defer restartedChain.Stop()
	if currentBlock := restartedChain.CurrentBlock(); currentBlock.Hash() != lastAcceptedBlock.Hash() {
		t.Fatalf("Expected restarted chain to have current block %s:%d, but found %s:%d", lastAcceptedBlock.Hash().Hex(), lastAcceptedBlock.NumberU64(), currentBlock.Hash().Hex(), currentBlock.Number.Uint64())
	}
	if restartedLastAcceptedBlock := restartedChain.LastConsensusAcceptedBlock(); restartedLastAcceptedBlock.Hash() != lastAcceptedBlock.Hash() {
		t.Fatalf("Expected restarted chain to have current block %s:%d, but found %s:%d", lastAcceptedBlock.Hash().Hex(), lastAcceptedBlock.NumberU64(), restartedLastAcceptedBlock.Hash().Hex(), restartedLastAcceptedBlock.NumberU64())
	}

	// Check that the state of [restartedChain] passes the check
	acceptedState, err = restartedChain.StateAt(lastAcceptedBlock.Root())
	if err != nil {
		t.Fatal(err)
	}
	if err := checkState(acceptedState); err != nil {
		t.Fatalf("Check state failed for restarted blockchain due to: %s", err)
	}

	return bc, newBlockChain, restartedChain
}

func TestInsertChainAcceptSingleBlock(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := big.NewInt(1000000)
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc:  types.GenesisAlloc{addr1: {Balance: genesisBalance}},
	}
	blockchain, err := create(chainDB, gspec, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer blockchain.Stop()

	// This call generates a chain of 3 blocks.
	signer := types.HomesteadSigner{}
	_, chain, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, 3, 10, func(i int, gen *BlockGen) {
		tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(10000), params.TxGas, nil, nil), signer, key1)
		gen.AddTx(tx)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert three blocks into the chain and accept only the first block.
	if _, err := blockchain.InsertChain(chain); err != nil {
		t.Fatal(err)
	}
	if err := blockchain.Accept(chain[0]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	// check the state of the last accepted block
	checkState := func(sdb *state.StateDB) error {
		nonce := sdb.GetNonce(addr1)
		if nonce != 1 {
			return fmt.Errorf("expected nonce addr1: 1, found nonce: %d", nonce)
		}
		transferredFunds := uint256.MustFromBig(big.NewInt(10000))
		balance1 := sdb.GetBalance(addr1)
		genesisBalance := uint256.MustFromBig(genesisBalance)
		expectedBalance1 := new(uint256.Int).Sub(genesisBalance, transferredFunds)
		if balance1.Cmp(expectedBalance1) != 0 {
			return fmt.Errorf("expected addr1 balance: %d, found balance: %d", expectedBalance1, balance1)
		}

		balance2 := sdb.GetBalance(addr2)
		expectedBalance2 := transferredFunds
		if balance2.Cmp(expectedBalance2) != 0 {
			return fmt.Errorf("expected addr2 balance: %d, found balance: %d", expectedBalance2, balance2)
		}

		nonce = sdb.GetNonce(addr2)
		if nonce != 0 {
			return fmt.Errorf("expected addr2 nonce: 0, found nonce: %d", nonce)
		}
		return nil
	}

	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkState)
}

func TestInsertLongForkedChain(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := big.NewInt(1000000000)
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc:  types.GenesisAlloc{addr1: {Balance: genesisBalance}},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer blockchain.Stop()

	numBlocks := 129
	signer := types.HomesteadSigner{}
	_, chain1, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, numBlocks, 10, func(i int, gen *BlockGen) {
		// Generate a transaction to create a unique block
		tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(10000), params.TxGas, nil, nil), signer, key1)
		gen.AddTx(tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	// Generate the forked chain to be longer than the original chain to check for a regression where
	// a longer chain can trigger a reorg.
	_, chain2, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, numBlocks+1, 10, func(i int, gen *BlockGen) {
		// Generate a transaction with a different amount to ensure [chain2] is different than [chain1].
		tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(5000), params.TxGas, nil, nil), signer, key1)
		gen.AddTx(tx)
	})
	if err != nil {
		t.Fatal(err)
	}

	if blockchain.snaps != nil {
		if want, got := 1, blockchain.snaps.NumBlockLayers(); got != want {
			t.Fatalf("incorrect snapshot layer count; got %d, want %d", got, want)
		}
	}

	// Insert both chains.
	if _, err := blockchain.InsertChain(chain1); err != nil {
		t.Fatal(err)
	}

	if blockchain.snaps != nil {
		if want, got := 1+len(chain1), blockchain.snaps.NumBlockLayers(); got != want {
			t.Fatalf("incorrect snapshot layer count; got %d, want %d", got, want)
		}
	}

	if _, err := blockchain.InsertChain(chain2); err != nil {
		t.Fatal(err)
	}

	if blockchain.snaps != nil {
		if want, got := 1+len(chain1)+len(chain2), blockchain.snaps.NumBlockLayers(); got != want {
			t.Fatalf("incorrect snapshot layer count; got %d, want %d", got, want)
		}
	}

	currentBlock := blockchain.CurrentBlock()
	expectedCurrentBlock := chain1[len(chain1)-1]
	if currentBlock.Hash() != expectedCurrentBlock.Hash() {
		t.Fatalf("Expected current block to be %s:%d, but found %s%d", expectedCurrentBlock.Hash().Hex(), expectedCurrentBlock.NumberU64(), currentBlock.Hash().Hex(), currentBlock.Number.Uint64())
	}

	if err := blockchain.ValidateCanonicalChain(); err != nil {
		t.Fatal(err)
	}

	// Accept the first block in [chain1], reject all blocks in [chain2] to
	// mimic the order that the consensus engine will call Accept/Reject in
	// and then Accept the rest of the blocks in [chain1].
	if err := blockchain.Accept(chain1[0]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	if blockchain.snaps != nil {
		// Snap layer count should be 1 fewer
		if want, got := len(chain1)+len(chain2), blockchain.snaps.NumBlockLayers(); got != want {
			t.Fatalf("incorrect snapshot layer count; got %d, want %d", got, want)
		}
	}

	for i := 0; i < len(chain2); i++ {
		if err := blockchain.Reject(chain2[i]); err != nil {
			t.Fatal(err)
		}

		if blockchain.snaps != nil {
			// Snap layer count should decrease by 1 per Reject
			if want, got := len(chain1)+len(chain2)-i-1, blockchain.snaps.NumBlockLayers(); got != want {
				t.Fatalf("incorrect snapshot layer count; got %d, want %d", got, want)
			}
		}
	}

	if blockchain.snaps != nil {
		if want, got := len(chain1), blockchain.snaps.NumBlockLayers(); got != want {
			t.Fatalf("incorrect snapshot layer count; got %d, want %d", got, want)
		}
	}

	for i := 1; i < len(chain1); i++ {
		if err := blockchain.Accept(chain1[i]); err != nil {
			t.Fatal(err)
		}
		blockchain.DrainAcceptorQueue()

		if blockchain.snaps != nil {
			// Snap layer count should decrease by 1 per Accept
			if want, got := len(chain1)-i, blockchain.snaps.NumBlockLayers(); got != want {
				t.Fatalf("incorrect snapshot layer count; got %d, want %d", got, want)
			}
		}
	}

	lastAcceptedBlock := blockchain.LastConsensusAcceptedBlock()
	expectedLastAcceptedBlock := chain1[len(chain1)-1]
	if lastAcceptedBlock.Hash() != expectedLastAcceptedBlock.Hash() {
		t.Fatalf("Expected last accepted block to be %s:%d, but found %s%d", expectedLastAcceptedBlock.Hash().Hex(), expectedLastAcceptedBlock.NumberU64(), lastAcceptedBlock.Hash().Hex(), lastAcceptedBlock.NumberU64())
	}
	if err := blockchain.ValidateCanonicalChain(); err != nil {
		t.Fatal(err)
	}

	// check the state of the last accepted block
	checkState := func(sdb *state.StateDB) error {
		nonce1 := sdb.GetNonce(addr1)
		if nonce1 != 129 {
			return fmt.Errorf("expected addr1 nonce: 129, found nonce %d", nonce1)
		}
		balance1 := sdb.GetBalance(addr1)
		transferredFunds := uint256.NewInt(129 * 10_000)
		genesisBalance := uint256.MustFromBig(genesisBalance)
		expectedBalance := new(uint256.Int).Sub(genesisBalance, transferredFunds)
		if balance1.Cmp(expectedBalance) != 0 {
			return fmt.Errorf("expected addr1 balance: %d, found balance: %d", expectedBalance, balance1)
		}
		nonce2 := sdb.GetNonce(addr2)
		if nonce2 != 0 {
			return fmt.Errorf("expected addr2 nonce: 0, found nonce: %d", nonce2)
		}
		balance2 := sdb.GetBalance(addr2)
		if balance2.Cmp(transferredFunds) != 0 {
			return fmt.Errorf("expected addr2 balance: %d, found balance: %d", transferredFunds, balance2)
		}
		return nil
	}

	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkState)
}

func TestAcceptNonCanonicalBlock(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		// We use two separate databases since GenerateChain commits the state roots to its underlying
		// database.
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := big.NewInt(1000000000)
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc:  types.GenesisAlloc{addr1: {Balance: genesisBalance}},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer blockchain.Stop()

	numBlocks := 3
	signer := types.HomesteadSigner{}
	_, chain1, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, numBlocks, 10, func(i int, gen *BlockGen) {
		// Generate a transaction to create a unique block
		tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(10000), params.TxGas, nil, nil), signer, key1)
		gen.AddTx(tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	_, chain2, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, numBlocks, 10, func(i int, gen *BlockGen) {
		// Generate a transaction with a different amount to create a chain of blocks different from [chain1]
		tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(5000), params.TxGas, nil, nil), signer, key1)
		gen.AddTx(tx)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert three blocks into the chain and accept only the first.
	if _, err := blockchain.InsertChain(chain1); err != nil {
		t.Fatal(err)
	}
	if _, err := blockchain.InsertChain(chain2); err != nil {
		t.Fatal(err)
	}

	currentBlock := blockchain.CurrentBlock()
	expectedCurrentBlock := chain1[len(chain1)-1]
	if currentBlock.Hash() != expectedCurrentBlock.Hash() {
		t.Fatalf("Expected current block to be %s:%d, but found %s%d", expectedCurrentBlock.Hash().Hex(), expectedCurrentBlock.NumberU64(), currentBlock.Hash().Hex(), currentBlock.Number.Uint64())
	}

	if err := blockchain.ValidateCanonicalChain(); err != nil {
		t.Fatal(err)
	}

	// Accept the first block in [chain2], reject all blocks in [chain1] to
	// mimic the order that the consensus engine will call Accept/Reject in.
	if err := blockchain.Accept(chain2[0]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	for i := 0; i < len(chain1); i++ {
		if err := blockchain.Reject(chain1[i]); err != nil {
			t.Fatal(err)
		}
		require.False(t, blockchain.HasBlock(chain1[i].Hash(), chain1[i].NumberU64()))
	}

	lastAcceptedBlock := blockchain.LastConsensusAcceptedBlock()
	expectedLastAcceptedBlock := chain2[0]
	if lastAcceptedBlock.Hash() != expectedLastAcceptedBlock.Hash() {
		t.Fatalf("Expected last accepted block to be %s:%d, but found %s%d", expectedLastAcceptedBlock.Hash().Hex(), expectedLastAcceptedBlock.NumberU64(), lastAcceptedBlock.Hash().Hex(), lastAcceptedBlock.NumberU64())
	}
	if err := blockchain.ValidateCanonicalChain(); err != nil {
		t.Fatal(err)
	}

	// check the state of the last accepted block
	checkState := func(sdb *state.StateDB) error {
		nonce1 := sdb.GetNonce(addr1)
		if nonce1 != 1 {
			return fmt.Errorf("expected addr1 nonce: 1, found nonce: %d", nonce1)
		}
		balance1 := sdb.GetBalance(addr1)
		transferredFunds := uint256.NewInt(5000)
		genesisBalance := uint256.MustFromBig(genesisBalance)
		expectedBalance := new(uint256.Int).Sub(genesisBalance, transferredFunds)
		if balance1.Cmp(expectedBalance) != 0 {
			return fmt.Errorf("expected balance1: %d, found balance: %d", expectedBalance, balance1)
		}
		nonce2 := sdb.GetNonce(addr2)
		if nonce2 != 0 {
			return fmt.Errorf("expected addr2 nonce: 0, found nonce %d", nonce2)
		}
		balance2 := sdb.GetBalance(addr2)
		if balance2.Cmp(transferredFunds) != 0 {
			return fmt.Errorf("expected balance2: %d, found %d", transferredFunds, balance2)
		}
		return nil
	}

	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkState)
}

func TestSetPreferenceRewind(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := big.NewInt(1000000000)
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc:  types.GenesisAlloc{addr1: {Balance: genesisBalance}},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer blockchain.Stop()

	numBlocks := 3
	signer := types.HomesteadSigner{}
	_, chain, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, numBlocks, 10, func(i int, gen *BlockGen) {
		// Generate a transaction to create a unique block
		tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(10000), params.TxGas, nil, nil), signer, key1)
		gen.AddTx(tx)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert three blocks into the chain and accept only the first.
	if _, err := blockchain.InsertChain(chain); err != nil {
		t.Fatal(err)
	}

	currentBlock := blockchain.CurrentBlock()
	expectedCurrentBlock := chain[len(chain)-1]
	if currentBlock.Hash() != expectedCurrentBlock.Hash() {
		t.Fatalf("Expected current block to be %s:%d, but found %s%d", expectedCurrentBlock.Hash().Hex(), expectedCurrentBlock.NumberU64(), currentBlock.Hash().Hex(), currentBlock.Number.Uint64())
	}

	if err := blockchain.ValidateCanonicalChain(); err != nil {
		t.Fatal(err)
	}

	// SetPreference to an ancestor of the currently preferred block. Test that this unlikely, but possible behavior
	// is handled correctly.
	if err := blockchain.SetPreference(chain[0]); err != nil {
		t.Fatal(err)
	}

	currentBlock = blockchain.CurrentBlock()
	expectedCurrentBlock = chain[0]
	if currentBlock.Hash() != expectedCurrentBlock.Hash() {
		t.Fatalf("Expected current block to be %s:%d, but found %s%d", expectedCurrentBlock.Hash().Hex(), expectedCurrentBlock.NumberU64(), currentBlock.Hash().Hex(), currentBlock.Number.Uint64())
	}

	lastAcceptedBlock := blockchain.LastConsensusAcceptedBlock()
	expectedLastAcceptedBlock := blockchain.Genesis()
	if lastAcceptedBlock.Hash() != expectedLastAcceptedBlock.Hash() {
		t.Fatalf("Expected last accepted block to be %s:%d, but found %s%d", expectedLastAcceptedBlock.Hash().Hex(), expectedLastAcceptedBlock.NumberU64(), lastAcceptedBlock.Hash().Hex(), lastAcceptedBlock.NumberU64())
	}
	if err := blockchain.ValidateCanonicalChain(); err != nil {
		t.Fatal(err)
	}
	// check the state of the last accepted block
	checkGenesisState := func(sdb *state.StateDB) error {
		nonce1 := sdb.GetNonce(addr1)
		if nonce1 != 0 {
			return fmt.Errorf("expected addr1 nonce: 0, found nonce: %d", nonce1)
		}
		balance1 := sdb.GetBalance(addr1)
		genesisBalance := uint256.MustFromBig(genesisBalance)
		if balance1.Cmp(genesisBalance) != 0 {
			return fmt.Errorf("expected addr1 balance: %d, found balance: %d", genesisBalance, balance1)
		}
		nonce2 := sdb.GetNonce(addr2)
		if nonce2 != 0 {
			return fmt.Errorf("expected addr2 nonce: 0, found nonce: %d", nonce2)
		}
		balance2 := sdb.GetBalance(addr2)
		if balance2.Cmp(common.U2560) != 0 {
			return fmt.Errorf("expected addr2 balance: 0, found balance %d", balance2)
		}
		return nil
	}
	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkGenesisState)

	if err := blockchain.Accept(chain[0]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	lastAcceptedBlock = blockchain.LastConsensusAcceptedBlock()
	expectedLastAcceptedBlock = chain[0]
	if lastAcceptedBlock.Hash() != expectedLastAcceptedBlock.Hash() {
		t.Fatalf("Expected last accepted block to be %s:%d, but found %s%d", expectedLastAcceptedBlock.Hash().Hex(), expectedLastAcceptedBlock.NumberU64(), lastAcceptedBlock.Hash().Hex(), lastAcceptedBlock.NumberU64())
	}
	if err := blockchain.ValidateCanonicalChain(); err != nil {
		t.Fatal(err)
	}
	checkUpdatedState := func(sdb *state.StateDB) error {
		nonce := sdb.GetNonce(addr1)
		if nonce != 1 {
			return fmt.Errorf("expected addr1 nonce: 1, found nonce: %d", nonce)
		}
		transferredFunds := uint256.NewInt(10000)
		balance1 := sdb.GetBalance(addr1)
		genesisBalance := uint256.MustFromBig(genesisBalance)
		expectedBalance1 := new(uint256.Int).Sub(genesisBalance, transferredFunds)
		if balance1.Cmp(expectedBalance1) != 0 {
			return fmt.Errorf("expected addr1 balance: %d, found balance %d", expectedBalance1, balance1)
		}

		balance2 := sdb.GetBalance(addr2)
		expectedBalance2 := transferredFunds
		if balance2.Cmp(expectedBalance2) != 0 {
			return fmt.Errorf("expected addr2 balance: %d, found balance: %d", expectedBalance2, balance2)
		}

		nonce = sdb.GetNonce(addr2)
		if nonce != 0 {
			return fmt.Errorf("expected addr2 nonce: 0, found nonce: %d", nonce)
		}
		return nil
	}
	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkUpdatedState)
}

func TestBuildOnVariousStages(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		key3, _ = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		addr3   = crypto.PubkeyToAddress(key3.PublicKey)
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := big.NewInt(1000000)
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc: types.GenesisAlloc{
			addr1: {Balance: genesisBalance},
			addr3: {Balance: genesisBalance},
		},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer blockchain.Stop()

	// This call generates a chain of 3 blocks.
	signer := types.HomesteadSigner{}
	genDB, chain1, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, 20, 10, func(i int, gen *BlockGen) {
		// Send all funds back and forth between the two accounts
		if i%2 == 0 {
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, genesisBalance, params.TxGas, nil, nil), signer, key1)
			gen.AddTx(tx)
		} else {
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr2), addr1, genesisBalance, params.TxGas, nil, nil), signer, key2)
			gen.AddTx(tx)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	// Build second chain forked off of the 10th block in [chain1]
	chain2, _, err := GenerateChain(gspec.Config, chain1[9], blockchain.engine, genDB, 10, 10, func(i int, gen *BlockGen) {
		// Send all funds back and forth between the two accounts
		if i%2 == 0 {
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr3), addr2, genesisBalance, params.TxGas, nil, nil), signer, key3)
			gen.AddTx(tx)
		} else {
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr2), addr3, genesisBalance, params.TxGas, nil, nil), signer, key2)
			gen.AddTx(tx)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	// Build third chain forked off of the 5th block in [chain1].
	// The parent of this chain will be accepted before this fork
	// is inserted.
	chain3, _, err := GenerateChain(gspec.Config, chain1[4], blockchain.engine, genDB, 10, 10, func(i int, gen *BlockGen) {
		// Send all funds back and forth between accounts 2 and 3.
		if i%2 == 0 {
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr2), addr3, genesisBalance, params.TxGas, nil, nil), signer, key2)
			gen.AddTx(tx)
		} else {
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr3), addr2, genesisBalance, params.TxGas, nil, nil), signer, key3)
			gen.AddTx(tx)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert first 10 blocks from [chain1]
	if _, err := blockchain.InsertChain(chain1); err != nil {
		t.Fatal(err)
	}
	// Accept the first 5 blocks
	for _, block := range chain1[0:5] {
		if err := blockchain.Accept(block); err != nil {
			t.Fatal(err)
		}
	}
	blockchain.DrainAcceptorQueue()

	// Insert the forked chain [chain2] which starts at the 10th
	// block in [chain1] ie. a block that is still in processing.
	if _, err := blockchain.InsertChain(chain2); err != nil {
		t.Fatal(err)
	}
	// Insert another forked chain starting at the last accepted
	// block from [chain1].
	if _, err := blockchain.InsertChain(chain3); err != nil {
		t.Fatal(err)
	}
	// Accept the next block in [chain1] and then reject all
	// of the blocks in [chain3], which would then be rejected.
	if err := blockchain.Accept(chain1[5]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()
	for _, block := range chain3 {
		if err := blockchain.Reject(block); err != nil {
			t.Fatal(err)
		}
	}
	// Accept the rest of the blocks in [chain1]
	for _, block := range chain1[6:10] {
		if err := blockchain.Accept(block); err != nil {
			t.Fatal(err)
		}
	}
	blockchain.DrainAcceptorQueue()

	// Accept the first block in [chain2] and reject the
	// subsequent blocks in [chain1] which would then be rejected.
	if err := blockchain.Accept(chain2[0]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	for _, block := range chain1[10:] {
		if err := blockchain.Reject(block); err != nil {
			t.Fatal(err)
		}
	}

	// check the state of the last accepted block
	checkState := func(sdb *state.StateDB) error {
		nonce := sdb.GetNonce(addr1)
		if nonce != 5 {
			return fmt.Errorf("expected nonce addr1: 5, found nonce: %d", nonce)
		}
		balance1 := sdb.GetBalance(addr1)
		genesisBalance := uint256.MustFromBig(genesisBalance)
		expectedBalance1 := genesisBalance
		if balance1.Cmp(expectedBalance1) != 0 {
			return fmt.Errorf("expected addr1 balance: %d, found balance: %d", expectedBalance1, balance1)
		}

		balance2 := sdb.GetBalance(addr2)
		expectedBalance2 := genesisBalance
		if balance2.Cmp(expectedBalance2) != 0 {
			return fmt.Errorf("expected addr2 balance: %d, found balance: %d", expectedBalance2, balance2)
		}

		nonce = sdb.GetNonce(addr2)
		if nonce != 5 {
			return fmt.Errorf("expected addr2 nonce: 5, found nonce: %d", nonce)
		}

		balance3 := sdb.GetBalance(addr3)
		expectedBalance3 := common.U2560
		if balance3.Cmp(expectedBalance3) != 0 {
			return fmt.Errorf("expected addr3 balance: %d, found balance: %d", expectedBalance3, balance3)
		}

		nonce = sdb.GetNonce(addr3)
		if nonce != 1 {
			return fmt.Errorf("expected addr3 nonce: 1, found nonce: %d", nonce)
		}
		return nil
	}

	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkState)
}

func TestEmptyBlocks(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	chainDB := rawdb.NewMemoryDatabase()

	// Ensure that key1 has some funds in the genesis block.
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc:  types.GenesisAlloc{},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer blockchain.Stop()

	_, chain, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, 3, 10, func(i int, gen *BlockGen) {})
	if err != nil {
		t.Fatal(err)
	}

	// Insert three blocks into the chain and accept only the first block.
	if _, err := blockchain.InsertChain(chain); err != nil {
		t.Fatal(err)
	}
	for _, block := range chain {
		if err := blockchain.Accept(block); err != nil {
			t.Fatal(err)
		}
	}
	blockchain.DrainAcceptorQueue()

	// Nothing to assert about the state
	checkState := func(sdb *state.StateDB) error {
		return nil
	}

	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkState)
}

func TestReorgReInsert(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := big.NewInt(1000000000)
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc:  types.GenesisAlloc{addr1: {Balance: genesisBalance}},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer blockchain.Stop()

	signer := types.HomesteadSigner{}
	numBlocks := 3
	_, chain, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, numBlocks, 10, func(i int, gen *BlockGen) {
		// Generate a transaction to create a unique block
		tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(10000), params.TxGas, nil, nil), signer, key1)
		gen.AddTx(tx)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert and accept first block
	if err := blockchain.InsertBlock(chain[0]); err != nil {
		t.Fatal(err)
	}
	if err := blockchain.Accept(chain[0]); err != nil {
		t.Fatal(err)
	}

	// Insert block and then set preference back (rewind) to last accepted blck
	if err := blockchain.InsertBlock(chain[1]); err != nil {
		t.Fatal(err)
	}
	if err := blockchain.SetPreference(chain[0]); err != nil {
		t.Fatal(err)
	}

	// Re-insert and accept block
	if err := blockchain.InsertBlock(chain[1]); err != nil {
		t.Fatal(err)
	}
	if err := blockchain.Accept(chain[1]); err != nil {
		t.Fatal(err)
	}

	// Build on top of the re-inserted block and accept
	if err := blockchain.InsertBlock(chain[2]); err != nil {
		t.Fatal(err)
	}
	if err := blockchain.Accept(chain[2]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	// Nothing to assert about the state
	checkState := func(sdb *state.StateDB) error {
		nonce1 := sdb.GetNonce(addr1)
		if nonce1 != 3 {
			return fmt.Errorf("expected addr1 nonce: 3, found nonce: %d", nonce1)
		}
		balance1 := sdb.GetBalance(addr1)
		transferredFunds := uint256.NewInt(30000)
		genesisBalance := uint256.MustFromBig(genesisBalance)
		expectedBalance := new(uint256.Int).Sub(genesisBalance, transferredFunds)
		if balance1.Cmp(expectedBalance) != 0 {
			return fmt.Errorf("expected balance1: %d, found balance: %d", expectedBalance, balance1)
		}
		nonce2 := sdb.GetNonce(addr2)
		if nonce2 != 0 {
			return fmt.Errorf("expected addr2 nonce: 0, found nonce %d", nonce2)
		}
		balance2 := sdb.GetBalance(addr2)
		if balance2.Cmp(transferredFunds) != 0 {
			return fmt.Errorf("expected balance2: %d, found %d", transferredFunds, balance2)
		}
		return nil
	}

	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkState)
}

// Insert two different chains that result in the identical state root.
// Once we accept one of the chains, we insert and accept A3 on top of the shared
// state root
//
//	  G   (genesis)
//	 / \
//	A1  B1
//	|   |
//	A2  B2 (A2 and B2 represent two different paths to the identical state trie)
//	|
//	A3
//
//nolint:goimports
func TestAcceptBlockIdenticalStateRoot(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := big.NewInt(1000000000)
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc:  types.GenesisAlloc{addr1: {Balance: genesisBalance}},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer blockchain.Stop()

	signer := types.HomesteadSigner{}
	_, chain1, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, 3, 10, func(i int, gen *BlockGen) {
		if i < 2 {
			// Send half the funds from addr1 to addr2 in one transaction per each of the two blocks in [chain1]
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(500000000), params.TxGas, nil, nil), signer, key1)
			gen.AddTx(tx)
		}
		// Allow the third block to be empty.
	})
	if err != nil {
		t.Fatal(err)
	}
	_, chain2, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, 2, 10, func(i int, gen *BlockGen) {
		// Send 1/4 of the funds from addr1 to addr2 in tx1 and 3/4 of the funds in tx2. This will produce the identical state
		// root in the second block of [chain2] as is present in the second block of [chain1].
		if i == 0 {
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(250000000), params.TxGas, nil, nil), signer, key1)
			gen.AddTx(tx)
		} else {
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(750000000), params.TxGas, nil, nil), signer, key1)
			gen.AddTx(tx)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assert that the block root of the second block in both chains is identical
	if chain1[1].Root() != chain2[1].Root() {
		t.Fatalf("Expected the latter block in both chain1 and chain2 to have identical state root, but found %s and %s", chain1[1].Root(), chain2[1].Root())
	}

	// Insert first two blocks of [chain1] and both blocks in [chain2]
	// This leaves us one additional block to insert on top of [chain1]
	// after testing that the state roots are handled correctly.
	if _, err := blockchain.InsertChain(chain1[:2]); err != nil {
		t.Fatal(err)
	}
	if _, err := blockchain.InsertChain(chain2); err != nil {
		t.Fatal(err)
	}

	currentBlock := blockchain.CurrentBlock()
	expectedCurrentBlock := chain1[1]
	if currentBlock.Hash() != expectedCurrentBlock.Hash() {
		t.Fatalf("Expected current block to be %s:%d, but found %s%d", expectedCurrentBlock.Hash().Hex(), expectedCurrentBlock.NumberU64(), currentBlock.Hash().Hex(), currentBlock.Number.Uint64())
	}

	// Accept the first block in [chain1] and reject all of [chain2]
	if err := blockchain.Accept(chain1[0]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	for _, block := range chain2 {
		if err := blockchain.Reject(block); err != nil {
			t.Fatal(err)
		}
	}

	// Accept the last two blocks in [chain1]. This is a regression test to ensure
	// that we do not discard a snapshot difflayer that is still in use by a
	// processing block, when a different block with the same root is rejected.
	if err := blockchain.Accept(chain1[1]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	lastAcceptedBlock := blockchain.LastConsensusAcceptedBlock()
	expectedLastAcceptedBlock := chain1[1]
	if lastAcceptedBlock.Hash() != expectedLastAcceptedBlock.Hash() {
		t.Fatalf("Expected last accepted block to be %s:%d, but found %s%d", expectedLastAcceptedBlock.Hash().Hex(), expectedLastAcceptedBlock.NumberU64(), lastAcceptedBlock.Hash().Hex(), lastAcceptedBlock.NumberU64())
	}

	if err := blockchain.InsertBlock(chain1[2]); err != nil {
		t.Fatal(err)
	}
	if err := blockchain.Accept(chain1[2]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	// check the state of the last accepted block
	checkState := func(sdb *state.StateDB) error {
		nonce1 := sdb.GetNonce(addr1)
		if nonce1 != 2 {
			return fmt.Errorf("expected addr1 nonce: 2, found nonce: %d", nonce1)
		}
		balance1 := sdb.GetBalance(addr1)
		expectedBalance := common.U2560
		if balance1.Cmp(expectedBalance) != 0 {
			return fmt.Errorf("expected balance1: %d, found balance: %d", expectedBalance, balance1)
		}
		nonce2 := sdb.GetNonce(addr2)
		if nonce2 != 0 {
			return fmt.Errorf("expected addr2 nonce: 0, found nonce %d", nonce2)
		}
		balance2 := sdb.GetBalance(addr2)
		genesisBalance := uint256.MustFromBig(genesisBalance)
		if balance2.Cmp(genesisBalance) != 0 {
			return fmt.Errorf("expected balance2: %d, found %d", genesisBalance, balance2)
		}
		return nil
	}

	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkState)
}

// Insert two different chains that result in the identical state root.
// Once we insert both of the chains, we restart, insert both the chains again,
// and then we accept one of the chains and accept A3 on top of the shared state
// root
//
//	  G   (genesis)
//	 / \
//	A1  B1
//	|   |
//	A2  B2 (A2 and B2 represent two different paths to the identical state trie)
//	|
//	A3
//
//nolint:goimports
func TestReprocessAcceptBlockIdenticalStateRoot(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := big.NewInt(1000000000)
	gspec := &Genesis{
		Config: &params.ChainConfig{HomesteadBlock: new(big.Int)},
		Alloc:  types.GenesisAlloc{addr1: {Balance: genesisBalance}},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}

	signer := types.HomesteadSigner{}
	_, chain1, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, 3, 10, func(i int, gen *BlockGen) {
		if i < 2 {
			// Send half the funds from addr1 to addr2 in one transaction per each of the two blocks in [chain1]
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(500000000), params.TxGas, nil, nil), signer, key1)
			gen.AddTx(tx)
		}
		// Allow the third block to be empty.
	})
	if err != nil {
		t.Fatal(err)
	}
	_, chain2, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, 2, 10, func(i int, gen *BlockGen) {
		// Send 1/4 of the funds from addr1 to addr2 in tx1 and 3/4 of the funds in tx2. This will produce the identical state
		// root in the second block of [chain2] as is present in the second block of [chain1].
		if i == 0 {
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(250000000), params.TxGas, nil, nil), signer, key1)
			gen.AddTx(tx)
		} else {
			tx, _ := types.SignTx(types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(750000000), params.TxGas, nil, nil), signer, key1)
			gen.AddTx(tx)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assert that the block root of the second block in both chains is identical
	if chain1[1].Root() != chain2[1].Root() {
		t.Fatalf("Expected the latter block in both chain1 and chain2 to have identical state root, but found %s and %s", chain1[1].Root(), chain2[1].Root())
	}

	// Insert first two blocks of [chain1] and both blocks in [chain2]
	// This leaves us one additional block to insert on top of [chain1]
	// after testing that the state roots are handled correctly.
	if _, err := blockchain.InsertChain(chain1[:2]); err != nil {
		t.Fatal(err)
	}
	if _, err := blockchain.InsertChain(chain2); err != nil {
		t.Fatal(err)
	}

	currentBlock := blockchain.CurrentBlock()
	expectedCurrentBlock := chain1[1]
	if currentBlock.Hash() != expectedCurrentBlock.Hash() {
		t.Fatalf("Expected current block to be %s:%d, but found %s%d", expectedCurrentBlock.Hash().Hex(), expectedCurrentBlock.NumberU64(), currentBlock.Hash().Hex(), currentBlock.Number.Uint64())
	}

	blockchain.Stop()

	chainDB = rawdb.NewMemoryDatabase()
	blockchain, err = create(chainDB, gspec, common.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer blockchain.Stop()

	// Insert first two blocks of [chain1] and both blocks in [chain2]
	// This leaves us one additional block to insert on top of [chain1]
	// after testing that the state roots are handled correctly.
	if _, err := blockchain.InsertChain(chain1[:2]); err != nil {
		t.Fatal(err)
	}
	if _, err := blockchain.InsertChain(chain2); err != nil {
		t.Fatal(err)
	}

	currentBlock = blockchain.CurrentBlock()
	expectedCurrentBlock = chain1[1]
	if currentBlock.Hash() != expectedCurrentBlock.Hash() {
		t.Fatalf("Expected current block to be %s:%d, but found %s%d", expectedCurrentBlock.Hash().Hex(), expectedCurrentBlock.NumberU64(), currentBlock.Hash().Hex(), currentBlock.Number.Uint64())
	}

	// Accept the first block in [chain1] and reject all of [chain2]
	if err := blockchain.Accept(chain1[0]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	for _, block := range chain2 {
		if err := blockchain.Reject(block); err != nil {
			t.Fatal(err)
		}
	}

	// Accept the last two blocks in [chain1]. This is a regression test to ensure
	// that we do not discard a snapshot difflayer that is still in use by a
	// processing block, when a different block with the same root is rejected.
	if err := blockchain.Accept(chain1[1]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	lastAcceptedBlock := blockchain.LastConsensusAcceptedBlock()
	expectedLastAcceptedBlock := chain1[1]
	if lastAcceptedBlock.Hash() != expectedLastAcceptedBlock.Hash() {
		t.Fatalf("Expected last accepted block to be %s:%d, but found %s%d", expectedLastAcceptedBlock.Hash().Hex(), expectedLastAcceptedBlock.NumberU64(), lastAcceptedBlock.Hash().Hex(), lastAcceptedBlock.NumberU64())
	}

	if err := blockchain.InsertBlock(chain1[2]); err != nil {
		t.Fatal(err)
	}
	if err := blockchain.Accept(chain1[2]); err != nil {
		t.Fatal(err)
	}
	blockchain.DrainAcceptorQueue()

	// check the state of the last accepted block
	checkState := func(sdb *state.StateDB) error {
		nonce1 := sdb.GetNonce(addr1)
		if nonce1 != 2 {
			return fmt.Errorf("expected addr1 nonce: 2, found nonce: %d", nonce1)
		}
		balance1 := sdb.GetBalance(addr1)
		expectedBalance := common.U2560
		if balance1.Cmp(expectedBalance) != 0 {
			return fmt.Errorf("expected balance1: %d, found balance: %d", expectedBalance, balance1)
		}
		nonce2 := sdb.GetNonce(addr2)
		if nonce2 != 0 {
			return fmt.Errorf("expected addr2 nonce: 0, found nonce %d", nonce2)
		}
		balance2 := sdb.GetBalance(addr2)
		genesisBalance := uint256.MustFromBig(genesisBalance)
		if balance2.Cmp(genesisBalance) != 0 {
			return fmt.Errorf("expected balance2: %d, found %d", genesisBalance, balance2)
		}
		return nil
	}

	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkState)
}

func TestGenerateChainInvalidBlockFee(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		require = require.New(t)
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := new(big.Int).Mul(big.NewInt(1000000), big.NewInt(params.Ether))
	gspec := &Genesis{
		Config: params.TestChainConfig,
		Alloc:  types.GenesisAlloc{addr1: {Balance: genesisBalance}},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	require.NoError(err)
	t.Cleanup(blockchain.Stop)

	// This call generates a chain of 3 blocks.
	signer := types.LatestSigner(params.TestChainConfig)
	_, _, _, err = GenerateChainWithGenesis(gspec, blockchain.engine, 3, ap4.TargetBlockRate-1, func(i int, gen *BlockGen) {
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   params.TestChainConfig.ChainID,
			Nonce:     gen.TxNonce(addr1),
			To:        &addr2,
			Gas:       params.TxGas,
			GasFeeCap: gen.BaseFee(),
			GasTipCap: big.NewInt(0),
			Data:      []byte{},
		})

		signedTx, err := types.SignTx(tx, signer, key1)
		require.NoError(err)
		gen.AddTx(signedTx)
	})
	require.ErrorIs(err, dummy.ErrInsufficientBlockGas)
}

func TestInsertChainInvalidBlockFee(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		require = require.New(t)
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := new(big.Int).Mul(big.NewInt(1000000), big.NewInt(params.Ether))
	gspec := &Genesis{
		Config: params.TestChainConfig,
		Alloc:  types.GenesisAlloc{addr1: {Balance: genesisBalance}},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	require.NoError(err)
	t.Cleanup(blockchain.Stop)

	// This call generates a chain of 3 blocks.
	signer := types.LatestSigner(params.TestChainConfig)
	eng := dummy.NewFakerWithMode(TestCallbacks, dummy.Mode{ModeSkipBlockFee: true, ModeSkipCoinbase: true})
	_, chain, _, err := GenerateChainWithGenesis(gspec, eng, 3, ap4.TargetBlockRate-1, func(i int, gen *BlockGen) {
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   params.TestChainConfig.ChainID,
			Nonce:     gen.TxNonce(addr1),
			To:        &addr2,
			Gas:       params.TxGas,
			GasFeeCap: gen.BaseFee(),
			GasTipCap: big.NewInt(0),
			Data:      []byte{},
		})

		signedTx, err := types.SignTx(tx, signer, key1)
		require.NoError(err)
		gen.AddTx(signedTx)
	})
	require.NoError(err)
	_, err = blockchain.InsertChain(chain)
	require.ErrorIs(err, dummy.ErrInsufficientBlockGas)
}

func TestInsertChainValidBlockFee(t *testing.T, create func(db ethdb.Database, gspec *Genesis, lastAcceptedHash common.Hash) (*BlockChain, error)) {
	var (
		require = require.New(t)
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		// We use two separate databases since GenerateChain commits the state roots to its underlying
		// database.
		chainDB = rawdb.NewMemoryDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	genesisBalance := new(big.Int).Mul(big.NewInt(1000000), big.NewInt(params.Ether))
	gspec := &Genesis{
		Config: params.TestChainConfig,
		Alloc:  types.GenesisAlloc{addr1: {Balance: genesisBalance}},
	}

	blockchain, err := create(chainDB, gspec, common.Hash{})
	require.NoError(err)
	t.Cleanup(blockchain.Stop)

	// This call generates a chain of 3 blocks.
	signer := types.LatestSigner(params.TestChainConfig)
	tip := big.NewInt(50000 * params.GWei)
	transfer := big.NewInt(10000)
	_, chain, _, err := GenerateChainWithGenesis(gspec, blockchain.engine, 3, ap4.TargetBlockRate-1, func(i int, gen *BlockGen) {
		feeCap := new(big.Int).Add(gen.BaseFee(), tip)
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   params.TestChainConfig.ChainID,
			Nonce:     gen.TxNonce(addr1),
			To:        &addr2,
			Gas:       params.TxGas,
			Value:     transfer,
			GasFeeCap: feeCap,
			GasTipCap: tip,
			Data:      []byte{},
		})

		signedTx, err := types.SignTx(tx, signer, key1)
		require.NoError(err)
		gen.AddTx(signedTx)
	})
	require.NoError(err)

	// Insert three blocks into the chain and accept only the first block.
	_, err = blockchain.InsertChain(chain)
	require.NoError(err)
	require.NoError(blockchain.Accept(chain[0]))
	blockchain.DrainAcceptorQueue()

	// check the state of the last accepted block
	checkState := func(sdb *state.StateDB) error {
		nonce := sdb.GetNonce(addr1)
		if nonce != 1 {
			return fmt.Errorf("expected nonce addr1: 1, found nonce: %d", nonce)
		}
		balance1 := sdb.GetBalance(addr1)
		transfer := uint256.MustFromBig(transfer)
		genesisBalance := uint256.MustFromBig(genesisBalance)
		expectedBalance1 := new(uint256.Int).Sub(genesisBalance, transfer)
		baseFee := chain[0].BaseFee()
		feeSpend := new(big.Int).Mul(new(big.Int).Add(baseFee, tip), new(big.Int).SetUint64(params.TxGas))
		expectedBalance1.Sub(expectedBalance1, uint256.MustFromBig(feeSpend))
		if balance1.Cmp(expectedBalance1) != 0 {
			return fmt.Errorf("expected addr1 balance: %d, found balance: %d", expectedBalance1, balance1)
		}

		balance2 := sdb.GetBalance(addr2)
		expectedBalance2 := transfer
		if balance2.Cmp(expectedBalance2) != 0 {
			return fmt.Errorf("expected addr2 balance: %d, found balance: %d", expectedBalance2, balance2)
		}

		nonce = sdb.GetNonce(addr2)
		if nonce != 0 {
			return fmt.Errorf("expected addr2 nonce: 0, found nonce: %d", nonce)
		}
		return nil
	}

	checkBlockChainState(t, blockchain, gspec, chainDB, create, checkState)
}

// CheckTxIndices checks that the transaction indices are correctly stored in the database ([tail, head]).
func CheckTxIndices(t *testing.T, expectedTail *uint64, head uint64, db ethdb.Database, allowNilBlocks bool) {
	var tailValue uint64
	if expectedTail != nil {
		tailValue = *expectedTail
	}
	checkTxIndicesHelper(t, expectedTail, tailValue, head, head, db, allowNilBlocks)
}

// checkTxIndicesHelper checks that the transaction indices are correctly stored in the database.
// [expectedTail] is the expected value of the tail index.
// [indexedFrom] is the block number from which the transactions should be indexed.
// [indexedTo] is the block number to which the transactions should be indexed.
// [head] is the block number of the head block.
func checkTxIndicesHelper(t *testing.T, expectedTail *uint64, indexedFrom uint64, indexedTo uint64, head uint64, db ethdb.Database, allowNilBlocks bool) {
	if expectedTail == nil {
		require.Nil(t, rawdb.ReadTxIndexTail(db))
	} else {
		var stored uint64
		tailValue := *expectedTail

		require.EventuallyWithTf(t,
			func(c *assert.CollectT) {
				stored = *rawdb.ReadTxIndexTail(db)
				assert.Equalf(c, tailValue, stored, "expected tail to be %d, found %d", tailValue, stored)
			},
			30*time.Second, 500*time.Millisecond, "expected tail to be %d eventually", tailValue)
	}

	for i := uint64(0); i <= head; i++ {
		block := rawdb.ReadBlock(db, rawdb.ReadCanonicalHash(db, i), i)
		if block == nil && allowNilBlocks {
			continue
		}
		for _, tx := range block.Transactions() {
			index := rawdb.ReadTxLookupEntry(db, tx.Hash())
			if i < indexedFrom {
				require.Nilf(t, index, "Transaction indices should be deleted, number %d hash %s", i, tx.Hash().Hex())
			} else if i <= indexedTo {
				require.NotNilf(t, index, "Missing transaction indices, number %d hash %s", i, tx.Hash().Hex())
			}
		}
	}
}
