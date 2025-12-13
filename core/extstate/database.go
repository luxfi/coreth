// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extstate

import (
	corethsnap "github.com/luxfi/coreth/core/state/snapshot"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/triedb"
)

// NewDatabase creates a new state database using the standard geth API.
// The coreth snapshot.Tree is not directly compatible with geth's snapshot.Tree,
// so we pass nil and let the state package handle it.
func NewDatabase(tdb *triedb.Database, snap *corethsnap.Tree) state.Database {
	// Note: coreth uses its own snapshot package which is not directly compatible
	// with geth's. We pass nil here as the state database will work without snapshots.
	_ = snap
	return state.NewDatabase(tdb, nil)
}

// NewDatabaseWithConfig creates a new state database with the given triedb config.
func NewDatabaseWithConfig(db ethdb.Database, config *triedb.Config) state.Database {
	tdb := triedb.NewDatabase(db, config)
	return state.NewDatabase(tdb, nil)
}

// NewDatabaseWithNodeDB creates a new state database with an existing triedb.
func NewDatabaseWithNodeDB(db ethdb.Database, tdb *triedb.Database) state.Database {
	return state.NewDatabase(tdb, nil)
}
