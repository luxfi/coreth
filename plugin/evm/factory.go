// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"github.com/luxfi/log"
)

// Factory is used to create a new VM instance
type Factory struct{}

// New returns a new EVM instance
func (f *Factory) New(logger log.Logger) (interface{}, error) {
	return &VM{}, nil
}
