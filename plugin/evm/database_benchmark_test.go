// (c) 2019-2025, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/luxfi/geth/core/rawdb"
)

// BenchmarkDatabaseBackends compares PebbleDB vs LevelDB performance
func BenchmarkDatabaseBackends(b *testing.B) {
	// Test data sizes
	keySizes := []int{32, 64, 128}
	valueSizes := []int{100, 1000, 10000}
	batchSizes := []int{100, 1000, 10000}

	for _, keySize := range keySizes {
		for _, valueSize := range valueSizes {
			for _, batchSize := range batchSizes {
				name := fmt.Sprintf("Key%d_Value%d_Batch%d", keySize, valueSize, batchSize)
				
				b.Run("PebbleDB_"+name, func(b *testing.B) {
					benchmarkDatabase(b, "pebbledb", keySize, valueSize, batchSize)
				})
				
				b.Run("LevelDB_"+name, func(b *testing.B) {
					benchmarkDatabase(b, "leveldb", keySize, valueSize, batchSize)
				})
			}
		}
	}
}

func benchmarkDatabase(b *testing.B, backend string, keySize, valueSize, batchSize int) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bench_"+backend+"_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create database
	var db ethdb.Database
	if backend == "pebbledb" {
		db, err = rawdb.NewPebbleDBDatabase(filepath.Join(tmpDir, "db"), 128, 500, "", false, false)
	} else {
		db, err = rawdb.NewLevelDBDatabase(filepath.Join(tmpDir, "db"), 128, 500, "", false)
	}
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// Generate test data
	keys := make([][]byte, batchSize)
	values := make([][]byte, batchSize)
	for i := 0; i < batchSize; i++ {
		keys[i] = make([]byte, keySize)
		values[i] = make([]byte, valueSize)
		rand.Read(keys[i])
		rand.Read(values[i])
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Write batch
		batch := db.NewBatch()
		for j := 0; j < batchSize; j++ {
			batch.Put(keys[j], values[j])
		}
		if err := batch.Write(); err != nil {
			b.Fatal(err)
		}

		// Read all keys
		for j := 0; j < batchSize; j++ {
			if _, err := db.Get(keys[j]); err != nil {
				b.Fatal(err)
			}
		}

		// Delete half the keys
		batch = db.NewBatch()
		for j := 0; j < batchSize/2; j++ {
			batch.Delete(keys[j])
		}
		if err := batch.Write(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRealWorldScenario simulates blockchain-like access patterns
func BenchmarkRealWorldScenario(b *testing.B) {
	backends := []string{"pebbledb", "leveldb"}
	
	for _, backend := range backends {
		b.Run(backend, func(b *testing.B) {
			benchmarkRealWorld(b, backend)
		})
	}
}

func benchmarkRealWorld(b *testing.B, backend string) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bench_real_"+backend+"_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create database with realistic cache sizes
	var db ethdb.Database
	if backend == "pebbledb" {
		db, err = rawdb.NewPebbleDBDatabase(filepath.Join(tmpDir, "db"), 512, 1000, "", false, false)
	} else {
		db, err = rawdb.NewLevelDBDatabase(filepath.Join(tmpDir, "db"), 512, 1000, "", false)
	}
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// Simulate blockchain operations
	b.ResetTimer()
	start := time.Now()

	for blockNum := 0; blockNum < b.N; blockNum++ {
		// Simulate block header (small, frequent writes)
		blockHash := common.Hash{byte(blockNum >> 24), byte(blockNum >> 16), byte(blockNum >> 8), byte(blockNum)}
		headerKey := append([]byte("h"), blockHash[:]...)
		headerValue := make([]byte, 512) // Typical header size
		rand.Read(headerValue)
		
		// Simulate state trie nodes (medium, very frequent)
		batch := db.NewBatch()
		for i := 0; i < 100; i++ {
			nodeKey := make([]byte, 32)
			nodeValue := make([]byte, 200)
			rand.Read(nodeKey)
			rand.Read(nodeValue)
			batch.Put(nodeKey, nodeValue)
		}

		// Simulate transactions (larger, less frequent)
		for i := 0; i < 10; i++ {
			txKey := make([]byte, 32)
			txValue := make([]byte, 2000)
			rand.Read(txKey)
			rand.Read(txValue)
			batch.Put(txKey, txValue)
		}

		// Write block data
		batch.Put(headerKey, headerValue)
		if err := batch.Write(); err != nil {
			b.Fatal(err)
		}

		// Simulate reads (state access during transaction execution)
		for i := 0; i < 50; i++ {
			readKey := make([]byte, 32)
			rand.Read(readKey)
			db.Get(readKey) // Ignore not found errors
		}

		// Simulate pruning old data
		if blockNum > 0 && blockNum%100 == 0 {
			oldBlockHash := common.Hash{byte((blockNum - 100) >> 24), byte((blockNum - 100) >> 16), byte((blockNum - 100) >> 8), byte(blockNum - 100)}
			oldHeaderKey := append([]byte("h"), oldBlockHash[:]...)
			db.Delete(oldHeaderKey)
		}
	}

	elapsed := time.Since(start)
	b.ReportMetric(float64(b.N)/elapsed.Seconds(), "blocks/sec")
}

// BenchmarkIteratorPerformance tests iterator performance
func BenchmarkIteratorPerformance(b *testing.B) {
	backends := []string{"pebbledb", "leveldb"}
	
	for _, backend := range backends {
		b.Run(backend, func(b *testing.B) {
			benchmarkIterator(b, backend)
		})
	}
}

func benchmarkIterator(b *testing.B, backend string) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bench_iter_"+backend+"_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create database
	var db ethdb.Database
	if backend == "pebbledb" {
		db, err = rawdb.NewPebbleDBDatabase(filepath.Join(tmpDir, "db"), 256, 500, "", false, false)
	} else {
		db, err = rawdb.NewLevelDBDatabase(filepath.Join(tmpDir, "db"), 256, 500, "", false)
	}
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// Populate database with sorted keys
	prefix := []byte("test")
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := append(prefix, fmt.Sprintf("%08d", i)...)
		value := make([]byte, 100)
		rand.Read(value)
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Iterate through all keys with prefix
		iter := db.NewIterator(prefix, nil)
		count := 0
		for iter.Next() {
			count++
			_ = iter.Key()
			_ = iter.Value()
		}
		iter.Release()
		
		if count != numKeys {
			b.Fatalf("expected %d keys, got %d", numKeys, count)
		}
	}
}