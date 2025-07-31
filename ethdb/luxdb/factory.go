// Copyright 2025 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package luxdb

import (
	"github.com/luxfi/database"
	"github.com/luxfi/database/leveldb"
	"github.com/luxfi/database/memdb"
	"github.com/luxfi/database/pebbledb"
	"github.com/luxfi/geth/ethdb"
)

// Open creates a new database using luxfi/database backend
func Open(path string, cache int, handles int, dbType string) (ethdb.Database, error) {
	var db database.Database
	var err error
	
	// Create the database directly without metrics for now
	switch dbType {
	case "leveldb":
		// leveldb.New(path string, blockCacheSize int, writeCacheSize int, handleCap int)
		db, err = leveldb.New(path, cache, cache, handles)
	case "pebbledb":
		// pebbledb.New(path string, cacheSize int, handles int, namespace string, readonly bool)
		db, err = pebbledb.New(path, cache, handles, "ethdb", false)
	case "memdb":
		db = memdb.New()
	default:
		// Default to leveldb
		db, err = leveldb.New(path, cache, cache, handles)
	}
	
	if err != nil {
		return nil, err
	}
	
	// Wrap it in the adapter
	return ethdb.NewDatabaseAdapter(db), nil
}

// OpenLevelDB creates a new LevelDB database
func OpenLevelDB(path string, cache int, handles int) (ethdb.Database, error) {
	return Open(path, cache, handles, "leveldb")
}

// OpenPebbleDB creates a new PebbleDB database
func OpenPebbleDB(path string, cache int, handles int) (ethdb.Database, error) {
	return Open(path, cache, handles, "pebbledb")
}

// OpenMemDB creates a new in-memory database
func OpenMemDB() (ethdb.Database, error) {
	return Open("", 0, 0, "memdb")
}