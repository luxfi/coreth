package evm

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/log"
)

// CheckImportMode checks if we should run in import mode
func CheckImportMode(importPath string, chainDataDir string) (bool, error) {
	if importPath == "" {
		return false, nil
	}

	// Check if import path exists
	if _, err := os.Stat(importPath); os.IsNotExist(err) {
		return false, fmt.Errorf("import path does not exist: %s", importPath)
	}

	// Check if we already have chain data
	chainDBPath := filepath.Join(chainDataDir, "db")
	if _, err := os.Stat(chainDBPath); err == nil {
		log.Info("Chain data already exists, skipping import", "path", chainDBPath)
		return false, nil
	}

	return true, nil
}

// GetImportedChainHeight reads the last block height from the import database
func GetImportedChainHeight(importPath string) (uint64, common.Hash, error) {
	// Open the source database
	sourceDB, err := rawdb.Open(rawdb.OpenOptions{
		Type:              "pebble",
		Directory:         importPath,
		AncientsDirectory: "",
		Namespace:         "",
		Cache:             0,
		Handles:           0,
		ReadOnly:          true,
		Ephemeral:         false,
		DisableFreezer:    true,
	})
	if err != nil {
		return 0, common.Hash{}, fmt.Errorf("failed to open source database: %w", err)
	}
	defer sourceDB.Close()

	// Find the last block
	var lastBlockNumber uint64
	var lastBlockHash common.Hash
	
	it := sourceDB.NewIterator([]byte("h"), nil)
	defer it.Release()
	
	for it.Next() {
		key := it.Key()
		if len(key) == 41 && key[0] == 'h' {
			blockNumber := binary.BigEndian.Uint64(key[1:9])
			if blockNumber > lastBlockNumber {
				lastBlockNumber = blockNumber
				copy(lastBlockHash[:], key[9:41])
			}
		}
	}
	
	return lastBlockNumber, lastBlockHash, nil
}

// CreateImportMarker creates a marker file to indicate import is in progress
func CreateImportMarker(chainDataDir string) error {
	markerPath := filepath.Join(chainDataDir, ".importing")
	return os.WriteFile(markerPath, []byte("importing"), 0644)
}

// RemoveImportMarker removes the import marker file
func RemoveImportMarker(chainDataDir string) error {
	markerPath := filepath.Join(chainDataDir, ".importing")
	return os.Remove(markerPath)
}

// IsImporting checks if import is in progress
func IsImporting(chainDataDir string) bool {
	markerPath := filepath.Join(chainDataDir, ".importing")
	_, err := os.Stat(markerPath)
	return err == nil
}