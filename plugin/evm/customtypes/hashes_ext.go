// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customtypes

import (
	"github.com/luxfi/crypto"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/rlp"
)

// EmptyExtDataHash is the known hash of empty extdata bytes.
var EmptyExtDataHash = rlpHash([]byte(nil))

func rlpHash(x interface{}) (h common.Hash) {
	hw := crypto.NewKeccakState()
	rlp.Encode(hw, x)
	hw.Read(h[:])
	return h
}
