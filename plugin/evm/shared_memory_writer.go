// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"github.com/luxdefi/node/chains/atomic"
	"github.com/luxdefi/node/ids"
	"github.com/luxdefi/coreth/precompile/precompileconfig"
)

var _ precompileconfig.SharedMemoryWriter = &sharedMemoryWriter{}

type sharedMemoryWriter struct {
	requests map[ids.ID]*atomic.Requests
}

func NewSharedMemoryWriter() *sharedMemoryWriter {
	return &sharedMemoryWriter{
		requests: make(map[ids.ID]*atomic.Requests),
	}
}

func (s *sharedMemoryWriter) AddSharedMemoryRequests(chainID ids.ID, requests *atomic.Requests) {
	mergeAtomicOpsToMap(s.requests, chainID, requests)
}
