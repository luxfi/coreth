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
// Copyright 2017 The go-ethereum Authors
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
	"math"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/coreth/utils"
	ethparams "github.com/luxfi/geth/params"
)

// Note: This file tests config compatibility. With the simplified Lux chain config
// (only Genesis and Mainnet), we only need to verify that configs are compatible
// with themselves and that rules are correctly applied.

func TestCheckCompatible(t *testing.T) {
	type test struct {
		stored, new   *ChainConfig
		headBlock     uint64
		headTimestamp uint64
		wantErr       *ethparams.ConfigCompatError
	}
	tests := []test{
		// Test configs are compatible with themselves
		{stored: TestChainConfig, new: TestChainConfig, headBlock: 0, headTimestamp: 0, wantErr: nil},
		{stored: TestChainConfig, new: TestChainConfig, headBlock: 0, headTimestamp: uint64(time.Now().Unix()), wantErr: nil},
		{stored: TestChainConfig, new: TestChainConfig, headBlock: 100, wantErr: nil},
	}

	for _, test := range tests {
		err := CheckCompatible(test.stored, test.new, test.headBlock, test.headTimestamp)
		if !reflect.DeepEqual(err, test.wantErr) {
			t.Errorf("error mismatch:\nstored: %v\nnew: %v\nblockHeight: %v\nerr: %v\nwant: %v", test.stored, test.new, test.headBlock, err, test.wantErr)
		}
	}
}

func TestConfigRules(t *testing.T) {
	c := WithExtra(
		&ChainConfig{},
		&extras.ChainConfig{
			NetworkUpgrades: extras.NetworkUpgrades{
				CortinaBlockTimestamp: utils.NewUint64(500),
			},
		},
	)
	var stamp uint64
	if _, rulesEx := GetRules(c, big.NewInt(0), IsMergeTODO, stamp); rulesEx.IsCortina {
		t.Errorf("expected %v to not be cortina", stamp)
	}
	stamp = 500
	if _, rulesEx := GetRules(c, big.NewInt(0), IsMergeTODO, stamp); !rulesEx.IsCortina {
		t.Errorf("expected %v to be cortina", stamp)
	}
	stamp = math.MaxInt64
	if _, rulesEx := GetRules(c, big.NewInt(0), IsMergeTODO, stamp); !rulesEx.IsCortina {
		t.Errorf("expected %v to be cortina", stamp)
	}
}
