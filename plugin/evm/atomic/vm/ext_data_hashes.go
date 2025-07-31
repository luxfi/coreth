// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	_ "embed"
	"encoding/json"

	"github.com/luxfi/geth/common"
)

var (
	//go:embed testnet_ext_data_hashes.json
	rawTestnetExtDataHashes []byte
	testnetExtDataHashes    map[common.Hash]common.Hash

	//go:embed mainnet_ext_data_hashes.json
	rawMainnetExtDataHashes []byte
	mainnetExtDataHashes    map[common.Hash]common.Hash
)

func init() {
	if err := json.Unmarshal(rawTestnetExtDataHashes, &testnetExtDataHashes); err != nil {
		panic(err)
	}
	rawTestnetExtDataHashes = nil
	if err := json.Unmarshal(rawMainnetExtDataHashes, &mainnetExtDataHashes); err != nil {
		panic(err)
	}
	rawMainnetExtDataHashes = nil
}
