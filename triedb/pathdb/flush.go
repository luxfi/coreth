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

	"github.com/VictoriaMetrics/fastcache"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/trie/trienode"
)

// nodeCacheKey constructs the unique key of clean cache. The assumption is held
// that zero address does not have any associated storage slots.
func nodeCacheKey(owner common.Hash, path []byte) []byte {
	if owner == (common.Hash{}) {
		return path
	}
	return append(owner.Bytes(), path...)
}

// writeNodes writes the trie nodes into the provided database batch.
// Note this function will also inject all the newly written nodes
// into clean cache.
func writeNodes(batch ethdb.Batch, nodes map[common.Hash]map[string]*trienode.Node, clean *fastcache.Cache) (total int) {
	for owner, subset := range nodes {
		for path, n := range subset {
			if n.IsDeleted() {
				if owner == (common.Hash{}) {
					rawdb.DeleteAccountTrieNode(batch, []byte(path))
				} else {
					rawdb.DeleteStorageTrieNode(batch, owner, []byte(path))
				}
				if clean != nil {
					clean.Del(nodeCacheKey(owner, []byte(path)))
				}
			} else {
				if owner == (common.Hash{}) {
					rawdb.WriteAccountTrieNode(batch, []byte(path), n.Blob)
				} else {
					rawdb.WriteStorageTrieNode(batch, owner, []byte(path), n.Blob)
				}
				if clean != nil {
					clean.Set(nodeCacheKey(owner, []byte(path)), n.Blob)
				}
			}
		}
		total += len(subset)
	}
	return total
}

// writeStates flushes state mutations into the provided database batch as a whole.
//
// This function assumes the background generator is already terminated and states
// before the supplied marker has been correctly generated.
//
// TODO(rjl493456442) do we really need this generation marker? The state updates
// after the marker can also be written and will be fixed by generator later if
// it's outdated.
func writeStates(batch ethdb.Batch, genMarker []byte, accountData map[common.Hash][]byte, storageData map[common.Hash]map[common.Hash][]byte, clean *fastcache.Cache) (int, int) {
	var (
		accounts int
		slots    int
	)
	for addrHash, blob := range accountData {
		// Skip any account not yet covered by the snapshot. The account
		// at the generation marker position (addrHash == genMarker[:common.HashLength])
		// should still be updated, as it would be skipped in the next
		// generation cycle.
		if genMarker != nil && bytes.Compare(addrHash[:], genMarker) > 0 {
			continue
		}
		accounts += 1
		if len(blob) == 0 {
			rawdb.DeleteAccountSnapshot(batch, addrHash)
			if clean != nil {
				clean.Set(addrHash[:], nil)
			}
		} else {
			rawdb.WriteAccountSnapshot(batch, addrHash, blob)
			if clean != nil {
				clean.Set(addrHash[:], blob)
			}
		}
	}
	for addrHash, storages := range storageData {
		// Skip any account not covered yet by the snapshot
		if genMarker != nil && bytes.Compare(addrHash[:], genMarker) > 0 {
			continue
		}
		midAccount := genMarker != nil && bytes.Equal(addrHash[:], genMarker[:common.HashLength])

		for storageHash, blob := range storages {
			// Skip any storage slot not yet covered by the snapshot. The storage slot
			// at the generation marker position (addrHash == genMarker[:common.HashLength]
			// and storageHash == genMarker[common.HashLength:]) should still be updated,
			// as it would be skipped in the next generation cycle.
			if midAccount && bytes.Compare(storageHash[:], genMarker[common.HashLength:]) > 0 {
				continue
			}
			slots += 1
			if len(blob) == 0 {
				rawdb.DeleteStorageSnapshot(batch, addrHash, storageHash)
				if clean != nil {
					clean.Set(append(addrHash[:], storageHash[:]...), nil)
				}
			} else {
				rawdb.WriteStorageSnapshot(batch, addrHash, storageHash, blob)
				if clean != nil {
					clean.Set(append(addrHash[:], storageHash[:]...), blob)
				}
			}
		}
	}
	return accounts, slots
}
