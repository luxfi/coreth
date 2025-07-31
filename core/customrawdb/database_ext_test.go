// Copyright (C) 2019-2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customrawdb

import (
	"testing"

	"github.com/luxfi/geth/common"
	ethrawdb "github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
)

func TestInspectDatabase(t *testing.T) {
	db := &stubDatabase{
		iterator: &stubIterator{},
	}

	// Extra metadata keys: (17 + 32) + (12 + 32) = 93 bytes
	WriteSnapshotBlockHash(db, common.Hash{})
	ethrawdb.WriteSnapshotRoot(db, common.Hash{})
	// Trie segments: (77 + 2) + 1 = 80 bytes
	_ = WriteSyncSegment(db, common.Hash{}, common.Hash{})
	// Storage tries to fetch: 76 + 1 = 77 bytes
	_ = WriteSyncStorageTrie(db, common.Hash{}, common.Hash{})
	// Code to fetch: 34 + 0 = 34 bytes
	AddCodeToFetch(db, common.Hash{})
	// Block numbers synced to: 22 + 1 = 23 bytes
	_ = WriteSyncPerformed(db, 0)

	keyPrefix := []byte(nil)
	keyStart := []byte(nil)

	err := InspectDatabase(db, keyPrefix, keyStart)
	if err != nil {
		t.Fatalf("InspectDatabase failed: %v", err)
	}
}

type stubDatabase struct {
	ethdb.Database
	iterator *stubIterator
}

func (s *stubDatabase) NewIterator(keyPrefix, keyStart []byte) ethdb.Iterator {
	return s.iterator
}

// AncientSize is used in [InspectDatabase] to determine the ancient sizes.
func (s *stubDatabase) AncientSize(kind string) (uint64, error) {
	return 0, nil
}

func (s *stubDatabase) Ancients() (uint64, error) {
	return 0, nil
}

func (s *stubDatabase) Tail() (uint64, error) {
	return 0, nil
}

func (s *stubDatabase) AncientDatadir() (string, error) {
	return "", nil
}

func (s *stubDatabase) Put(key, value []byte) error {
	s.iterator.kvs = append(s.iterator.kvs, keyValue{key: key, value: value})
	return nil
}

func (s *stubDatabase) Get(key []byte) ([]byte, error) {
	return nil, nil
}

func (s *stubDatabase) ReadAncients(fn func(ethdb.AncientReaderOp) error) error {
	return nil
}

type stubIterator struct {
	ethdb.Iterator
	i   int // see [stubIterator.pos]
	kvs []keyValue
}

type keyValue struct {
	key   []byte
	value []byte
}

// pos returns the true iterator position, which is otherwise off by one because
// Next() is called _before_ usage.
func (s *stubIterator) pos() int {
	return s.i - 1
}

func (s *stubIterator) Next() bool {
	s.i++
	return s.pos() < len(s.kvs)
}

func (s *stubIterator) Release() {}

func (s *stubIterator) Key() []byte {
	return s.kvs[s.pos()].key
}

func (s *stubIterator) Value() []byte {
	return s.kvs[s.pos()].value
}
