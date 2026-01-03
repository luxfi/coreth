// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extstate

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/core/tracing"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/triedb"
	"github.com/stretchr/testify/require"
)

func TestMultiCoinOperations(t *testing.T) {
	memdb := rawdb.NewMemoryDatabase()
	tdb := triedb.NewDatabase(memdb, nil)
	db := state.NewDatabase(tdb, nil)
	statedb, err := state.New(types.EmptyRootHash, db)
	require.NoError(t, err, "creating empty statedb")

	addr := common.Address{1}
	assetID := common.Hash{2}

	statedb.AddBalance(addr, new(uint256.Int), tracing.BalanceChangeUnspecified)

	wrappedStateDB := New(statedb)
	balance := wrappedStateDB.GetBalanceMultiCoin(addr, assetID)
	require.Equal(t, "0", balance.String(), "expected zero big.Int multicoin balance as string")

	wrappedStateDB.AddBalanceMultiCoin(addr, assetID, big.NewInt(10))
	wrappedStateDB.SubBalanceMultiCoin(addr, assetID, big.NewInt(5))
	wrappedStateDB.AddBalanceMultiCoin(addr, assetID, big.NewInt(3))

	balance = wrappedStateDB.GetBalanceMultiCoin(addr, assetID)
	require.Equal(t, "8", balance.String(), "unexpected multicoin balance string")
}
