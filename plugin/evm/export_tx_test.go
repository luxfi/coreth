// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/luxfi/coreth/plugin/evm/atomic"
	atomicvm "github.com/luxfi/coreth/plugin/evm/atomic/vm"

	"github.com/stretchr/testify/require"

	luxatomic "github.com/luxfi/node/chains/atomic"
	"github.com/luxfi/node/ids"
	commonEng "github.com/luxfi/node/snow/engine/common"
	"github.com/luxfi/node/snow/snowtest"
	"github.com/luxfi/node/upgrade/upgradetest"
	luxutils "github.com/luxfi/node/utils"
	"github.com/luxfi/node/utils/constants"
	"github.com/luxfi/node/utils/crypto/secp256k1"
	"github.com/luxfi/node/utils/units"
	"github.com/luxfi/node/vms/components/lux"
	"github.com/luxfi/node/vms/secp256k1fx"
	"github.com/luxfi/coreth/core/extstate"
	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/utils"
	"github.com/luxfi/geth/common"
	"github.com/holiman/uint256"
)

// createExportTxOptions adds funds to shared memory, imports them, and returns a list of export transactions
// that attempt to send the funds to each of the test keys (list of length 3).
func createExportTxOptions(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) []*atomic.Tx {
	// Add a UTXO to shared memory
	utxo := &lux.UTXO{
		UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
		Asset:  lux.Asset{ID: vm.Ctx.LUXAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt: uint64(50000000),
			OutputOwners: secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{testKeys[0].Address()},
			},
		},
	}
	utxoBytes, err := atomic.Codec.Marshal(atomic.CodecVersion, utxo)
	if err != nil {
		t.Fatal(err)
	}

	xChainSharedMemory := sharedMemory.NewSharedMemory(vm.Ctx.XChainID)
	inputID := utxo.InputID()
	if err := xChainSharedMemory.Apply(map[ids.ID]*luxatomic.Requests{vm.Ctx.ChainID: {PutRequests: []*luxatomic.Element{{
		Key:   inputID[:],
		Value: utxoBytes,
		Traits: [][]byte{
			testKeys[0].Address().Bytes(),
		},
	}}}}); err != nil {
		t.Fatal(err)
	}

	// Import the funds
	importTx, err := vm.NewImportTx(vm.Ctx.XChainID, testEthAddrs[0], initialBaseFee, []*secp256k1.PrivateKey{testKeys[0]})
	if err != nil {
		t.Fatal(err)
	}

	if err := vm.AtomicMempool.AddLocalTx(importTx); err != nil {
		t.Fatal(err)
	}

	msg, err := vm.WaitForEvent(context.Background())
	require.NoError(t, err)
	require.Equal(t, commonEng.PendingTxs, msg)

	blk, err := vm.BuildBlock(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if err := blk.Verify(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := vm.SetPreference(context.Background(), blk.ID()); err != nil {
		t.Fatal(err)
	}

	if err := blk.Accept(context.Background()); err != nil {
		t.Fatal(err)
	}

	statedb, err := vm.Blockchain().State()
	if err != nil {
		t.Fatal(err)
	}

	// Use the funds to create 3 conflicting export transactions sending the funds to each of the test addresses
	exportTxs := make([]*atomic.Tx, 0, 3)
	wrappedStateDB := extstate.New(statedb)
	for _, addr := range testShortIDAddrs {
		exportTx, err := atomic.NewExportTx(vm.Ctx, vm.CurrentRules(), wrappedStateDB, vm.Ctx.LUXAssetID, uint64(5000000), vm.Ctx.XChainID, addr, initialBaseFee, []*secp256k1.PrivateKey{testKeys[0]})
		if err != nil {
			t.Fatal(err)
		}
		exportTxs = append(exportTxs, exportTx)
	}

	return exportTxs
}

func TestExportTxEVMStateTransfer(t *testing.T) {
	key := testKeys[0]
	addr := key.Address()
	ethAddr := key.EthAddress()

	luxAmount := 50 * units.MilliLux
	luxUTXOID := lux.UTXOID{
		OutputIndex: 0,
	}
	luxInputID := luxUTXOID.InputID()

	customAmount := uint64(100)
	customAssetID := ids.ID{1, 2, 3, 4, 5, 7}
	customUTXOID := lux.UTXOID{
		OutputIndex: 1,
	}
	customInputID := customUTXOID.InputID()

	customUTXO := &lux.UTXO{
		UTXOID: customUTXOID,
		Asset:  lux.Asset{ID: customAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt: customAmount,
			OutputOwners: secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{addr},
			},
		},
	}

	tests := []struct {
		name          string
		tx            []atomic.EVMInput
		luxBalance   *uint256.Int
		balances      map[ids.ID]*big.Int
		expectedNonce uint64
		shouldErr     bool
	}{
		{
			name:        "no transfers",
			tx:          nil,
			luxBalance: uint256.NewInt(luxAmount * atomic.X2CRateUint64),
			balances: map[ids.ID]*big.Int{
				customAssetID: big.NewInt(int64(customAmount)),
			},
			expectedNonce: 0,
			shouldErr:     false,
		},
		{
			name: "spend half LUX",
			tx: []atomic.EVMInput{
				{
					Address: ethAddr,
					Amount:  luxAmount / 2,
					AssetID: snowtest.LUXAssetID,
					Nonce:   0,
				},
			},
			luxBalance: uint256.NewInt(luxAmount / 2 * atomic.X2CRateUint64),
			balances: map[ids.ID]*big.Int{
				customAssetID: big.NewInt(int64(customAmount)),
			},
			expectedNonce: 1,
			shouldErr:     false,
		},
		{
			name: "spend all LUX",
			tx: []atomic.EVMInput{
				{
					Address: ethAddr,
					Amount:  luxAmount,
					AssetID: snowtest.LUXAssetID,
					Nonce:   0,
				},
			},
			luxBalance: uint256.NewInt(0),
			balances: map[ids.ID]*big.Int{
				customAssetID: big.NewInt(int64(customAmount)),
			},
			expectedNonce: 1,
			shouldErr:     false,
		},
		{
			name: "spend too much LUX",
			tx: []atomic.EVMInput{
				{
					Address: ethAddr,
					Amount:  luxAmount + 1,
					AssetID: snowtest.LUXAssetID,
					Nonce:   0,
				},
			},
			luxBalance: uint256.NewInt(0),
			balances: map[ids.ID]*big.Int{
				customAssetID: big.NewInt(int64(customAmount)),
			},
			expectedNonce: 1,
			shouldErr:     true,
		},
		{
			name: "spend half custom",
			tx: []atomic.EVMInput{
				{
					Address: ethAddr,
					Amount:  customAmount / 2,
					AssetID: customAssetID,
					Nonce:   0,
				},
			},
			luxBalance: uint256.NewInt(luxAmount * atomic.X2CRateUint64),
			balances: map[ids.ID]*big.Int{
				customAssetID: big.NewInt(int64(customAmount / 2)),
			},
			expectedNonce: 1,
			shouldErr:     false,
		},
		{
			name: "spend all custom",
			tx: []atomic.EVMInput{
				{
					Address: ethAddr,
					Amount:  customAmount,
					AssetID: customAssetID,
					Nonce:   0,
				},
			},
			luxBalance: uint256.NewInt(luxAmount * atomic.X2CRateUint64),
			balances: map[ids.ID]*big.Int{
				customAssetID: big.NewInt(0),
			},
			expectedNonce: 1,
			shouldErr:     false,
		},
		{
			name: "spend too much custom",
			tx: []atomic.EVMInput{
				{
					Address: ethAddr,
					Amount:  customAmount + 1,
					AssetID: customAssetID,
					Nonce:   0,
				},
			},
			luxBalance: uint256.NewInt(luxAmount * atomic.X2CRateUint64),
			balances: map[ids.ID]*big.Int{
				customAssetID: big.NewInt(0),
			},
			expectedNonce: 1,
			shouldErr:     true,
		},
		{
			name: "spend everything",
			tx: []atomic.EVMInput{
				{
					Address: ethAddr,
					Amount:  customAmount,
					AssetID: customAssetID,
					Nonce:   0,
				},
				{
					Address: ethAddr,
					Amount:  luxAmount,
					AssetID: snowtest.LUXAssetID,
					Nonce:   0,
				},
			},
			luxBalance: uint256.NewInt(0),
			balances: map[ids.ID]*big.Int{
				customAssetID: big.NewInt(0),
			},
			expectedNonce: 1,
			shouldErr:     false,
		},
		{
			name: "spend everything wrong nonce",
			tx: []atomic.EVMInput{
				{
					Address: ethAddr,
					Amount:  customAmount,
					AssetID: customAssetID,
					Nonce:   1,
				},
				{
					Address: ethAddr,
					Amount:  luxAmount,
					AssetID: snowtest.LUXAssetID,
					Nonce:   1,
				},
			},
			luxBalance: uint256.NewInt(0),
			balances: map[ids.ID]*big.Int{
				customAssetID: big.NewInt(0),
			},
			expectedNonce: 1,
			shouldErr:     true,
		},
		{
			name: "spend everything changing nonces",
			tx: []atomic.EVMInput{
				{
					Address: ethAddr,
					Amount:  customAmount,
					AssetID: customAssetID,
					Nonce:   0,
				},
				{
					Address: ethAddr,
					Amount:  luxAmount,
					AssetID: snowtest.LUXAssetID,
					Nonce:   1,
				},
			},
			luxBalance: uint256.NewInt(0),
			balances: map[ids.ID]*big.Int{
				customAssetID: big.NewInt(0),
			},
			expectedNonce: 1,
			shouldErr:     true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fork := upgradetest.NoUpgrades
			tvm := newVM(t, testVMConfig{
				fork: &fork,
			})
			defer func() {
				if err := tvm.vm.Shutdown(context.Background()); err != nil {
					t.Fatal(err)
				}
			}()

			luxUTXO := &lux.UTXO{
				UTXOID: luxUTXOID,
				Asset:  lux.Asset{ID: tvm.vm.ctx.LUXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: luxAmount,
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1,
						Addrs:     []ids.ShortID{addr},
					},
				},
			}

			luxUTXOBytes, err := atomic.Codec.Marshal(atomic.CodecVersion, luxUTXO)
			if err != nil {
				t.Fatal(err)
			}

			customUTXOBytes, err := atomic.Codec.Marshal(atomic.CodecVersion, customUTXO)
			if err != nil {
				t.Fatal(err)
			}

			xChainSharedMemory := tvm.atomicMemory.NewSharedMemory(tvm.vm.ctx.XChainID)
			if err := xChainSharedMemory.Apply(map[ids.ID]*luxatomic.Requests{tvm.vm.ctx.ChainID: {PutRequests: []*luxatomic.Element{
				{
					Key:   luxInputID[:],
					Value: luxUTXOBytes,
					Traits: [][]byte{
						addr.Bytes(),
					},
				},
				{
					Key:   customInputID[:],
					Value: customUTXOBytes,
					Traits: [][]byte{
						addr.Bytes(),
					},
				},
			}}}); err != nil {
				t.Fatal(err)
			}

			tx, err := tvm.atomicVM.NewImportTx(tvm.vm.ctx.XChainID, testEthAddrs[0], initialBaseFee, []*secp256k1.PrivateKey{testKeys[0]})
			if err != nil {
				t.Fatal(err)
			}

			if err := tvm.atomicVM.AtomicMempool.AddLocalTx(tx); err != nil {
				t.Fatal(err)
			}

			require.Equal(t, commonEng.PendingTxs, tvm.WaitForEvent(context.Background()))

			blk, err := tvm.vm.BuildBlock(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if err := blk.Verify(context.Background()); err != nil {
				t.Fatal(err)
			}

			if err := tvm.vm.SetPreference(context.Background(), blk.ID()); err != nil {
				t.Fatal(err)
			}

			if err := blk.Accept(context.Background()); err != nil {
				t.Fatal(err)
			}

			newTx := atomic.UnsignedExportTx{
				Ins: test.tx,
			}

			statedb, err := tvm.vm.blockChain.State()
			if err != nil {
				t.Fatal(err)
			}

			wrappedStateDB := extstate.New(statedb)
			err = newTx.EVMStateTransfer(tvm.vm.ctx, wrappedStateDB)
			if test.shouldErr {
				if err == nil {
					t.Fatal("expected EVMStateTransfer to fail")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			luxBalance := wrappedStateDB.GetBalance(ethAddr)
			if luxBalance.Cmp(test.luxBalance) != 0 {
				t.Fatalf("address balance %s equal %s not %s", addr.String(), luxBalance, test.luxBalance)
			}

			for assetID, expectedBalance := range test.balances {
				balance := wrappedStateDB.GetBalanceMultiCoin(ethAddr, common.Hash(assetID))
				if luxBalance.Cmp(test.luxBalance) != 0 {
					t.Fatalf("%s address balance %s equal %s not %s", assetID, addr.String(), balance, expectedBalance)
				}
			}

			if wrappedStateDB.GetNonce(ethAddr) != test.expectedNonce {
				t.Fatalf("failed to set nonce to %d", test.expectedNonce)
			}
		})
	}
}

func TestExportTxSemanticVerify(t *testing.T) {
	fork := upgradetest.NoUpgrades
	tvm := newVM(t, testVMConfig{
		fork: &fork,
	})
	vm := tvm.atomicVM
	defer func() {
		if err := vm.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	parent := vm.LastAcceptedExtendedBlock()

	key := testKeys[0]
	addr := key.Address()
	ethAddr := testEthAddrs[0]

	var (
		luxBalance           = 10 * units.Lux
		custom0Balance uint64 = 100
		custom0AssetID        = ids.ID{1, 2, 3, 4, 5}
		custom1Balance uint64 = 1000
		custom1AssetID        = ids.ID{1, 2, 3, 4, 5, 6}
	)

	validExportTx := &atomic.UnsignedExportTx{
		NetworkID:        vm.Ctx.NetworkID,
		BlockchainID:     vm.Ctx.ChainID,
		DestinationChain: vm.Ctx.XChainID,
		Ins: []atomic.EVMInput{
			{
				Address: ethAddr,
				Amount:  luxBalance,
				AssetID: vm.Ctx.LUXAssetID,
				Nonce:   0,
			},
			{
				Address: ethAddr,
				Amount:  custom0Balance,
				AssetID: custom0AssetID,
				Nonce:   0,
			},
			{
				Address: ethAddr,
				Amount:  custom1Balance,
				AssetID: custom1AssetID,
				Nonce:   0,
			},
		},
		ExportedOutputs: []*lux.TransferableOutput{
			{
				Asset: lux.Asset{ID: custom0AssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: custom0Balance,
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1,
						Addrs:     []ids.ShortID{addr},
					},
				},
			},
		},
	}
	luxutils.Sort(validExportTx.Ins)

	validLUXExportTx := &atomic.UnsignedExportTx{
		NetworkID:        vm.Ctx.NetworkID,
		BlockchainID:     vm.Ctx.ChainID,
		DestinationChain: vm.Ctx.XChainID,
		Ins: []atomic.EVMInput{
			{
				Address: ethAddr,
				Amount:  luxBalance,
				AssetID: vm.Ctx.LUXAssetID,
				Nonce:   0,
			},
		},
		ExportedOutputs: []*lux.TransferableOutput{
			{
				Asset: lux.Asset{ID: vm.Ctx.LUXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: luxBalance / 2,
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1,
						Addrs:     []ids.ShortID{addr},
					},
				},
			},
		},
	}

	tests := []struct {
		name      string
		tx        *atomic.Tx
		signers   [][]*secp256k1.PrivateKey
		baseFee   *big.Int
		rules     extras.Rules
		shouldErr bool
	}{
		{
			name: "valid",
			tx:   &atomic.Tx{UnsignedAtomicTx: validExportTx},
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: false,
		},
		{
			name: "P-chain before AP5",
			tx: func() *atomic.Tx {
				validExportTx := *validLUXExportTx
				validExportTx.DestinationChain = constants.PlatformChainID
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "P-chain after AP5",
			tx: func() *atomic.Tx {
				validExportTx := *validLUXExportTx
				validExportTx.DestinationChain = constants.PlatformChainID
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase5,
			shouldErr: false,
		},
		{
			name: "random chain after AP5",
			tx: func() *atomic.Tx {
				validExportTx := *validLUXExportTx
				validExportTx.DestinationChain = ids.GenerateTestID()
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase5,
			shouldErr: true,
		},
		{
			name: "P-chain multi-coin before AP5",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.DestinationChain = constants.PlatformChainID
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "P-chain multi-coin after AP5",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.DestinationChain = constants.PlatformChainID
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase5,
			shouldErr: true,
		},
		{
			name: "random chain multi-coin  after AP5",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.DestinationChain = ids.GenerateTestID()
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase5,
			shouldErr: true,
		},
		{
			name: "no outputs",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.ExportedOutputs = nil
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "wrong networkID",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.NetworkID++
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "wrong chainID",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.BlockchainID = ids.GenerateTestID()
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "invalid input",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.Ins = append([]atomic.EVMInput{}, validExportTx.Ins...)
				validExportTx.Ins[2].Amount = 0
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "invalid output",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.ExportedOutputs = []*lux.TransferableOutput{{
					Asset: lux.Asset{ID: custom0AssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: custom0Balance,
						OutputOwners: secp256k1fx.OutputOwners{
							Threshold: 0,
							Addrs:     []ids.ShortID{addr},
						},
					},
				}}
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "unsorted outputs",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				exportOutputs := []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: custom0AssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: custom0Balance/2 + 1,
							OutputOwners: secp256k1fx.OutputOwners{
								Threshold: 1,
								Addrs:     []ids.ShortID{addr},
							},
						},
					},
					{
						Asset: lux.Asset{ID: custom0AssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: custom0Balance/2 - 1,
							OutputOwners: secp256k1fx.OutputOwners{
								Threshold: 1,
								Addrs:     []ids.ShortID{addr},
							},
						},
					},
				}
				// Sort the outputs and then swap the ordering to ensure that they are ordered incorrectly
				lux.SortTransferableOutputs(exportOutputs, atomic.Codec)
				exportOutputs[0], exportOutputs[1] = exportOutputs[1], exportOutputs[0]
				validExportTx.ExportedOutputs = exportOutputs
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "not unique inputs",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.Ins = append([]atomic.EVMInput{}, validExportTx.Ins...)
				validExportTx.Ins[2] = validExportTx.Ins[1]
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "custom asset insufficient funds",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.ExportedOutputs = []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: custom0AssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: custom0Balance + 1,
							OutputOwners: secp256k1fx.OutputOwners{
								Threshold: 1,
								Addrs:     []ids.ShortID{addr},
							},
						},
					},
				}
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "lux insufficient funds",
			tx: func() *atomic.Tx {
				validExportTx := *validExportTx
				validExportTx.ExportedOutputs = []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: vm.Ctx.LUXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: luxBalance, // after fees this should be too much
							OutputOwners: secp256k1fx.OutputOwners{
								Threshold: 1,
								Addrs:     []ids.ShortID{addr},
							},
						},
					},
				}
				return &atomic.Tx{UnsignedAtomicTx: &validExportTx}
			}(),
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "too many signatures",
			tx:   &atomic.Tx{UnsignedAtomicTx: validExportTx},
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "too few signatures",
			tx:   &atomic.Tx{UnsignedAtomicTx: validExportTx},
			signers: [][]*secp256k1.PrivateKey{
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "too many signatures on credential",
			tx:   &atomic.Tx{UnsignedAtomicTx: validExportTx},
			signers: [][]*secp256k1.PrivateKey{
				{key, testKeys[1]},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "too few signatures on credential",
			tx:   &atomic.Tx{UnsignedAtomicTx: validExportTx},
			signers: [][]*secp256k1.PrivateKey{
				{},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name: "wrong signature on credential",
			tx:   &atomic.Tx{UnsignedAtomicTx: validExportTx},
			signers: [][]*secp256k1.PrivateKey{
				{testKeys[1]},
				{key},
				{key},
			},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
		{
			name:      "no signatures",
			tx:        &atomic.Tx{UnsignedAtomicTx: validExportTx},
			signers:   [][]*secp256k1.PrivateKey{},
			baseFee:   initialBaseFee,
			rules:     apricotRulesPhase3,
			shouldErr: true,
		},
	}
	for _, test := range tests {
		if err := test.tx.Sign(atomic.Codec, test.signers); err != nil {
			t.Fatal(err)
		}

		backend := atomicvm.NewVerifierBackend(vm, test.rules)

		t.Run(test.name, func(t *testing.T) {
			tx := test.tx

			err := backend.SemanticVerify(tx, parent, test.baseFee)
			if test.shouldErr && err == nil {
				t.Fatalf("should have errored but returned valid")
			}
			if !test.shouldErr && err != nil {
				t.Fatalf("shouldn't have errored but returned %s", err)
			}
		})
	}
}

func TestExportTxAccept(t *testing.T) {
	fork := upgradetest.NoUpgrades
	tvm := newVM(t, testVMConfig{
		fork: &fork,
	})
	defer func() {
		if err := tvm.vm.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	xChainSharedMemory := tvm.atomicMemory.NewSharedMemory(tvm.vm.ctx.XChainID)

	key := testKeys[0]
	addr := key.Address()
	ethAddr := testEthAddrs[0]

	var (
		luxBalance           = 10 * units.Lux
		custom0Balance uint64 = 100
		custom0AssetID        = ids.ID{1, 2, 3, 4, 5}
	)

	exportTx := &atomic.UnsignedExportTx{
		NetworkID:        tvm.vm.ctx.NetworkID,
		BlockchainID:     tvm.vm.ctx.ChainID,
		DestinationChain: tvm.vm.ctx.XChainID,
		Ins: []atomic.EVMInput{
			{
				Address: ethAddr,
				Amount:  luxBalance,
				AssetID: tvm.vm.ctx.LUXAssetID,
				Nonce:   0,
			},
			{
				Address: ethAddr,
				Amount:  custom0Balance,
				AssetID: custom0AssetID,
				Nonce:   0,
			},
		},
		ExportedOutputs: []*lux.TransferableOutput{
			{
				Asset: lux.Asset{ID: tvm.vm.ctx.LUXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: luxBalance,
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1,
						Addrs:     []ids.ShortID{addr},
					},
				},
			},
			{
				Asset: lux.Asset{ID: custom0AssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: custom0Balance,
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1,
						Addrs:     []ids.ShortID{addr},
					},
				},
			},
		},
	}

	tx := &atomic.Tx{UnsignedAtomicTx: exportTx}

	signers := [][]*secp256k1.PrivateKey{
		{key},
		{key},
		{key},
	}

	if err := tx.Sign(atomic.Codec, signers); err != nil {
		t.Fatal(err)
	}

	commitBatch, err := tvm.vm.versiondb.CommitBatch()
	if err != nil {
		t.Fatalf("Failed to create commit batch for VM due to %s", err)
	}
	chainID, atomicRequests, err := tx.AtomicOps()
	if err != nil {
		t.Fatalf("Failed to accept export transaction due to: %s", err)
	}

	if err := tvm.vm.ctx.SharedMemory.Apply(map[ids.ID]*luxatomic.Requests{chainID: atomicRequests}, commitBatch); err != nil {
		t.Fatal(err)
	}
	indexedValues, _, _, err := xChainSharedMemory.Indexed(tvm.vm.ctx.ChainID, [][]byte{addr.Bytes()}, nil, nil, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(indexedValues) != 2 {
		t.Fatalf("expected 2 values but got %d", len(indexedValues))
	}

	luxUTXOID := lux.UTXOID{
		TxID:        tx.ID(),
		OutputIndex: 0,
	}
	luxInputID := luxUTXOID.InputID()

	customUTXOID := lux.UTXOID{
		TxID:        tx.ID(),
		OutputIndex: 1,
	}
	customInputID := customUTXOID.InputID()

	fetchedValues, err := xChainSharedMemory.Get(tvm.vm.ctx.ChainID, [][]byte{
		customInputID[:],
		luxInputID[:],
	})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(fetchedValues[0], indexedValues[0]) {
		t.Fatalf("inconsistent values returned fetched %x indexed %x", fetchedValues[0], indexedValues[0])
	}
	if !bytes.Equal(fetchedValues[1], indexedValues[1]) {
		t.Fatalf("inconsistent values returned fetched %x indexed %x", fetchedValues[1], indexedValues[1])
	}

	customUTXOBytes, err := atomic.Codec.Marshal(atomic.CodecVersion, &lux.UTXO{
		UTXOID: customUTXOID,
		Asset:  lux.Asset{ID: custom0AssetID},
		Out:    exportTx.ExportedOutputs[1].Out,
	})
	if err != nil {
		t.Fatal(err)
	}

	luxUTXOBytes, err := atomic.Codec.Marshal(atomic.CodecVersion, &lux.UTXO{
		UTXOID: luxUTXOID,
		Asset:  lux.Asset{ID: tvm.vm.ctx.LUXAssetID},
		Out:    exportTx.ExportedOutputs[0].Out,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(fetchedValues[0], customUTXOBytes) {
		t.Fatalf("incorrect values returned expected %x got %x", customUTXOBytes, fetchedValues[0])
	}
	if !bytes.Equal(fetchedValues[1], luxUTXOBytes) {
		t.Fatalf("incorrect values returned expected %x got %x", luxUTXOBytes, fetchedValues[1])
	}
}

func TestExportTxVerify(t *testing.T) {
	var exportAmount uint64 = 10000000
	exportTx := &atomic.UnsignedExportTx{
		NetworkID:        constants.UnitTestID,
		BlockchainID:     snowtest.CChainID,
		DestinationChain: snowtest.XChainID,
		Ins: []atomic.EVMInput{
			{
				Address: testEthAddrs[0],
				Amount:  exportAmount,
				AssetID: snowtest.LUXAssetID,
				Nonce:   0,
			},
			{
				Address: testEthAddrs[2],
				Amount:  exportAmount,
				AssetID: snowtest.LUXAssetID,
				Nonce:   0,
			},
		},
		ExportedOutputs: []*lux.TransferableOutput{
			{
				Asset: lux.Asset{ID: snowtest.LUXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: exportAmount,
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Threshold: 1,
						Addrs:     []ids.ShortID{testShortIDAddrs[0]},
					},
				},
			},
			{
				Asset: lux.Asset{ID: snowtest.LUXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: exportAmount,
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Threshold: 1,
						Addrs:     []ids.ShortID{testShortIDAddrs[1]},
					},
				},
			},
		},
	}

	// Sort the inputs and outputs to ensure the transaction is canonical
	lux.SortTransferableOutputs(exportTx.ExportedOutputs, atomic.Codec)
	// Pass in a list of signers here with the appropriate length
	// to avoid causing a nil-pointer error in the helper method
	emptySigners := make([][]*secp256k1.PrivateKey, 2)
	atomic.SortEVMInputsAndSigners(exportTx.Ins, emptySigners)

	ctx := snowtest.Context(t, snowtest.CChainID)

	tests := map[string]atomicTxVerifyTest{
		"nil tx": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				return (*atomic.UnsignedExportTx)(nil)
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrNilTx.Error(),
		},
		"valid export tx": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				return exportTx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: "",
		},
		"valid export tx banff": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				return exportTx
			},
			ctx:         ctx,
			rules:       banffRules,
			expectedErr: "",
		},
		"incorrect networkID": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.NetworkID++
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrWrongNetworkID.Error(),
		},
		"incorrect blockchainID": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.BlockchainID = ids.GenerateTestID()
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrWrongChainID.Error(),
		},
		"incorrect destination chain": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.DestinationChain = ids.GenerateTestID()
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrWrongChainID.Error(), // TODO make this error more specific to destination not just chainID
		},
		"no exported outputs": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.ExportedOutputs = nil
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrNoExportOutputs.Error(),
		},
		"unsorted outputs": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.ExportedOutputs = []*lux.TransferableOutput{
					tx.ExportedOutputs[1],
					tx.ExportedOutputs[0],
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrOutputsNotSorted.Error(),
		},
		"invalid exported output": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.ExportedOutputs = []*lux.TransferableOutput{tx.ExportedOutputs[0], nil}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: "nil transferable output is not valid",
		},
		"unsorted EVM inputs before AP1": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.Ins = []atomic.EVMInput{
					tx.Ins[1],
					tx.Ins[0],
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: "",
		},
		"unsorted EVM inputs after AP1": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.Ins = []atomic.EVMInput{
					tx.Ins[1],
					tx.Ins[0],
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase1,
			expectedErr: atomic.ErrInputsNotSortedUnique.Error(),
		},
		"EVM input with amount 0": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.Ins = []atomic.EVMInput{
					{
						Address: testEthAddrs[0],
						Amount:  0,
						AssetID: snowtest.LUXAssetID,
						Nonce:   0,
					},
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrNoValueInput.Error(),
		},
		"non-unique EVM input before AP1": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.Ins = []atomic.EVMInput{tx.Ins[0], tx.Ins[0]}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: "",
		},
		"non-unique EVM input after AP1": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.Ins = []atomic.EVMInput{tx.Ins[0], tx.Ins[0]}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase1,
			expectedErr: atomic.ErrInputsNotSortedUnique.Error(),
		},
		"non-LUX input Apricot Phase 6": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.Ins = []atomic.EVMInput{
					{
						Address: testEthAddrs[0],
						Amount:  1,
						AssetID: ids.GenerateTestID(),
						Nonce:   0,
					},
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase6,
			expectedErr: "",
		},
		"non-LUX output Apricot Phase 6": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.ExportedOutputs = []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: ids.GenerateTestID()},
						Out: &secp256k1fx.TransferOutput{
							Amt: exportAmount,
							OutputOwners: secp256k1fx.OutputOwners{
								Locktime:  0,
								Threshold: 1,
								Addrs:     []ids.ShortID{testShortIDAddrs[0]},
							},
						},
					},
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase6,
			expectedErr: "",
		},
		"non-LUX input Banff": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.Ins = []atomic.EVMInput{
					{
						Address: testEthAddrs[0],
						Amount:  1,
						AssetID: ids.GenerateTestID(),
						Nonce:   0,
					},
				}
				return &tx
			},
			ctx:         ctx,
			rules:       banffRules,
			expectedErr: atomic.ErrExportNonLUXInputBanff.Error(),
		},
		"non-LUX output Banff": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *exportTx
				tx.ExportedOutputs = []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: ids.GenerateTestID()},
						Out: &secp256k1fx.TransferOutput{
							Amt: exportAmount,
							OutputOwners: secp256k1fx.OutputOwners{
								Locktime:  0,
								Threshold: 1,
								Addrs:     []ids.ShortID{testShortIDAddrs[0]},
							},
						},
					},
				}
				return &tx
			},
			ctx:         ctx,
			rules:       banffRules,
			expectedErr: atomic.ErrExportNonLUXOutputBanff.Error(),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			executeTxVerifyTest(t, test)
		})
	}
}

// Note: this is a brittle test to ensure that the gas cost of a transaction does
// not change
func TestExportTxGasCost(t *testing.T) {
	luxAssetID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()
	xChainID := ids.GenerateTestID()
	networkID := uint32(5)
	exportAmount := uint64(5000000)

	tests := map[string]struct {
		UnsignedExportTx *atomic.UnsignedExportTx
		Keys             [][]*secp256k1.PrivateKey

		BaseFee         *big.Int
		ExpectedGasUsed uint64
		ExpectedFee     uint64
		FixedFee        bool
	}{
		"simple export 1wei BaseFee": {
			UnsignedExportTx: &atomic.UnsignedExportTx{
				NetworkID:        networkID,
				BlockchainID:     chainID,
				DestinationChain: xChainID,
				Ins: []atomic.EVMInput{
					{
						Address: testEthAddrs[0],
						Amount:  exportAmount,
						AssetID: luxAssetID,
						Nonce:   0,
					},
				},
				ExportedOutputs: []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: luxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: exportAmount,
							OutputOwners: secp256k1fx.OutputOwners{
								Locktime:  0,
								Threshold: 1,
								Addrs:     []ids.ShortID{testShortIDAddrs[0]},
							},
						},
					},
				},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0]}},
			ExpectedGasUsed: 1230,
			ExpectedFee:     1,
			BaseFee:         big.NewInt(1),
		},
		"simple export 1wei BaseFee + fixed fee": {
			UnsignedExportTx: &atomic.UnsignedExportTx{
				NetworkID:        networkID,
				BlockchainID:     chainID,
				DestinationChain: xChainID,
				Ins: []atomic.EVMInput{
					{
						Address: testEthAddrs[0],
						Amount:  exportAmount,
						AssetID: luxAssetID,
						Nonce:   0,
					},
				},
				ExportedOutputs: []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: luxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: exportAmount,
							OutputOwners: secp256k1fx.OutputOwners{
								Locktime:  0,
								Threshold: 1,
								Addrs:     []ids.ShortID{testShortIDAddrs[0]},
							},
						},
					},
				},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0]}},
			ExpectedGasUsed: 11230,
			ExpectedFee:     1,
			BaseFee:         big.NewInt(1),
			FixedFee:        true,
		},
		"simple export 25Gwei BaseFee": {
			UnsignedExportTx: &atomic.UnsignedExportTx{
				NetworkID:        networkID,
				BlockchainID:     chainID,
				DestinationChain: xChainID,
				Ins: []atomic.EVMInput{
					{
						Address: testEthAddrs[0],
						Amount:  exportAmount,
						AssetID: luxAssetID,
						Nonce:   0,
					},
				},
				ExportedOutputs: []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: luxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: exportAmount,
							OutputOwners: secp256k1fx.OutputOwners{
								Locktime:  0,
								Threshold: 1,
								Addrs:     []ids.ShortID{testShortIDAddrs[0]},
							},
						},
					},
				},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0]}},
			ExpectedGasUsed: 1230,
			ExpectedFee:     30750,
			BaseFee:         big.NewInt(25 * utils.GWei),
		},
		"simple export 225Gwei BaseFee": {
			UnsignedExportTx: &atomic.UnsignedExportTx{
				NetworkID:        networkID,
				BlockchainID:     chainID,
				DestinationChain: xChainID,
				Ins: []atomic.EVMInput{
					{
						Address: testEthAddrs[0],
						Amount:  exportAmount,
						AssetID: luxAssetID,
						Nonce:   0,
					},
				},
				ExportedOutputs: []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: luxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: exportAmount,
							OutputOwners: secp256k1fx.OutputOwners{
								Locktime:  0,
								Threshold: 1,
								Addrs:     []ids.ShortID{testShortIDAddrs[0]},
							},
						},
					},
				},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0]}},
			ExpectedGasUsed: 1230,
			ExpectedFee:     276750,
			BaseFee:         big.NewInt(225 * utils.GWei),
		},
		"complex export 25Gwei BaseFee": {
			UnsignedExportTx: &atomic.UnsignedExportTx{
				NetworkID:        networkID,
				BlockchainID:     chainID,
				DestinationChain: xChainID,
				Ins: []atomic.EVMInput{
					{
						Address: testEthAddrs[0],
						Amount:  exportAmount,
						AssetID: luxAssetID,
						Nonce:   0,
					},
					{
						Address: testEthAddrs[1],
						Amount:  exportAmount,
						AssetID: luxAssetID,
						Nonce:   0,
					},
					{
						Address: testEthAddrs[2],
						Amount:  exportAmount,
						AssetID: luxAssetID,
						Nonce:   0,
					},
				},
				ExportedOutputs: []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: luxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: exportAmount * 3,
							OutputOwners: secp256k1fx.OutputOwners{
								Locktime:  0,
								Threshold: 1,
								Addrs:     []ids.ShortID{testShortIDAddrs[0]},
							},
						},
					},
				},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0], testKeys[0], testKeys[0]}},
			ExpectedGasUsed: 3366,
			ExpectedFee:     84150,
			BaseFee:         big.NewInt(25 * utils.GWei),
		},
		"complex export 225Gwei BaseFee": {
			UnsignedExportTx: &atomic.UnsignedExportTx{
				NetworkID:        networkID,
				BlockchainID:     chainID,
				DestinationChain: xChainID,
				Ins: []atomic.EVMInput{
					{
						Address: testEthAddrs[0],
						Amount:  exportAmount,
						AssetID: luxAssetID,
						Nonce:   0,
					},
					{
						Address: testEthAddrs[1],
						Amount:  exportAmount,
						AssetID: luxAssetID,
						Nonce:   0,
					},
					{
						Address: testEthAddrs[2],
						Amount:  exportAmount,
						AssetID: luxAssetID,
						Nonce:   0,
					},
				},
				ExportedOutputs: []*lux.TransferableOutput{
					{
						Asset: lux.Asset{ID: luxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: exportAmount * 3,
							OutputOwners: secp256k1fx.OutputOwners{
								Locktime:  0,
								Threshold: 1,
								Addrs:     []ids.ShortID{testShortIDAddrs[0]},
							},
						},
					},
				},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0], testKeys[0], testKeys[0]}},
			ExpectedGasUsed: 3366,
			ExpectedFee:     757350,
			BaseFee:         big.NewInt(225 * utils.GWei),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			tx := &atomic.Tx{UnsignedAtomicTx: test.UnsignedExportTx}

			// Sign with the correct key
			if err := tx.Sign(atomic.Codec, test.Keys); err != nil {
				t.Fatal(err)
			}

			gasUsed, err := tx.GasUsed(test.FixedFee)
			if err != nil {
				t.Fatal(err)
			}
			if gasUsed != test.ExpectedGasUsed {
				t.Fatalf("Expected gasUsed to be %d, but found %d", test.ExpectedGasUsed, gasUsed)
			}

			fee, err := atomic.CalculateDynamicFee(gasUsed, test.BaseFee)
			if err != nil {
				t.Fatal(err)
			}
			if fee != test.ExpectedFee {
				t.Fatalf("Expected fee to be %d, but found %d", test.ExpectedFee, fee)
			}
		})
	}
}

func TestNewExportTx(t *testing.T) {
	tests := []struct {
		fork               upgradetest.Fork
		bal                uint64
		expectedBurnedLUX uint64
	}{
		{
			fork:               upgradetest.NoUpgrades,
			bal:                44000000,
			expectedBurnedLUX: 1000000,
		},
		{
			fork:               upgradetest.ApricotPhase1,
			bal:                44000000,
			expectedBurnedLUX: 1000000,
		},
		{
			fork:               upgradetest.ApricotPhase2,
			bal:                43000000,
			expectedBurnedLUX: 1000000,
		},
		{
			fork:               upgradetest.ApricotPhase3,
			bal:                44446500,
			expectedBurnedLUX: 276750,
		},
		{
			fork:               upgradetest.ApricotPhase4,
			bal:                44446500,
			expectedBurnedLUX: 276750,
		},
		{
			fork:               upgradetest.ApricotPhase5,
			bal:                39946500,
			expectedBurnedLUX: 2526750,
		},
	}
	for _, test := range tests {
		t.Run(test.fork.String(), func(t *testing.T) {
			tvm := newVM(t, testVMConfig{
				fork: &test.fork,
			})
			defer func() {
				if err := tvm.vm.Shutdown(context.Background()); err != nil {
					t.Fatal(err)
				}
			}()

			parent := tvm.vm.LastAcceptedExtendedBlock()
			importAmount := uint64(50000000)
			utxoID := lux.UTXOID{TxID: ids.GenerateTestID()}

			utxo := &lux.UTXO{
				UTXOID: utxoID,
				Asset:  lux.Asset{ID: tvm.vm.ctx.LUXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: importAmount,
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1,
						Addrs:     []ids.ShortID{testKeys[0].Address()},
					},
				},
			}
			utxoBytes, err := atomic.Codec.Marshal(atomic.CodecVersion, utxo)
			if err != nil {
				t.Fatal(err)
			}

			xChainSharedMemory := tvm.atomicMemory.NewSharedMemory(tvm.vm.ctx.XChainID)
			inputID := utxo.InputID()
			if err := xChainSharedMemory.Apply(map[ids.ID]*luxatomic.Requests{tvm.vm.ctx.ChainID: {PutRequests: []*luxatomic.Element{{
				Key:   inputID[:],
				Value: utxoBytes,
				Traits: [][]byte{
					testKeys[0].Address().Bytes(),
				},
			}}}}); err != nil {
				t.Fatal(err)
			}

			tx, err := tvm.atomicVM.NewImportTx(tvm.vm.ctx.XChainID, testEthAddrs[0], initialBaseFee, []*secp256k1.PrivateKey{testKeys[0]})
			if err != nil {
				t.Fatal(err)
			}

			if err := tvm.atomicVM.AtomicMempool.AddLocalTx(tx); err != nil {
				t.Fatal(err)
			}

			require.Equal(t, commonEng.PendingTxs, tvm.WaitForEvent(context.Background()))

			blk, err := tvm.vm.BuildBlock(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if err := blk.Verify(context.Background()); err != nil {
				t.Fatal(err)
			}

			if err := tvm.vm.SetPreference(context.Background(), blk.ID()); err != nil {
				t.Fatal(err)
			}

			if err := blk.Accept(context.Background()); err != nil {
				t.Fatal(err)
			}

			parent = tvm.vm.LastAcceptedExtendedBlock()
			exportAmount := uint64(5000000)

			statedb, err := tvm.vm.blockChain.State()
			if err != nil {
				t.Fatal(err)
			}

			wrappedStateDB := extstate.New(statedb)
			tx, err = atomic.NewExportTx(tvm.vm.ctx, tvm.vm.currentRules(), wrappedStateDB, tvm.vm.ctx.LUXAssetID, exportAmount, tvm.vm.ctx.XChainID, testShortIDAddrs[0], initialBaseFee, []*secp256k1.PrivateKey{testKeys[0]})
			if err != nil {
				t.Fatal(err)
			}

			exportTx := tx.UnsignedAtomicTx

			backend := atomicvm.NewVerifierBackend(tvm.atomicVM, tvm.vm.currentRules())

			if err := backend.SemanticVerify(tx, parent, parent.GetEthBlock().BaseFee()); err != nil {
				t.Fatal("newExportTx created an invalid transaction", err)
			}

			burnedLUX, err := exportTx.Burned(tvm.vm.ctx.LUXAssetID)
			if err != nil {
				t.Fatal(err)
			}
			if burnedLUX != test.expectedBurnedLUX {
				t.Fatalf("burned wrong amount of LUX - expected %d burned %d", test.expectedBurnedLUX, burnedLUX)
			}

			commitBatch, err := tvm.vm.versiondb.CommitBatch()
			if err != nil {
				t.Fatalf("Failed to create commit batch for VM due to %s", err)
			}
			chainID, atomicRequests, err := exportTx.AtomicOps()
			if err != nil {
				t.Fatalf("Failed to accept export transaction due to: %s", err)
			}

			if err := tvm.vm.ctx.SharedMemory.Apply(map[ids.ID]*luxatomic.Requests{chainID: atomicRequests}, commitBatch); err != nil {
				t.Fatal(err)
			}

			statedb, err = tvm.vm.blockChain.State()
			if err != nil {
				t.Fatal(err)
			}
			wrappedStateDB = extstate.New(statedb)
			err = exportTx.EVMStateTransfer(tvm.vm.ctx, wrappedStateDB)
			if err != nil {
				t.Fatal(err)
			}

			addr := testKeys[0].EthAddress()
			if wrappedStateDB.GetBalance(addr).Cmp(uint256.NewInt(test.bal*units.Lux)) != 0 {
				t.Fatalf("address balance %s equal %s not %s", addr.String(), wrappedStateDB.GetBalance(addr), new(big.Int).SetUint64(test.bal*units.Lux))
			}
		})
	}
}

func TestNewExportTxMulticoin(t *testing.T) {
	tests := []struct {
		fork  upgradetest.Fork
		bal   uint64
		balmc uint64
	}{
		{
			fork:  upgradetest.NoUpgrades,
			bal:   49000000,
			balmc: 25000000,
		},
		{
			fork:  upgradetest.ApricotPhase1,
			bal:   49000000,
			balmc: 25000000,
		},
		{
			fork:  upgradetest.ApricotPhase2,
			bal:   48000000,
			balmc: 25000000,
		},
		{
			fork:  upgradetest.ApricotPhase3,
			bal:   48947900,
			balmc: 25000000,
		},
	}
	for _, test := range tests {
		t.Run(test.fork.String(), func(t *testing.T) {
			tvm := newVM(t, testVMConfig{
				fork: &test.fork,
			})
			defer func() {
				if err := tvm.vm.Shutdown(context.Background()); err != nil {
					t.Fatal(err)
				}
			}()

			parent := tvm.vm.LastAcceptedExtendedBlock()
			importAmount := uint64(50000000)
			utxoID := lux.UTXOID{TxID: ids.GenerateTestID()}

			utxo := &lux.UTXO{
				UTXOID: utxoID,
				Asset:  lux.Asset{ID: tvm.vm.ctx.LUXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: importAmount,
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1,
						Addrs:     []ids.ShortID{testKeys[0].Address()},
					},
				},
			}
			utxoBytes, err := atomic.Codec.Marshal(atomic.CodecVersion, utxo)
			if err != nil {
				t.Fatal(err)
			}

			inputID := utxo.InputID()

			tid := ids.GenerateTestID()
			importAmount2 := uint64(30000000)
			utxoID2 := lux.UTXOID{TxID: ids.GenerateTestID()}
			utxo2 := &lux.UTXO{
				UTXOID: utxoID2,
				Asset:  lux.Asset{ID: tid},
				Out: &secp256k1fx.TransferOutput{
					Amt: importAmount2,
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1,
						Addrs:     []ids.ShortID{testKeys[0].Address()},
					},
				},
			}
			utxoBytes2, err := atomic.Codec.Marshal(atomic.CodecVersion, utxo2)
			if err != nil {
				t.Fatal(err)
			}

			xChainSharedMemory := tvm.atomicMemory.NewSharedMemory(tvm.vm.ctx.XChainID)
			inputID2 := utxo2.InputID()
			if err := xChainSharedMemory.Apply(map[ids.ID]*luxatomic.Requests{tvm.vm.ctx.ChainID: {PutRequests: []*luxatomic.Element{
				{
					Key:   inputID[:],
					Value: utxoBytes,
					Traits: [][]byte{
						testKeys[0].Address().Bytes(),
					},
				},
				{
					Key:   inputID2[:],
					Value: utxoBytes2,
					Traits: [][]byte{
						testKeys[0].Address().Bytes(),
					},
				},
			}}}); err != nil {
				t.Fatal(err)
			}

			tx, err := tvm.atomicVM.NewImportTx(tvm.vm.ctx.XChainID, testEthAddrs[0], initialBaseFee, []*secp256k1.PrivateKey{testKeys[0]})
			if err != nil {
				t.Fatal(err)
			}

			if err := tvm.atomicVM.AtomicMempool.AddRemoteTx(tx); err != nil {
				t.Fatal(err)
			}

			require.Equal(t, commonEng.PendingTxs, tvm.WaitForEvent(context.Background()))

			blk, err := tvm.vm.BuildBlock(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if err := blk.Verify(context.Background()); err != nil {
				t.Fatal(err)
			}

			if err := tvm.vm.SetPreference(context.Background(), blk.ID()); err != nil {
				t.Fatal(err)
			}

			if err := blk.Accept(context.Background()); err != nil {
				t.Fatal(err)
			}

			parent = tvm.vm.LastAcceptedExtendedBlock()
			exportAmount := uint64(5000000)

			testKeys0Addr := testKeys[0].EthAddress()
			exportId, err := ids.ToShortID(testKeys0Addr[:])
			if err != nil {
				t.Fatal(err)
			}

			statedb, err := tvm.vm.blockChain.State()
			if err != nil {
				t.Fatal(err)
			}

			wrappedStateDB := extstate.New(statedb)
			tx, err = atomic.NewExportTx(tvm.vm.ctx, tvm.vm.currentRules(), wrappedStateDB, tid, exportAmount, tvm.vm.ctx.XChainID, exportId, initialBaseFee, []*secp256k1.PrivateKey{testKeys[0]})
			if err != nil {
				t.Fatal(err)
			}

			exportTx := tx.UnsignedAtomicTx
			backend := atomicvm.NewVerifierBackend(tvm.atomicVM, tvm.vm.currentRules())

			if err := backend.SemanticVerify(tx, parent, parent.GetEthBlock().BaseFee()); err != nil {
				t.Fatal("newExportTx created an invalid transaction", err)
			}

			commitBatch, err := tvm.vm.versiondb.CommitBatch()
			if err != nil {
				t.Fatalf("Failed to create commit batch for VM due to %s", err)
			}
			chainID, atomicRequests, err := exportTx.AtomicOps()
			if err != nil {
				t.Fatalf("Failed to accept export transaction due to: %s", err)
			}

			if err := tvm.vm.ctx.SharedMemory.Apply(map[ids.ID]*luxatomic.Requests{chainID: atomicRequests}, commitBatch); err != nil {
				t.Fatal(err)
			}

			statedb, err = tvm.vm.blockChain.State()
			if err != nil {
				t.Fatal(err)
			}
			wrappedStateDB = extstate.New(statedb)
			err = exportTx.EVMStateTransfer(tvm.vm.ctx, wrappedStateDB)
			if err != nil {
				t.Fatal(err)
			}

			addr := testKeys[0].EthAddress()
			if wrappedStateDB.GetBalance(addr).Cmp(uint256.NewInt(test.bal*units.Lux)) != 0 {
				t.Fatalf("address balance %s equal %s not %s", addr.String(), wrappedStateDB.GetBalance(addr), new(big.Int).SetUint64(test.bal*units.Lux))
			}
			if wrappedStateDB.GetBalanceMultiCoin(addr, common.BytesToHash(tid[:])).Cmp(new(big.Int).SetUint64(test.balmc)) != 0 {
				t.Fatalf("address balance multicoin %s equal %s not %s", addr.String(), wrappedStateDB.GetBalanceMultiCoin(addr, common.BytesToHash(tid[:])), new(big.Int).SetUint64(test.balmc))
			}
		})
	}
}
