// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomictest

import (
	"github.com/luxfi/coreth/plugin/evm/atomic"
	"github.com/luxfi/ids"
	luxatomic "github.com/luxfi/vm/chains/atomic"
)

func ConvertToAtomicOps(tx *atomic.Tx) (map[ids.ID]*luxatomic.Requests, error) {
	id, reqs, err := tx.AtomicOps()
	if err != nil {
		return nil, err
	}
	return map[ids.ID]*luxatomic.Requests{id: reqs}, nil
}
