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

package stateless

import (
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/crypto"
	"github.com/luxfi/geth/ethdb"
)

// MakeHashDB imports tries, codes and block hashes from a witness into a new
// hash-based memory db. We could eventually rewrite this into a pathdb, but
// simple is better for now.
//
// Note, this hashdb approach is quite strictly self-validating:
//   - Headers are persisted keyed by hash, so blockhash will error on junk
//   - Codes are persisted keyed by hash, so bytecode lookup will error on junk
//   - Trie nodes are persisted keyed by hash, so trie expansion will error on junk
//
// Acceleration structures built would need to explicitly validate the witness.
func (w *Witness) MakeHashDB() ethdb.Database {
	var (
		memdb  = rawdb.NewMemoryDatabase()
		hasher = crypto.NewKeccakState()
		hash   = make([]byte, 32)
	)
	// Inject all the "block hashes" (i.e. headers) into the ephemeral database
	for _, header := range w.Headers {
		rawdb.WriteHeader(memdb, header)
	}
	// Inject all the bytecodes into the ephemeral database
	for code := range w.Codes {
		blob := []byte(code)

		hasher.Reset()
		hasher.Write(blob)
		hasher.Read(hash)

		rawdb.WriteCode(memdb, common.BytesToHash(hash), blob)
	}
	// Inject all the MPT trie nodes into the ephemeral database
	for node := range w.State {
		blob := []byte(node)

		hasher.Reset()
		hasher.Write(blob)
		hasher.Read(hash)

		rawdb.WriteLegacyTrieNode(memdb, common.BytesToHash(hash), blob)
	}
	return memdb
}
