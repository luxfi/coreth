// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
//
// This file is a derived work, based on the go-ethereum library whose original
// notices appear below.
//
// It is distributed under a license compatible with the licensing terms of the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********
// Copyright 2015 The go-ethereum Authors
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

package tests

import (
	"github.com/holiman/uint256"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/core/state/snapshot"
	"github.com/luxfi/geth/core/tracing"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/triedb"
	"github.com/luxfi/geth/triedb/hashdb"
	"github.com/luxfi/geth/triedb/pathdb"
)

// StateTestState groups all the state database objects together for use in tests.
type StateTestState struct {
	StateDB   *state.StateDB
	TrieDB    *triedb.Database
	Snapshots *snapshot.Tree
}

// MakePreState creates a state containing the given allocation.
func MakePreState(db ethdb.Database, accounts types.GenesisAlloc, snapshotter bool, scheme string) StateTestState {
	tconf := &triedb.Config{Preimages: true}
	if scheme == rawdb.HashScheme {
		tconf.HashDB = hashdb.Defaults
	} else {
		tconf.PathDB = pathdb.Defaults
	}

	tdb := triedb.NewDatabase(db, tconf)
	sdb := state.NewDatabase(tdb, nil)
	statedb, _ := state.New(types.EmptyRootHash, sdb)
	for addr, a := range accounts {
		statedb.SetCode(addr, a.Code, tracing.CodeChangeUnspecified)
		statedb.SetNonce(addr, a.Nonce, tracing.NonceChangeUnspecified)
		statedb.SetBalance(addr, uint256.MustFromBig(a.Balance), tracing.BalanceChangeUnspecified)
		for k, v := range a.Storage {
			statedb.SetState(addr, k, v)
		}
	}
	// Commit and re-open to start with a clean state.
	root, _ := statedb.Commit(0, false, false)

	// If snapshot is requested, initialize the snapshotter and use it in state.
	var snaps *snapshot.Tree
	if snapshotter && scheme == rawdb.HashScheme {
		snapconfig := snapshot.Config{
			CacheSize:  1,
			Recovery:   false,
			NoBuild:    false,
			AsyncBuild: false,
		}
		snaps, _ = snapshot.New(snapconfig, db, tdb, root)
	}
	sdb = state.NewDatabase(tdb, snaps)
	statedb, _ = state.New(root, sdb)
	return StateTestState{statedb, tdb, snaps}
}

// Close should be called when the state is no longer needed, ie. after running the test.
func (st *StateTestState) Close() {
	if st.TrieDB != nil {
		st.TrieDB.Close()
		st.TrieDB = nil
	}
	if st.Snapshots != nil {
		// Need to call Disable here to quit the snapshot generator goroutine.
		st.Snapshots.Disable()
		st.Snapshots.Release()
		st.Snapshots = nil
	}
}
