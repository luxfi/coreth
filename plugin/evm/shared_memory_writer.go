// (c) 2023, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"github.com/luxfi/node/chains/atomic"
	"github.com/luxfi/node/ids"
	"github.com/luxfi/coreth/precompile/precompileconfig"
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

// mergeAtomicOps merges atomic ops for [chainID] represented by [requests]
// to the [output] map provided.
func mergeAtomicOpsToMap(output map[ids.ID]*atomic.Requests, chainID ids.ID, requests *atomic.Requests) {
	if request, exists := output[chainID]; exists {
		request.PutRequests = append(request.PutRequests, requests.PutRequests...)
		request.RemoveRequests = append(request.RemoveRequests, requests.RemoveRequests...)
	} else {
		output[chainID] = requests
	}
}
