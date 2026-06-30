// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package header

import (
	"math/big"
	"testing"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/plugin/evm/upgrade/lp176"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/vm/components/gas"
	"github.com/stretchr/testify/require"
)

// Under activate-all-implicitly there are no per-upgrade gates left to test.
// BaseFee/EstimateNextBaseFee now ignore the config entirely; what matters is
// the parent header's encoded fee state and the child timestamp.

func TestBaseFee(t *testing.T) {
	tests := []struct {
		name      string
		parent    *types.Header
		timestamp uint64
		want      *big.Int
		wantErr   error
	}{
		{
			name: "genesis_block",
			parent: &types.Header{
				Number: big.NewInt(0),
			},
			want: big.NewInt(lp176.MinGasPrice),
		},
		{
			name: "invalid_timestamp",
			parent: &types.Header{
				Number: big.NewInt(1),
				Time:   1,
				Extra:  (&lp176.State{}).Bytes(),
			},
			timestamp: 0,
			wantErr:   errInvalidTimestamp,
		},
		{
			name: "first_fortuna_block",
			parent: &types.Header{
				Number: big.NewInt(1),
				Extra:  (&lp176.State{}).Bytes(),
			},
			timestamp: 1,
			want:      big.NewInt(lp176.MinGasPrice),
		},
		{
			name: "invalid_fee_state",
			parent: &types.Header{
				Number: big.NewInt(1),
				Extra:  make([]byte, lp176.StateSize-1),
			},
			wantErr: lp176.ErrStateInsufficientLength,
		},
		{
			name: "current_gas_price",
			parent: &types.Header{
				Number: big.NewInt(1),
				Extra: (&lp176.State{
					Gas: gas.State{
						Excess: 2_704_386_192, // 1_500_000 * ln(nLUX) * [lp176.TargetToPriceUpdateConversion]
					},
					TargetExcess: 13_605_152, // 2^25 * ln(1.5)
				}).Bytes(),
			},
			want: big.NewInt(1_000_000_002), // nLUX + 2 due to rounding
		},
		{
			name: "gas_price_decrease",
			parent: &types.Header{
				Number: big.NewInt(1),
				Extra: (&lp176.State{
					Gas: gas.State{
						Excess: 2_704_386_192,
					},
					TargetExcess: 13_605_152,
				}).Bytes(),
			},
			timestamp: 1,
			want:      big.NewInt(988_571_555), // e^((2_704_386_192 - 1_500_000) / 1_500_000 / [lp176.TargetToPriceUpdateConversion])
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			config := &extras.ChainConfig{}
			got, err := BaseFee(config, test.parent, test.timestamp)
			require.ErrorIs(err, test.wantErr)
			require.Equal(test.want, got)

			// Verify that [common.Big1] is not modified by [BaseFee].
			require.Equal(big.NewInt(1), common.Big1)
		})
	}
}

func TestEstimateNextBaseFee(t *testing.T) {
	tests := []struct {
		name      string
		parent    *types.Header
		timestamp uint64
		want      *big.Int
		wantErr   error
	}{
		{
			name: "mainnet",
			parent: &types.Header{
				Number: big.NewInt(1),
				Extra: (&lp176.State{
					Gas: gas.State{
						Excess: 2_704_386_192,
					},
					TargetExcess: 13_605_152,
				}).Bytes(),
			},
			timestamp: 1,
			want:      big.NewInt(988_571_555),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			config := &extras.ChainConfig{}
			got, err := EstimateNextBaseFee(config, test.parent, test.timestamp)
			require.ErrorIs(err, test.wantErr)
			require.Equal(test.want, got)
		})
	}
}
