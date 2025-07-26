// Copyright 2025 The go-ethereum Authors
// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
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

package badgerdb

import (
	"bytes"
	"errors"

	db "github.com/luxfi/database"
	"github.com/luxfi/database/badgerdb"
	db "github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/prometheus/client_golang/prometheus"
)

var errNotSupported = errors.New("not supported")

// New returns a wrapped badgerdb database that implements ethdb.Database
func New(file string, cache int, handles int, namespace string, readonly bool) (ethdb.Database, error) {
	// Create badgerdb instance
	luxDB, err := badgerdb.New(file, nil, namespace, prometheus.DefaultRegisterer)
	if err != nil {
		return nil, err
	}

	return &database{db: luxDB}, nil
}

// database wraps a luxfi/database to implement ethdb.Database
type database struct {
	db db.Database
}

// Close implements ethdb.Database
func (d *database) Close() error {
	return d.db.Close()
}

// Has implements ethdb.KeyValueReader
func (d *database) Has(key []byte) (bool, error) {
	return d.db.Has(key)
}

// Get implements ethdb.KeyValueReader
func (d *database) Get(key []byte) ([]byte, error) {
	return d.db.Get(key)
}

// Put implements ethdb.KeyValueWriter
func (d *database) Put(key []byte, value []byte) error {
	return d.db.Put(key, value)
}

// Delete implements ethdb.KeyValueDeleter
func (d *database) Delete(key []byte) error {
	return d.db.Delete(key)
}

// DeleteRange deletes all keys with prefix (not fully supported)
func (d *database) DeleteRange(start, end []byte) error {
	// BadgerDB doesn't have native DeleteRange, so we iterate and delete
	it := d.db.NewIteratorWithStartAndPrefix(start, nil)
	defer it.Release()

	batch := d.db.NewBatch()
	for it.Next() {
		key := it.Key()
		if end != nil && bytes.Compare(key, end) >= 0 {
			break
		}
		if err := batch.Delete(key); err != nil {
			return err
		}
	}
	return batch.Write()
}

// NewBatch implements ethdb.Batcher
func (d *database) NewBatch() ethdb.Batch {
	return &batch{b: d.db.NewBatch()}
}

// NewBatchWithSize implements ethdb.Batcher
func (d *database) NewBatchWithSize(size int) ethdb.Batch {
	return &batch{b: d.db.NewBatch()}
}

// NewIterator implements ethdb.Iteratee
func (d *database) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return &iterator{it: d.db.NewIteratorWithStartAndPrefix(start, prefix)}
}

// Stat implements ethdb.Stater
func (d *database) Stat() (string, error) {
	return "badgerdb", nil
}

// Compact implements ethdb.Compacter
func (d *database) Compact(start []byte, limit []byte) error {
	return d.db.Compact(start, limit)
}

// SyncKeyValue implements ethdb.Database
func (d *database) SyncKeyValue() error {
	// BadgerDB automatically syncs data, this is a no-op
	return nil
}

// Ancient store methods (not supported by badgerdb)

// AncientDatadir returns the path of the ancient store
func (d *database) AncientDatadir() (string, error) {
	return "", errNotSupported
}

// HasAncient returns an indicator whether the specified ancient data exists
func (d *database) HasAncient(kind string, number uint64) (bool, error) {
	return false, nil
}

// Ancient retrieves an ancient binary blob
func (d *database) Ancient(kind string, number uint64) ([]byte, error) {
	return nil, errNotSupported
}

// AncientRange retrieves multiple ancient binary blobs
func (d *database) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) {
	return nil, errNotSupported
}

// Ancients returns the number of ancient items
func (d *database) Ancients() (uint64, error) {
	return 0, nil
}

// Tail returns the number of the first item
func (d *database) Tail() (uint64, error) {
	return 0, nil
}

// AncientSize returns the size of the ancient store
func (d *database) AncientSize(kind string) (uint64, error) {
	return 0, nil
}

// ReadAncients runs a read operation on the ancient store
func (d *database) ReadAncients(fn func(ethdb.AncientReaderOp) error) error {
	return fn(d)
}

// ModifyAncients runs a write operation on the ancient store
func (d *database) ModifyAncients(fn func(ethdb.AncientWriteOp) error) (int64, error) {
	return 0, errNotSupported
}

// SyncAncient flushes ancient store data to disk
func (d *database) SyncAncient() error {
	return nil
}

// TruncateHead discards recent ancient data
func (d *database) TruncateHead(n uint64) (uint64, error) {
	return 0, errNotSupported
}

// TruncateTail discards oldest ancient data
func (d *database) TruncateTail(n uint64) (uint64, error) {
	return 0, errNotSupported
}

// MigrateTable migrates a table to new format (no-op for badgerdb)
func (d *database) MigrateTable(kind string, convert func([]byte) ([]byte, error)) error {
	return nil
}

// batch wraps a luxfi/database batch to implement ethdb.Batch
type batch struct {
	b    db.Batch
	size int
}

// Put implements ethdb.Batch
func (b *batch) Put(key []byte, value []byte) error {
	b.size += len(key) + len(value)
	return b.b.Put(key, value)
}

// Delete implements ethdb.Batch
func (b *batch) Delete(key []byte) error {
	b.size += len(key)
	return b.b.Delete(key)
}

// DeleteRange deletes all keys with prefix (not supported)
func (b *batch) DeleteRange(start, end []byte) error {
	// Not supported by badgerdb batch
	return errNotSupported
}

// ValueSize implements ethdb.Batch
func (b *batch) ValueSize() int {
	return b.size
}

// Write implements ethdb.Batch
func (b *batch) Write() error {
	return b.b.Write()
}

// Reset implements ethdb.Batch
func (b *batch) Reset() {
	b.b.Reset()
	b.size = 0
}

// Replay implements ethdb.Batch
func (b *batch) Replay(w ethdb.KeyValueWriter) error {
	return b.b.Replay(&wrapper{w: w})
}

// wrapper wraps ethdb.KeyValueWriter to implement db.KeyValueWriterDeleter
type wrapper struct {
	w ethdb.KeyValueWriter
}

func (w *wrapper) Put(key []byte, value []byte) error {
	return w.w.Put(key, value)
}

func (w *wrapper) Delete(key []byte) error {
	// Try to cast to deleter
	if deleter, ok := w.w.(interface{ Delete([]byte) error }); ok {
		return deleter.Delete(key)
	}
	// If not a deleter, just put empty value
	return w.w.Put(key, nil)
}

// iterator wraps a luxfi/database iterator to implement ethdb.Iterator
type iterator struct {
	it db.Iterator
}

// Next implements ethdb.Iterator
func (i *iterator) Next() bool {
	return i.it.Next()
}

// Error implements ethdb.Iterator
func (i *iterator) Error() error {
	return i.it.Error()
}

// Key implements ethdb.Iterator
func (i *iterator) Key() []byte {
	return i.it.Key()
}

// Value implements ethdb.Iterator
func (i *iterator) Value() []byte {
	return i.it.Value()
}

// Release implements ethdb.Iterator
func (i *iterator) Release() {
	i.it.Release()
}
