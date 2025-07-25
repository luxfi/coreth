// Copyright 2022 The go-ethereum Authors
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

package triedb

import (
	"errors"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/log"
	"github.com/luxfi/geth/trie/trienode"
	"github.com/luxfi/geth/triedb/database"
	"github.com/luxfi/geth/triedb/hashdb"
	"github.com/luxfi/geth/triedb/pathdb"
)

// Config defines all necessary options for database.
type Config struct {
	Preimages bool           // Flag whether the preimage of node key is recorded
	IsVerkle  bool           // Flag whether the db is holding a verkle tree
	HashDB    *hashdb.Config // Configs for hash-based scheme
	PathDB    *pathdb.Config // Configs for experimental path-based scheme
}

// HashDefaults represents a config for using hash-based scheme with
// default settings.
var HashDefaults = &Config{
	Preimages: false,
	IsVerkle:  false,
	HashDB:    hashdb.Defaults,
}

// VerkleDefaults represents a config for holding verkle trie data
// using path-based scheme with default settings.
var VerkleDefaults = &Config{
	Preimages: false,
	IsVerkle:  true,
	PathDB:    pathdb.Defaults,
}

// backend defines the methods needed to access/update trie nodes in different
// state scheme.
type backend interface {
	// NodeReader returns a reader for accessing trie nodes within the specified state.
	// An error will be returned if the specified state is not available.
	NodeReader(root common.Hash) (database.NodeReader, error)

	// StateReader returns a reader for accessing flat states within the specified
	// state. An error will be returned if the specified state is not available.
	StateReader(root common.Hash) (database.StateReader, error)

	// Size returns the current storage size of the diff layers on top of the
	// disk layer and the storage size of the nodes cached in the disk layer.
	//
	// For hash scheme, there is no differentiation between diff layer nodes
	// and dirty disk layer nodes, so both are merged into the second return.
	Size() (common.StorageSize, common.StorageSize)

	// Commit writes all relevant trie nodes belonging to the specified state
	// to disk. Report specifies whether logs will be displayed in info level.
	Commit(root common.Hash, report bool) error

	// Close closes the trie database backend and releases all held resources.
	Close() error
}

// Database is the wrapper of the underlying backend which is shared by different
// types of node backend as an entrypoint. It's responsible for all interactions
// relevant with trie nodes and node preimages.
type Database struct {
	disk      ethdb.Database
	config    *Config        // Configuration for trie database
	preimages *preimageStore // The store for caching preimages
	backend   backend        // The backend for managing trie nodes
}

// NewDatabase initializes the trie database with default settings, note
// the legacy hash-based scheme is used by default.
func NewDatabase(diskdb ethdb.Database, config *Config) *Database {
	// Sanitize the config and use the default one if it's not specified.
	if config == nil {
		config = HashDefaults
	}
	var preimages *preimageStore
	if config.Preimages {
		preimages = newPreimageStore(diskdb)
	}
	db := &Database{
		disk:      diskdb,
		config:    config,
		preimages: preimages,
	}
	if config.HashDB != nil && config.PathDB != nil {
		log.Crit("Both 'hash' and 'path' mode are configured")
	}
	if config.PathDB != nil {
		db.backend = pathdb.New(diskdb, config.PathDB, config.IsVerkle)
	} else {
		db.backend = hashdb.New(diskdb, config.HashDB)
	}
	return db
}

// NodeReader returns a reader for accessing trie nodes within the specified state.
// An error will be returned if the specified state is not available.
func (db *Database) NodeReader(blockRoot common.Hash) (database.NodeReader, error) {
	return db.backend.NodeReader(blockRoot)
}

// StateReader returns a reader that allows access to the state data associated
// with the specified state. An error will be returned if the specified state is
// not available.
func (db *Database) StateReader(blockRoot common.Hash) (database.StateReader, error) {
	return db.backend.StateReader(blockRoot)
}

// HistoricReader constructs a reader for accessing the requested historic state.
func (db *Database) HistoricReader(root common.Hash) (*pathdb.HistoricalStateReader, error) {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return nil, errors.New("not supported")
	}
	return pdb.HistoricReader(root)
}

// Update performs a state transition by committing dirty nodes contained in the
// given set in order to update state from the specified parent to the specified
// root. The held pre-images accumulated up to this point will be flushed in case
// the size exceeds the threshold.
//
// The passed in maps(nodes, states) will be retained to avoid copying everything.
// Therefore, these maps must not be changed afterwards.
func (db *Database) Update(root common.Hash, parent common.Hash, block uint64, nodes *trienode.MergedNodeSet, states *StateSet) error {
	if db.preimages != nil {
		db.preimages.commit(false)
	}
	switch b := db.backend.(type) {
	case *hashdb.Database:
		return b.Update(root, parent, block, nodes)
	case *pathdb.Database:
		return b.Update(root, parent, block, nodes, states.internal())
	}
	return errors.New("unknown backend")
}

// Commit iterates over all the children of a particular node, writes them out
// to disk. As a side effect, all pre-images accumulated up to this point are
// also written.
func (db *Database) Commit(root common.Hash, report bool) error {
	if db.preimages != nil {
		db.preimages.commit(true)
	}
	return db.backend.Commit(root, report)
}

// Size returns the storage size of diff layer nodes above the persistent disk
// layer, the dirty nodes buffered within the disk layer, and the size of cached
// preimages.
func (db *Database) Size() (common.StorageSize, common.StorageSize, common.StorageSize) {
	var (
		diffs, nodes common.StorageSize
		preimages    common.StorageSize
	)
	diffs, nodes = db.backend.Size()
	if db.preimages != nil {
		preimages = db.preimages.size()
	}
	return diffs, nodes, preimages
}

// Scheme returns the node scheme used in the database.
func (db *Database) Scheme() string {
	if db.config.PathDB != nil {
		return rawdb.PathScheme
	}
	return rawdb.HashScheme
}

// Close flushes the dangling preimages to disk and closes the trie database.
// It is meant to be called when closing the blockchain object, so that all
// resources held can be released correctly.
func (db *Database) Close() error {
	db.WritePreimages()
	return db.backend.Close()
}

// WritePreimages flushes all accumulated preimages to disk forcibly.
func (db *Database) WritePreimages() {
	if db.preimages != nil {
		db.preimages.commit(true)
	}
}

// Preimage retrieves a cached trie node pre-image from preimage store.
func (db *Database) Preimage(hash common.Hash) []byte {
	if db.preimages == nil {
		return nil
	}
	return db.preimages.preimage(hash)
}

// InsertPreimage writes pre-images of trie node to the preimage store.
func (db *Database) InsertPreimage(preimages map[common.Hash][]byte) {
	if db.preimages == nil {
		return
	}
	db.preimages.insertPreimage(preimages)
}

// PreimageEnabled returns the indicator if the pre-image store is enabled.
func (db *Database) PreimageEnabled() bool {
	return db.preimages != nil
}

// Cap iteratively flushes old but still referenced trie nodes until the total
// memory usage goes below the given threshold. The held pre-images accumulated
// up to this point will be flushed in case the size exceeds the threshold.
//
// It's only supported by hash-based database and will return an error for others.
func (db *Database) Cap(limit common.StorageSize) error {
	hdb, ok := db.backend.(*hashdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	if db.preimages != nil {
		db.preimages.commit(false)
	}
	return hdb.Cap(limit)
}

// Reference adds a new reference from a parent node to a child node. This function
// is used to add reference between internal trie node and external node(e.g. storage
// trie root), all internal trie nodes are referenced together by database itself.
//
// It's only supported by hash-based database and will return an error for others.
func (db *Database) Reference(root common.Hash, parent common.Hash) error {
	hdb, ok := db.backend.(*hashdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	hdb.Reference(root, parent)
	return nil
}

// Dereference removes an existing reference from a root node. It's only
// supported by hash-based database and will return an error for others.
func (db *Database) Dereference(root common.Hash) error {
	hdb, ok := db.backend.(*hashdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	hdb.Dereference(root)
	return nil
}

// Recover rollbacks the database to a specified historical point. The state is
// supported as the rollback destination only if it's canonical state and the
// corresponding trie histories are existent. It's only supported by path-based
// database and will return an error for others.
func (db *Database) Recover(target common.Hash) error {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	return pdb.Recover(target)
}

// Recoverable returns the indicator if the specified state is enabled to be
// recovered. It's only supported by path-based database and will return an
// error for others.
func (db *Database) Recoverable(root common.Hash) (bool, error) {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return false, errors.New("not supported")
	}
	return pdb.Recoverable(root), nil
}

// Disable deactivates the database and invalidates all available state layers
// as stale to prevent access to the persistent state, which is in the syncing
// stage.
//
// It's only supported by path-based database and will return an error for others.
func (db *Database) Disable() error {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	return pdb.Disable()
}

// Enable activates database and resets the state tree with the provided persistent
// state root once the state sync is finished.
func (db *Database) Enable(root common.Hash) error {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	return pdb.Enable(root)
}

// Journal commits an entire diff hierarchy to disk into a single journal entry.
// This is meant to be used during shutdown to persist the snapshot without
// flattening everything down (bad for reorgs). It's only supported by path-based
// database and will return an error for others.
func (db *Database) Journal(root common.Hash) error {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	return pdb.Journal(root)
}

// VerifyState traverses the flat states specified by the given state root and
// ensures they are matched with each other.
func (db *Database) VerifyState(root common.Hash) error {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	return pdb.VerifyState(root)
}

// AccountIterator creates a new account iterator for the specified root hash and
// seeks to a starting account hash.
func (db *Database) AccountIterator(root common.Hash, seek common.Hash) (pathdb.AccountIterator, error) {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return nil, errors.New("not supported")
	}
	return pdb.AccountIterator(root, seek)
}

// StorageIterator creates a new storage iterator for the specified root hash and
// account. The iterator will be move to the specific start position.
func (db *Database) StorageIterator(root common.Hash, account common.Hash, seek common.Hash) (pathdb.StorageIterator, error) {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return nil, errors.New("not supported")
	}
	return pdb.StorageIterator(root, account, seek)
}

// IndexProgress returns the indexing progress made so far. It provides the
// number of states that remain unindexed.
func (db *Database) IndexProgress() (uint64, error) {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return 0, errors.New("not supported")
	}
	return pdb.IndexProgress()
}

// IsVerkle returns the indicator if the database is holding a verkle tree.
func (db *Database) IsVerkle() bool {
	return db.config.IsVerkle
}

// Disk returns the underlying disk database.
func (db *Database) Disk() ethdb.Database {
	return db.disk
}
