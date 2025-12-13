// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"github.com/luxfi/log"
	"github.com/luxfi/node/utils/timer/mockable"

	"github.com/luxfi/coreth/plugin/evm/extension"
	"github.com/luxfi/coreth/plugin/evm/message"
)

// Factory is used to create a new VM instance
type Factory struct{}

// New returns a new EVM instance
func (f *Factory) New(logger log.Logger) (interface{}, error) {
	vm := &VM{}

	// Set default extension config with required fields
	// This can be overridden by SetExtensionConfig before Initialize
	defaultConfig := &extension.Config{
		SyncSummaryProvider: &message.BlockSyncSummaryProvider{},
		SyncableParser:      message.NewBlockSyncSummaryParser(),
		Clock:               &mockable.Clock{},
	}
	vm.extensionConfig = defaultConfig

	return vm, nil
}
