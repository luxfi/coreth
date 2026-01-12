//go:build pebble

package evm

import (
	"path/filepath"

	"github.com/luxfi/database/manager"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
)

// openSourceDatabase opens a source database for import operations
func openSourceDatabase(dataPath string) (ethdb.Database, error) {
	// Open the source database using luxfi/database (pebbledb build tag).
	db, err := manager.NewManager("", nil).New(&manager.Config{
		Type:     "pebbledb",
		Path:     dataPath,
		ReadOnly: true,
	})
	if err != nil {
		return nil, err
	}

	// Wrap with rawdb for higher-level access
	// The ancient data is stored in a subdirectory of the chaindata directory
	ancientPath := filepath.Join(dataPath, "ancient")
	sourceDB, err := rawdb.Open(&ethdbAdapter{db: db}, rawdb.OpenOptions{
		ReadOnly: true,
		Ancient:  ancientPath,
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return sourceDB, nil
}
