// Copyright 2018 The go-ethereum Authors
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

// Package rawdb contains a collection of low level database accessors.
package rawdb

import (
	"bytes"
	"encoding/binary"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/crypto"
	"github.com/luxfi/geth/metrics"
)

// The fields below define the low level database schema prefixing.
var (
	// databaseVersionKey tracks the current database version.
	databaseVersionKey = []byte("DatabaseVersion")

	// headHeaderKey tracks the latest known header's hash.
	headHeaderKey = []byte("LastHeader")

	// headBlockKey tracks the latest known full block's hash.
	headBlockKey = []byte("LastBlock")

	// headFastBlockKey tracks the latest known incomplete block's hash during fast sync.
	headFastBlockKey = []byte("LastFast")

	// headFinalizedBlockKey tracks the latest known finalized block hash.
	headFinalizedBlockKey = []byte("LastFinalized")

	// persistentStateIDKey tracks the id of latest stored state(for path-based only).
	persistentStateIDKey = []byte("LastStateID")

	// lastPivotKey tracks the last pivot block used by fast sync (to reenable on sethead).
	lastPivotKey = []byte("LastPivot")

	// fastTrieProgressKey tracks the number of trie entries imported during fast sync.
	fastTrieProgressKey = []byte("TrieSync")

	// snapshotDisabledKey flags that the snapshot should not be maintained due to initial sync.
	snapshotDisabledKey = []byte("SnapshotDisabled")

	// SnapshotRootKey tracks the hash of the last snapshot.
	SnapshotRootKey = []byte("SnapshotRoot")

	// snapshotJournalKey tracks the in-memory diff layers across restarts.
	snapshotJournalKey = []byte("SnapshotJournal")

	// snapshotGeneratorKey tracks the snapshot generation marker across restarts.
	snapshotGeneratorKey = []byte("SnapshotGenerator")

	// snapshotRecoveryKey tracks the snapshot recovery marker across restarts.
	snapshotRecoveryKey = []byte("SnapshotRecovery")

	// snapshotSyncStatusKey tracks the snapshot sync status across restarts.
	snapshotSyncStatusKey = []byte("SnapshotSyncStatus")

	// skeletonSyncStatusKey tracks the skeleton sync status across restarts.
	skeletonSyncStatusKey = []byte("SkeletonSyncStatus")

	// trieJournalKey tracks the in-memory trie node layers across restarts.
	trieJournalKey = []byte("TrieJournal")

	// headStateHistoryIndexKey tracks the ID of the latest state history that has
	// been indexed.
	headStateHistoryIndexKey = []byte("LastStateHistoryIndex")

	// txIndexTailKey tracks the oldest block whose transactions have been indexed.
	txIndexTailKey = []byte("TransactionIndexTail")

	// fastTxLookupLimitKey tracks the transaction lookup limit during fast sync.
	// This flag is deprecated, it's kept to avoid reporting errors when inspect
	// database.
	fastTxLookupLimitKey = []byte("FastTransactionLookupLimit")

	// badBlockKey tracks the list of bad blocks seen by local
	badBlockKey = []byte("InvalidBlock")

	// uncleanShutdownKey tracks the list of local crashes
	uncleanShutdownKey = []byte("unclean-shutdown") // config prefix for the db

	// transitionStatusKey tracks the eth2 transition status.
	transitionStatusKey = []byte("eth2-transition")

	// snapSyncStatusFlagKey flags that status of snap sync.
	snapSyncStatusFlagKey = []byte("SnapSyncStatus")

	// Data item prefixes (use single byte to avoid mixing data types, avoid `i`, used for indexes).
	headerPrefix       = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
	headerTDSuffix     = []byte("t") // headerPrefix + num (uint64 big endian) + hash + headerTDSuffix -> td (deprecated)
	headerHashSuffix   = []byte("n") // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerNumberPrefix = []byte("H") // headerNumberPrefix + hash -> num (uint64 big endian)

	blockBodyPrefix     = []byte("b") // blockBodyPrefix + num (uint64 big endian) + hash -> block body
	blockReceiptsPrefix = []byte("r") // blockReceiptsPrefix + num (uint64 big endian) + hash -> block receipts

	txLookupPrefix        = []byte("l") // txLookupPrefix + hash -> transaction/receipt lookup metadata
	bloomBitsPrefix       = []byte("B") // bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash -> bloom bits
	SnapshotAccountPrefix = []byte("a") // SnapshotAccountPrefix + account hash -> account trie value
	SnapshotStoragePrefix = []byte("o") // SnapshotStoragePrefix + account hash + storage hash -> storage trie value
	CodePrefix            = []byte("c") // CodePrefix + code hash -> account code
	skeletonHeaderPrefix  = []byte("S") // skeletonHeaderPrefix + num (uint64 big endian) -> header

	// Path-based storage scheme of merkle patricia trie.
	TrieNodeAccountPrefix = []byte("A") // TrieNodeAccountPrefix + hexPath -> trie node
	TrieNodeStoragePrefix = []byte("O") // TrieNodeStoragePrefix + accountHash + hexPath -> trie node
	stateIDPrefix         = []byte("L") // stateIDPrefix + state root -> state id

	// State history indexing within path-based storage scheme
	StateHistoryIndexPrefix           = []byte("m")   // The global prefix of state history index data
	StateHistoryAccountMetadataPrefix = []byte("ma")  // StateHistoryAccountMetadataPrefix + account address hash => account metadata
	StateHistoryStorageMetadataPrefix = []byte("ms")  // StateHistoryStorageMetadataPrefix + account address hash + storage slot hash => slot metadata
	StateHistoryAccountBlockPrefix    = []byte("mba") // StateHistoryAccountBlockPrefix + account address hash + blockID => account block
	StateHistoryStorageBlockPrefix    = []byte("mbs") // StateHistoryStorageBlockPrefix + account address hash + storage slot hash + blockID => slot block

	// VerklePrefix is the database prefix for Verkle trie data, which includes:
	// (a) Trie nodes
	// (b) In-memory trie node journal
	// (c) Persistent state ID
	// (d) State ID lookups, etc.
	VerklePrefix = []byte("v")

	PreimagePrefix = []byte("secure-key-")       // PreimagePrefix + hash -> preimage
	configPrefix   = []byte("ethereum-config-")  // config prefix for the db
	genesisPrefix  = []byte("ethereum-genesis-") // genesis state prefix for the db

	CliqueSnapshotPrefix = []byte("clique-")

	BestUpdateKey         = []byte("update-")    // bigEndian64(syncPeriod) -> RLP(types.LightClientUpdate)  (nextCommittee only referenced by root hash)
	FixedCommitteeRootKey = []byte("fixedRoot-") // bigEndian64(syncPeriod) -> committee root hash
	SyncCommitteeKey      = []byte("committee-") // bigEndian64(syncPeriod) -> serialized committee

	// new log index
	filterMapsPrefix         = "fm-"
	filterMapsRangeKey       = []byte(filterMapsPrefix + "R")
	filterMapRowPrefix       = []byte(filterMapsPrefix + "r") // filterMapRowPrefix + mapRowIndex (uint64 big endian) -> filter row
	filterMapLastBlockPrefix = []byte(filterMapsPrefix + "b") // filterMapLastBlockPrefix + mapIndex (uint32 big endian) -> block number (uint64 big endian)
	filterMapBlockLVPrefix   = []byte(filterMapsPrefix + "p") // filterMapBlockLVPrefix + num (uint64 big endian) -> log value pointer (uint64 big endian)

	// old log index
	bloomBitsMetaPrefix = []byte("iB")

	preimageCounter     = metrics.NewRegisteredCounter("db/preimage/total", nil)
	preimageHitsCounter = metrics.NewRegisteredCounter("db/preimage/hits", nil)
	preimageMissCounter = metrics.NewRegisteredCounter("db/preimage/miss", nil)
)

// LegacyTxLookupEntry is the legacy TxLookupEntry definition with some unnecessary
// fields.
type LegacyTxLookupEntry struct {
	BlockHash  common.Hash
	BlockIndex uint64
	Index      uint64
}

// encodeBlockNumber encodes a block number as big endian uint64
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

// headerKeyPrefix = headerPrefix + num (uint64 big endian)
func headerKeyPrefix(number uint64) []byte {
	return append(headerPrefix, encodeBlockNumber(number)...)
}

// headerKey = headerPrefix + num (uint64 big endian) + hash
func headerKey(number uint64, hash common.Hash) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

// headerHashKey = headerPrefix + num (uint64 big endian) + headerHashSuffix
func headerHashKey(number uint64) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), headerHashSuffix...)
}

// headerNumberKey = headerNumberPrefix + hash
func headerNumberKey(hash common.Hash) []byte {
	return append(headerNumberPrefix, hash.Bytes()...)
}

// blockBodyKey = blockBodyPrefix + num (uint64 big endian) + hash
func blockBodyKey(number uint64, hash common.Hash) []byte {
	return append(append(blockBodyPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

// blockReceiptsKey = blockReceiptsPrefix + num (uint64 big endian) + hash
func blockReceiptsKey(number uint64, hash common.Hash) []byte {
	return append(append(blockReceiptsPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

// txLookupKey = txLookupPrefix + hash
func txLookupKey(hash common.Hash) []byte {
	return append(txLookupPrefix, hash.Bytes()...)
}

// accountSnapshotKey = SnapshotAccountPrefix + hash
func accountSnapshotKey(hash common.Hash) []byte {
	return append(SnapshotAccountPrefix, hash.Bytes()...)
}

// storageSnapshotKey = SnapshotStoragePrefix + account hash + storage hash
func storageSnapshotKey(accountHash, storageHash common.Hash) []byte {
	buf := make([]byte, len(SnapshotStoragePrefix)+common.HashLength+common.HashLength)
	n := copy(buf, SnapshotStoragePrefix)
	n += copy(buf[n:], accountHash.Bytes())
	copy(buf[n:], storageHash.Bytes())
	return buf
}

// storageSnapshotsKey = SnapshotStoragePrefix + account hash + storage hash
func storageSnapshotsKey(accountHash common.Hash) []byte {
	return append(SnapshotStoragePrefix, accountHash.Bytes()...)
}

// skeletonHeaderKey = skeletonHeaderPrefix + num (uint64 big endian)
func skeletonHeaderKey(number uint64) []byte {
	return append(skeletonHeaderPrefix, encodeBlockNumber(number)...)
}

// preimageKey = PreimagePrefix + hash
func preimageKey(hash common.Hash) []byte {
	return append(PreimagePrefix, hash.Bytes()...)
}

// codeKey = CodePrefix + hash
func codeKey(hash common.Hash) []byte {
	return append(CodePrefix, hash.Bytes()...)
}

// IsCodeKey reports whether the given byte slice is the key of contract code,
// if so return the raw code hash as well.
func IsCodeKey(key []byte) (bool, []byte) {
	if bytes.HasPrefix(key, CodePrefix) && len(key) == common.HashLength+len(CodePrefix) {
		return true, key[len(CodePrefix):]
	}
	return false, nil
}

// configKey = configPrefix + hash
func configKey(hash common.Hash) []byte {
	return append(configPrefix, hash.Bytes()...)
}

// genesisStateSpecKey = genesisPrefix + hash
func genesisStateSpecKey(hash common.Hash) []byte {
	return append(genesisPrefix, hash.Bytes()...)
}

// stateIDKey = stateIDPrefix + root (32 bytes)
func stateIDKey(root common.Hash) []byte {
	return append(stateIDPrefix, root.Bytes()...)
}

// accountTrieNodeKey = TrieNodeAccountPrefix + nodePath.
func accountTrieNodeKey(path []byte) []byte {
	return append(TrieNodeAccountPrefix, path...)
}

// storageTrieNodeKey = TrieNodeStoragePrefix + accountHash + nodePath.
func storageTrieNodeKey(accountHash common.Hash, path []byte) []byte {
	buf := make([]byte, len(TrieNodeStoragePrefix)+common.HashLength+len(path))
	n := copy(buf, TrieNodeStoragePrefix)
	n += copy(buf[n:], accountHash.Bytes())
	copy(buf[n:], path)
	return buf
}

// IsLegacyTrieNode reports whether a provided database entry is a legacy trie
// node. The characteristics of legacy trie node are:
// - the key length is 32 bytes
// - the key is the hash of val
func IsLegacyTrieNode(key []byte, val []byte) bool {
	if len(key) != common.HashLength {
		return false
	}
	return bytes.Equal(key, crypto.Keccak256(val))
}

// ResolveAccountTrieNodeKey reports whether a provided database entry is an
// account trie node in path-based state scheme, and returns the resolved
// node path if so.
func ResolveAccountTrieNodeKey(key []byte) (bool, []byte) {
	if !bytes.HasPrefix(key, TrieNodeAccountPrefix) {
		return false, nil
	}
	// The remaining key should only consist a hex node path
	// whose length is in the range 0 to 64 (64 is excluded
	// since leaves are always wrapped with shortNode).
	if len(key) >= len(TrieNodeAccountPrefix)+common.HashLength*2 {
		return false, nil
	}
	return true, key[len(TrieNodeAccountPrefix):]
}

// IsAccountTrieNode reports whether a provided database entry is an account
// trie node in path-based state scheme.
func IsAccountTrieNode(key []byte) bool {
	ok, _ := ResolveAccountTrieNodeKey(key)
	return ok
}

// ResolveStorageTrieNode reports whether a provided database entry is a storage
// trie node in path-based state scheme, and returns the resolved account hash
// and node path if so.
func ResolveStorageTrieNode(key []byte) (bool, common.Hash, []byte) {
	if !bytes.HasPrefix(key, TrieNodeStoragePrefix) {
		return false, common.Hash{}, nil
	}
	// The remaining key consists of 2 parts:
	// - 32 bytes account hash
	// - hex node path whose length is in the range 0 to 64
	if len(key) < len(TrieNodeStoragePrefix)+common.HashLength {
		return false, common.Hash{}, nil
	}
	if len(key) >= len(TrieNodeStoragePrefix)+common.HashLength+common.HashLength*2 {
		return false, common.Hash{}, nil
	}
	accountHash := common.BytesToHash(key[len(TrieNodeStoragePrefix) : len(TrieNodeStoragePrefix)+common.HashLength])
	return true, accountHash, key[len(TrieNodeStoragePrefix)+common.HashLength:]
}

// IsStorageTrieNode reports whether a provided database entry is a storage
// trie node in path-based state scheme.
func IsStorageTrieNode(key []byte) bool {
	ok, _, _ := ResolveStorageTrieNode(key)
	return ok
}

// filterMapRowKey = filterMapRowPrefix + mapRowIndex (uint64 big endian)
func filterMapRowKey(mapRowIndex uint64, base bool) []byte {
	extLen := 8
	if base {
		extLen = 9
	}
	l := len(filterMapRowPrefix)
	key := make([]byte, l+extLen)
	copy(key[:l], filterMapRowPrefix)
	binary.BigEndian.PutUint64(key[l:l+8], mapRowIndex)
	return key
}

// filterMapLastBlockKey = filterMapLastBlockPrefix + mapIndex (uint32 big endian)
func filterMapLastBlockKey(mapIndex uint32) []byte {
	l := len(filterMapLastBlockPrefix)
	key := make([]byte, l+4)
	copy(key[:l], filterMapLastBlockPrefix)
	binary.BigEndian.PutUint32(key[l:], mapIndex)
	return key
}

// filterMapBlockLVKey = filterMapBlockLVPrefix + num (uint64 big endian)
func filterMapBlockLVKey(number uint64) []byte {
	l := len(filterMapBlockLVPrefix)
	key := make([]byte, l+8)
	copy(key[:l], filterMapBlockLVPrefix)
	binary.BigEndian.PutUint64(key[l:], number)
	return key
}

// accountHistoryIndexKey = StateHistoryAccountMetadataPrefix + addressHash
func accountHistoryIndexKey(addressHash common.Hash) []byte {
	return append(StateHistoryAccountMetadataPrefix, addressHash.Bytes()...)
}

// storageHistoryIndexKey = StateHistoryStorageMetadataPrefix + addressHash + storageHash
func storageHistoryIndexKey(addressHash common.Hash, storageHash common.Hash) []byte {
	return append(append(StateHistoryStorageMetadataPrefix, addressHash.Bytes()...), storageHash.Bytes()...)
}

// accountHistoryIndexBlockKey = StateHistoryAccountBlockPrefix + addressHash + blockID
func accountHistoryIndexBlockKey(addressHash common.Hash, blockID uint32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], blockID)
	return append(append(StateHistoryAccountBlockPrefix, addressHash.Bytes()...), buf[:]...)
}

// storageHistoryIndexBlockKey = StateHistoryStorageBlockPrefix + addressHash + storageHash + blockID
func storageHistoryIndexBlockKey(addressHash common.Hash, storageHash common.Hash, blockID uint32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], blockID)
	return append(append(append(StateHistoryStorageBlockPrefix, addressHash.Bytes()...), storageHash.Bytes()...), buf[:]...)
}
