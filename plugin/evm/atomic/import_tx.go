// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomic

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/upgrade/ap0"
	"github.com/luxfi/coreth/plugin/evm/upgrade/ap5"
	"github.com/holiman/uint256"

	"github.com/luxfi/node/chains/atomic"
	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/snow"
	"github.com/luxfi/node/utils"
	"github.com/luxfi/node/utils/crypto/secp256k1"
	"github.com/luxfi/node/utils/math"
	"github.com/luxfi/node/utils/set"
	"github.com/luxfi/node/vms/components/lux"
	"github.com/luxfi/node/vms/components/verify"
	"github.com/luxfi/node/vms/secp256k1fx"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/log"
)

var (
	_                           UnsignedAtomicTx       = (*UnsignedImportTx)(nil)
	_                           secp256k1fx.UnsignedTx = (*UnsignedImportTx)(nil)
	ErrImportNonLUXInputBanff                         = errors.New("import input cannot contain non-LUX in Banff")
	ErrImportNonLUXOutputBanff                        = errors.New("import output cannot contain non-LUX in Banff")
	ErrNoImportInputs                                  = errors.New("tx has no imported inputs")
	ErrWrongChainID                                    = errors.New("tx has wrong chain ID")
	ErrNoEVMOutputs                                    = errors.New("tx has no EVM outputs")
	ErrInputsNotSortedUnique                           = errors.New("inputs not sorted and unique")
	ErrOutputsNotSortedUnique                          = errors.New("outputs not sorted and unique")
	ErrOutputsNotSorted                                = errors.New("tx outputs not sorted")
	errNilBaseFeeApricotPhase3                         = errors.New("nil base fee is invalid after apricotPhase3")
	errInsufficientFundsForFee                         = errors.New("insufficient LUX funds to pay transaction fee")
)

// UnsignedImportTx is an unsigned ImportTx
type UnsignedImportTx struct {
	Metadata
	// ID of the network on which this tx was issued
	NetworkID uint32 `serialize:"true" json:"networkID"`
	// ID of this blockchain.
	BlockchainID ids.ID `serialize:"true" json:"blockchainID"`
	// Which chain to consume the funds from
	SourceChain ids.ID `serialize:"true" json:"sourceChain"`
	// Inputs that consume UTXOs produced on the chain
	ImportedInputs []*lux.TransferableInput `serialize:"true" json:"importedInputs"`
	// Outputs
	Outs []EVMOutput `serialize:"true" json:"outputs"`
}

// InputUTXOs returns the UTXOIDs of the imported funds
func (utx *UnsignedImportTx) InputUTXOs() set.Set[ids.ID] {
	set := set.NewSet[ids.ID](len(utx.ImportedInputs))
	for _, in := range utx.ImportedInputs {
		set.Add(in.InputID())
	}
	return set
}

// Verify this transaction is well-formed
func (utx *UnsignedImportTx) Verify(
	ctx *snow.Context,
	rules extras.Rules,
) error {
	switch {
	case utx == nil:
		return ErrNilTx
	case len(utx.ImportedInputs) == 0:
		return ErrNoImportInputs
	case utx.NetworkID != ctx.NetworkID:
		return ErrWrongNetworkID
	case ctx.ChainID != utx.BlockchainID:
		return ErrWrongChainID
	case rules.IsApricotPhase3 && len(utx.Outs) == 0:
		return ErrNoEVMOutputs
	}

	// Make sure that the tx has a valid peer chain ID
	if rules.IsApricotPhase5 {
		// Note that SameSubnet verifies that [tx.SourceChain] isn't this
		// chain's ID
		if err := verify.SameSubnet(context.TODO(), ctx, utx.SourceChain); err != nil {
			return ErrWrongChainID
		}
	} else {
		if utx.SourceChain != ctx.XChainID {
			return ErrWrongChainID
		}
	}

	for _, out := range utx.Outs {
		if err := out.Verify(); err != nil {
			return fmt.Errorf("EVM Output failed verification: %w", err)
		}
		if rules.IsBanff && out.AssetID != ctx.LUXAssetID {
			return ErrImportNonLUXOutputBanff
		}
	}

	for _, in := range utx.ImportedInputs {
		if err := in.Verify(); err != nil {
			return fmt.Errorf("atomic input failed verification: %w", err)
		}
		if rules.IsBanff && in.AssetID() != ctx.LUXAssetID {
			return ErrImportNonLUXInputBanff
		}
	}
	if !utils.IsSortedAndUnique(utx.ImportedInputs) {
		return ErrInputsNotSortedUnique
	}

	if rules.IsApricotPhase2 {
		if !utils.IsSortedAndUnique(utx.Outs) {
			return ErrOutputsNotSortedUnique
		}
	} else if rules.IsApricotPhase1 {
		if !slices.IsSortedFunc(utx.Outs, EVMOutput.Compare) {
			return ErrOutputsNotSorted
		}
	}

	return nil
}

func (utx *UnsignedImportTx) GasUsed(fixedFee bool) (uint64, error) {
	var (
		cost = calcBytesCost(len(utx.Bytes()))
		err  error
	)
	for _, in := range utx.ImportedInputs {
		inCost, err := in.In.Cost()
		if err != nil {
			return 0, err
		}
		cost, err = math.Add(cost, inCost)
		if err != nil {
			return 0, err
		}
	}
	if fixedFee {
		cost, err = math.Add(cost, ap5.AtomicTxIntrinsicGas)
		if err != nil {
			return 0, err
		}
	}
	return cost, nil
}

// Amount of [assetID] burned by this transaction
func (utx *UnsignedImportTx) Burned(assetID ids.ID) (uint64, error) {
	var (
		spent uint64
		input uint64
		err   error
	)
	for _, out := range utx.Outs {
		if out.AssetID == assetID {
			spent, err = math.Add(spent, out.Amount)
			if err != nil {
				return 0, err
			}
		}
	}
	for _, in := range utx.ImportedInputs {
		if in.AssetID() == assetID {
			input, err = math.Add(input, in.Input().Amount())
			if err != nil {
				return 0, err
			}
		}
	}

	return math.Sub(input, spent)
}

// AtomicOps returns imported inputs spent on this transaction
// We spend imported UTXOs here rather than in verification because
// we don't want to remove an imported UTXO in verification
// only to have the transaction not be Accepted. This would be inconsistent.
// Recall that imported UTXOs are not kept in a versionDB.
func (utx *UnsignedImportTx) AtomicOps() (ids.ID, *atomic.Requests, error) {
	utxoIDs := make([][]byte, len(utx.ImportedInputs))
	for i, in := range utx.ImportedInputs {
		inputID := in.InputID()
		utxoIDs[i] = inputID[:]
	}
	return utx.SourceChain, &atomic.Requests{RemoveRequests: utxoIDs}, nil
}

// NewImportTx returns a new ImportTx
func NewImportTx(
	ctx *snow.Context,
	rules extras.Rules,
	time uint64,
	chainID ids.ID, // chain to import from
	to common.Address, // Address of recipient
	baseFee *big.Int, // fee to use post-AP3
	kc *secp256k1fx.Keychain, // Keychain to use for signing the atomic UTXOs
	atomicUTXOs []*lux.UTXO, // UTXOs to spend
) (*Tx, error) {
	importedInputs := []*lux.TransferableInput{}
	signers := [][]*secp256k1.PrivateKey{}

	importedAmount := make(map[ids.ID]uint64)
	for _, utxo := range atomicUTXOs {
		inputIntf, utxoSigners, err := kc.Spend(utxo.Out, time)
		if err != nil {
			continue
		}
		input, ok := inputIntf.(lux.TransferableIn)
		if !ok {
			continue
		}
		aid := utxo.AssetID()
		importedAmount[aid], err = math.Add(importedAmount[aid], input.Amount())
		if err != nil {
			return nil, err
		}
		importedInputs = append(importedInputs, &lux.TransferableInput{
			UTXOID: utxo.UTXOID,
			Asset:  utxo.Asset,
			In:     input,
		})
		signers = append(signers, utxoSigners)
	}
	lux.SortTransferableInputsWithSigners(importedInputs, signers)
	importedLUXAmount := importedAmount[ctx.LUXAssetID]

	outs := make([]EVMOutput, 0, len(importedAmount))
	// This will create unique outputs (in the context of sorting)
	// since each output will have a unique assetID
	for assetID, amount := range importedAmount {
		// Skip the LUX amount since it is included separately to account for
		// the fee
		if assetID == ctx.LUXAssetID || amount == 0 {
			continue
		}
		outs = append(outs, EVMOutput{
			Address: to,
			Amount:  amount,
			AssetID: assetID,
		})
	}

	var (
		txFeeWithoutChange uint64
		txFeeWithChange    uint64
	)
	switch {
	case rules.IsApricotPhase3:
		if baseFee == nil {
			return nil, errNilBaseFeeApricotPhase3
		}
		utx := &UnsignedImportTx{
			NetworkID:      ctx.NetworkID,
			BlockchainID:   ctx.ChainID,
			Outs:           outs,
			ImportedInputs: importedInputs,
			SourceChain:    chainID,
		}
		tx := &Tx{UnsignedAtomicTx: utx}
		if err := tx.Sign(Codec, nil); err != nil {
			return nil, err
		}

		gasUsedWithoutChange, err := tx.GasUsed(rules.IsApricotPhase5)
		if err != nil {
			return nil, err
		}
		gasUsedWithChange := gasUsedWithoutChange + EVMOutputGas

		txFeeWithoutChange, err = CalculateDynamicFee(gasUsedWithoutChange, baseFee)
		if err != nil {
			return nil, err
		}
		txFeeWithChange, err = CalculateDynamicFee(gasUsedWithChange, baseFee)
		if err != nil {
			return nil, err
		}
	case rules.IsApricotPhase2:
		txFeeWithoutChange = ap0.AtomicTxFee
		txFeeWithChange = ap0.AtomicTxFee
	}

	// LUX output
	if importedLUXAmount < txFeeWithoutChange { // imported amount goes toward paying tx fee
		return nil, errInsufficientFundsForFee
	}

	if importedLUXAmount > txFeeWithChange {
		outs = append(outs, EVMOutput{
			Address: to,
			Amount:  importedLUXAmount - txFeeWithChange,
			AssetID: ctx.LUXAssetID,
		})
	}

	// If no outputs are produced, return an error.
	// Note: this can happen if there is exactly enough LUX to pay the
	// transaction fee, but no other funds to be imported.
	if len(outs) == 0 {
		return nil, ErrNoEVMOutputs
	}

	utils.Sort(outs)

	// Create the transaction
	utx := &UnsignedImportTx{
		NetworkID:      ctx.NetworkID,
		BlockchainID:   ctx.ChainID,
		Outs:           outs,
		ImportedInputs: importedInputs,
		SourceChain:    chainID,
	}
	tx := &Tx{UnsignedAtomicTx: utx}
	if err := tx.Sign(Codec, signers); err != nil {
		return nil, err
	}
	return tx, utx.Verify(ctx, rules)
}

// EVMStateTransfer performs the state transfer to increase the balances of
// accounts accordingly with the imported EVMOutputs
func (utx *UnsignedImportTx) EVMStateTransfer(ctx *snow.Context, state StateDB) error {
	for _, to := range utx.Outs {
		if to.AssetID == ctx.LUXAssetID {
			log.Debug("import_tx", "src", utx.SourceChain, "addr", to.Address, "amount", to.Amount, "assetID", "LUX")
			// If the asset is LUX, convert the input amount in nLUX to gWei by
			// multiplying by the x2c rate.
			amount := new(uint256.Int).Mul(uint256.NewInt(to.Amount), X2CRate)
			state.AddBalance(to.Address, amount)
		} else {
			log.Debug("import_tx", "src", utx.SourceChain, "addr", to.Address, "amount", to.Amount, "assetID", to.AssetID)
			amount := new(big.Int).SetUint64(to.Amount)
			state.AddBalanceMultiCoin(to.Address, common.Hash(to.AssetID), amount)
		}
	}
	return nil
}

func (utx *UnsignedImportTx) Visit(v Visitor) error { return v.ImportTx(utx) }
