// (c) 2025, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/luxfi/node/database/manager"
	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/snow"
	"github.com/luxfi/node/utils/logging"
	"github.com/luxfi/node/version"
	"github.com/stretchr/testify/require"
)

// TestEthAPIsDebug traces the eth API registration issue
func TestEthAPIsDebug(t *testing.T) {
	t.Log("🔍 Debugging Eth APIs Registration")
	
	// Set up test directory
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "db")
	
	// Create logging
	log := logging.NewLogger("test", logging.NewWrappedCore(logging.Debug, os.Stdout, logging.Colors.ConsoleEncoder()))
	
	// Create VM
	vm := &VM{}
	vm.ctx = &snow.Context{
		NetworkID:   96369,
		SubnetID:    ids.GenerateTestID(),
		ChainID:     generateCChainID(),  // Use actual C-Chain ID
		NodeID:      ids.GenerateTestNodeID(),
		Log:         log,
		LUXAssetID:  ids.GenerateTestID(),
		XChainID:    ids.GenerateTestID(),
	}
	
	// Create config with all APIs
	config := Config{
		EthAPIs:     []string{"eth", "eth-filter", "net", "web3", "internal-eth", "internal-blockchain", "internal-transaction", "admin", "debug", "personal", "txpool", "miner"},
		RPCGasCap:   50000000,
		RPCTxFeeCap: 100,
		LogLevel:    "debug",
	}
	
	// Create genesis with chain ID 96369
	genesisJSON := `{
		"config": {
			"chainId": 96369,
			"homesteadBlock": 0,
			"eip150Block": 0,
			"eip155Block": 0,
			"eip158Block": 0,
			"byzantiumBlock": 0,
			"constantinopleBlock": 0,
			"petersburgBlock": 0,
			"istanbulBlock": 0,
			"muirGlacierBlock": 0,
			"berlinBlock": 0,
			"londonBlock": 0,
			"apricotPhase1BlockTimestamp": 0,
			"apricotPhase2BlockTimestamp": 0,
			"apricotPhase3BlockTimestamp": 0,
			"apricotPhase4BlockTimestamp": 0,
			"apricotPhase5BlockTimestamp": 0
		},
		"nonce": "0x0",
		"timestamp": "0x0",
		"extraData": "0x",
		"gasLimit": "0x7A1200",
		"difficulty": "0x0",
		"mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
		"coinbase": "0x0000000000000000000000000000000000000000",
		"alloc": {
			"8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC": {
				"balance": "0x52B7D2DCC80CD2E4000000"
			}
		}
	}`
	
	config.GenesisBytes = []byte(genesisJSON)
	
	// Encode config
	configBytes, err := json.Marshal(config)
	require.NoError(t, err)
	
	// Create DB manager
	dbManager := &testDBManager{
		db: rawdb.NewMemoryDatabase(),
		path: dbPath,
	}
	
	t.Log("📋 Step 1: Initialize VM")
	err = vm.Initialize(
		context.Background(),
		vm.ctx,
		dbManager,
		[]byte(genesisJSON),
		nil, // upgrade bytes
		configBytes,
		nil, // toEngine
		nil, // fxs
	)
	require.NoError(t, err)
	
	// Check what was initialized
	t.Log("📋 Step 2: Check VM state after initialization")
	t.Logf("  - vm.eth: %v", vm.eth)
	t.Logf("  - vm.ethService: %v", vm.ethService)
	t.Logf("  - vm.rpc: %v", vm.rpc)
	t.Logf("  - vm.ethAPIs configured: %v", vm.config.EthAPIs)
	
	// If eth is nil, that's our problem
	if vm.eth == nil {
		t.Log("❌ vm.eth is nil - attachEthService failed!")
		
		// Let's trace why
		t.Log("📋 Step 3: Manually test eth service creation")
		
		// Create eth backend config
		ethConfig := ethconfig.Config{
			Genesis:     core.DefaultGenesisBlock(),
			NetworkId:   96369,
			RPCGasCap:   50000000,
			RPCTxFeeCap: 100,
		}
		
		t.Logf("  - Created eth config with NetworkId: %d", ethConfig.NetworkId)
		
		// Check if blockchain exists
		if vm.blockChain != nil {
			t.Log("✅ Blockchain exists")
			t.Logf("  - ChainID: %v", vm.blockChain.Config().ChainID)
			t.Logf("  - Current block: %v", vm.blockChain.CurrentBlock().Number)
		} else {
			t.Log("❌ Blockchain is nil!")
		}
	}
	
	// Test RPC registration directly
	t.Log("📋 Step 4: Test RPC registration")
	
	// Create a new RPC server to test
	testServer := rpc.NewServer()
	
	// Create simple test API
	type TestEthAPI struct{}
	
	testAPI := &TestEthAPI{}
	
	// Add eth_chainId method
	type ChainIdResult struct {
		ChainId string `json:"chainId"`
	}
	
	// Register test API
	err = testServer.RegisterName("eth", testAPI)
	require.NoError(t, err)
	
	t.Log("✅ Successfully registered test API")
	
	// Now let's understand why the real eth API isn't being registered
	t.Log("📋 Step 5: Trace attachEthService")
	
	// The issue is likely in attachEthService - let's check if it's being called
	// and if it's creating the eth backend properly
	
	// Clean up
	require.NoError(t, vm.Shutdown(context.Background()))
}

// TestEthServiceCreation tests creating eth service manually
func TestEthServiceCreation(t *testing.T) {
	t.Log("🔧 Testing Manual Eth Service Creation")
	
	// This test will help us understand what's needed to create eth service
	testDir := t.TempDir()
	
	// Create a minimal blockchain
	db := rawdb.NewMemoryDatabase()
	
	// Create genesis
	genesis := &core.Genesis{
		Config: &params.ChainConfig{
			ChainID:             big.NewInt(96369),
			HomesteadBlock:      big.NewInt(0),
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			MuirGlacierBlock:    big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			LondonBlock:         big.NewInt(0),
		},
		Difficulty: big.NewInt(0),
		GasLimit:   8000000,
		Alloc: core.GenesisAlloc{
			common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"): {
				Balance: new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)),
			},
		},
	}
	
	// Initialize blockchain
	chainConfig, _, err := core.SetupGenesisBlock(db, genesis)
	require.NoError(t, err)
	
	t.Logf("✅ Created blockchain with ChainID: %v", chainConfig.ChainID)
	
	// This shows us what we need to create eth service properly
}

// Helper to generate C-Chain ID
func generateCChainID() ids.ID {
	// This is the actual C-Chain ID
	id, _ := ids.FromString("2CA6j5zYzasynPsFeNoqWkmTCt3VScMvXUZHbfDJ8k3oGzAPtU")
	return id
}

// Test DB Manager
type testDBManager struct {
	db   ethdb.Database
	path string
}

func (m *testDBManager) Current() *manager.VersionedDatabase {
	return &manager.VersionedDatabase{
		Database: m.db,
		Version:  version.CurrentDatabase,
	}
}

func (m *testDBManager) Get(version.Version) (*manager.VersionedDatabase, error) {
	return m.Current(), nil
}

func (m *testDBManager) Close() error {
	return nil
}