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

package trie

import (
	"bytes"
	"fmt"
	"maps"
	"math/rand"
	"testing"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/crypto"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/ethdb/memorydb"
	"github.com/luxfi/geth/trie/trienode"
)

// makeTestTrie create a sample test trie to test node-wise reconstruction.
func makeTestTrie(scheme string) (ethdb.Database, *testDb, *StateTrie, map[string][]byte) {
	// Create an empty trie
	db := rawdb.NewMemoryDatabase()
	triedb := newTestDatabase(db, scheme)
	trie, _ := NewStateTrie(TrieID(types.EmptyRootHash), triedb)

	// Fill it with some arbitrary data
	content := make(map[string][]byte)
	for i := byte(0); i < 255; i++ {
		// Map the same data under multiple keys
		key, val := common.LeftPadBytes([]byte{1, i}, 32), []byte{i}
		content[string(key)] = val
		trie.MustUpdate(key, val)

		key, val = common.LeftPadBytes([]byte{2, i}, 32), []byte{i}
		content[string(key)] = val
		trie.MustUpdate(key, val)

		// Add some other data to inflate the trie
		for j := byte(3); j < 13; j++ {
			key, val = common.LeftPadBytes([]byte{j, i}, 32), []byte{j, i}
			content[string(key)] = val
			trie.MustUpdate(key, val)
		}
	}
	root, nodes := trie.Commit(false)
	if err := triedb.Update(root, types.EmptyRootHash, trienode.NewWithNodeSet(nodes)); err != nil {
		panic(fmt.Errorf("failed to commit db %v", err))
	}
	if err := triedb.Commit(root); err != nil {
		panic(err)
	}
	// Re-create the trie based on the new state
	trie, _ = NewStateTrie(TrieID(root), triedb)
	return db, triedb, trie, content
}

// checkTrieContents cross references a reconstructed trie with an expected data
// content map.
func checkTrieContents(t *testing.T, db ethdb.Database, scheme string, root []byte, content map[string][]byte, rawTrie bool) {
	// Check root availability and trie contents
	ndb := newTestDatabase(db, scheme)
	if err := checkTrieConsistency(db, scheme, common.BytesToHash(root), rawTrie); err != nil {
		t.Fatalf("inconsistent trie at %x: %v", root, err)
	}
	type reader interface {
		MustGet(key []byte) []byte
	}
	var r reader
	if rawTrie {
		trie, err := New(TrieID(common.BytesToHash(root)), ndb)
		if err != nil {
			t.Fatalf("failed to create trie at %x: %v", root, err)
		}
		r = trie
	} else {
		trie, err := NewStateTrie(TrieID(common.BytesToHash(root)), ndb)
		if err != nil {
			t.Fatalf("failed to create trie at %x: %v", root, err)
		}
		r = trie
	}
	for key, val := range content {
		if have := r.MustGet([]byte(key)); !bytes.Equal(have, val) {
			t.Errorf("entry %x: content mismatch: have %x, want %x", key, have, val)
		}
	}
}

// checkTrieConsistency checks that all nodes in a trie are indeed present.
func checkTrieConsistency(db ethdb.Database, scheme string, root common.Hash, rawTrie bool) error {
	ndb := newTestDatabase(db, scheme)
	var it NodeIterator
	if rawTrie {
		trie, err := New(TrieID(root), ndb)
		if err != nil {
			return nil // Consider a non existent state consistent
		}
		it = trie.MustNodeIterator(nil)
	} else {
		trie, err := NewStateTrie(TrieID(root), ndb)
		if err != nil {
			return nil // Consider a non existent state consistent
		}
		it = trie.MustNodeIterator(nil)
	}
	for it.Next(true) {
	}
	return it.Error()
}

// trieElement represents the element in the state trie(bytecode or trie node).
type trieElement struct {
	path     string
	hash     common.Hash
	syncPath SyncPath
}

// Tests that an empty trie is not scheduled for syncing.
func TestEmptySync(t *testing.T) {
	dbA := newTestDatabase(rawdb.NewMemoryDatabase(), rawdb.HashScheme)
	dbB := newTestDatabase(rawdb.NewMemoryDatabase(), rawdb.HashScheme)
	dbC := newTestDatabase(rawdb.NewMemoryDatabase(), rawdb.PathScheme)
	dbD := newTestDatabase(rawdb.NewMemoryDatabase(), rawdb.PathScheme)

	emptyA := NewEmpty(dbA)
	emptyB, _ := New(TrieID(types.EmptyRootHash), dbB)
	emptyC := NewEmpty(dbC)
	emptyD, _ := New(TrieID(types.EmptyRootHash), dbD)

	for i, trie := range []*Trie{emptyA, emptyB, emptyC, emptyD} {
		sync := NewSync(trie.Hash(), memorydb.New(), nil, []*testDb{dbA, dbB, dbC, dbD}[i].Scheme())
		if paths, nodes, codes := sync.Missing(1); len(paths) != 0 || len(nodes) != 0 || len(codes) != 0 {
			t.Errorf("test %d: content requested for empty trie: %v, %v, %v", i, paths, nodes, codes)
		}
	}
}

// Tests that given a root hash, a trie can sync iteratively on a single thread,
// requesting retrieval tasks and returning all of them in one go.
func TestIterativeSync(t *testing.T) {
	testIterativeSync(t, 1, false, rawdb.HashScheme)
	testIterativeSync(t, 100, false, rawdb.HashScheme)
	testIterativeSync(t, 1, true, rawdb.HashScheme)
	testIterativeSync(t, 100, true, rawdb.HashScheme)
	testIterativeSync(t, 1, false, rawdb.PathScheme)
	testIterativeSync(t, 100, false, rawdb.PathScheme)
	testIterativeSync(t, 1, true, rawdb.PathScheme)
	testIterativeSync(t, 100, true, rawdb.PathScheme)
}

func testIterativeSync(t *testing.T, count int, bypath bool, scheme string) {
	// Create a random trie to copy
	_, srcDb, srcTrie, srcData := makeTestTrie(scheme)

	// Create a destination trie and sync with the scheduler
	diskdb := rawdb.NewMemoryDatabase()
	sched := NewSync(srcTrie.Hash(), diskdb, nil, srcDb.Scheme())

	// The code requests are ignored here since there is no code
	// at the testing trie.
	paths, nodes, _ := sched.Missing(count)
	var elements []trieElement
	for i := 0; i < len(paths); i++ {
		elements = append(elements, trieElement{
			path:     paths[i],
			hash:     nodes[i],
			syncPath: NewSyncPath([]byte(paths[i])),
		})
	}
	reader, err := srcDb.NodeReader(srcTrie.Hash())
	if err != nil {
		t.Fatalf("State is not available %x", srcTrie.Hash())
	}
	for len(elements) > 0 {
		results := make([]NodeSyncResult, len(elements))
		if !bypath {
			for i, element := range elements {
				owner, inner := ResolvePath([]byte(element.path))
				data, err := reader.Node(owner, inner, element.hash)
				if err != nil {
					t.Fatalf("failed to retrieve node data for hash %x: %v", element.hash, err)
				}
				results[i] = NodeSyncResult{element.path, data}
			}
		} else {
			for i, element := range elements {
				data, _, err := srcTrie.GetNode(element.syncPath[len(element.syncPath)-1])
				if err != nil {
					t.Fatalf("failed to retrieve node data for path %x: %v", element.path, err)
				}
				results[i] = NodeSyncResult{element.path, data}
			}
		}
		for _, result := range results {
			if err := sched.ProcessNode(result); err != nil {
				t.Fatalf("failed to process result %v", err)
			}
		}
		batch := diskdb.NewBatch()
		if err := sched.Commit(batch); err != nil {
			t.Fatalf("failed to commit data: %v", err)
		}
		batch.Write()

		paths, nodes, _ = sched.Missing(count)
		elements = elements[:0]
		for i := 0; i < len(paths); i++ {
			elements = append(elements, trieElement{
				path:     paths[i],
				hash:     nodes[i],
				syncPath: NewSyncPath([]byte(paths[i])),
			})
		}
	}
	// Cross check that the two tries are in sync
	checkTrieContents(t, diskdb, srcDb.Scheme(), srcTrie.Hash().Bytes(), srcData, false)
}

// Tests that the trie scheduler can correctly reconstruct the state even if only
// partial results are returned, and the others sent only later.
func TestIterativeDelayedSync(t *testing.T) {
	testIterativeDelayedSync(t, rawdb.HashScheme)
	testIterativeDelayedSync(t, rawdb.PathScheme)
}

func testIterativeDelayedSync(t *testing.T, scheme string) {
	// Create a random trie to copy
	_, srcDb, srcTrie, srcData := makeTestTrie(scheme)

	// Create a destination trie and sync with the scheduler
	diskdb := rawdb.NewMemoryDatabase()
	sched := NewSync(srcTrie.Hash(), diskdb, nil, srcDb.Scheme())

	// The code requests are ignored here since there is no code
	// at the testing trie.
	paths, nodes, _ := sched.Missing(10000)
	var elements []trieElement
	for i := 0; i < len(paths); i++ {
		elements = append(elements, trieElement{
			path:     paths[i],
			hash:     nodes[i],
			syncPath: NewSyncPath([]byte(paths[i])),
		})
	}
	reader, err := srcDb.NodeReader(srcTrie.Hash())
	if err != nil {
		t.Fatalf("State is not available %x", srcTrie.Hash())
	}
	for len(elements) > 0 {
		// Sync only half of the scheduled nodes
		results := make([]NodeSyncResult, len(elements)/2+1)
		for i, element := range elements[:len(results)] {
			owner, inner := ResolvePath([]byte(element.path))
			data, err := reader.Node(owner, inner, element.hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", element.hash, err)
			}
			results[i] = NodeSyncResult{element.path, data}
		}
		for _, result := range results {
			if err := sched.ProcessNode(result); err != nil {
				t.Fatalf("failed to process result %v", err)
			}
		}
		batch := diskdb.NewBatch()
		if err := sched.Commit(batch); err != nil {
			t.Fatalf("failed to commit data: %v", err)
		}
		batch.Write()

		paths, nodes, _ = sched.Missing(10000)
		elements = elements[len(results):]
		for i := 0; i < len(paths); i++ {
			elements = append(elements, trieElement{
				path:     paths[i],
				hash:     nodes[i],
				syncPath: NewSyncPath([]byte(paths[i])),
			})
		}
	}
	// Cross check that the two tries are in sync
	checkTrieContents(t, diskdb, srcDb.Scheme(), srcTrie.Hash().Bytes(), srcData, false)
}

// Tests that given a root hash, a trie can sync iteratively on a single thread,
// requesting retrieval tasks and returning all of them in one go, however in a
// random order.
func TestIterativeRandomSyncIndividual(t *testing.T) {
	testIterativeRandomSync(t, 1, rawdb.HashScheme)
	testIterativeRandomSync(t, 100, rawdb.HashScheme)
	testIterativeRandomSync(t, 1, rawdb.PathScheme)
	testIterativeRandomSync(t, 100, rawdb.PathScheme)
}

func testIterativeRandomSync(t *testing.T, count int, scheme string) {
	// Create a random trie to copy
	_, srcDb, srcTrie, srcData := makeTestTrie(scheme)

	// Create a destination trie and sync with the scheduler
	diskdb := rawdb.NewMemoryDatabase()
	sched := NewSync(srcTrie.Hash(), diskdb, nil, srcDb.Scheme())

	// The code requests are ignored here since there is no code
	// at the testing trie.
	paths, nodes, _ := sched.Missing(count)
	queue := make(map[string]trieElement)
	for i, path := range paths {
		queue[path] = trieElement{
			path:     paths[i],
			hash:     nodes[i],
			syncPath: NewSyncPath([]byte(paths[i])),
		}
	}
	reader, err := srcDb.NodeReader(srcTrie.Hash())
	if err != nil {
		t.Fatalf("State is not available %x", srcTrie.Hash())
	}
	for len(queue) > 0 {
		// Fetch all the queued nodes in a random order
		results := make([]NodeSyncResult, 0, len(queue))
		for path, element := range queue {
			owner, inner := ResolvePath([]byte(element.path))
			data, err := reader.Node(owner, inner, element.hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", element.hash, err)
			}
			results = append(results, NodeSyncResult{path, data})
		}
		// Feed the retrieved results back and queue new tasks
		for _, result := range results {
			if err := sched.ProcessNode(result); err != nil {
				t.Fatalf("failed to process result %v", err)
			}
		}
		batch := diskdb.NewBatch()
		if err := sched.Commit(batch); err != nil {
			t.Fatalf("failed to commit data: %v", err)
		}
		batch.Write()

		paths, nodes, _ = sched.Missing(count)
		queue = make(map[string]trieElement)
		for i, path := range paths {
			queue[path] = trieElement{
				path:     path,
				hash:     nodes[i],
				syncPath: NewSyncPath([]byte(path)),
			}
		}
	}
	// Cross check that the two tries are in sync
	checkTrieContents(t, diskdb, srcDb.Scheme(), srcTrie.Hash().Bytes(), srcData, false)
}

// Tests that the trie scheduler can correctly reconstruct the state even if only
// partial results are returned (Even those randomly), others sent only later.
func TestIterativeRandomDelayedSync(t *testing.T) {
	testIterativeRandomDelayedSync(t, rawdb.HashScheme)
	testIterativeRandomDelayedSync(t, rawdb.PathScheme)
}

func testIterativeRandomDelayedSync(t *testing.T, scheme string) {
	// Create a random trie to copy
	_, srcDb, srcTrie, srcData := makeTestTrie(scheme)

	// Create a destination trie and sync with the scheduler
	diskdb := rawdb.NewMemoryDatabase()
	sched := NewSync(srcTrie.Hash(), diskdb, nil, srcDb.Scheme())

	// The code requests are ignored here since there is no code
	// at the testing trie.
	paths, nodes, _ := sched.Missing(10000)
	queue := make(map[string]trieElement)
	for i, path := range paths {
		queue[path] = trieElement{
			path:     path,
			hash:     nodes[i],
			syncPath: NewSyncPath([]byte(path)),
		}
	}
	reader, err := srcDb.NodeReader(srcTrie.Hash())
	if err != nil {
		t.Fatalf("State is not available %x", srcTrie.Hash())
	}
	for len(queue) > 0 {
		// Sync only half of the scheduled nodes, even those in random order
		results := make([]NodeSyncResult, 0, len(queue)/2+1)
		for path, element := range queue {
			owner, inner := ResolvePath([]byte(element.path))
			data, err := reader.Node(owner, inner, element.hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", element.hash, err)
			}
			results = append(results, NodeSyncResult{path, data})

			if len(results) >= cap(results) {
				break
			}
		}
		// Feed the retrieved results back and queue new tasks
		for _, result := range results {
			if err := sched.ProcessNode(result); err != nil {
				t.Fatalf("failed to process result %v", err)
			}
		}
		batch := diskdb.NewBatch()
		if err := sched.Commit(batch); err != nil {
			t.Fatalf("failed to commit data: %v", err)
		}
		batch.Write()
		for _, result := range results {
			delete(queue, result.Path)
		}
		paths, nodes, _ = sched.Missing(10000)
		for i, path := range paths {
			queue[path] = trieElement{
				path:     path,
				hash:     nodes[i],
				syncPath: NewSyncPath([]byte(path)),
			}
		}
	}
	// Cross check that the two tries are in sync
	checkTrieContents(t, diskdb, srcDb.Scheme(), srcTrie.Hash().Bytes(), srcData, false)
}

// Tests that a trie sync will not request nodes multiple times, even if they
// have such references.
func TestDuplicateAvoidanceSync(t *testing.T) {
	testDuplicateAvoidanceSync(t, rawdb.HashScheme)
	testDuplicateAvoidanceSync(t, rawdb.PathScheme)
}

func testDuplicateAvoidanceSync(t *testing.T, scheme string) {
	// Create a random trie to copy
	_, srcDb, srcTrie, srcData := makeTestTrie(scheme)

	// Create a destination trie and sync with the scheduler
	diskdb := rawdb.NewMemoryDatabase()
	sched := NewSync(srcTrie.Hash(), diskdb, nil, srcDb.Scheme())

	// The code requests are ignored here since there is no code
	// at the testing trie.
	paths, nodes, _ := sched.Missing(0)
	var elements []trieElement
	for i := 0; i < len(paths); i++ {
		elements = append(elements, trieElement{
			path:     paths[i],
			hash:     nodes[i],
			syncPath: NewSyncPath([]byte(paths[i])),
		})
	}
	reader, err := srcDb.NodeReader(srcTrie.Hash())
	if err != nil {
		t.Fatalf("State is not available %x", srcTrie.Hash())
	}
	requested := make(map[common.Hash]struct{})
	for len(elements) > 0 {
		results := make([]NodeSyncResult, len(elements))
		for i, element := range elements {
			owner, inner := ResolvePath([]byte(element.path))
			data, err := reader.Node(owner, inner, element.hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", element.hash, err)
			}
			if _, ok := requested[element.hash]; ok {
				t.Errorf("hash %x already requested once", element.hash)
			}
			requested[element.hash] = struct{}{}

			results[i] = NodeSyncResult{element.path, data}
		}
		for _, result := range results {
			if err := sched.ProcessNode(result); err != nil {
				t.Fatalf("failed to process result %v", err)
			}
		}
		batch := diskdb.NewBatch()
		if err := sched.Commit(batch); err != nil {
			t.Fatalf("failed to commit data: %v", err)
		}
		batch.Write()

		paths, nodes, _ = sched.Missing(0)
		elements = elements[:0]
		for i := 0; i < len(paths); i++ {
			elements = append(elements, trieElement{
				path:     paths[i],
				hash:     nodes[i],
				syncPath: NewSyncPath([]byte(paths[i])),
			})
		}
	}
	// Cross check that the two tries are in sync
	checkTrieContents(t, diskdb, srcDb.Scheme(), srcTrie.Hash().Bytes(), srcData, false)
}

// Tests that at any point in time during a sync, only complete sub-tries are in
// the database.
func TestIncompleteSyncHash(t *testing.T) {
	testIncompleteSync(t, rawdb.HashScheme)
	testIncompleteSync(t, rawdb.PathScheme)
}

func testIncompleteSync(t *testing.T, scheme string) {
	// Create a random trie to copy
	_, srcDb, srcTrie, _ := makeTestTrie(scheme)

	// Create a destination trie and sync with the scheduler
	diskdb := rawdb.NewMemoryDatabase()
	sched := NewSync(srcTrie.Hash(), diskdb, nil, srcDb.Scheme())

	// The code requests are ignored here since there is no code
	// at the testing trie.
	var (
		addedKeys   []string
		addedHashes []common.Hash
		elements    []trieElement
		root        = srcTrie.Hash()
	)
	paths, nodes, _ := sched.Missing(1)
	for i := 0; i < len(paths); i++ {
		elements = append(elements, trieElement{
			path:     paths[i],
			hash:     nodes[i],
			syncPath: NewSyncPath([]byte(paths[i])),
		})
	}
	reader, err := srcDb.NodeReader(srcTrie.Hash())
	if err != nil {
		t.Fatalf("State is not available %x", srcTrie.Hash())
	}
	for len(elements) > 0 {
		// Fetch a batch of trie nodes
		results := make([]NodeSyncResult, len(elements))
		for i, element := range elements {
			owner, inner := ResolvePath([]byte(element.path))
			data, err := reader.Node(owner, inner, element.hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", element.hash, err)
			}
			results[i] = NodeSyncResult{element.path, data}
		}
		// Process each of the trie nodes
		for _, result := range results {
			if err := sched.ProcessNode(result); err != nil {
				t.Fatalf("failed to process result %v", err)
			}
		}
		batch := diskdb.NewBatch()
		if err := sched.Commit(batch); err != nil {
			t.Fatalf("failed to commit data: %v", err)
		}
		batch.Write()

		for _, result := range results {
			hash := crypto.Keccak256Hash(result.Data)
			if hash != root {
				addedKeys = append(addedKeys, result.Path)
				addedHashes = append(addedHashes, hash)
			}
		}
		// Fetch the next batch to retrieve
		paths, nodes, _ = sched.Missing(1)
		elements = elements[:0]
		for i := 0; i < len(paths); i++ {
			elements = append(elements, trieElement{
				path:     paths[i],
				hash:     nodes[i],
				syncPath: NewSyncPath([]byte(paths[i])),
			})
		}
	}
	// Sanity check that removing any node from the database is detected
	for i, path := range addedKeys {
		if rand.Int31n(100) > 5 {
			// Only check 5 percent of added keys as a sanity check
			continue
		}
		owner, inner := ResolvePath([]byte(path))
		nodeHash := addedHashes[i]
		value := rawdb.ReadTrieNode(diskdb, owner, inner, nodeHash, scheme)
		rawdb.DeleteTrieNode(diskdb, owner, inner, nodeHash, scheme)
		if err := checkTrieConsistency(diskdb, srcDb.Scheme(), root, false); err == nil {
			t.Fatalf("trie inconsistency not caught, missing: %x", path)
		}
		rawdb.WriteTrieNode(diskdb, owner, inner, nodeHash, value, scheme)
	}
}

// Tests that trie nodes get scheduled lexicographically when having the same
// depth.
func TestSyncOrdering(t *testing.T) {
	testSyncOrdering(t, rawdb.HashScheme)
	testSyncOrdering(t, rawdb.PathScheme)
}

func testSyncOrdering(t *testing.T, scheme string) {
	// Create a random trie to copy
	_, srcDb, srcTrie, srcData := makeTestTrie(scheme)

	// Create a destination trie and sync with the scheduler, tracking the requests
	diskdb := rawdb.NewMemoryDatabase()
	sched := NewSync(srcTrie.Hash(), diskdb, nil, srcDb.Scheme())

	// The code requests are ignored here since there is no code
	// at the testing trie.
	var (
		reqs     []SyncPath
		elements []trieElement
	)
	paths, nodes, _ := sched.Missing(1)
	for i := 0; i < len(paths); i++ {
		elements = append(elements, trieElement{
			path:     paths[i],
			hash:     nodes[i],
			syncPath: NewSyncPath([]byte(paths[i])),
		})
		reqs = append(reqs, NewSyncPath([]byte(paths[i])))
	}
	reader, err := srcDb.NodeReader(srcTrie.Hash())
	if err != nil {
		t.Fatalf("State is not available %x", srcTrie.Hash())
	}
	for len(elements) > 0 {
		results := make([]NodeSyncResult, len(elements))
		for i, element := range elements {
			owner, inner := ResolvePath([]byte(element.path))
			data, err := reader.Node(owner, inner, element.hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", element.hash, err)
			}
			results[i] = NodeSyncResult{element.path, data}
		}
		for _, result := range results {
			if err := sched.ProcessNode(result); err != nil {
				t.Fatalf("failed to process result %v", err)
			}
		}
		batch := diskdb.NewBatch()
		if err := sched.Commit(batch); err != nil {
			t.Fatalf("failed to commit data: %v", err)
		}
		batch.Write()

		paths, nodes, _ = sched.Missing(1)
		elements = elements[:0]
		for i := 0; i < len(paths); i++ {
			elements = append(elements, trieElement{
				path:     paths[i],
				hash:     nodes[i],
				syncPath: NewSyncPath([]byte(paths[i])),
			})
			reqs = append(reqs, NewSyncPath([]byte(paths[i])))
		}
	}
	// Cross check that the two tries are in sync
	checkTrieContents(t, diskdb, srcDb.Scheme(), srcTrie.Hash().Bytes(), srcData, false)

	// Check that the trie nodes have been requested path-ordered
	for i := 0; i < len(reqs)-1; i++ {
		if len(reqs[i]) > 1 || len(reqs[i+1]) > 1 {
			// In the case of the trie tests, there's no storage so the tuples
			// must always be single items. 2-tuples should be tested in state.
			t.Errorf("Invalid request tuples: len(%v) or len(%v) > 1", reqs[i], reqs[i+1])
		}
		if bytes.Compare(compactToHex(reqs[i][0]), compactToHex(reqs[i+1][0])) > 0 {
			t.Errorf("Invalid request order: %v before %v", compactToHex(reqs[i][0]), compactToHex(reqs[i+1][0]))
		}
	}
}
func syncWith(t *testing.T, root common.Hash, db ethdb.Database, srcDb *testDb) {
	syncWithHookWriter(t, root, db, srcDb, nil)
}

func syncWithHookWriter(t *testing.T, root common.Hash, db ethdb.Database, srcDb *testDb, hookWriter ethdb.KeyValueWriter) {
	// Create a destination trie and sync with the scheduler
	sched := NewSync(root, db, nil, srcDb.Scheme())

	// The code requests are ignored here since there is no code
	// at the testing trie.
	paths, nodes, _ := sched.Missing(0)
	var elements []trieElement
	for i := 0; i < len(paths); i++ {
		elements = append(elements, trieElement{
			path:     paths[i],
			hash:     nodes[i],
			syncPath: NewSyncPath([]byte(paths[i])),
		})
	}
	reader, err := srcDb.NodeReader(root)
	if err != nil {
		t.Fatalf("State is not available %x", root)
	}
	for len(elements) > 0 {
		results := make([]NodeSyncResult, len(elements))
		for i, element := range elements {
			owner, inner := ResolvePath([]byte(element.path))
			data, err := reader.Node(owner, inner, element.hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for hash %x: %v", element.hash, err)
			}
			results[i] = NodeSyncResult{element.path, data}
		}
		for index, result := range results {
			if err := sched.ProcessNode(result); err != nil {
				t.Fatalf("failed to process result[%d][%v] data %v %v", index, []byte(result.Path), result.Data, err)
			}
		}
		batch := db.NewBatch()
		if err := sched.Commit(batch); err != nil {
			t.Fatalf("failed to commit data: %v", err)
		}
		if hookWriter != nil {
			batch.Replay(hookWriter)
		} else {
			batch.Write()
		}
		paths, nodes, _ = sched.Missing(0)
		elements = elements[:0]
		for i := 0; i < len(paths); i++ {
			elements = append(elements, trieElement{
				path:     paths[i],
				hash:     nodes[i],
				syncPath: NewSyncPath([]byte(paths[i])),
			})
		}
	}
}

// Tests that the syncing target is keeping moving which may overwrite the stale
// states synced in the last cycle.
func TestSyncMovingTarget(t *testing.T) {
	testSyncMovingTarget(t, rawdb.HashScheme)
	testSyncMovingTarget(t, rawdb.PathScheme)
}

func testSyncMovingTarget(t *testing.T, scheme string) {
	// Create a random trie to copy
	_, srcDb, srcTrie, srcData := makeTestTrie(scheme)

	// Create a destination trie and sync with the scheduler
	diskdb := rawdb.NewMemoryDatabase()
	syncWith(t, srcTrie.Hash(), diskdb, srcDb)
	checkTrieContents(t, diskdb, srcDb.Scheme(), srcTrie.Hash().Bytes(), srcData, false)

	// Push more modifications into the src trie, to see if dest trie can still
	// sync with it(overwrite stale states)
	var (
		preRoot = srcTrie.Hash()
		diff    = make(map[string][]byte)
	)
	for i := byte(0); i < 10; i++ {
		key, val := randBytes(32), randBytes(32)
		srcTrie.MustUpdate(key, val)
		diff[string(key)] = val
	}
	root, nodes := srcTrie.Commit(false)
	if err := srcDb.Update(root, preRoot, trienode.NewWithNodeSet(nodes)); err != nil {
		panic(err)
	}
	if err := srcDb.Commit(root); err != nil {
		panic(err)
	}
	preRoot = root
	srcTrie, _ = NewStateTrie(TrieID(root), srcDb)

	syncWith(t, srcTrie.Hash(), diskdb, srcDb)
	checkTrieContents(t, diskdb, srcDb.Scheme(), srcTrie.Hash().Bytes(), diff, false)

	// Revert added modifications from the src trie, to see if dest trie can still
	// sync with it(overwrite reverted states)
	var reverted = make(map[string][]byte)
	for k := range diff {
		srcTrie.MustDelete([]byte(k))
		reverted[k] = nil
	}
	for k := range srcData {
		val := randBytes(32)
		srcTrie.MustUpdate([]byte(k), val)
		reverted[k] = val
	}
	root, nodes = srcTrie.Commit(false)
	if err := srcDb.Update(root, preRoot, trienode.NewWithNodeSet(nodes)); err != nil {
		panic(err)
	}
	if err := srcDb.Commit(root); err != nil {
		panic(err)
	}
	srcTrie, _ = NewStateTrie(TrieID(root), srcDb)

	syncWith(t, srcTrie.Hash(), diskdb, srcDb)
	checkTrieContents(t, diskdb, srcDb.Scheme(), srcTrie.Hash().Bytes(), reverted, false)
}

// Tests if state syncer can correctly catch up the pivot move.
func TestPivotMove(t *testing.T) {
	testPivotMove(t, rawdb.HashScheme, true)
	testPivotMove(t, rawdb.HashScheme, false)
	testPivotMove(t, rawdb.PathScheme, true)
	testPivotMove(t, rawdb.PathScheme, false)
}

func testPivotMove(t *testing.T, scheme string, tiny bool) {
	var (
		srcDisk    = rawdb.NewMemoryDatabase()
		srcTrieDB  = newTestDatabase(srcDisk, scheme)
		srcTrie, _ = New(TrieID(types.EmptyRootHash), srcTrieDB)

		deleteFn = func(key []byte, tr *Trie, states map[string][]byte) {
			tr.Delete(key)
			delete(states, string(key))
		}
		writeFn = func(key []byte, val []byte, tr *Trie, states map[string][]byte) {
			if val == nil {
				if tiny {
					val = randBytes(4)
				} else {
					val = randBytes(32)
				}
			}
			tr.Update(key, val)
			states[string(key)] = common.CopyBytes(val)
		}
	)
	stateA := make(map[string][]byte)
	writeFn([]byte{0x01, 0x23}, nil, srcTrie, stateA)
	writeFn([]byte{0x01, 0x24}, nil, srcTrie, stateA)
	writeFn([]byte{0x12, 0x33}, nil, srcTrie, stateA)
	writeFn([]byte{0x12, 0x34}, nil, srcTrie, stateA)
	writeFn([]byte{0x02, 0x34}, nil, srcTrie, stateA)
	writeFn([]byte{0x13, 0x44}, nil, srcTrie, stateA)

	rootA, nodesA := srcTrie.Commit(false)
	if err := srcTrieDB.Update(rootA, types.EmptyRootHash, trienode.NewWithNodeSet(nodesA)); err != nil {
		panic(err)
	}
	if err := srcTrieDB.Commit(rootA); err != nil {
		panic(err)
	}
	// Create a destination trie and sync with the scheduler
	destDisk := rawdb.NewMemoryDatabase()
	syncWith(t, rootA, destDisk, srcTrieDB)
	checkTrieContents(t, destDisk, scheme, srcTrie.Hash().Bytes(), stateA, true)

	// Delete element to collapse trie
	stateB := maps.Clone(stateA)
	srcTrie, _ = New(TrieID(rootA), srcTrieDB)
	deleteFn([]byte{0x02, 0x34}, srcTrie, stateB)
	deleteFn([]byte{0x13, 0x44}, srcTrie, stateB)
	writeFn([]byte{0x01, 0x24}, nil, srcTrie, stateB)

	rootB, nodesB := srcTrie.Commit(false)
	if err := srcTrieDB.Update(rootB, rootA, trienode.NewWithNodeSet(nodesB)); err != nil {
		panic(err)
	}
	if err := srcTrieDB.Commit(rootB); err != nil {
		panic(err)
	}
	syncWith(t, rootB, destDisk, srcTrieDB)
	checkTrieContents(t, destDisk, scheme, srcTrie.Hash().Bytes(), stateB, true)

	// Add elements to expand trie
	stateC := maps.Clone(stateB)
	srcTrie, _ = New(TrieID(rootB), srcTrieDB)

	writeFn([]byte{0x01, 0x24}, stateA[string([]byte{0x01, 0x24})], srcTrie, stateC)
	writeFn([]byte{0x02, 0x34}, nil, srcTrie, stateC)
	writeFn([]byte{0x13, 0x44}, nil, srcTrie, stateC)

	rootC, nodesC := srcTrie.Commit(false)
	if err := srcTrieDB.Update(rootC, rootB, trienode.NewWithNodeSet(nodesC)); err != nil {
		panic(err)
	}
	if err := srcTrieDB.Commit(rootC); err != nil {
		panic(err)
	}
	syncWith(t, rootC, destDisk, srcTrieDB)
	checkTrieContents(t, destDisk, scheme, srcTrie.Hash().Bytes(), stateC, true)
}

func TestSyncAbort(t *testing.T) {
	testSyncAbort(t, rawdb.PathScheme)
	testSyncAbort(t, rawdb.HashScheme)
}

type hookWriter struct {
	db     ethdb.KeyValueStore
	filter func(key []byte, value []byte) bool
}

// Put inserts the given value into the key-value data store.
func (w *hookWriter) Put(key []byte, value []byte) error {
	if w.filter != nil && w.filter(key, value) {
		return nil
	}
	return w.db.Put(key, value)
}

// Delete removes the key from the key-value data store.
func (w *hookWriter) Delete(key []byte) error {
	return w.db.Delete(key)
}

func testSyncAbort(t *testing.T, scheme string) {
	var (
		srcDisk    = rawdb.NewMemoryDatabase()
		srcTrieDB  = newTestDatabase(srcDisk, scheme)
		srcTrie, _ = New(TrieID(types.EmptyRootHash), srcTrieDB)

		deleteFn = func(key []byte, tr *Trie, states map[string][]byte) {
			tr.Delete(key)
			delete(states, string(key))
		}
		writeFn = func(key []byte, val []byte, tr *Trie, states map[string][]byte) {
			if val == nil {
				val = randBytes(32)
			}
			tr.Update(key, val)
			states[string(key)] = common.CopyBytes(val)
		}
	)
	var (
		stateA = make(map[string][]byte)
		key    = randBytes(32)
		val    = randBytes(32)
	)
	for i := 0; i < 256; i++ {
		writeFn(randBytes(32), nil, srcTrie, stateA)
	}
	writeFn(key, val, srcTrie, stateA)

	rootA, nodesA := srcTrie.Commit(false)
	if err := srcTrieDB.Update(rootA, types.EmptyRootHash, trienode.NewWithNodeSet(nodesA)); err != nil {
		panic(err)
	}
	if err := srcTrieDB.Commit(rootA); err != nil {
		panic(err)
	}
	// Create a destination trie and sync with the scheduler
	destDisk := rawdb.NewMemoryDatabase()
	syncWith(t, rootA, destDisk, srcTrieDB)
	checkTrieContents(t, destDisk, scheme, srcTrie.Hash().Bytes(), stateA, true)

	// Delete the element from the trie
	stateB := maps.Clone(stateA)
	srcTrie, _ = New(TrieID(rootA), srcTrieDB)
	deleteFn(key, srcTrie, stateB)

	rootB, nodesB := srcTrie.Commit(false)
	if err := srcTrieDB.Update(rootB, rootA, trienode.NewWithNodeSet(nodesB)); err != nil {
		panic(err)
	}
	if err := srcTrieDB.Commit(rootB); err != nil {
		panic(err)
	}

	// Sync the new state, but never persist the new root node. Before the
	// fix #28595, the original old root node will still be left in database
	// which breaks the next healing cycle.
	syncWithHookWriter(t, rootB, destDisk, srcTrieDB, &hookWriter{db: destDisk, filter: func(key []byte, value []byte) bool {
		if scheme == rawdb.HashScheme {
			return false
		}
		if len(value) == 0 {
			return false
		}
		ok, path := rawdb.ResolveAccountTrieNodeKey(key)
		return ok && len(path) == 0
	}})

	// Add elements to expand trie
	stateC := maps.Clone(stateB)
	srcTrie, _ = New(TrieID(rootB), srcTrieDB)

	writeFn(key, val, srcTrie, stateC)
	rootC, nodesC := srcTrie.Commit(false)
	if err := srcTrieDB.Update(rootC, rootB, trienode.NewWithNodeSet(nodesC)); err != nil {
		panic(err)
	}
	if err := srcTrieDB.Commit(rootC); err != nil {
		panic(err)
	}
	syncWith(t, rootC, destDisk, srcTrieDB)
	checkTrieContents(t, destDisk, scheme, srcTrie.Hash().Bytes(), stateC, true)
}
