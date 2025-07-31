// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package factory

import (
	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/snow/engine/snowman/block"
	"github.com/luxfi/node/utils/logging"
	"github.com/luxfi/node/vms"
	atomicvm "github.com/luxfi/coreth/plugin/evm/atomic/vm"

	"github.com/luxfi/coreth/plugin/evm"
)

var (
	// ID this VM should be referenced by
	ID = ids.ID{'e', 'v', 'm'}

	_ vms.Factory = (*Factory)(nil)
)

type Factory struct{}

func (*Factory) New(logging.Logger) (interface{}, error) {
	return atomicvm.WrapVM(&evm.VM{}), nil
}

func NewPluginVM() block.ChainVM {
	return atomicvm.WrapVM(&evm.VM{IsPlugin: true})
}
