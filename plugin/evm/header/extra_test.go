// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"math/big"
	"testing"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/customtypes"
	"github.com/luxfi/coreth/plugin/evm/upgrade/lp176"
	"github.com/luxfi/coreth/utils"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/vm/vms/components/gas"
	"github.com/stretchr/testify/require"
)

func TestExtraPrefix(t *testing.T) {
	tests := []struct {
		name                string
		upgrades            extras.NetworkUpgrades
		parent              *types.Header
		header              *types.Header
		desiredTargetExcess *gas.Gas
		want                []byte
		wantErr             error
	}{
		{
			name:     "genesis_block",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			header: customtypes.WithHeaderExtra(
				&types.Header{
					Time:    1,
					GasUsed: 2,
				},
				&customtypes.HeaderExtra{
					ExtDataGasUsed: big.NewInt(1),
				},
			),
			desiredTargetExcess: (*gas.Gas)(utils.NewUint64(3)),
			want: (&lp176.State{
				Gas: gas.State{
					Capacity: lp176.MinMaxPerSecond - 3,
					Excess:   3,
				},
				TargetExcess: 3,
			}).Bytes(),
		},
		{
			name:     "mainnet_invalid_fee_state",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(1),
			},
			header:  &types.Header{},
			wantErr: lp176.ErrStateInsufficientLength,
		},
		{
			name:     "mainnet_invalid_gas_used",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(1),
				Extra:  (&lp176.State{}).Bytes(),
			},
			header: &types.Header{
				GasUsed: 1,
			},
			wantErr: gas.ErrInsufficientCapacity,
		},
		{
			name:     "mainnet_valid",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(1),
				Extra: (&lp176.State{
					Gas: gas.State{
						Capacity: lp176.MinMaxPerSecond,
					},
				}).Bytes(),
			},
			header: customtypes.WithHeaderExtra(
				&types.Header{
					Time:    1,
					GasUsed: 1,
				},
				&customtypes.HeaderExtra{
					ExtDataGasUsed: big.NewInt(1),
				},
			),
			want: (&lp176.State{
				Gas: gas.State{
					Capacity: 2*lp176.MinMaxPerSecond - 2,
					Excess:   2,
				},
			}).Bytes(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			config := &extras.ChainConfig{
				NetworkUpgrades: test.upgrades,
			}
			got, err := ExtraPrefix(config, test.parent, test.header, test.desiredTargetExcess)
			require.ErrorIs(err, test.wantErr)
			require.Equal(test.want, got)
		})
	}
}

func TestVerifyExtraPrefix(t *testing.T) {
	tests := []struct {
		name     string
		upgrades extras.NetworkUpgrades
		parent   *types.Header
		header   *types.Header
		wantErr  error
	}{
		{
			name:     "mainnet_invalid_header",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			header:   &types.Header{},
			wantErr:  lp176.ErrStateInsufficientLength,
		},
		{
			name:     "mainnet_invalid_gas_consumed",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			header: &types.Header{
				GasUsed: 1,
				Extra:   (&lp176.State{}).Bytes(),
			},
			wantErr: gas.ErrInsufficientCapacity,
		},
		{
			name:     "mainnet_wrong_fee_state",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			header: &types.Header{
				Time:    1,
				GasUsed: 1,
				Extra: (&lp176.State{
					Gas: gas.State{
						Capacity: lp176.MinMaxPerSecond - 1,
						Excess:   1,
					},
					TargetExcess: lp176.MaxTargetExcessDiff + 1, // Too much of a diff
				}).Bytes(),
			},
			wantErr: errIncorrectFeeState,
		},
		{
			name:     "mainnet_valid",
			upgrades: extras.TestChainConfig.NetworkUpgrades,
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			header: &types.Header{
				Time:    1,
				GasUsed: 1,
				Extra: (&lp176.State{
					Gas: gas.State{
						Capacity: lp176.MinMaxPerSecond - 1,
						Excess:   1,
					},
					TargetExcess: lp176.MaxTargetExcessDiff,
				}).Bytes(),
			},
			wantErr: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &extras.ChainConfig{
				NetworkUpgrades: test.upgrades,
			}
			err := VerifyExtraPrefix(config, test.parent, test.header)
			require.ErrorIs(t, err, test.wantErr)
		})
	}
}

func TestVerifyExtra(t *testing.T) {
	tests := []struct {
		name     string
		rules    extras.LuxRules
		extra    []byte
		expected error
	}{
		{
			name: "mainnet_valid_min",
			rules: extras.LuxRules{
				IsFortuna: true,
			},
			extra:    make([]byte, lp176.StateSize),
			expected: nil,
		},
		{
			name: "mainnet_valid_extra",
			rules: extras.LuxRules{
				IsFortuna: true,
			},
			extra:    make([]byte, lp176.StateSize+1),
			expected: nil,
		},
		{
			name: "mainnet_invalid",
			rules: extras.LuxRules{
				IsFortuna: true,
			},
			extra:    make([]byte, lp176.StateSize-1),
			expected: errInvalidExtraLength,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := VerifyExtra(test.rules, test.extra)
			require.ErrorIs(t, err, test.expected)
		})
	}
}

func TestPredicateBytesFromExtra(t *testing.T) {
	tests := []struct {
		name     string
		rules    extras.LuxRules
		extra    []byte
		expected []byte
	}{
		{
			name: "mainnet_empty_extra",
			rules: extras.LuxRules{
				IsFortuna: true,
			},
			extra:    nil,
			expected: nil,
		},
		{
			name: "mainnet_too_short",
			rules: extras.LuxRules{
				IsFortuna: true,
			},
			extra:    make([]byte, lp176.StateSize-1),
			expected: nil,
		},
		{
			name: "mainnet_empty_predicate",
			rules: extras.LuxRules{
				IsFortuna: true,
			},
			extra:    make([]byte, lp176.StateSize),
			expected: nil,
		},
		{
			name: "mainnet_non_empty_predicate",
			rules: extras.LuxRules{
				IsFortuna: true,
			},
			extra: []byte{
				lp176.StateSize: 5,
			},
			expected: []byte{5},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := PredicateBytesFromExtra(test.rules, test.extra)
			require.Equal(t, test.expected, got)
		})
	}
}

func TestSetPredicateBytesInExtra(t *testing.T) {
	tests := []struct {
		name      string
		rules     extras.LuxRules
		extra     []byte
		predicate []byte
		want      []byte
	}{
		{
			name: "mainnet_empty_extra_predicate",
			rules: extras.LuxRules{
				IsFortuna: true,
			},
			want: make([]byte, lp176.StateSize),
		},
		{
			name: "mainnet_extra_too_short",
			rules: extras.LuxRules{
				IsFortuna: true,
			},
			extra:     []byte{1},
			predicate: []byte{2},
			want: []byte{
				0:               1,
				lp176.StateSize: 2,
			},
		},
		{
			name: "mainnet_extra_too_long",
			rules: extras.LuxRules{
				IsFortuna: true,
			},
			extra: []byte{
				lp176.StateSize: 1,
			},
			predicate: []byte{2},
			want: []byte{
				lp176.StateSize: 2,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := SetPredicateBytesInExtra(test.rules, test.extra, test.predicate)
			require.Equal(t, test.want, got)
		})
	}
}
