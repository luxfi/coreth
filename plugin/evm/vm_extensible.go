// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"
	"errors"

	"github.com/luxfi/node/database/versiondb"
	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/network/p2p"
	"github.com/luxfi/coreth/core"
	"github.com/luxfi/coreth/params"
	"github.com/luxfi/coreth/plugin/evm/config"
	"github.com/luxfi/coreth/plugin/evm/extension"
	vmsync "github.com/luxfi/coreth/sync/vm"
	"github.com/prometheus/client_golang/prometheus"
)

var _ extension.InnerVM = (*VM)(nil)

var (
	errVMAlreadyInitialized      = errors.New("vm already initialized")
	errExtensionConfigAlreadySet = errors.New("extension config already set")
)

func (vm *VM) SetExtensionConfig(config *extension.Config) error {
	if vm.ctx != nil {
		return errVMAlreadyInitialized
	}
	if vm.extensionConfig != nil {
		return errExtensionConfigAlreadySet
	}
	vm.extensionConfig = config
	return nil
}

// All these methods below assumes that VM is already initialized

func (vm *VM) GetExtendedBlock(ctx context.Context, blkID ids.ID) (extension.ExtendedBlock, error) {
	// Since each internal handler used by [vm.State] always returns a block
	// with non-nil ethBlock value, GetBlockInternal should never return a
	// (*Block) with a nil ethBlock value.
	blk, err := vm.GetBlockInternal(ctx, blkID)
	if err != nil {
		return nil, err
	}

	return blk.(*wrappedBlock), nil
}

func (vm *VM) LastAcceptedExtendedBlock() extension.ExtendedBlock {
	lastAcceptedBlock := vm.LastAcceptedBlockInternal()
	if lastAcceptedBlock == nil {
		return nil
	}
	return lastAcceptedBlock.(*wrappedBlock)
}

// ChainConfig returns the chain config for the VM
// Even though this is available through Blockchain().Config(),
// ChainConfig() here will be available before the blockchain is initialized.
func (vm *VM) ChainConfig() *params.ChainConfig {
	return vm.chainConfig
}

func (vm *VM) Blockchain() *core.BlockChain {
	return vm.blockChain
}

func (vm *VM) Config() config.Config {
	return vm.config
}

func (vm *VM) MetricRegistry() *prometheus.Registry {
	return vm.sdkMetrics
}

func (vm *VM) Validators() *p2p.Validators {
	return vm.P2PValidators()
}

func (vm *VM) VersionDB() *versiondb.Database {
	return vm.versiondb
}

func (vm *VM) SyncerClient() vmsync.Client {
	return vm.Client
}
