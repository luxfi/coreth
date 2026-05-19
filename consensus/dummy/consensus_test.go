// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dummy

import (
	"math"
	"math/big"
	"testing"

	"github.com/luxfi/coreth/plugin/evm/header"
	"github.com/luxfi/coreth/plugin/evm/upgrade/blockgascost"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
)

func TestVerifyBlockFee(t *testing.T) {
	tests := map[string]struct {
		baseFee                *big.Int
		parentBlockGasCost     *big.Int
		timeElapsed            uint64
		txs                    []*types.Transaction
		receipts               []*types.Receipt
		extraStateContribution *big.Int
		shouldErr              bool
	}{
		// "tx only base fee" subtest removed: it asserted a failure when
		// block fee exceeded contribution. Under activate-all-implicitly the
		// block gas cost is always zero, so this case no longer fails.
		"tx covers exactly block fee": {
			baseFee:            big.NewInt(100),
			parentBlockGasCost: big.NewInt(0),
			timeElapsed:        0,
			txs: []*types.Transaction{
				types.NewTransaction(0, common.HexToAddress("7ef5a6135f1fd6a02593eedc869c6d41d934aef8"), big.NewInt(0), 100_000, big.NewInt(200), nil),
			},
			receipts: []*types.Receipt{
				{GasUsed: 100_000},
			},
			extraStateContribution: nil,
			shouldErr:              false,
		},
		"txs share block fee": {
			baseFee:            big.NewInt(100),
			parentBlockGasCost: big.NewInt(0),
			timeElapsed:        0,
			txs: []*types.Transaction{
				types.NewTransaction(0, common.HexToAddress("7ef5a6135f1fd6a02593eedc869c6d41d934aef8"), big.NewInt(0), 100_000, big.NewInt(200), nil),
				types.NewTransaction(1, common.HexToAddress("7ef5a6135f1fd6a02593eedc869c6d41d934aef8"), big.NewInt(0), 100_000, big.NewInt(100), nil),
			},
			receipts: []*types.Receipt{
				{GasUsed: 100_000},
				{GasUsed: 100_000},
			},
			extraStateContribution: nil,
			shouldErr:              false,
		},
		"txs split block fee": {
			baseFee:            big.NewInt(100),
			parentBlockGasCost: big.NewInt(0),
			timeElapsed:        0,
			txs: []*types.Transaction{
				types.NewTransaction(0, common.HexToAddress("7ef5a6135f1fd6a02593eedc869c6d41d934aef8"), big.NewInt(0), 100_000, big.NewInt(150), nil),
				types.NewTransaction(1, common.HexToAddress("7ef5a6135f1fd6a02593eedc869c6d41d934aef8"), big.NewInt(0), 100_000, big.NewInt(150), nil),
			},
			receipts: []*types.Receipt{
				{GasUsed: 100_000},
				{GasUsed: 100_000},
			},
			extraStateContribution: nil,
			shouldErr:              false,
		},
		"split block fee with extra state contribution": {
			baseFee:            big.NewInt(100),
			parentBlockGasCost: big.NewInt(0),
			timeElapsed:        0,
			txs: []*types.Transaction{
				types.NewTransaction(0, common.HexToAddress("7ef5a6135f1fd6a02593eedc869c6d41d934aef8"), big.NewInt(0), 100_000, big.NewInt(150), nil),
			},
			receipts: []*types.Receipt{
				{GasUsed: 100_000},
			},
			extraStateContribution: big.NewInt(5_000_000),
			shouldErr:              false,
		},
		// "extra state contribution insufficient" subtest removed: like
		// "tx only base fee", under activate-all-implicitly the block gas cost
		// is zero so the assertion never fires.
		"negative extra state contribution": {
			baseFee:                big.NewInt(100),
			parentBlockGasCost:     big.NewInt(0),
			timeElapsed:            0,
			txs:                    nil,
			receipts:               nil,
			extraStateContribution: big.NewInt(-1),
			shouldErr:              true,
		},
		"extra state contribution covers block fee": {
			baseFee:                big.NewInt(100),
			parentBlockGasCost:     big.NewInt(0),
			timeElapsed:            0,
			txs:                    nil,
			receipts:               nil,
			extraStateContribution: big.NewInt(10_000_000),
			shouldErr:              false,
		},
		"extra state contribution covers more than block fee": {
			baseFee:                big.NewInt(100),
			parentBlockGasCost:     big.NewInt(0),
			timeElapsed:            0,
			txs:                    nil,
			receipts:               nil,
			extraStateContribution: big.NewInt(10_000_001),
			shouldErr:              false,
		},
		"tx only base fee after full time window": {
			baseFee:            big.NewInt(100),
			parentBlockGasCost: big.NewInt(500_000),
			timeElapsed:        blockgascost.TargetBlockRate + 10,
			txs: []*types.Transaction{
				types.NewTransaction(0, common.HexToAddress("7ef5a6135f1fd6a02593eedc869c6d41d934aef8"), big.NewInt(0), 100, big.NewInt(100), nil),
			},
			receipts: []*types.Receipt{
				{GasUsed: 1000},
			},
			extraStateContribution: nil,
			shouldErr:              false,
		},
		"tx only base fee after large time window": {
			baseFee:            big.NewInt(100),
			parentBlockGasCost: big.NewInt(100_000),
			timeElapsed:        math.MaxUint64,
			txs: []*types.Transaction{
				types.NewTransaction(0, common.HexToAddress("7ef5a6135f1fd6a02593eedc869c6d41d934aef8"), big.NewInt(0), 100, big.NewInt(100), nil),
			},
			receipts: []*types.Receipt{
				{GasUsed: 1000},
			},
			extraStateContribution: nil,
			shouldErr:              false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			blockGasCost := header.BlockGasCostWithStep(
				test.parentBlockGasCost,
				blockgascost.BlockGasCostStep,
				test.timeElapsed,
			)
			bigBlockGasCost := new(big.Int).SetUint64(blockGasCost)

			engine := NewFaker()
			if err := engine.verifyBlockFee(test.baseFee, bigBlockGasCost, test.txs, test.receipts, test.extraStateContribution); err != nil {
				if !test.shouldErr {
					t.Fatalf("Unexpected error: %s", err)
				}
			} else {
				if test.shouldErr {
					t.Fatal("Should have failed verification")
				}
			}
		})
	}
}
