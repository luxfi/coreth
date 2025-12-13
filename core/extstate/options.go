// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extstate

import (
	"github.com/luxfi/coreth/utils"
)

// WorkerPool is a bounded worker pool for concurrent operations.
type WorkerPool struct {
	*utils.BoundedWorkers
}

// Done waits for all workers to finish.
func (wp *WorkerPool) Done() {
	wp.BoundedWorkers.Wait()
}

// NewWorkerPool creates a new worker pool with the specified number of workers.
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		BoundedWorkers: utils.NewBoundedWorkers(workers),
	}
}
