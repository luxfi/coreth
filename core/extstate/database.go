// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extstate

import (
	"github.com/luxfi/geth/triedb/database"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/triedb"
)

func NewDatabaseWithConfig(db ethdb.Database, config *triedb.Config) state.Database {
	coredb := state.NewDatabaseWithConfig(db, config)
	return wrapIfDatabase(coredb)
}

func NewDatabaseWithNodeDB(db ethdb.Database, triedb *triedb.Database) state.Database {
	coredb := state.NewDatabaseWithNodeDB(db, triedb)
	return wrapIfDatabase(coredb)
}

func wrapIfDatabase(db state.Database) state.Database {
	fw, ok := db.TrieDB().Backend().(*database.Database)
	if !ok {
		return db
	}
	return &databaseAccessorDb{
		Database: db,
		fw:       fw,
	}
}
