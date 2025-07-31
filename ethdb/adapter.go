// Copyright 2025 The go-ethereum Authors
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

package ethdb

import (
	"bytes"
	"errors"

	"github.com/luxfi/database"
)

// DatabaseAdapter adapts a luxfi/database.Database to implement ethdb interfaces
type DatabaseAdapter struct {
	db database.Database
}

// NewDatabaseAdapter creates a new adapter
func NewDatabaseAdapter(db database.Database) *DatabaseAdapter {
	return &DatabaseAdapter{db: db}
}

// Has retrieves if a key is present in the key-value data store.
func (a *DatabaseAdapter) Has(key []byte) (bool, error) {
	return a.db.Has(key)
}

// Get retrieves the given key if it's present in the key-value data store.
func (a *DatabaseAdapter) Get(key []byte) ([]byte, error) {
	return a.db.Get(key)
}

// Put inserts the given value into the key-value data store.
func (a *DatabaseAdapter) Put(key []byte, value []byte) error {
	return a.db.Put(key, value)
}

// Delete removes the key from the key-value data store.
func (a *DatabaseAdapter) Delete(key []byte) error {
	return a.db.Delete(key)
}

// DeleteRange deletes all of the keys (and values) in the range [start,end)
func (a *DatabaseAdapter) DeleteRange(start, end []byte) error {
	// luxfi/database doesn't have DeleteRange, so we need to iterate and delete
	iter := a.db.NewIteratorWithStart(start)
	defer iter.Release()
	
	for iter.Next() {
		key := iter.Key()
		if end != nil && bytes.Compare(key, end) >= 0 {
			break
		}
		if err := a.db.Delete(key); err != nil {
			return err
		}
	}
	return iter.Error()
}

// NewBatch creates a write-only database that buffers changes to its host db
func (a *DatabaseAdapter) NewBatch() Batch {
	return &luxBatchAdapter{batch: a.db.NewBatch()}
}

// NewBatchWithSize creates a write-only database batch with pre-allocated buffer.
func (a *DatabaseAdapter) NewBatchWithSize(size int) Batch {
	// luxfi/database doesn't have NewBatchWithSize, so just use NewBatch
	return &luxBatchAdapter{batch: a.db.NewBatch()}
}

// NewIterator creates a binary-alphabetical iterator over a subset
func (a *DatabaseAdapter) NewIterator(prefix []byte, start []byte) Iterator {
	if prefix != nil && start != nil {
		return &luxIteratorAdapter{iter: a.db.NewIteratorWithStartAndPrefix(start, prefix)}
	} else if prefix != nil {
		return &luxIteratorAdapter{iter: a.db.NewIteratorWithPrefix(prefix)}
	} else if start != nil {
		return &luxIteratorAdapter{iter: a.db.NewIteratorWithStart(start)}
	}
	return &luxIteratorAdapter{iter: a.db.NewIterator()}
}

// Stat returns the statistic data of the database.
func (a *DatabaseAdapter) Stat() (string, error) {
	// luxfi/database doesn't have Stat
	return "DatabaseAdapter", nil
}

// SyncKeyValue ensures that all pending writes are flushed to disk
func (a *DatabaseAdapter) SyncKeyValue() error {
	// luxfi/database doesn't have explicit sync, assume it's handled internally
	return nil
}

// Compact flattens the underlying data store for the given key range.
func (a *DatabaseAdapter) Compact(start []byte, limit []byte) error {
	return a.db.Compact(start, limit)
}

// Close closes the database
func (a *DatabaseAdapter) Close() error {
	return a.db.Close()
}

// luxBatchAdapter adapts a luxfi/database.Batch to ethdb.Batch
type luxBatchAdapter struct {
	batch database.Batch
}

func (b *luxBatchAdapter) Put(key []byte, value []byte) error {
	return b.batch.Put(key, value)
}

func (b *luxBatchAdapter) Delete(key []byte) error {
	return b.batch.Delete(key)
}

func (b *luxBatchAdapter) ValueSize() int {
	return b.batch.Size()
}

func (b *luxBatchAdapter) Write() error {
	return b.batch.Write()
}

func (b *luxBatchAdapter) Reset() {
	b.batch.Reset()
}

func (b *luxBatchAdapter) Replay(w KeyValueWriter) error {
	// luxfi/database.Batch.Replay expects KeyValueWriterDeleter
	// Create a wrapper that implements the interface
	wrapper := &replayWrapper{writer: w}
	return b.batch.Replay(wrapper)
}

// DeleteRange is not supported by luxfi/database batches
func (b *luxBatchAdapter) DeleteRange(start, end []byte) error {
	// We could iterate and delete each key, but that would be inefficient
	// Return an error indicating this operation is not supported
	return errors.New("DeleteRange not supported in batch")
}

// replayWrapper wraps a KeyValueWriter to implement database.KeyValueWriterDeleter
type replayWrapper struct {
	writer KeyValueWriter
}

func (r *replayWrapper) Put(key []byte, value []byte) error {
	return r.writer.Put(key, value)
}

func (r *replayWrapper) Delete(key []byte) error {
	// KeyValueWriter includes Delete method
	return r.writer.Delete(key)
}

// luxIteratorAdapter adapts a luxfi/database.Iterator to ethdb.Iterator
type luxIteratorAdapter struct {
	iter database.Iterator
}

func (i *luxIteratorAdapter) Next() bool {
	return i.iter.Next()
}

func (i *luxIteratorAdapter) Error() error {
	return i.iter.Error()
}

func (i *luxIteratorAdapter) Key() []byte {
	return i.iter.Key()
}

func (i *luxIteratorAdapter) Value() []byte {
	return i.iter.Value()
}

func (i *luxIteratorAdapter) Release() {
	i.iter.Release()
}

// Ancient retrieves an ancient binary blob from the append-only immutable files.
func (a *DatabaseAdapter) Ancient(kind string, number uint64) ([]byte, error) {
	// luxfi/database doesn't support ancient store
	return nil, errors.New("ancient store not supported")
}

// AncientRange retrieves multiple items in sequence, starting from the index 'start'.
func (a *DatabaseAdapter) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) {
	// luxfi/database doesn't support ancient store
	return nil, errors.New("ancient store not supported")
}

// Ancients returns the ancient item numbers in the ancient store.
func (a *DatabaseAdapter) Ancients() (uint64, error) {
	// luxfi/database doesn't support ancient store
	return 0, nil
}

// Tail returns the number of first stored item in the ancient store.
func (a *DatabaseAdapter) Tail() (uint64, error) {
	// luxfi/database doesn't support ancient store
	return 0, nil
}

// AncientSize returns the ancient size of the specified category.
func (a *DatabaseAdapter) AncientSize(kind string) (uint64, error) {
	// luxfi/database doesn't support ancient store
	return 0, nil
}

// ReadAncients runs the given read operation while ensuring that no writes take place
func (a *DatabaseAdapter) ReadAncients(fn func(AncientReaderOp) error) error {
	// luxfi/database doesn't support ancient store
	return fn(a)
}

// ModifyAncients runs a write operation on the ancient store.
func (a *DatabaseAdapter) ModifyAncients(fn func(AncientWriteOp) error) (int64, error) {
	// luxfi/database doesn't support ancient store
	return 0, errors.New("ancient store not supported")
}

// SyncAncient flushes all in-memory ancient store data to disk.
func (a *DatabaseAdapter) SyncAncient() error {
	// luxfi/database doesn't support ancient store
	return nil
}

// TruncateHead discards all but the first n ancient data from the ancient store.
func (a *DatabaseAdapter) TruncateHead(n uint64) (uint64, error) {
	// luxfi/database doesn't support ancient store
	return 0, errors.New("ancient store not supported")
}

// TruncateTail discards the first n ancient data from the ancient store.
func (a *DatabaseAdapter) TruncateTail(n uint64) (uint64, error) {
	// luxfi/database doesn't support ancient store
	return 0, errors.New("ancient store not supported")
}

// AncientDatadir returns the path of the ancient store directory.
func (a *DatabaseAdapter) AncientDatadir() (string, error) {
	// luxfi/database doesn't support ancient store
	return "", errors.New("ancient store not supported")
}

// Ensure the adapter implements all required interfaces
var _ Database = (*DatabaseAdapter)(nil)