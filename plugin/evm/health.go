// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"

	"github.com/luxfi/consensus/engine/chain/block"
)

// HealthCheck returns health status of this chain.
func (vm *VM) HealthCheck(context.Context) (block.HealthCheckResult, error) {
	return block.HealthCheckResult{
		Healthy: true,
		Details: nil,
	}, nil
}
