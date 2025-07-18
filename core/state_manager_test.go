// (c) 2019-2020, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package core

import (
	"math/big"
	"testing"

	"github.com/luxfi/geth/core/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

type MockTrieDB struct {
	LastDereference common.Hash
	LastCommit      common.Hash
}

func (t *MockTrieDB) Dereference(root common.Hash) error {
	t.LastDereference = root
	return nil
}
func (t *MockTrieDB) Commit(root common.Hash, report bool) error {
	t.LastCommit = root
	return nil
}
func (t *MockTrieDB) Size() (common.StorageSize, common.StorageSize, common.StorageSize) {
	return 0, 0, 0
}
func (t *MockTrieDB) Cap(limit common.StorageSize) error {
	return nil
}

func TestCappedMemoryTrieWriter(t *testing.T) {
	m := &MockTrieDB{}
	cacheConfig := &CacheConfig{Pruning: true, CommitInterval: 4096}
	w := NewTrieWriter(m, cacheConfig)
	assert := assert.New(t)
	for i := 0; i < int(cacheConfig.CommitInterval)+1; i++ {
		bigI := big.NewInt(int64(i))
		block := types.NewBlock(
			&types.Header{
				Root:   common.BigToHash(bigI),
				Number: bigI,
			},
			nil, nil, nil, nil,
		)

		assert.NoError(w.InsertTrie(block))
		assert.Equal(common.Hash{}, m.LastDereference, "should not have dereferenced block on insert")
		assert.Equal(common.Hash{}, m.LastCommit, "should not have committed block on insert")

		w.AcceptTrie(block)
		if i <= TipBufferSize {
			assert.Equal(common.Hash{}, m.LastDereference, "should not have dereferenced block on accept")
		} else {
			assert.Equal(common.BigToHash(big.NewInt(int64(i-TipBufferSize))), m.LastDereference, "should have dereferenced old block on last accept")
			m.LastDereference = common.Hash{}
		}
		if i < int(cacheConfig.CommitInterval) {
			assert.Equal(common.Hash{}, m.LastCommit, "should not have committed block on accept")
		} else {
			assert.Equal(block.Root(), m.LastCommit, "should have committed block after CommitInterval")
			m.LastCommit = common.Hash{}
		}

		w.RejectTrie(block)
		assert.Equal(block.Root(), m.LastDereference, "should have dereferenced block on reject")
		assert.Equal(common.Hash{}, m.LastCommit, "should not have committed block on reject")
		m.LastDereference = common.Hash{}
	}
}

func TestNoPruningTrieWriter(t *testing.T) {
	m := &MockTrieDB{}
	w := NewTrieWriter(m, &CacheConfig{})
	assert := assert.New(t)
	for i := 0; i < TipBufferSize+1; i++ {
		bigI := big.NewInt(int64(i))
		block := types.NewBlock(
			&types.Header{
				Root:   common.BigToHash(bigI),
				Number: bigI,
			},
			nil, nil, nil, nil,
		)

		assert.NoError(w.InsertTrie(block))
		assert.Equal(common.Hash{}, m.LastDereference, "should not have dereferenced block on insert")
		assert.Equal(common.Hash{}, m.LastCommit, "should not have committed block on insert")

		w.AcceptTrie(block)
		assert.Equal(common.Hash{}, m.LastDereference, "should not have dereferenced block on accept")
		assert.Equal(block.Root(), m.LastCommit, "should have committed block on accept")
		m.LastCommit = common.Hash{}

		w.RejectTrie(block)
		assert.Equal(block.Root(), m.LastDereference, "should have dereferenced block on reject")
		assert.Equal(common.Hash{}, m.LastCommit, "should not have committed block on reject")
		m.LastDereference = common.Hash{}
	}
}
