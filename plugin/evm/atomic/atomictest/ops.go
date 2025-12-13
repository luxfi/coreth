// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomictest

import (
	luxatomic "github.com/luxfi/node/chains/atomic"
	"github.com/luxfi/ids"
	"github.com/luxfi/coreth/plugin/evm/atomic"
)

func ConvertToAtomicOps(tx *atomic.Tx) (map[ids.ID]*luxatomic.Requests, error) {
	id, reqs, err := tx.AtomicOps()
	if err != nil {
		return nil, err
	}
	return map[ids.ID]*luxatomic.Requests{id: reqs}, nil
}
