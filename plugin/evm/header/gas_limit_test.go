// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"math/big"
	"testing"

	"github.com/luxfi/node/utils/math"
	"github.com/luxfi/node/vms/components/gas"
	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/customtypes"
	"github.com/luxfi/coreth/plugin/evm/upgrade/lp176"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/stretchr/testify/require"
)

func TestGasLimit(t *testing.T) {
	tests := []struct {
		name      string
		upgrades  extras.NetworkUpgrades
		parent    *types.Header
		timestamp uint64
		want      uint64
		wantErr   error
	}{
		{
			name:     "mainnet_invalid_parent_header",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(1),
			},
			wantErr: lp176.ErrStateInsufficientLength,
		},
		{
			name:     "mainnet_initial_max_capacity",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			want: lp176.MinMaxCapacity,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			config := &extras.ChainConfig{
				NetworkUpgrades: test.upgrades,
			}
			got, err := GasLimit(config, test.parent, test.timestamp)
			require.ErrorIs(err, test.wantErr)
			require.Equal(test.want, got)
		})
	}
}

func TestVerifyGasUsed(t *testing.T) {
	tests := []struct {
		name     string
		upgrades extras.NetworkUpgrades
		parent   *types.Header
		header   *types.Header
		want     error
	}{
		{
			name:     "mainnet_massive_extra_gas_used",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			header: customtypes.WithHeaderExtra(
				&types.Header{
					Number: big.NewInt(300),
				},
				&customtypes.HeaderExtra{
					ExtDataGasUsed: new(big.Int).Lsh(common.Big1, 64),
				},
			),
			want: errInvalidExtraDataGasUsed,
		},
		{
			name:     "mainnet_gas_used_overflow",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			header: customtypes.WithHeaderExtra(
				&types.Header{
					Number:  big.NewInt(301),
					GasUsed: math.MaxUint[uint64](),
				},
				&customtypes.HeaderExtra{
					ExtDataGasUsed: common.Big1,
				},
			),
			want: math.ErrOverflow,
		},
		{
			name:     "mainnet_invalid_capacity",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(302),
			},
			header: &types.Header{
				Number: big.NewInt(303),
			},
			want: lp176.ErrStateInsufficientLength,
		},
		{
			name:     "mainnet_invalid_usage",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			header: &types.Header{
				Time:    1,
				GasUsed: lp176.MinMaxPerSecond + 1,
			},
			want: errInvalidGasUsed,
		},
		{
			name:     "mainnet_max_consumption",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			header: &types.Header{
				Time:    1,
				GasUsed: lp176.MinMaxPerSecond,
			},
			want: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &extras.ChainConfig{
				NetworkUpgrades: test.upgrades,
			}
			err := VerifyGasUsed(config, test.parent, test.header)
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestVerifyGasLimit(t *testing.T) {
	tests := []struct {
		name     string
		upgrades extras.NetworkUpgrades
		parent   *types.Header
		header   *types.Header
		want     error
	}{
		{
			name:     "mainnet_invalid_header",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(1),
			},
			header: &types.Header{},
			want:   lp176.ErrStateInsufficientLength,
		},
		{
			name:     "mainnet_invalid",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			header: &types.Header{
				GasLimit: lp176.MinMaxCapacity + 1,
			},
			want: errInvalidGasLimit,
		},
		{
			name:     "mainnet_valid",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			header: &types.Header{
				GasLimit: lp176.MinMaxCapacity,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &extras.ChainConfig{
				NetworkUpgrades: test.upgrades,
			}
			err := VerifyGasLimit(config, test.parent, test.header)
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestGasCapacity(t *testing.T) {
	tests := []struct {
		name      string
		upgrades  extras.NetworkUpgrades
		parent    *types.Header
		timestamp uint64
		want      uint64
		wantErr   error
	}{
		{
			name:     "mainnet_invalid_header",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(1),
			},
			wantErr: lp176.ErrStateInsufficientLength,
		},
		{
			name:     "mainnet_after_1s",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			timestamp: 1,
			want:      lp176.MinMaxPerSecond,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			config := &extras.ChainConfig{
				NetworkUpgrades: test.upgrades,
			}
			got, err := GasCapacity(config, test.parent, test.timestamp)
			require.ErrorIs(err, test.wantErr)
			require.Equal(test.want, got)
		})
	}
}

func TestRemainingAtomicGasCapacity(t *testing.T) {
	tests := []struct {
		name     string
		upgrades extras.NetworkUpgrades
		parent   *types.Header
		header   *types.Header
		want     uint64
		wantErr  error
	}{
		{
			name:     "mainnet_invalid_header",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(1),
			},
			header:  &types.Header{},
			wantErr: lp176.ErrStateInsufficientLength,
		},
		{
			name:     "mainnet_negative_capacity",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			header: &types.Header{
				GasUsed: 1,
			},
			wantErr: gas.ErrInsufficientCapacity,
		},
		{
			name:     "mainnet_with_capacity",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			header: &types.Header{
				Time:    1,
				GasUsed: 1,
			},
			want: lp176.MinMaxPerSecond - 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			config := &extras.ChainConfig{
				NetworkUpgrades: test.upgrades,
			}
			got, err := RemainingAtomicGasCapacity(config, test.parent, test.header)
			require.ErrorIs(err, test.wantErr)
			require.Equal(test.want, got)
		})
	}
}
