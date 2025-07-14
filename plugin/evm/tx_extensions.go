// (c) 2021-2025, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"time"

	"github.com/luxfi/geth/core/types"
)

// txFirstSeenMap tracks when transactions were first seen
var txFirstSeenMap = make(map[string]time.Time)

// FirstSeen returns when the transaction was first seen
func FirstSeen(tx *types.Transaction) time.Time {
	hash := tx.Hash().String()
	if firstSeen, exists := txFirstSeenMap[hash]; exists {
		return firstSeen
	}
	// If not tracked, use current time
	now := time.Now()
	txFirstSeenMap[hash] = now
	return now
}