//go:build !pebble

package evm

import (
	"errors"

	"github.com/luxfi/geth/ethdb"
)

// openSourceDatabase returns an error when pebble is not available
func openSourceDatabase(dataPath string) (ethdb.Database, error) {
	return nil, errors.New("database import requires pebble support (build with -tags=pebble)")
}
