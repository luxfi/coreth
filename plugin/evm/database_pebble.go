//go:build pebble

package evm

import (
	"path/filepath"

	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/ethdb/pebble"
)

// openSourceDatabase opens a source database for import operations
func openSourceDatabase(dataPath string) (ethdb.Database, error) {
	// Open the source database using pebble directly
	pebbleDB, err := pebble.New(dataPath, 16, 16, "", true)
	if err != nil {
		return nil, err
	}

	// Wrap with rawdb for higher-level access
	// The ancient data is stored in a subdirectory of the chaindata directory
	ancientPath := filepath.Join(dataPath, "ancient")
	sourceDB, err := rawdb.Open(pebbleDB, rawdb.OpenOptions{
		ReadOnly: true,
		Ancient:  ancientPath,
	})
	if err != nil {
		pebbleDB.Close()
		return nil, err
	}
	return sourceDB, nil
}
