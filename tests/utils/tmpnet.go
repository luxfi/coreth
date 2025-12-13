// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"github.com/luxfi/node/config"
	"github.com/luxfi/node/tests/fixture/tmpnet"
)

var DefaultChainConfig = map[string]any{
	"log-level":         "debug",
	"warp-api-enabled":  true,
	"local-txs-enabled": true,
}

func NewTmpnetNetwork(owner string, nodes []*tmpnet.Node, flags tmpnet.Flags) *tmpnet.Network {
	defaultFlags := make(tmpnet.Flags)
	// Copy provided flags
	for k, v := range flags {
		defaultFlags[k] = v
	}
	// Set default proposer VM flag
	if _, exists := defaultFlags[config.ProposerVMUseCurrentHeightKey]; !exists {
		defaultFlags[config.ProposerVMUseCurrentHeightKey] = "true"
	}
	return &tmpnet.Network{
		Owner:        owner,
		DefaultFlags: defaultFlags,
		Nodes:        nodes,
	}
}
