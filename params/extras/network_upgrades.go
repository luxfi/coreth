// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extras

import (
	"github.com/luxfi/upgrade"
)

// luxGenesisTimestamp pins the Lux C-Chain genesis-activation timestamp.
var luxGenesisTimestamp = uint64(upgrade.InitiallyActiveTime.Unix())

// LuxRules carries the only chain rule that still varies at runtime: whether the
// header is the genesis block. Every prior upgrade-gating field was deleted
// under the activate-all-implicitly directive — all upgrades are live from
// rule-set unconditionally.
type LuxRules struct {
	IsGenesis bool
}

// GetLuxRules returns the canonical Lux chain rules at the given block
// timestamp. The only runtime axis left is the genesis-block discriminator,
// used by historic-mainnet gas-accounting paths.
func (c *ChainConfig) GetLuxRules(timestamp uint64) LuxRules {
	return LuxRules{IsGenesis: timestamp <= luxGenesisTimestamp}
}
