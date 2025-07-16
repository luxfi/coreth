// (c) 2019-2025, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/node/database"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/luxfi/geth/core/rawdb"
)

// DatabaseBackend represents the type of database backend
type DatabaseBackend string

const (
	// PebbleDBBackend uses PebbleDB (faster, more efficient)
	PebbleDBBackend DatabaseBackend = "pebbledb"
	// LevelDBBackend uses LevelDB (legacy, slower)
	LevelDBBackend DatabaseBackend = "leveldb"
	// DefaultBackend uses the wrapped Avalanche database
	DefaultBackend DatabaseBackend = "default"
)

// CreateChainDatabase creates a chain database based on the configured backend
func CreateChainDatabase(dataDir string, backend DatabaseBackend, cache int, handles int) (ethdb.Database, error) {
	// Ensure the data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	switch backend {
	case PebbleDBBackend:
		dbPath := filepath.Join(dataDir, "chaindata")
		return rawdb.NewPebbleDBDatabase(dbPath, cache, handles, "chaindata", false, false)
	
	case LevelDBBackend:
		dbPath := filepath.Join(dataDir, "chaindata")
		return rawdb.NewLevelDBDatabase(dbPath, cache, handles, "chaindata", false)
	
	default:
		return nil, fmt.Errorf("unsupported database backend: %s", backend)
	}
}

// WrapDatabase wraps an Avalanche database to implement ethdb.Database
func WrapDatabase(db database.Database) ethdb.Database {
	return rawdb.NewDatabase(Database{db})
}

// OpenExistingDatabase opens an existing database for reading
func OpenExistingDatabase(path string, readonly bool) (ethdb.Database, error) {
	// Check if it's PebbleDB or LevelDB
	pebblePath := filepath.Join(path, "pebbledb")
	if _, err := os.Stat(pebblePath); err == nil {
		// It's PebbleDB
		return rawdb.NewPebbleDBDatabase(path, 0, 0, "", readonly, false)
	}
	
	// Try LevelDB
	return rawdb.NewLevelDBDatabase(path, 0, 0, "", readonly)
}