// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package params

import (
	"math/big"
	"testing"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/utils"
	"github.com/luxfi/node/upgrade/upgradetest"
	"github.com/stretchr/testify/require"
)

func TestSetEthUpgrades(t *testing.T) {
	// Since all Apricot phases (1-5) are always active on Lux mainnet,
	// Berlin and London are always enabled at genesis.
	// We only test the upgrades that have variable timestamps: Durango and Etna.
	genesisBlock := big.NewInt(0)
	genesisTimestamp := utils.NewUint64(initiallyActive)
	tests := []struct {
		fork     upgradetest.Fork
		expected *ChainConfig
	}{
		{
			fork: upgradetest.Durango,
			expected: &ChainConfig{
				HomesteadBlock:      genesisBlock,
				DAOForkBlock:        genesisBlock,
				DAOForkSupport:      true,
				EIP150Block:         genesisBlock,
				EIP155Block:         genesisBlock,
				EIP158Block:         genesisBlock,
				ByzantiumBlock:      genesisBlock,
				ConstantinopleBlock: genesisBlock,
				PetersburgBlock:     genesisBlock,
				IstanbulBlock:       genesisBlock,
				MuirGlacierBlock:    genesisBlock,
				BerlinBlock:         genesisBlock, // Always active (AP1-5 always on)
				LondonBlock:         genesisBlock, // Always active (AP1-5 always on)
				ShanghaiTime:        genesisTimestamp,
				CancunTime:          nil,
			},
		},
		{
			fork: upgradetest.Etna,
			expected: &ChainConfig{
				HomesteadBlock:      genesisBlock,
				DAOForkBlock:        genesisBlock,
				DAOForkSupport:      true,
				EIP150Block:         genesisBlock,
				EIP155Block:         genesisBlock,
				EIP158Block:         genesisBlock,
				ByzantiumBlock:      genesisBlock,
				ConstantinopleBlock: genesisBlock,
				PetersburgBlock:     genesisBlock,
				IstanbulBlock:       genesisBlock,
				MuirGlacierBlock:    genesisBlock,
				BerlinBlock:         genesisBlock, // Always active (AP1-5 always on)
				LondonBlock:         genesisBlock, // Always active (AP1-5 always on)
				ShanghaiTime:        genesisTimestamp,
				CancunTime:          genesisTimestamp,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.fork.String(), func(t *testing.T) {
			require := require.New(t)

			extraConfig := &extras.ChainConfig{
				NetworkUpgrades: extras.GetNetworkUpgrades(upgradetest.GetConfig(test.fork)),
			}
			actual := WithExtra(
				&ChainConfig{},
				extraConfig,
			)
			require.NoError(SetEthUpgrades(actual))

			expected := WithExtra(
				test.expected,
				extraConfig,
			)
			require.Equal(expected, actual)
		})
	}
}
