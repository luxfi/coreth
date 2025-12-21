//go:build !pebble

package evm

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/log"
)

var errPebbleNotSupported = errors.New("import mode requires pebble database support (build with -tags=pebble)")

// CheckImportMode checks if we should run in import mode
func CheckImportMode(importPath string, chainDataDir string) (bool, error) {
	if importPath == "" {
		return false, nil
	}

	// Check if import path exists
	if _, err := os.Stat(importPath); os.IsNotExist(err) {
		return false, errPebbleNotSupported
	}

	// Check if we already have chain data
	chainDBPath := filepath.Join(chainDataDir, "db")
	if _, err := os.Stat(chainDBPath); err == nil {
		log.Info("Chain data already exists, skipping import", "path", chainDBPath)
		return false, nil
	}

	return false, errPebbleNotSupported
}

// GetImportedChainHeight reads the last block height from the import database
func GetImportedChainHeight(importPath string) (uint64, common.Hash, error) {
	return 0, common.Hash{}, errPebbleNotSupported
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
