// Copyright 2021 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package ethtest

import (
	crand "crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/common/hexutil"
	"github.com/luxfi/geth/eth"
	"github.com/luxfi/geth/eth/catalyst"
	"github.com/luxfi/geth/eth/ethconfig"
	"github.com/luxfi/geth/internal/utesting"
	"github.com/luxfi/geth/node"
	"github.com/luxfi/geth/p2p"
)

func makeJWTSecret(t *testing.T) (string, [32]byte, error) {
	var secret [32]byte
	if _, err := crand.Read(secret[:]); err != nil {
		return "", secret, fmt.Errorf("failed to create jwt secret: %v", err)
	}
	jwtPath := filepath.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(jwtPath, []byte(hexutil.Encode(secret[:])), 0600); err != nil {
		return "", secret, fmt.Errorf("failed to prepare jwt secret file: %v", err)
	}
	return jwtPath, secret, nil
}

func TestEthSuite(t *testing.T) {
	jwtPath, secret, err := makeJWTSecret(t)
	if err != nil {
		t.Fatalf("could not make jwt secret: %v", err)
	}
	geth, err := runGeth("./testdata", jwtPath)
	if err != nil {
		t.Fatalf("could not run geth: %v", err)
	}
	defer geth.Close()

	suite, err := NewSuite(geth.Server().Self(), "./testdata", geth.HTTPAuthEndpoint(), common.Bytes2Hex(secret[:]))
	if err != nil {
		t.Fatalf("could not create new test suite: %v", err)
	}
	for _, test := range suite.EthTests() {
		t.Run(test.Name, func(t *testing.T) {
			if test.Slow && testing.Short() {
				t.Skipf("%s: skipping in -short mode", test.Name)
			}
			result := utesting.RunTests([]utesting.Test{{Name: test.Name, Fn: test.Fn}}, os.Stdout)
			if result[0].Failed {
				t.Fatal()
			}
		})
	}
}

func TestSnapSuite(t *testing.T) {
	jwtPath, secret, err := makeJWTSecret(t)
	if err != nil {
		t.Fatalf("could not make jwt secret: %v", err)
	}
	geth, err := runGeth("./testdata", jwtPath)
	if err != nil {
		t.Fatalf("could not run geth: %v", err)
	}
	defer geth.Close()

	suite, err := NewSuite(geth.Server().Self(), "./testdata", geth.HTTPAuthEndpoint(), common.Bytes2Hex(secret[:]))
	if err != nil {
		t.Fatalf("could not create new test suite: %v", err)
	}
	for _, test := range suite.SnapTests() {
		t.Run(test.Name, func(t *testing.T) {
			result := utesting.RunTests([]utesting.Test{{Name: test.Name, Fn: test.Fn}}, os.Stdout)
			if result[0].Failed {
				t.Fatal()
			}
		})
	}
}

// runGeth creates and starts a geth node
func runGeth(dir string, jwtPath string) (*node.Node, error) {
	stack, err := node.New(&node.Config{
		AuthAddr: "127.0.0.1",
		AuthPort: 0,
		P2P: p2p.Config{
			ListenAddr:  "127.0.0.1:0",
			NoDiscovery: true,
			MaxPeers:    10, // in case a test requires multiple connections, can be changed in the future
			NoDial:      true,
		},
		JWTSecret: jwtPath,
	})
	if err != nil {
		return nil, err
	}

	err = setupGeth(stack, dir)
	if err != nil {
		stack.Close()
		return nil, err
	}
	if err = stack.Start(); err != nil {
		stack.Close()
		return nil, err
	}
	return stack, nil
}

func setupGeth(stack *node.Node, dir string) error {
	chain, err := NewChain(dir)
	if err != nil {
		return err
	}
	backend, err := eth.New(stack, &ethconfig.Config{
		Genesis:        &chain.genesis,
		NetworkId:      chain.genesis.Config.ChainID.Uint64(), // 19763
		DatabaseCache:  10,
		TrieCleanCache: 10,
		TrieDirtyCache: 16,
		TrieTimeout:    60 * time.Minute,
		SnapshotCache:  10,
	})
	if err != nil {
		return err
	}
	if err := catalyst.Register(stack, backend); err != nil {
		return fmt.Errorf("failed to register catalyst service: %v", err)
	}
	_, err = backend.BlockChain().InsertChain(chain.blocks[1:])
	return err
}
