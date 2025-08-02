// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"math/big"
	"testing"

	"github.com/luxfi/coreth/core/extstate"
	"github.com/luxfi/coreth/plugin/evm/atomic"
	"github.com/luxfi/coreth/plugin/evm/atomic/vm"
	atomicvm "github.com/luxfi/coreth/plugin/evm/atomic/vm"
	"github.com/luxfi/coreth/plugin/evm/upgrade/ap0"
	"github.com/luxfi/coreth/utils"
	"github.com/luxfi/geth/common"
	"github.com/holiman/uint256"

	luxatomic "github.com/luxfi/node/chains/atomic"
	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/quasar/quasartest"
	"github.com/luxfi/node/upgrade/upgradetest"
	luxutils "github.com/luxfi/node/utils"
	"github.com/luxfi/node/utils/constants"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/node/utils/set"
	"github.com/luxfi/node/vms/components/lux"
	"github.com/luxfi/node/vms/secp256k1fx"
)

// createImportTxOptions adds a UTXO to shared memory and generates a list of import transactions sending this UTXO
// to each of the three test keys (conflicting transactions)
func createImportTxOptions(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) []*atomic.Tx {
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

	importTxs := make([]*atomic.Tx, 0, 3)
	for _, ethAddr := range testEthAddrs {
		importTx, err := vm.NewImportTx(vm.Ctx.XChainID, ethAddr, initialBaseFee, []*secp256k1.PrivateKey{testKeys[0]})
		if err != nil {
			t.Fatal(err)
		}
		importTxs = append(importTxs, importTx)
	}

	return importTxs
}

func TestImportTxVerify(t *testing.T) {
	ctx := quasartest.Context(t, quasartest.CChainID)

	var importAmount uint64 = 10000000
	txID := ids.GenerateTestID()
	importTx := &atomic.UnsignedImportTx{
		NetworkID:    ctx.NetworkID,
		BlockchainID: ctx.ChainID,
		SourceChain:  ctx.XChainID,
		ImportedInputs: []*lux.TransferableInput{
			{
				UTXOID: lux.UTXOID{
					TxID:        txID,
					OutputIndex: uint32(0),
				},
				Asset: lux.Asset{ID: ctx.LUXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt: importAmount,
					Input: secp256k1fx.Input{
						SigIndices: []uint32{0},
					},
				},
			},
			{
				UTXOID: lux.UTXOID{
					TxID:        txID,
					OutputIndex: uint32(1),
				},
				Asset: lux.Asset{ID: ctx.LUXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt: importAmount,
					Input: secp256k1fx.Input{
						SigIndices: []uint32{0},
					},
				},
			},
		},
		Outs: []atomic.EVMOutput{
			{
				Address: testEthAddrs[0],
				Amount:  importAmount - ap0.AtomicTxFee,
				AssetID: ctx.LUXAssetID,
			},
			{
				Address: testEthAddrs[1],
				Amount:  importAmount,
				AssetID: ctx.LUXAssetID,
			},
		},
	}

	// Sort the inputs and outputs to ensure the transaction is canonical
	luxutils.Sort(importTx.ImportedInputs)
	luxutils.Sort(importTx.Outs)

	tests := map[string]atomicTxVerifyTest{
		"nil tx": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				var importTx *atomic.UnsignedImportTx
				return importTx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrNilTx.Error(),
		},
		"valid import tx": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				return importTx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: "", // Expect this transaction to be valid in Apricot Phase 0
		},
		"valid import tx banff": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				return importTx
			},
			ctx:         ctx,
			rules:       banffRules,
			expectedErr: "", // Expect this transaction to be valid in Banff
		},
		"invalid network ID": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.NetworkID++
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrWrongNetworkID.Error(),
		},
		"invalid blockchain ID": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.BlockchainID = ids.GenerateTestID()
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrWrongChainID.Error(),
		},
		"P-chain source before AP5": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.SourceChain = constants.PlatformChainID
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrWrongChainID.Error(),
		},
		"P-chain source after AP5": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.SourceChain = constants.PlatformChainID
				return &tx
			},
			ctx:   ctx,
			rules: apricotRulesPhase5,
		},
		"invalid source chain ID": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.SourceChain = ids.GenerateTestID()
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase5,
			expectedErr: atomic.ErrWrongChainID.Error(),
		},
		"no inputs": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.ImportedInputs = nil
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrNoImportInputs.Error(),
		},
		"inputs sorted incorrectly": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.ImportedInputs = []*lux.TransferableInput{
					tx.ImportedInputs[1],
					tx.ImportedInputs[0],
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: atomic.ErrInputsNotSortedUnique.Error(),
		},
		"invalid input": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.ImportedInputs = []*lux.TransferableInput{
					tx.ImportedInputs[0],
					nil,
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: "atomic input failed verification",
		},
		"unsorted outputs phase 0 passes verification": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.Outs = []atomic.EVMOutput{
					tx.Outs[1],
					tx.Outs[0],
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: "",
		},
		"non-unique outputs phase 0 passes verification": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.Outs = []atomic.EVMOutput{
					tx.Outs[0],
					tx.Outs[0],
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: "",
		},
		"unsorted outputs phase 1 fails verification": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.Outs = []atomic.EVMOutput{
					tx.Outs[1],
					tx.Outs[0],
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase1,
			expectedErr: atomic.ErrOutputsNotSorted.Error(),
		},
		"non-unique outputs phase 1 passes verification": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.Outs = []atomic.EVMOutput{
					tx.Outs[0],
					tx.Outs[0],
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase1,
			expectedErr: "",
		},
		"outputs not sorted and unique phase 2 fails verification": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.Outs = []atomic.EVMOutput{
					tx.Outs[0],
					tx.Outs[0],
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase2,
			expectedErr: atomic.ErrOutputsNotSortedUnique.Error(),
		},
		"outputs not sorted phase 2 fails verification": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.Outs = []atomic.EVMOutput{
					tx.Outs[1],
					tx.Outs[0],
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase2,
			expectedErr: atomic.ErrOutputsNotSortedUnique.Error(),
		},
		"invalid EVMOutput fails verification": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.Outs = []atomic.EVMOutput{
					{
						Address: testEthAddrs[0],
						Amount:  0,
						AssetID: quasartest.LUXAssetID,
					},
				}
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase0,
			expectedErr: "EVM Output failed verification",
		},
		"no outputs apricot phase 3": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.Outs = nil
				return &tx
			},
			ctx:         ctx,
			rules:       apricotRulesPhase3,
			expectedErr: atomic.ErrNoEVMOutputs.Error(),
		},
		"non-LUX input Apricot Phase 6": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.ImportedInputs = []*lux.TransferableInput{
					{
						UTXOID: lux.UTXOID{
							TxID:        txID,
							OutputIndex: uint32(0),
						},
						Asset: lux.Asset{ID: ids.GenerateTestID()},
						In: &secp256k1fx.TransferInput{
							Amt: importAmount,
							Input: secp256k1fx.Input{
								SigIndices: []uint32{0},
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
		"non-LUX output Apricot Phase 6": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.Outs = []atomic.EVMOutput{
					{
						Address: importTx.Outs[0].Address,
						Amount:  importTx.Outs[0].Amount,
						AssetID: ids.GenerateTestID(),
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
				tx := *importTx
				tx.ImportedInputs = []*lux.TransferableInput{
					{
						UTXOID: lux.UTXOID{
							TxID:        txID,
							OutputIndex: uint32(0),
						},
						Asset: lux.Asset{ID: ids.GenerateTestID()},
						In: &secp256k1fx.TransferInput{
							Amt: importAmount,
							Input: secp256k1fx.Input{
								SigIndices: []uint32{0},
							},
						},
					},
				}
				return &tx
			},
			ctx:         ctx,
			rules:       banffRules,
			expectedErr: atomic.ErrImportNonLUXInputBanff.Error(),
		},
		"non-LUX output Banff": {
			generate: func(t *testing.T) atomic.UnsignedAtomicTx {
				tx := *importTx
				tx.Outs = []atomic.EVMOutput{
					{
						Address: importTx.Outs[0].Address,
						Amount:  importTx.Outs[0].Amount,
						AssetID: ids.GenerateTestID(),
					},
				}
				return &tx
			},
			ctx:         ctx,
			rules:       banffRules,
			expectedErr: atomic.ErrImportNonLUXOutputBanff.Error(),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			executeTxVerifyTest(t, test)
		})
	}
}

func TestNewImportTx(t *testing.T) {
	importAmount := uint64(5000000)
	// createNewImportLUXTx adds a UTXO to shared memory and then constructs a new import transaction
	// and checks that it has the correct fee for the base fee that has been used
	createNewImportLUXTx := func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
		txID := ids.GenerateTestID()
		_, err := addUTXO(sharedMemory, vm.Ctx, txID, 0, vm.Ctx.LUXAssetID, importAmount, testShortIDAddrs[0])
		if err != nil {
			t.Fatal(err)
		}

		tx, err := vm.NewImportTx(vm.Ctx.XChainID, testEthAddrs[0], initialBaseFee, []*secp256k1.PrivateKey{testKeys[0]})
		if err != nil {
			t.Fatal(err)
		}
		importTx := tx.UnsignedAtomicTx
		var actualFee uint64
		actualLUXBurned, err := importTx.Burned(vm.Ctx.LUXAssetID)
		if err != nil {
			t.Fatal(err)
		}
		rules := vm.CurrentRules()
		switch {
		case rules.IsApricotPhase3:
			actualCost, err := importTx.GasUsed(rules.IsApricotPhase5)
			if err != nil {
				t.Fatal(err)
			}
			actualFee, err = atomic.CalculateDynamicFee(actualCost, initialBaseFee)
			if err != nil {
				t.Fatal(err)
			}
		case rules.IsApricotPhase2:
			actualFee = 1000000
		default:
			actualFee = 0
		}

		if actualLUXBurned != actualFee {
			t.Fatalf("LUX burned (%d) != actual fee (%d)", actualLUXBurned, actualFee)
		}

		return tx
	}
	checkState := func(t *testing.T, vm *atomicvm.VM) {
		txs := vm.LastAcceptedExtendedBlock().GetBlockExtension().(atomic.AtomicBlockContext).AtomicTxs()
		if len(txs) != 1 {
			t.Fatalf("Expected one import tx to be in the last accepted block, but found %d", len(txs))
		}

		tx := txs[0]
		actualLUXBurned, err := tx.UnsignedAtomicTx.Burned(vm.Ctx.LUXAssetID)
		if err != nil {
			t.Fatal(err)
		}

		// Ensure that the UTXO has been removed from shared memory within Accept
		addrSet := set.Set[ids.ShortID]{}
		addrSet.Add(testShortIDAddrs[0])
		utxos, _, _, err := lux.GetAtomicUTXOs(vm.Ctx.SharedMemory, atomic.Codec, vm.Ctx.XChainID, addrSet, ids.ShortEmpty, ids.Empty, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(utxos) != 0 {
			t.Fatalf("Expected to find 0 UTXOs after accepting import transaction, but found %d", len(utxos))
		}

		// Ensure that the call to EVMStateTransfer correctly updates the balance of [addr]
		sdb, err := vm.Blockchain().State()
		if err != nil {
			t.Fatal(err)
		}

		expectedRemainingBalance := new(uint256.Int).Mul(
			uint256.NewInt(importAmount-actualLUXBurned), atomic.X2CRate)
		addr := testKeys[0].EthAddress()
		if actualBalance := sdb.GetBalance(addr); actualBalance.Cmp(expectedRemainingBalance) != 0 {
			t.Fatalf("address remaining balance %s equal %s not %s", addr.String(), actualBalance, expectedRemainingBalance)
		}
	}
	tests2 := map[string]atomicTxTest{
		"apricot phase 0": {
			setup:      createNewImportLUXTx,
			checkState: checkState,
			fork:       upgradetest.NoUpgrades,
		},
		"apricot phase 1": {
			setup:      createNewImportLUXTx,
			checkState: checkState,
			fork:       upgradetest.ApricotPhase1,
		},
		"apricot phase 2": {
			setup:      createNewImportLUXTx,
			checkState: checkState,
			fork:       upgradetest.ApricotPhase2,
		},
		"apricot phase 3": {
			setup:      createNewImportLUXTx,
			checkState: checkState,
			fork:       upgradetest.ApricotPhase3,
		},
	}

	for name, test := range tests2 {
		t.Run(name, func(t *testing.T) {
			executeTxTest(t, test)
		})
	}
}

// Note: this is a brittle test to ensure that the gas cost of a transaction does
// not change
func TestImportTxGasCost(t *testing.T) {
	luxAssetID := ids.GenerateTestID()
	antAssetID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()
	xChainID := ids.GenerateTestID()
	networkID := uint32(5)
	importAmount := uint64(5000000)

	tests := map[string]struct {
		UnsignedImportTx *atomic.UnsignedImportTx
		Keys             [][]*secp256k1.PrivateKey

		ExpectedGasUsed uint64
		ExpectedFee     uint64
		BaseFee         *big.Int
		FixedFee        bool
	}{
		"simple import": {
			UnsignedImportTx: &atomic.UnsignedImportTx{
				NetworkID:    networkID,
				BlockchainID: chainID,
				SourceChain:  xChainID,
				ImportedInputs: []*lux.TransferableInput{{
					UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
					Asset:  lux.Asset{ID: luxAssetID},
					In: &secp256k1fx.TransferInput{
						Amt:   importAmount,
						Input: secp256k1fx.Input{SigIndices: []uint32{0}},
					},
				}},
				Outs: []atomic.EVMOutput{{
					Address: testEthAddrs[0],
					Amount:  importAmount,
					AssetID: luxAssetID,
				}},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0]}},
			ExpectedGasUsed: 1230,
			ExpectedFee:     30750,
			BaseFee:         big.NewInt(25 * utils.GWei),
		},
		"simple import 1wei": {
			UnsignedImportTx: &atomic.UnsignedImportTx{
				NetworkID:    networkID,
				BlockchainID: chainID,
				SourceChain:  xChainID,
				ImportedInputs: []*lux.TransferableInput{{
					UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
					Asset:  lux.Asset{ID: luxAssetID},
					In: &secp256k1fx.TransferInput{
						Amt:   importAmount,
						Input: secp256k1fx.Input{SigIndices: []uint32{0}},
					},
				}},
				Outs: []atomic.EVMOutput{{
					Address: testEthAddrs[0],
					Amount:  importAmount,
					AssetID: luxAssetID,
				}},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0]}},
			ExpectedGasUsed: 1230,
			ExpectedFee:     1,
			BaseFee:         big.NewInt(1),
		},
		"simple import 1wei + fixed fee": {
			UnsignedImportTx: &atomic.UnsignedImportTx{
				NetworkID:    networkID,
				BlockchainID: chainID,
				SourceChain:  xChainID,
				ImportedInputs: []*lux.TransferableInput{{
					UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
					Asset:  lux.Asset{ID: luxAssetID},
					In: &secp256k1fx.TransferInput{
						Amt:   importAmount,
						Input: secp256k1fx.Input{SigIndices: []uint32{0}},
					},
				}},
				Outs: []atomic.EVMOutput{{
					Address: testEthAddrs[0],
					Amount:  importAmount,
					AssetID: luxAssetID,
				}},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0]}},
			ExpectedGasUsed: 11230,
			ExpectedFee:     1,
			BaseFee:         big.NewInt(1),
			FixedFee:        true,
		},
		"simple ANT import": {
			UnsignedImportTx: &atomic.UnsignedImportTx{
				NetworkID:    networkID,
				BlockchainID: chainID,
				SourceChain:  xChainID,
				ImportedInputs: []*lux.TransferableInput{
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: antAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
				},
				Outs: []atomic.EVMOutput{
					{
						Address: testEthAddrs[0],
						Amount:  importAmount,
						AssetID: antAssetID,
					},
				},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0]}, {testKeys[0]}},
			ExpectedGasUsed: 2318,
			ExpectedFee:     57950,
			BaseFee:         big.NewInt(25 * utils.GWei),
		},
		"complex ANT import": {
			UnsignedImportTx: &atomic.UnsignedImportTx{
				NetworkID:    networkID,
				BlockchainID: chainID,
				SourceChain:  xChainID,
				ImportedInputs: []*lux.TransferableInput{
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: antAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
				},
				Outs: []atomic.EVMOutput{
					{
						Address: testEthAddrs[0],
						Amount:  importAmount,
						AssetID: luxAssetID,
					},
					{
						Address: testEthAddrs[0],
						Amount:  importAmount,
						AssetID: antAssetID,
					},
				},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0]}, {testKeys[0]}},
			ExpectedGasUsed: 2378,
			ExpectedFee:     59450,
			BaseFee:         big.NewInt(25 * utils.GWei),
		},
		"multisig import": {
			UnsignedImportTx: &atomic.UnsignedImportTx{
				NetworkID:    networkID,
				BlockchainID: chainID,
				SourceChain:  xChainID,
				ImportedInputs: []*lux.TransferableInput{{
					UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
					Asset:  lux.Asset{ID: luxAssetID},
					In: &secp256k1fx.TransferInput{
						Amt:   importAmount,
						Input: secp256k1fx.Input{SigIndices: []uint32{0, 1}},
					},
				}},
				Outs: []atomic.EVMOutput{{
					Address: testEthAddrs[0],
					Amount:  importAmount,
					AssetID: luxAssetID,
				}},
			},
			Keys:            [][]*secp256k1.PrivateKey{{testKeys[0], testKeys[1]}},
			ExpectedGasUsed: 2234,
			ExpectedFee:     55850,
			BaseFee:         big.NewInt(25 * utils.GWei),
		},
		"large import": {
			UnsignedImportTx: &atomic.UnsignedImportTx{
				NetworkID:    networkID,
				BlockchainID: chainID,
				SourceChain:  xChainID,
				ImportedInputs: []*lux.TransferableInput{
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
					{
						UTXOID: lux.UTXOID{TxID: ids.GenerateTestID()},
						Asset:  lux.Asset{ID: luxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   importAmount,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					},
				},
				Outs: []atomic.EVMOutput{
					{
						Address: testEthAddrs[0],
						Amount:  importAmount * 10,
						AssetID: luxAssetID,
					},
				},
			},
			Keys: [][]*secp256k1.PrivateKey{
				{testKeys[0]},
				{testKeys[0]},
				{testKeys[0]},
				{testKeys[0]},
				{testKeys[0]},
				{testKeys[0]},
				{testKeys[0]},
				{testKeys[0]},
				{testKeys[0]},
				{testKeys[0]},
			},
			ExpectedGasUsed: 11022,
			ExpectedFee:     275550,
			BaseFee:         big.NewInt(25 * utils.GWei),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			tx := &atomic.Tx{UnsignedAtomicTx: test.UnsignedImportTx}

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

func TestImportTxSemanticVerify(t *testing.T) {
	tests := map[string]atomicTxTest{
		"UTXO not present during bootstrapping": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: lux.UTXOID{
							TxID: ids.GenerateTestID(),
						},
						Asset: lux.Asset{ID: vm.Ctx.LUXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   1,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{{
						Address: testEthAddrs[0],
						Amount:  1,
						AssetID: vm.Ctx.LUXAssetID,
					}},
				}}
				if err := tx.Sign(atomic.Codec, [][]*secp256k1.PrivateKey{{testKeys[0]}}); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			bootstrapping: true,
		},
		"UTXO not present": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: lux.UTXOID{
							TxID: ids.GenerateTestID(),
						},
						Asset: lux.Asset{ID: vm.Ctx.LUXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   1,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{{
						Address: testEthAddrs[0],
						Amount:  1,
						AssetID: vm.Ctx.LUXAssetID,
					}},
				}}
				if err := tx.Sign(atomic.Codec, [][]*secp256k1.PrivateKey{{testKeys[0]}}); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			semanticVerifyErr: "failed to fetch import UTXOs from",
		},
		"garbage UTXO": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				utxoID := lux.UTXOID{TxID: ids.GenerateTestID()}
				xChainSharedMemory := sharedMemory.NewSharedMemory(vm.Ctx.XChainID)
				inputID := utxoID.InputID()
				if err := xChainSharedMemory.Apply(map[ids.ID]*luxatomic.Requests{vm.Ctx.ChainID: {PutRequests: []*luxatomic.Element{{
					Key:   inputID[:],
					Value: []byte("hey there"),
					Traits: [][]byte{
						testShortIDAddrs[0].Bytes(),
					},
				}}}}); err != nil {
					t.Fatal(err)
				}

				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: utxoID,
						Asset:  lux.Asset{ID: vm.Ctx.LUXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   1,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{{
						Address: testEthAddrs[0],
						Amount:  1,
						AssetID: vm.Ctx.LUXAssetID,
					}},
				}}
				if err := tx.Sign(atomic.Codec, [][]*secp256k1.PrivateKey{{testKeys[0]}}); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			semanticVerifyErr: "failed to unmarshal UTXO",
		},
		"UTXO AssetID mismatch": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				txID := ids.GenerateTestID()
				expectedAssetID := ids.GenerateTestID()
				utxo, err := addUTXO(sharedMemory, vm.Ctx, txID, 0, expectedAssetID, 1, testShortIDAddrs[0])
				if err != nil {
					t.Fatal(err)
				}

				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: utxo.UTXOID,
						Asset:  lux.Asset{ID: vm.Ctx.LUXAssetID}, // Use a different assetID then the actual UTXO
						In: &secp256k1fx.TransferInput{
							Amt:   1,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{{
						Address: testEthAddrs[0],
						Amount:  1,
						AssetID: vm.Ctx.LUXAssetID,
					}},
				}}
				if err := tx.Sign(atomic.Codec, [][]*secp256k1.PrivateKey{{testKeys[0]}}); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			semanticVerifyErr: vm.ErrAssetIDMismatch.Error(),
		},
		"insufficient LUX funds": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				txID := ids.GenerateTestID()
				utxo, err := addUTXO(sharedMemory, vm.Ctx, txID, 0, vm.Ctx.LUXAssetID, 1, testShortIDAddrs[0])
				if err != nil {
					t.Fatal(err)
				}

				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: utxo.UTXOID,
						Asset:  lux.Asset{ID: vm.Ctx.LUXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   1,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{{
						Address: testEthAddrs[0],
						Amount:  2, // Produce more output than is consumed by the transaction
						AssetID: vm.Ctx.LUXAssetID,
					}},
				}}
				if err := tx.Sign(atomic.Codec, [][]*secp256k1.PrivateKey{{testKeys[0]}}); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			semanticVerifyErr: "import tx flow check failed due to",
		},
		"insufficient non-LUX funds": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				txID := ids.GenerateTestID()
				assetID := ids.GenerateTestID()
				utxo, err := addUTXO(sharedMemory, vm.Ctx, txID, 0, assetID, 1, testShortIDAddrs[0])
				if err != nil {
					t.Fatal(err)
				}

				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: utxo.UTXOID,
						Asset:  lux.Asset{ID: assetID},
						In: &secp256k1fx.TransferInput{
							Amt:   1,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{{
						Address: testEthAddrs[0],
						Amount:  2, // Produce more output than is consumed by the transaction
						AssetID: assetID,
					}},
				}}
				if err := tx.Sign(atomic.Codec, [][]*secp256k1.PrivateKey{{testKeys[0]}}); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			semanticVerifyErr: "import tx flow check failed due to",
		},
		"no signatures": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				txID := ids.GenerateTestID()
				utxo, err := addUTXO(sharedMemory, vm.Ctx, txID, 0, vm.Ctx.LUXAssetID, 1, testShortIDAddrs[0])
				if err != nil {
					t.Fatal(err)
				}

				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: utxo.UTXOID,
						Asset:  lux.Asset{ID: vm.Ctx.LUXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   1,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{{
						Address: testEthAddrs[0],
						Amount:  1,
						AssetID: vm.Ctx.LUXAssetID,
					}},
				}}
				if err := tx.Sign(atomic.Codec, nil); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			semanticVerifyErr: "import tx contained mismatched number of inputs/credentials",
		},
		"incorrect signature": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				txID := ids.GenerateTestID()
				utxo, err := addUTXO(sharedMemory, vm.Ctx, txID, 0, vm.Ctx.LUXAssetID, 1, testShortIDAddrs[0])
				if err != nil {
					t.Fatal(err)
				}

				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: utxo.UTXOID,
						Asset:  lux.Asset{ID: vm.Ctx.LUXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   1,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{{
						Address: testEthAddrs[0],
						Amount:  1,
						AssetID: vm.Ctx.LUXAssetID,
					}},
				}}
				// Sign the transaction with the incorrect key
				if err := tx.Sign(atomic.Codec, [][]*secp256k1.PrivateKey{{testKeys[1]}}); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			semanticVerifyErr: "import tx transfer failed verification",
		},
		"non-unique EVM Outputs": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				txID := ids.GenerateTestID()
				utxo, err := addUTXO(sharedMemory, vm.Ctx, txID, 0, vm.Ctx.LUXAssetID, 2, testShortIDAddrs[0])
				if err != nil {
					t.Fatal(err)
				}

				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: utxo.UTXOID,
						Asset:  lux.Asset{ID: vm.Ctx.LUXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   2,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{
						{
							Address: testEthAddrs[0],
							Amount:  1,
							AssetID: vm.Ctx.LUXAssetID,
						},
						{
							Address: testEthAddrs[0],
							Amount:  1,
							AssetID: vm.Ctx.LUXAssetID,
						},
					},
				}}
				if err := tx.Sign(atomic.Codec, [][]*secp256k1.PrivateKey{{testKeys[0]}}); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			fork:              upgradetest.ApricotPhase3,
			semanticVerifyErr: atomic.ErrOutputsNotSortedUnique.Error(),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			executeTxTest(t, test)
		})
	}
}

func TestImportTxEVMStateTransfer(t *testing.T) {
	assetID := ids.GenerateTestID()
	tests := map[string]atomicTxTest{
		"LUX UTXO": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				txID := ids.GenerateTestID()
				utxo, err := addUTXO(sharedMemory, vm.Ctx, txID, 0, vm.Ctx.LUXAssetID, 1, testShortIDAddrs[0])
				if err != nil {
					t.Fatal(err)
				}

				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: utxo.UTXOID,
						Asset:  lux.Asset{ID: vm.Ctx.LUXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt:   1,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{{
						Address: testEthAddrs[0],
						Amount:  1,
						AssetID: vm.Ctx.LUXAssetID,
					}},
				}}
				if err := tx.Sign(atomic.Codec, [][]*secp256k1.PrivateKey{{testKeys[0]}}); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			checkState: func(t *testing.T, vm *atomicvm.VM) {
				lastAcceptedBlock := vm.LastAcceptedExtendedBlock()

				sdb, err := vm.Blockchain().StateAt(lastAcceptedBlock.GetEthBlock().Root())
				if err != nil {
					t.Fatal(err)
				}

				luxBalance := sdb.GetBalance(testEthAddrs[0])
				if luxBalance.Cmp(atomic.X2CRate) != 0 {
					t.Fatalf("Expected LUX balance to be %d, found balance: %d", *atomic.X2CRate, luxBalance)
				}
			},
		},
		"non-LUX UTXO": {
			setup: func(t *testing.T, vm *atomicvm.VM, sharedMemory *luxatomic.Memory) *atomic.Tx {
				txID := ids.GenerateTestID()
				utxo, err := addUTXO(sharedMemory, vm.Ctx, txID, 0, assetID, 1, testShortIDAddrs[0])
				if err != nil {
					t.Fatal(err)
				}

				tx := &atomic.Tx{UnsignedAtomicTx: &atomic.UnsignedImportTx{
					NetworkID:    vm.Ctx.NetworkID,
					BlockchainID: vm.Ctx.ChainID,
					SourceChain:  vm.Ctx.XChainID,
					ImportedInputs: []*lux.TransferableInput{{
						UTXOID: utxo.UTXOID,
						Asset:  lux.Asset{ID: assetID},
						In: &secp256k1fx.TransferInput{
							Amt:   1,
							Input: secp256k1fx.Input{SigIndices: []uint32{0}},
						},
					}},
					Outs: []atomic.EVMOutput{{
						Address: testEthAddrs[0],
						Amount:  1,
						AssetID: assetID,
					}},
				}}
				if err := tx.Sign(atomic.Codec, [][]*secp256k1.PrivateKey{{testKeys[0]}}); err != nil {
					t.Fatal(err)
				}
				return tx
			},
			checkState: func(t *testing.T, vm *atomicvm.VM) {
				lastAcceptedBlock := vm.LastAcceptedExtendedBlock()

				statedb, err := vm.Blockchain().StateAt(lastAcceptedBlock.GetEthBlock().Root())
				if err != nil {
					t.Fatal(err)
				}

				wrappedStateDB := extstate.New(statedb)
				assetBalance := wrappedStateDB.GetBalanceMultiCoin(testEthAddrs[0], common.Hash(assetID))
				if assetBalance.Cmp(common.Big1) != 0 {
					t.Fatalf("Expected asset balance to be %d, found balance: %d", common.Big1, assetBalance)
				}
				luxBalance := wrappedStateDB.GetBalance(testEthAddrs[0])
				if luxBalance.Cmp(common.U2560) != 0 {
					t.Fatalf("Expected LUX balance to be 0, found balance: %d", luxBalance)
				}
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			executeTxTest(t, test)
		})
	}
}
