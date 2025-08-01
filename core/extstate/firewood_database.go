// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package extstate

import (
	"fmt"

	"github.com/luxfi/geth/triedb/database"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/state"
)

var (
	_ state.Database = (*databaseAccessorDb)(nil)
	_ state.Trie     = (*database.AccountTrie)(nil)
	_ state.Trie     = (*database.StorageTrie)(nil)
)

type databaseAccessorDb struct {
	state.Database
	fw *database.Database
}

// OpenTrie opens the main account trie.
func (db *databaseAccessorDb) OpenTrie(root common.Hash) (state.Trie, error) {
	return database.NewAccountTrie(root, db.fw)
}

// OpenStorageTrie opens a wrapped version of the account trie.
func (db *databaseAccessorDb) OpenStorageTrie(stateRoot common.Hash, address common.Address, root common.Hash, self state.Trie) (state.Trie, error) {
	accountTrie, ok := self.(*database.AccountTrie)
	if !ok {
		return nil, fmt.Errorf("Invalid account trie type: %T", self)
	}
	return database.NewStorageTrie(accountTrie, root)
}

// CopyTrie returns a deep copy of the given trie.
// It can be altered by the caller.
func (db *databaseAccessorDb) CopyTrie(t state.Trie) state.Trie {
	switch t := t.(type) {
	case *database.AccountTrie:
		return t.Copy()
	case *database.StorageTrie:
		return nil // The storage trie just wraps the account trie, so we must re-open it separately.
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}
