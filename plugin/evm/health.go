// (c) 2019-2020, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import "context"

// Health returns nil if this chain is healthy.
// Also returns details, which should be one of:
// string, []byte, map[string]string
func (vm *VM) HealthCheck(context.Context) (interface{}, error) {
	// TODO perform actual health check
	return nil, nil
}
