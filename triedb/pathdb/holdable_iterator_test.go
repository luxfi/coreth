// Copyright 2024 The go-ethereum Authors
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

package pathdb

import (
	"bytes"
	"testing"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/ethdb/memorydb"
)

func TestIteratorHold(t *testing.T) {
	// Create the key-value data store
	var (
		content = map[string]string{"k1": "v1", "k2": "v2", "k3": "v3"}
		order   = []string{"k1", "k2", "k3"}
		db      = rawdb.NewMemoryDatabase()
	)
	for key, val := range content {
		if err := db.Put([]byte(key), []byte(val)); err != nil {
			t.Fatalf("failed to insert item %s:%s into database: %v", key, val, err)
		}
	}
	// Iterate over the database with the given configs and verify the results
	it, idx := newHoldableIterator(db.NewIterator(nil, nil)), 0

	// Nothing should be affected for calling Discard on non-initialized iterator
	it.Hold()

	for it.Next() {
		if len(content) <= idx {
			t.Errorf("more items than expected: checking idx=%d (key %q), expecting len=%d", idx, it.Key(), len(order))
			break
		}
		if !bytes.Equal(it.Key(), []byte(order[idx])) {
			t.Errorf("item %d: key mismatch: have %s, want %s", idx, string(it.Key()), order[idx])
		}
		if !bytes.Equal(it.Value(), []byte(content[order[idx]])) {
			t.Errorf("item %d: value mismatch: have %s, want %s", idx, string(it.Value()), content[order[idx]])
		}
		// Should be safe to call discard multiple times
		it.Hold()
		it.Hold()

		// Shift iterator to the discarded element
		it.Next()
		if !bytes.Equal(it.Key(), []byte(order[idx])) {
			t.Errorf("item %d: key mismatch: have %s, want %s", idx, string(it.Key()), order[idx])
		}
		if !bytes.Equal(it.Value(), []byte(content[order[idx]])) {
			t.Errorf("item %d: value mismatch: have %s, want %s", idx, string(it.Value()), content[order[idx]])
		}

		// Discard/Next combo should work always
		it.Hold()
		it.Next()
		if !bytes.Equal(it.Key(), []byte(order[idx])) {
			t.Errorf("item %d: key mismatch: have %s, want %s", idx, string(it.Key()), order[idx])
		}
		if !bytes.Equal(it.Value(), []byte(content[order[idx]])) {
			t.Errorf("item %d: value mismatch: have %s, want %s", idx, string(it.Value()), content[order[idx]])
		}
		idx++
	}
	if err := it.Error(); err != nil {
		t.Errorf("iteration failed: %v", err)
	}
	if idx != len(order) {
		t.Errorf("iteration terminated prematurely: have %d, want %d", idx, len(order))
	}
	db.Close()
}

func TestReopenIterator(t *testing.T) {
	var (
		content = map[common.Hash]string{
			common.HexToHash("a1"): "v1",
			common.HexToHash("a2"): "v2",
			common.HexToHash("a3"): "v3",
			common.HexToHash("a4"): "v4",
			common.HexToHash("a5"): "v5",
			common.HexToHash("a6"): "v6",
		}
		order = []common.Hash{
			common.HexToHash("a1"),
			common.HexToHash("a2"),
			common.HexToHash("a3"),
			common.HexToHash("a4"),
			common.HexToHash("a5"),
			common.HexToHash("a6"),
		}
		db = rawdb.NewMemoryDatabase()

		reopen = func(db ethdb.KeyValueStore, iter *holdableIterator) *holdableIterator {
			if !iter.Next() {
				iter.Release()
				return newHoldableIterator(memorydb.New().NewIterator(nil, nil))
			}
			next := iter.Key()
			iter.Release()
			return newHoldableIterator(db.NewIterator(rawdb.SnapshotAccountPrefix, next[1:]))
		}
	)
	for key, val := range content {
		rawdb.WriteAccountSnapshot(db, key, []byte(val))
	}
	checkVal := func(it *holdableIterator, index int) {
		if !bytes.Equal(it.Key(), append(rawdb.SnapshotAccountPrefix, order[index].Bytes()...)) {
			t.Fatalf("Unexpected data entry key, want %v got %v", order[index], it.Key())
		}
		if !bytes.Equal(it.Value(), []byte(content[order[index]])) {
			t.Fatalf("Unexpected data entry key, want %v got %v", []byte(content[order[index]]), it.Value())
		}
	}
	// Iterate over the database with the given configs and verify the results
	dbIter := db.NewIterator(rawdb.SnapshotAccountPrefix, nil)
	iter, idx := newHoldableIterator(rawdb.NewKeyLengthIterator(dbIter, 1+common.HashLength)), -1

	idx++
	iter.Next()
	checkVal(iter, idx)

	iter = reopen(db, iter)
	idx++
	iter.Next()
	checkVal(iter, idx)

	// reopen twice
	iter = reopen(db, iter)
	iter = reopen(db, iter)
	idx++
	iter.Next()
	checkVal(iter, idx)

	// reopen iterator with held value
	iter.Next()
	iter.Hold()
	iter = reopen(db, iter)
	idx++
	iter.Next()
	checkVal(iter, idx)

	// reopen twice iterator with held value
	iter.Next()
	iter.Hold()
	iter = reopen(db, iter)
	iter = reopen(db, iter)
	idx++
	iter.Next()
	checkVal(iter, idx)

	// shift to the end and reopen
	iter.Next() // the end
	iter = reopen(db, iter)
	iter.Next()
	if iter.Key() != nil {
		t.Fatal("Unexpected iterated entry")
	}
}
