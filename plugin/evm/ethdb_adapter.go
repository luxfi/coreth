// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"bytes"

	"github.com/luxfi/database"
	"github.com/luxfi/geth/ethdb"
)

// ethdbAdapter wraps a luxfi/database.Database to satisfy geth's ethdb.KeyValueStore.
type ethdbAdapter struct {
	db database.Database
}

func (a *ethdbAdapter) Has(key []byte) (bool, error)      { return a.db.Has(key) }
func (a *ethdbAdapter) Get(key []byte) ([]byte, error)    { return a.db.Get(key) }
func (a *ethdbAdapter) Put(key, value []byte) error       { return a.db.Put(key, value) }
func (a *ethdbAdapter) Delete(key []byte) error           { return a.db.Delete(key) }
func (a *ethdbAdapter) SyncKeyValue() error               { return a.db.Sync() }
func (a *ethdbAdapter) Compact(start, limit []byte) error { return a.db.Compact(start, limit) }
func (a *ethdbAdapter) Close() error                      { return a.db.Close() }

func (a *ethdbAdapter) Stat() (string, error) {
	return "", nil
}

func (a *ethdbAdapter) DeleteRange(start, end []byte) error {
	// Best-effort range delete via iteration.
	iter := a.newIterator(nil, start)
	defer iter.Release()

	var keys [][]byte
	for iter.Next() {
		key := iter.Key()
		if end != nil && bytes.Compare(key, end) >= 0 {
			break
		}
		keys = append(keys, append([]byte(nil), key...))
	}
	if err := iter.Error(); err != nil {
		return err
	}
	for _, key := range keys {
		if err := a.db.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (a *ethdbAdapter) NewBatch() ethdb.Batch {
	return &ethdbBatchAdapter{db: a.db, batch: a.db.NewBatch()}
}

func (a *ethdbAdapter) NewBatchWithSize(_ int) ethdb.Batch {
	return a.NewBatch()
}

func (a *ethdbAdapter) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return a.newIterator(prefix, start)
}

func (a *ethdbAdapter) newIterator(prefix []byte, start []byte) ethdb.Iterator {
	switch {
	case prefix != nil && start != nil:
		startKey := append(append([]byte(nil), prefix...), start...)
		return &ethdbIterAdapter{it: a.db.NewIteratorWithStartAndPrefix(startKey, prefix)}
	case prefix != nil:
		return &ethdbIterAdapter{it: a.db.NewIteratorWithPrefix(prefix)}
	case start != nil:
		return &ethdbIterAdapter{it: a.db.NewIteratorWithStart(start)}
	default:
		return &ethdbIterAdapter{it: a.db.NewIterator()}
	}
}

type ethdbIterAdapter struct {
	it database.Iterator
}

func (i *ethdbIterAdapter) Next() bool    { return i.it.Next() }
func (i *ethdbIterAdapter) Error() error  { return i.it.Error() }
func (i *ethdbIterAdapter) Key() []byte   { return i.it.Key() }
func (i *ethdbIterAdapter) Value() []byte { return i.it.Value() }
func (i *ethdbIterAdapter) Release()      { i.it.Release() }

type ethdbBatchAdapter struct {
	db    database.Database
	batch database.Batch
}

func (b *ethdbBatchAdapter) Put(key []byte, value []byte) error {
	return b.batch.Put(key, value)
}

func (b *ethdbBatchAdapter) Delete(key []byte) error {
	return b.batch.Delete(key)
}

func (b *ethdbBatchAdapter) DeleteRange(start, end []byte) error {
	// Fallback to adapter's range delete on the parent DB.
	return (&ethdbAdapter{db: b.db}).DeleteRange(start, end)
}

func (b *ethdbBatchAdapter) ValueSize() int { return b.batch.Size() }
func (b *ethdbBatchAdapter) Write() error   { return b.batch.Write() }
func (b *ethdbBatchAdapter) Reset()         { b.batch.Reset() }

func (b *ethdbBatchAdapter) Replay(w ethdb.KeyValueWriter) error {
	return b.batch.Replay(w)
}
