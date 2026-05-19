// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"math/big"
	"testing"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/customtypes"
	"github.com/luxfi/coreth/plugin/evm/upgrade/blockgascost"
	"github.com/luxfi/geth/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockGasCost(t *testing.T) {
	// Under activate-all-implicitly BlockGasCost always returns 0: the legacy
	// AP4/AP5 block-gas-cost mechanism is gone, LP-176 capacity pricing is the
	// only fee model.
	tests := []struct {
		name       string
		parentTime uint64
		parentCost *big.Int
		timestamp  uint64
	}{
		{
			name:       "mainnet",
			parentTime: 10,
			parentCost: big.NewInt(blockgascost.MaxBlockGasCost),
			timestamp:  10 + blockgascost.TargetBlockRate + 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &extras.ChainConfig{}
			parent := customtypes.WithHeaderExtra(
				&types.Header{
					Time: test.parentTime,
				},
				&customtypes.HeaderExtra{
					BlockGasCost: test.parentCost,
				},
			)

			got := BlockGasCost(config, parent, test.timestamp)
			assert.Equal(t, int64(0), got.Int64())
		})
	}
}

func TestBlockGasCostWithStep(t *testing.T) {
	// Under activate-all-implicitly BlockGasCostWithStep always returns 0:
	// the legacy AP4/AP5 step-based block-gas-cost model is gone, replaced by
	// LP-176 capacity-based pricing.
	tests := []struct {
		name        string
		parentCost  *big.Int
		timeElapsed uint64
	}{
		{name: "nil_parentCost", parentCost: nil, timeElapsed: 0},
		{name: "at_target", parentCost: big.NewInt(900_000), timeElapsed: blockgascost.TargetBlockRate},
		{name: "over_target", parentCost: big.NewInt(blockgascost.MaxBlockGasCost), timeElapsed: 10},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, uint64(0), BlockGasCostWithStep(
				test.parentCost,
				blockgascost.BlockGasCostStep,
				test.timeElapsed,
			))
		})
	}
}

func TestEstimateRequiredTip(t *testing.T) {
	// Under activate-all-implicitly EstimateRequiredTip always returns 0 once
	// the prerequisite header fields are present: LP-176 sources the per-block
	// fee from BaseFee directly, no block-gas-cost surcharge.
	tests := []struct {
		name    string
		header  *types.Header
		want    *big.Int
		wantErr error
	}{
		{
			name: "nil_base_fee",
			header: customtypes.WithHeaderExtra(
				&types.Header{
					Number: big.NewInt(100),
				},
				&customtypes.HeaderExtra{
					ExtDataGasUsed: big.NewInt(1),
					BlockGasCost:   big.NewInt(1),
				},
			),
			wantErr: errBaseFeeNil,
		},
		{
			name: "nil_block_gas_cost",
			header: customtypes.WithHeaderExtra(
				&types.Header{
					Number:  big.NewInt(101),
					GasUsed: 1,
					BaseFee: big.NewInt(1),
				},
				&customtypes.HeaderExtra{
					ExtDataGasUsed: big.NewInt(1),
				},
			),
			want: new(big.Int),
		},
		{
			name: "success",
			header: customtypes.WithHeaderExtra(
				&types.Header{
					Number:  big.NewInt(104),
					GasUsed: 123,
					BaseFee: big.NewInt(456),
				},
				&customtypes.HeaderExtra{
					ExtDataGasUsed: big.NewInt(789),
					BlockGasCost:   big.NewInt(101112),
				},
			),
			want: new(big.Int),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			config := &extras.ChainConfig{}
			requiredTip, err := EstimateRequiredTip(config, test.header)
			require.ErrorIs(err, test.wantErr)
			if test.want == nil {
				require.Nil(requiredTip)
			} else {
				require.NotNil(requiredTip)
				require.Zero(test.want.Cmp(requiredTip), "expected %v, got %v", test.want, requiredTip)
			}
		})
	}
}
