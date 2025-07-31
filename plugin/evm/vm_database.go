// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"time"

	luxdatabase "github.com/luxfi/node/database"
	"github.com/luxfi/node/database/prefixdb"
	"github.com/luxfi/node/database/versiondb"
	"github.com/luxfi/coreth/plugin/evm/database"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/log"
)

// initializeDBs initializes the databases used by the VM.
// coreth always uses the luxd provided database.
func (vm *VM) initializeDBs(db luxdatabase.Database) error {
	// Use NewNested rather than New so that the structure of the database
	// remains the same regardless of the provided baseDB type.
	vm.chaindb = rawdb.NewDatabase(database.WrapDatabase(prefixdb.NewNested(ethDBPrefix, db)))
	vm.versiondb = versiondb.New(db)
	vm.acceptedBlockDB = prefixdb.New(acceptedPrefix, vm.versiondb)
	vm.metadataDB = prefixdb.New(metadataPrefix, vm.versiondb)
	vm.db = db
	// Note warpDB is not part of versiondb because it is not necessary
	// that warp signatures are committed to the database atomically with
	// the last accepted block.
	vm.warpDB = prefixdb.New(warpPrefix, db)
	return nil
}

func (vm *VM) inspectDatabases() error {
	start := time.Now()
	log.Info("Starting database inspection")
	if err := rawdb.InspectDatabase(vm.chaindb, nil, nil); err != nil {
		return err
	}
	if err := inspectDB(vm.acceptedBlockDB, "acceptedBlockDB"); err != nil {
		return err
	}
	if err := inspectDB(vm.metadataDB, "metadataDB"); err != nil {
		return err
	}
	if err := inspectDB(vm.warpDB, "warpDB"); err != nil {
		return err
	}
	log.Info("Completed database inspection", "elapsed", time.Since(start))
	return nil
}

func inspectDB(db luxdatabase.Database, label string) error {
	it := db.NewIterator()
	defer it.Release()

	var (
		count  int64
		start  = time.Now()
		logged = time.Now()

		// Totals
		total common.StorageSize
	)
	// Inspect key-value database first.
	for it.Next() {
		var (
			key  = it.Key()
			size = common.StorageSize(len(key) + len(it.Value()))
		)
		total += size
		count++
		if count%1000 == 0 && time.Since(logged) > 8*time.Second {
			log.Info("Inspecting database", "label", label, "count", count, "elapsed", common.PrettyDuration(time.Since(start)))
			logged = time.Now()
		}
	}
	// Display the database statistic.
	log.Info("Database statistics", "label", label, "total", total.String(), "count", count)
	return nil
}
