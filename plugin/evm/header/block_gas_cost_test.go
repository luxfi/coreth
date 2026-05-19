// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"math/big"
	"testing"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/customtypes"
	"github.com/luxfi/coreth/plugin/evm/upgrade/blockgascost"
	"github.com/luxfi/coreth/plugin/evm/upgrade/atomicgas"
	"github.com/luxfi/geth/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockGasCost(t *testing.T) {
	tests := []struct {
		name       string
		upgrades   extras.NetworkUpgrades
		parentTime uint64
		parentCost *big.Int
		timestamp  uint64
		expected   *big.Int
	}{
		{
			name:       "mainnet",
			upgrades:   extras.TestChainConfig.NetworkUpgrades,
			parentTime: 10,
			parentCost: big.NewInt(blockgascost.MaxBlockGasCost),
			timestamp:  10 + blockgascost.TargetBlockRate + 1,
			expected:   big.NewInt(blockgascost.MaxBlockGasCost - atomicgas.BlockGasCostStep),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &extras.ChainConfig{
				NetworkUpgrades: test.upgrades,
			}
			parent := customtypes.WithHeaderExtra(
				&types.Header{
					Time: test.parentTime,
				},
				&customtypes.HeaderExtra{
					BlockGasCost: test.parentCost,
				},
			)

			assert.Equal(t, test.expected, BlockGasCost(
				config,
				parent,
				test.timestamp,
			))
		})
	}
}

func TestBlockGasCostWithStep(t *testing.T) {
	tests := []struct {
		name        string
		parentCost  *big.Int
		timeElapsed uint64
		expected    uint64
	}{
		{
			name:        "nil_parentCost",
			parentCost:  nil,
			timeElapsed: 0,
			expected:    blockgascost.MinBlockGasCost,
		},
		{
			name:        "at_target",
			parentCost:  big.NewInt(900_000),
			timeElapsed: blockgascost.TargetBlockRate,
			expected:    900_000,
		},
		{
			name:        "over_target",
			parentCost:  big.NewInt(blockgascost.MaxBlockGasCost),
			timeElapsed: 10,
			expected:    blockgascost.MaxBlockGasCost - (10-blockgascost.TargetBlockRate)*blockgascost.BlockGasCostStep,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, BlockGasCostWithStep(
				test.parentCost,
				blockgascost.BlockGasCostStep,
				test.timeElapsed,
			))
		})
	}
}

func TestEstimateRequiredTip(t *testing.T) {
	tests := []struct {
		name     string
		upgrades extras.NetworkUpgrades
		header   *types.Header
		want     *big.Int
		wantErr  error
	}{
		{
			name:     "nil_base_fee",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
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
			name:     "nil_block_gas_cost",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
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
			// nil BlockGasCost is now treated as 0 for imported blocks
			want: new(big.Int),
		},
		{
			name:     "success",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
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
			want: big.NewInt((101112 * 456) / (123 + 789)),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			config := &extras.ChainConfig{
				NetworkUpgrades: test.upgrades,
			}
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
