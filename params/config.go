// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
//
// This file is a derived work, based on the go-ethereum library whose original
// notices appear below.
//
// It is distributed under a license compatible with the licensing terms of the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********
// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package params

import (
	"math/big"

	"github.com/luxfi/node/upgrade"
	"github.com/luxfi/constants"
	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/utils"
	ethparams "github.com/luxfi/geth/params"
)

// Lux ChainIDs
var (
	// LuxMainnetChainID is the C-Chain mainnet chain ID
	LuxMainnetChainID = big.NewInt(96369)
	// LuxTestnetChainID is the C-Chain testnet chain ID
	LuxTestnetChainID = big.NewInt(96368)
	// LuxLocalChainID is the C-Chain local network chain ID
	LuxLocalChainID = big.NewInt(1337)
)

// Guarantees extras initialisation before a call to [params.ChainConfig.Rules].
var _ = gethInit()

var (
	TestChainConfig = WithExtra(
		&ChainConfig{
			ChainID:             big.NewInt(1),
			HomesteadBlock:      big.NewInt(0),
			DAOForkBlock:        big.NewInt(0),
			DAOForkSupport:      true,
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			MuirGlacierBlock:    big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			LondonBlock:         big.NewInt(0),
			ShanghaiTime:        utils.TimeToNewUint64(upgrade.GetConfig(constants.UnitTestID).DurangoTime),
			CancunTime:          utils.TimeToNewUint64(upgrade.GetConfig(constants.UnitTestID).EtnaTime),
			BlobScheduleConfig:  ethparams.DefaultBlobSchedule,
		},
		extras.TestChainConfig,
	)

	// Legacy test configs - all phases are now always active on mainnet,
	// so they all point to the same TestChainConfig for backward compatibility.
	TestLaunchConfig            = TestChainConfig
	TestApricotPhase1Config     = TestChainConfig
	TestApricotPhase2Config     = TestChainConfig
	TestApricotPhase3Config     = TestChainConfig
	TestApricotPhase4Config     = TestChainConfig
	TestApricotPhase5Config     = TestChainConfig
	TestApricotPhasePre6Config  = TestChainConfig
	TestApricotPhase6Config     = TestChainConfig
	TestApricotPhasePost6Config = TestChainConfig

	TestBanffChainConfig = WithExtra(
		&ChainConfig{
			ChainID:             big.NewInt(1),
			HomesteadBlock:      big.NewInt(0),
			DAOForkBlock:        big.NewInt(0),
			DAOForkSupport:      true,
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			MuirGlacierBlock:    big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			LondonBlock:         big.NewInt(0),
		},
		extras.TestBanffChainConfig,
	)

	TestCortinaChainConfig = WithExtra(
		&ChainConfig{
			ChainID:             big.NewInt(1),
			HomesteadBlock:      big.NewInt(0),
			DAOForkBlock:        big.NewInt(0),
			DAOForkSupport:      true,
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			MuirGlacierBlock:    big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			LondonBlock:         big.NewInt(0),
		},
		extras.TestCortinaChainConfig,
	)

	TestDurangoChainConfig = WithExtra(
		&ChainConfig{
			ChainID:             big.NewInt(1),
			HomesteadBlock:      big.NewInt(0),
			DAOForkBlock:        big.NewInt(0),
			DAOForkSupport:      true,
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			MuirGlacierBlock:    big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			LondonBlock:         big.NewInt(0),
			ShanghaiTime:        utils.NewUint64(0),
		},
		extras.TestDurangoChainConfig,
	)

	TestEtnaChainConfig = WithExtra(
		&ChainConfig{
			ChainID:             big.NewInt(1),
			HomesteadBlock:      big.NewInt(0),
			DAOForkBlock:        big.NewInt(0),
			DAOForkSupport:      true,
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			MuirGlacierBlock:    big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			LondonBlock:         big.NewInt(0),
			ShanghaiTime:        utils.NewUint64(0),
			CancunTime:          utils.NewUint64(0),
			BlobScheduleConfig:  ethparams.DefaultBlobSchedule,
		},
		extras.TestEtnaChainConfig,
	)

	TestFortunaChainConfig = WithExtra(
		&ChainConfig{
			ChainID:             big.NewInt(1),
			HomesteadBlock:      big.NewInt(0),
			DAOForkBlock:        big.NewInt(0),
			DAOForkSupport:      true,
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			MuirGlacierBlock:    big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			LondonBlock:         big.NewInt(0),
			ShanghaiTime:        utils.NewUint64(0),
			CancunTime:          utils.NewUint64(0),
			BlobScheduleConfig:  ethparams.DefaultBlobSchedule,
		},
		extras.TestFortunaChainConfig,
	)

	TestGraniteChainConfig = WithExtra(
		&ChainConfig{
			ChainID:             big.NewInt(1),
			HomesteadBlock:      big.NewInt(0),
			DAOForkBlock:        big.NewInt(0),
			DAOForkSupport:      true,
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			MuirGlacierBlock:    big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			LondonBlock:         big.NewInt(0),
			ShanghaiTime:        utils.NewUint64(0),
			CancunTime:          utils.NewUint64(0),
			BlobScheduleConfig:  ethparams.DefaultBlobSchedule,
		},
		extras.TestGraniteChainConfig,
	)

	TestRules = TestChainConfig.Rules(new(big.Int), IsMergeTODO, 0)
)

// ChainConfig is the core config which determines the blockchain settings.
//
// ChainConfig is stored in the database on a per block basis. This means
// that any network, identified by its genesis block, can have its own
// set of configuration options.
type ChainConfig = ethparams.ChainConfig

// Rules wraps ChainConfig and is merely syntactic sugar or can be used for functions
// that do not have or require information about the block.
//
// Rules is a one time interface meaning that it shouldn't be used in between transition
// phases.
type Rules = ethparams.Rules
