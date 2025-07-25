// Copyright 2023 The go-ethereum Authors
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

package state

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/state/snapshot"
	"github.com/luxfi/geth/core/tracing"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/crypto"
	"github.com/luxfi/geth/rlp"
	"github.com/luxfi/geth/trie"
	"github.com/luxfi/geth/triedb"
	"github.com/luxfi/geth/triedb/pathdb"
	"github.com/holiman/uint256"
)

// A stateTest checks that the state changes are correctly captured. Instances
// of this test with pseudorandom content are created by Generate.
//
// The test works as follows:
//
// A list of states are created by applying actions. The state changes between
// each state instance are tracked and be verified.
type stateTest struct {
	addrs   []common.Address // all account addresses
	actions [][]testAction   // modifications to the state, grouped by block
	chunk   int              // The number of actions per chunk
	err     error            // failure details are reported through this field
}

// newStateTestAction creates a random action that changes state.
func newStateTestAction(addr common.Address, r *rand.Rand, index int) testAction {
	actions := []testAction{
		{
			name: "SetBalance",
			fn: func(a testAction, s *StateDB) {
				s.SetBalance(addr, uint256.NewInt(uint64(a.args[0])), tracing.BalanceChangeUnspecified)
			},
			args: make([]int64, 1),
		},
		{
			name: "SetNonce",
			fn: func(a testAction, s *StateDB) {
				s.SetNonce(addr, uint64(a.args[0]), tracing.NonceChangeUnspecified)
			},
			args: make([]int64, 1),
		},
		{
			name: "SetStorage",
			fn: func(a testAction, s *StateDB) {
				var key, val common.Hash
				binary.BigEndian.PutUint16(key[:], uint16(a.args[0]))
				binary.BigEndian.PutUint16(val[:], uint16(a.args[1]))
				s.SetState(addr, key, val)
			},
			args: make([]int64, 2),
		},
		{
			name: "SetCode",
			fn: func(a testAction, s *StateDB) {
				code := make([]byte, 16)
				binary.BigEndian.PutUint64(code, uint64(a.args[0]))
				binary.BigEndian.PutUint64(code[8:], uint64(a.args[1]))
				s.SetCode(addr, code)
			},
			args: make([]int64, 2),
		},
		{
			name: "CreateAccount",
			fn: func(a testAction, s *StateDB) {
				if !s.Exist(addr) {
					s.CreateAccount(addr)
				}
			},
		},
		{
			name: "Selfdestruct",
			fn: func(a testAction, s *StateDB) {
				s.SelfDestruct(addr)
			},
		},
	}
	var nonRandom = index != -1
	if index == -1 {
		index = r.Intn(len(actions))
	}
	action := actions[index]
	var names []string
	if !action.noAddr {
		names = append(names, addr.Hex())
	}
	for i := range action.args {
		if nonRandom {
			action.args[i] = rand.Int63n(10000) + 1 // set balance to non-zero
		} else {
			action.args[i] = rand.Int63n(10000)
		}
		names = append(names, fmt.Sprint(action.args[i]))
	}
	action.name += " " + strings.Join(names, ", ")
	return action
}

// Generate returns a new snapshot test of the given size. All randomness is
// derived from r.
func (*stateTest) Generate(r *rand.Rand, size int) reflect.Value {
	addrs := make([]common.Address, 5)
	for i := range addrs {
		addrs[i][0] = byte(i)
	}
	actions := make([][]testAction, rand.Intn(5)+1)

	for i := 0; i < len(actions); i++ {
		actions[i] = make([]testAction, size)
		for j := range actions[i] {
			if j == 0 {
				// Always include a set balance action to make sure
				// the state changes are not empty.
				actions[i][j] = newStateTestAction(common.HexToAddress("0xdeadbeef"), r, 0)
				continue
			}
			actions[i][j] = newStateTestAction(addrs[r.Intn(len(addrs))], r, -1)
		}
	}
	chunk := int(math.Sqrt(float64(size)))
	if size > 0 && chunk == 0 {
		chunk = 1
	}
	return reflect.ValueOf(&stateTest{
		addrs:   addrs,
		actions: actions,
		chunk:   chunk,
	})
}

func (test *stateTest) String() string {
	out := new(bytes.Buffer)
	for i, actions := range test.actions {
		fmt.Fprintf(out, "---- block %d ----\n", i)
		for j, action := range actions {
			if j%test.chunk == 0 {
				fmt.Fprintf(out, "---- transaction %d ----\n", j/test.chunk)
			}
			fmt.Fprintf(out, "%4d: %s\n", j%test.chunk, action.name)
		}
	}
	return out.String()
}

func (test *stateTest) run() bool {
	var (
		roots         []common.Hash
		accounts      []map[common.Hash][]byte
		accountOrigin []map[common.Address][]byte
		storages      []map[common.Hash]map[common.Hash][]byte
		storageOrigin []map[common.Address]map[common.Hash][]byte
		copyUpdate    = func(update *stateUpdate) {
			accounts = append(accounts, maps.Clone(update.accounts))
			accountOrigin = append(accountOrigin, maps.Clone(update.accountsOrigin))
			storages = append(storages, maps.Clone(update.storages))
			storageOrigin = append(storageOrigin, maps.Clone(update.storagesOrigin))
		}
		disk      = rawdb.NewMemoryDatabase()
		tdb       = triedb.NewDatabase(disk, &triedb.Config{PathDB: pathdb.Defaults})
		byzantium = rand.Intn(2) == 0
	)
	defer disk.Close()
	defer tdb.Close()

	var snaps *snapshot.Tree
	if rand.Intn(3) == 0 {
		snaps, _ = snapshot.New(snapshot.Config{
			CacheSize:  1,
			Recovery:   false,
			NoBuild:    false,
			AsyncBuild: false,
		}, disk, tdb, types.EmptyRootHash)
	}
	for i, actions := range test.actions {
		root := types.EmptyRootHash
		if i != 0 {
			root = roots[len(roots)-1]
		}
		state, err := New(root, NewDatabase(tdb, snaps))
		if err != nil {
			panic(err)
		}
		for i, action := range actions {
			if i%test.chunk == 0 && i != 0 {
				if byzantium {
					state.Finalise(true) // call finalise at the transaction boundary
				} else {
					state.IntermediateRoot(true) // call intermediateRoot at the transaction boundary
				}
			}
			action.fn(action, state)
		}
		if byzantium {
			state.Finalise(true) // call finalise at the transaction boundary
		} else {
			state.IntermediateRoot(true) // call intermediateRoot at the transaction boundary
		}
		ret, err := state.commitAndFlush(0, true, false) // call commit at the block boundary
		if err != nil {
			panic(err)
		}
		if ret.empty() {
			return true
		}
		copyUpdate(ret)
		roots = append(roots, ret.root)
	}
	for i := 0; i < len(test.actions); i++ {
		root := types.EmptyRootHash
		if i != 0 {
			root = roots[i-1]
		}
		test.err = test.verify(root, roots[i], tdb, accounts[i], accountOrigin[i], storages[i], storageOrigin[i])
		if test.err != nil {
			return false
		}
	}
	return true
}

// verifyAccountCreation this function is called once the state diff says that
// specific account was not present. A serial of checks will be performed to
// ensure the state diff is correct, includes:
//
// - the account was indeed not present in trie
// - the account is present in new trie, nil->nil is regarded as invalid
// - the slots transition is correct
func (test *stateTest) verifyAccountCreation(next common.Hash, db *triedb.Database, otr, ntr *trie.Trie, addr common.Address, account []byte, storages map[common.Hash][]byte, storagesOrigin map[common.Hash][]byte) error {
	// Verify account change
	addrHash := crypto.Keccak256Hash(addr.Bytes())
	oBlob, err := otr.Get(addrHash.Bytes())
	if err != nil {
		return err
	}
	nBlob, err := ntr.Get(addrHash.Bytes())
	if err != nil {
		return err
	}
	if len(oBlob) != 0 {
		return fmt.Errorf("unexpected account in old trie, %x", addrHash)
	}
	if len(nBlob) == 0 {
		return fmt.Errorf("missing account in new trie, %x", addrHash)
	}
	full, err := types.FullAccountRLP(account)
	if err != nil {
		return err
	}
	if !bytes.Equal(nBlob, full) {
		return fmt.Errorf("unexpected account data, want: %v, got: %v", full, nBlob)
	}

	// Verify storage changes
	var nAcct types.StateAccount
	if err := rlp.DecodeBytes(nBlob, &nAcct); err != nil {
		return err
	}
	// Account has no slot, empty slot set is expected
	if nAcct.Root == types.EmptyRootHash {
		if len(storagesOrigin) != 0 {
			return fmt.Errorf("unexpected slot changes %x", addrHash)
		}
		if len(storages) != 0 {
			return fmt.Errorf("unexpected slot changes %x", addrHash)
		}
		return nil
	}
	// Account has slots, ensure all new slots are contained
	st, err := trie.New(trie.StorageTrieID(next, addrHash, nAcct.Root), db)
	if err != nil {
		return err
	}
	for key, val := range storagesOrigin {
		if _, exist := storages[key]; !exist {
			return errors.New("storage data is not found")
		}
		got, err := st.Get(key.Bytes())
		if err != nil {
			return err
		}
		if !bytes.Equal(got, storages[key]) {
			return fmt.Errorf("unexpected storage data, want: %v, got: %v", storages[key], got)
		}
		st.Update(key.Bytes(), val)
	}
	if len(storagesOrigin) != len(storages) {
		return fmt.Errorf("extra storage found, want: %d, got: %d", len(storagesOrigin), len(storages))
	}
	if st.Hash() != types.EmptyRootHash {
		return errors.New("invalid slot changes")
	}
	return nil
}

// verifyAccountUpdate this function is called once the state diff says that
// specific account was present. A serial of checks will be performed to
// ensure the state diff is correct, includes:
//
// - the account was indeed present in trie
// - the account in old trie matches the provided value
// - the slots transition is correct
func (test *stateTest) verifyAccountUpdate(next common.Hash, db *triedb.Database, otr, ntr *trie.Trie, addr common.Address, account []byte, accountOrigin []byte, storages map[common.Hash][]byte, storageOrigin map[common.Hash][]byte) error {
	// Verify account change
	addrHash := crypto.Keccak256Hash(addr.Bytes())
	oBlob, err := otr.Get(addrHash.Bytes())
	if err != nil {
		return err
	}
	nBlob, err := ntr.Get(addrHash.Bytes())
	if err != nil {
		return err
	}
	if len(oBlob) == 0 {
		return fmt.Errorf("missing account in old trie, %x", addrHash)
	}
	full, err := types.FullAccountRLP(accountOrigin)
	if err != nil {
		return err
	}
	if !bytes.Equal(full, oBlob) {
		return fmt.Errorf("account value is not matched, %x", addrHash)
	}
	if len(nBlob) == 0 {
		if len(account) != 0 {
			return errors.New("unexpected account data")
		}
	} else {
		full, _ = types.FullAccountRLP(account)
		if !bytes.Equal(full, nBlob) {
			return fmt.Errorf("unexpected account data, %x, want %v, got: %v", addrHash, full, nBlob)
		}
	}
	// Decode accounts
	var (
		oAcct types.StateAccount
		nAcct types.StateAccount
		nRoot common.Hash
	)
	if err := rlp.DecodeBytes(oBlob, &oAcct); err != nil {
		return err
	}
	if len(nBlob) == 0 {
		nRoot = types.EmptyRootHash
	} else {
		if err := rlp.DecodeBytes(nBlob, &nAcct); err != nil {
			return err
		}
		nRoot = nAcct.Root
	}

	// Verify storage
	st, err := trie.New(trie.StorageTrieID(next, addrHash, nRoot), db)
	if err != nil {
		return err
	}
	for key, val := range storageOrigin {
		if _, exist := storages[key]; !exist {
			return errors.New("storage data is not found")
		}
		got, err := st.Get(key.Bytes())
		if err != nil {
			return err
		}
		if !bytes.Equal(got, storages[key]) {
			return fmt.Errorf("unexpected storage data, want: %v, got: %v", storages[key], got)
		}
		st.Update(key.Bytes(), val)
	}
	if len(storageOrigin) != len(storages) {
		return fmt.Errorf("extra storage found, want: %d, got: %d", len(storageOrigin), len(storages))
	}
	if st.Hash() != oAcct.Root {
		return errors.New("invalid slot changes")
	}
	return nil
}

func (test *stateTest) verify(root common.Hash, next common.Hash, db *triedb.Database, accounts map[common.Hash][]byte, accountsOrigin map[common.Address][]byte, storages map[common.Hash]map[common.Hash][]byte, storagesOrigin map[common.Address]map[common.Hash][]byte) error {
	otr, err := trie.New(trie.StateTrieID(root), db)
	if err != nil {
		return err
	}
	ntr, err := trie.New(trie.StateTrieID(next), db)
	if err != nil {
		return err
	}
	for addr, accountOrigin := range accountsOrigin {
		var (
			err      error
			addrHash = crypto.Keccak256Hash(addr.Bytes())
		)
		if len(accountOrigin) == 0 {
			err = test.verifyAccountCreation(next, db, otr, ntr, addr, accounts[addrHash], storages[addrHash], storagesOrigin[addr])
		} else {
			err = test.verifyAccountUpdate(next, db, otr, ntr, addr, accounts[addrHash], accountsOrigin[addr], storages[addrHash], storagesOrigin[addr])
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func TestStateChanges(t *testing.T) {
	config := &quick.Config{MaxCount: 1000}
	err := quick.Check((*stateTest).run, config)
	if cerr, ok := err.(*quick.CheckError); ok {
		test := cerr.In[0].(*stateTest)
		t.Errorf("%v:\n%s", test.err, test)
	} else if err != nil {
		t.Error(err)
	}
}
