// (c) 2023, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package state

import (
	"testing"

	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/precompile/contract"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func NewTestStateDB(t testing.TB) contract.StateDB {
	db := rawdb.NewMemoryDatabase()
	stateDB, err := New(common.Hash{}, NewDatabase(db), nil)
	require.NoError(t, err)
	return stateDB
}
