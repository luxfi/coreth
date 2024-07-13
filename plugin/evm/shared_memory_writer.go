// (c) 2023-2024, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"github.com/luxfi/coreth/precompile/precompileconfig"
	"github.com/luxfi/node/chains/atomic"
	"github.com/luxfi/node/ids"
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
