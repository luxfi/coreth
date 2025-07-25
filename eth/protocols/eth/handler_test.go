// Copyright 2020 The go-ethereum Authors
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

package eth

import (
	"bytes"
	"crypto/sha256"
	"math"
	"math/big"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/consensus/beacon"
	"github.com/luxfi/geth/consensus/ethash"
	"github.com/luxfi/geth/core"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/txpool"
	"github.com/luxfi/geth/core/txpool/blobpool"
	"github.com/luxfi/geth/core/txpool/legacypool"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/crypto"
	"github.com/luxfi/geth/crypto/kzg4844"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/p2p"
	"github.com/luxfi/geth/p2p/enode"
	"github.com/luxfi/geth/params"
	"github.com/luxfi/geth/rlp"
	"github.com/holiman/uint256"
)

var (
	// testKey is a private key to use for funding a tester account.
	testKey, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")

	// testAddr is the Ethereum address of the tester account.
	testAddr = crypto.PubkeyToAddress(testKey.PublicKey)
)

func u64(val uint64) *uint64 { return &val }

// testBackend is a mock implementation of the live Ethereum message handler. Its
// purpose is to allow testing the request/reply workflows and wire serialization
// in the `eth` protocol without actually doing any data processing.
type testBackend struct {
	db     ethdb.Database
	chain  *core.BlockChain
	txpool *txpool.TxPool
}

// newTestBackend creates an empty chain and wraps it into a mock backend.
func newTestBackend(blocks int) *testBackend {
	return newTestBackendWithGenerator(blocks, false, false, nil)
}

// newTestBackendWithGenerator creates a chain with a number of explicitly defined blocks and
// wraps it into a mock backend.
func newTestBackendWithGenerator(blocks int, shanghai bool, cancun bool, generator func(int, *core.BlockGen)) *testBackend {
	var (
		// Create a database pre-initialize with a genesis block
		db     = rawdb.NewMemoryDatabase()
		config = params.TestChainConfig
		engine = beacon.New(ethash.NewFaker())
	)
	if shanghai {
		config = &params.ChainConfig{
			ChainID:                 big.NewInt(1),
			HomesteadBlock:          big.NewInt(0),
			DAOForkBlock:            nil,
			DAOForkSupport:          true,
			EIP150Block:             big.NewInt(0),
			EIP155Block:             big.NewInt(0),
			EIP158Block:             big.NewInt(0),
			ByzantiumBlock:          big.NewInt(0),
			ConstantinopleBlock:     big.NewInt(0),
			PetersburgBlock:         big.NewInt(0),
			IstanbulBlock:           big.NewInt(0),
			MuirGlacierBlock:        big.NewInt(0),
			BerlinBlock:             big.NewInt(0),
			LondonBlock:             big.NewInt(0),
			ArrowGlacierBlock:       big.NewInt(0),
			GrayGlacierBlock:        big.NewInt(0),
			MergeNetsplitBlock:      big.NewInt(0),
			ShanghaiTime:            u64(0),
			TerminalTotalDifficulty: big.NewInt(0),
			Ethash:                  new(params.EthashConfig),
		}
	}

	if cancun {
		config.CancunTime = u64(0)
		config.BlobScheduleConfig = &params.BlobScheduleConfig{
			Cancun: &params.BlobConfig{
				Target:         3,
				Max:            6,
				UpdateFraction: params.DefaultCancunBlobConfig.UpdateFraction,
			},
		}
	}

	gspec := &core.Genesis{
		Config:     config,
		Alloc:      types.GenesisAlloc{testAddr: {Balance: big.NewInt(100_000_000_000_000_000)}},
		Difficulty: common.Big0,
	}
	chain, _ := core.NewBlockChain(db, gspec, engine, nil)

	_, bs, _ := core.GenerateChainWithGenesis(gspec, engine, blocks, generator)
	if _, err := chain.InsertChain(bs); err != nil {
		panic(err)
	}
	for _, block := range bs {
		chain.TrieDB().Commit(block.Root(), false)
	}
	txconfig := legacypool.DefaultConfig
	txconfig.Journal = "" // Don't litter the disk with test journals

	storage, _ := os.MkdirTemp("", "blobpool-")
	defer os.RemoveAll(storage)

	blobPool := blobpool.New(blobpool.Config{Datadir: storage}, chain, nil)
	legacyPool := legacypool.New(txconfig, chain)
	txpool, _ := txpool.New(txconfig.PriceLimit, chain, []txpool.SubPool{legacyPool, blobPool})

	return &testBackend{
		db:     db,
		chain:  chain,
		txpool: txpool,
	}
}

// close tears down the transaction pool and chain behind the mock backend.
func (b *testBackend) close() {
	b.txpool.Close()
	b.chain.Stop()
}

func (b *testBackend) Chain() *core.BlockChain { return b.chain }
func (b *testBackend) TxPool() TxPool          { return b.txpool }

func (b *testBackend) RunPeer(peer *Peer, handler Handler) error {
	// Normally the backend would do peer maintenance and handshakes. All that
	// is omitted and we will just give control back to the handler.
	return handler(peer)
}
func (b *testBackend) PeerInfo(enode.ID) interface{} { panic("not implemented") }

func (b *testBackend) AcceptTxs() bool {
	return true
	//panic("data processing tests should be done in the handler package")
}
func (b *testBackend) Handle(*Peer, Packet) error {
	return nil
	//panic("data processing tests should be done in the handler package")
}

// Tests that block headers can be retrieved from a remote chain based on user queries.
func TestGetBlockHeaders68(t *testing.T) { testGetBlockHeaders(t, ETH68) }

func testGetBlockHeaders(t *testing.T, protocol uint) {
	t.Parallel()

	backend := newTestBackend(maxHeadersServe + 15)
	defer backend.close()

	peer, _ := newTestPeer("peer", protocol, backend)
	defer peer.close()

	// Create a "random" unknown hash for testing
	var unknown common.Hash
	for i := range unknown {
		unknown[i] = byte(i)
	}
	getHashes := func(from, limit uint64) (hashes []common.Hash) {
		for i := uint64(0); i < limit; i++ {
			hashes = append(hashes, backend.chain.GetCanonicalHash(from-1-i))
		}
		return hashes
	}
	// Create a batch of tests for various scenarios
	limit := uint64(maxHeadersServe)
	tests := []struct {
		query  *GetBlockHeadersRequest // The query to execute for header retrieval
		expect []common.Hash           // The hashes of the block whose headers are expected
	}{
		// A single random block should be retrievable by hash
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Hash: backend.chain.GetBlockByNumber(limit / 2).Hash()}, Amount: 1},
			[]common.Hash{backend.chain.GetBlockByNumber(limit / 2).Hash()},
		},
		// A single random block should be retrievable by number
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: limit / 2}, Amount: 1},
			[]common.Hash{backend.chain.GetBlockByNumber(limit / 2).Hash()},
		},
		// Multiple headers should be retrievable in both directions
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: limit / 2}, Amount: 3},
			[]common.Hash{
				backend.chain.GetBlockByNumber(limit / 2).Hash(),
				backend.chain.GetBlockByNumber(limit/2 + 1).Hash(),
				backend.chain.GetBlockByNumber(limit/2 + 2).Hash(),
			},
		}, {
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: limit / 2}, Amount: 3, Reverse: true},
			[]common.Hash{
				backend.chain.GetBlockByNumber(limit / 2).Hash(),
				backend.chain.GetBlockByNumber(limit/2 - 1).Hash(),
				backend.chain.GetBlockByNumber(limit/2 - 2).Hash(),
			},
		},
		// Multiple headers with skip lists should be retrievable
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: limit / 2}, Skip: 3, Amount: 3},
			[]common.Hash{
				backend.chain.GetBlockByNumber(limit / 2).Hash(),
				backend.chain.GetBlockByNumber(limit/2 + 4).Hash(),
				backend.chain.GetBlockByNumber(limit/2 + 8).Hash(),
			},
		}, {
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: limit / 2}, Skip: 3, Amount: 3, Reverse: true},
			[]common.Hash{
				backend.chain.GetBlockByNumber(limit / 2).Hash(),
				backend.chain.GetBlockByNumber(limit/2 - 4).Hash(),
				backend.chain.GetBlockByNumber(limit/2 - 8).Hash(),
			},
		},
		// The chain endpoints should be retrievable
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: 0}, Amount: 1},
			[]common.Hash{backend.chain.GetBlockByNumber(0).Hash()},
		},
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: backend.chain.CurrentBlock().Number.Uint64()}, Amount: 1},
			[]common.Hash{backend.chain.CurrentBlock().Hash()},
		},
		{ // If the peer requests a bit into the future, we deliver what we have
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: backend.chain.CurrentBlock().Number.Uint64()}, Amount: 10},
			[]common.Hash{backend.chain.CurrentBlock().Hash()},
		},
		// Ensure protocol limits are honored
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: backend.chain.CurrentBlock().Number.Uint64() - 1}, Amount: limit + 10, Reverse: true},
			getHashes(backend.chain.CurrentBlock().Number.Uint64(), limit),
		},
		// Check that requesting more than available is handled gracefully
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: backend.chain.CurrentBlock().Number.Uint64() - 4}, Skip: 3, Amount: 3},
			[]common.Hash{
				backend.chain.GetBlockByNumber(backend.chain.CurrentBlock().Number.Uint64() - 4).Hash(),
				backend.chain.GetBlockByNumber(backend.chain.CurrentBlock().Number.Uint64()).Hash(),
			},
		}, {
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: 4}, Skip: 3, Amount: 3, Reverse: true},
			[]common.Hash{
				backend.chain.GetBlockByNumber(4).Hash(),
				backend.chain.GetBlockByNumber(0).Hash(),
			},
		},
		// Check that requesting more than available is handled gracefully, even if mid skip
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: backend.chain.CurrentBlock().Number.Uint64() - 4}, Skip: 2, Amount: 3},
			[]common.Hash{
				backend.chain.GetBlockByNumber(backend.chain.CurrentBlock().Number.Uint64() - 4).Hash(),
				backend.chain.GetBlockByNumber(backend.chain.CurrentBlock().Number.Uint64() - 1).Hash(),
			},
		}, {
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: 4}, Skip: 2, Amount: 3, Reverse: true},
			[]common.Hash{
				backend.chain.GetBlockByNumber(4).Hash(),
				backend.chain.GetBlockByNumber(1).Hash(),
			},
		},
		// Check a corner case where requesting more can iterate past the endpoints
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: 2}, Amount: 5, Reverse: true},
			[]common.Hash{
				backend.chain.GetBlockByNumber(2).Hash(),
				backend.chain.GetBlockByNumber(1).Hash(),
				backend.chain.GetBlockByNumber(0).Hash(),
			},
		},
		// Check a corner case where skipping causes overflow with reverse=false
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: 1}, Amount: 2, Reverse: false, Skip: math.MaxUint64 - 1},
			[]common.Hash{
				backend.chain.GetBlockByNumber(1).Hash(),
			},
		},
		// Check a corner case where skipping causes overflow with reverse=true
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: 1}, Amount: 2, Reverse: true, Skip: math.MaxUint64 - 1},
			[]common.Hash{
				backend.chain.GetBlockByNumber(1).Hash(),
			},
		},
		// Check another corner case where skipping causes overflow with reverse=false
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: 1}, Amount: 2, Reverse: false, Skip: math.MaxUint64},
			[]common.Hash{
				backend.chain.GetBlockByNumber(1).Hash(),
			},
		},
		// Check another corner case where skipping causes overflow with reverse=true
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: 1}, Amount: 2, Reverse: true, Skip: math.MaxUint64},
			[]common.Hash{
				backend.chain.GetBlockByNumber(1).Hash(),
			},
		},
		// Check a corner case where skipping overflow loops back into the chain start
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Hash: backend.chain.GetBlockByNumber(3).Hash()}, Amount: 2, Reverse: false, Skip: math.MaxUint64 - 1},
			[]common.Hash{
				backend.chain.GetBlockByNumber(3).Hash(),
			},
		},
		// Check a corner case where skipping overflow loops back to the same header
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Hash: backend.chain.GetBlockByNumber(1).Hash()}, Amount: 2, Reverse: false, Skip: math.MaxUint64},
			[]common.Hash{
				backend.chain.GetBlockByNumber(1).Hash(),
			},
		},
		// Check that non existing headers aren't returned
		{
			&GetBlockHeadersRequest{Origin: HashOrNumber{Hash: unknown}, Amount: 1},
			[]common.Hash{},
		}, {
			&GetBlockHeadersRequest{Origin: HashOrNumber{Number: backend.chain.CurrentBlock().Number.Uint64() + 1}, Amount: 1},
			[]common.Hash{},
		},
	}
	// Run each of the tests and verify the results against the chain
	for i, tt := range tests {
		// Collect the headers to expect in the response
		var headers []*types.Header
		for _, hash := range tt.expect {
			headers = append(headers, backend.chain.GetBlockByHash(hash).Header())
		}
		// Send the hash request and verify the response
		p2p.Send(peer.app, GetBlockHeadersMsg, &GetBlockHeadersPacket{
			RequestId:              123,
			GetBlockHeadersRequest: tt.query,
		})
		if err := p2p.ExpectMsg(peer.app, BlockHeadersMsg, &BlockHeadersPacket{
			RequestId:           123,
			BlockHeadersRequest: headers,
		}); err != nil {
			t.Errorf("test %d: headers mismatch: %v", i, err)
		}
		// If the test used number origins, repeat with hashes as the too
		if tt.query.Origin.Hash == (common.Hash{}) {
			if origin := backend.chain.GetBlockByNumber(tt.query.Origin.Number); origin != nil {
				tt.query.Origin.Hash, tt.query.Origin.Number = origin.Hash(), 0

				p2p.Send(peer.app, GetBlockHeadersMsg, &GetBlockHeadersPacket{
					RequestId:              456,
					GetBlockHeadersRequest: tt.query,
				})
				expected := &BlockHeadersPacket{RequestId: 456, BlockHeadersRequest: headers}
				if err := p2p.ExpectMsg(peer.app, BlockHeadersMsg, expected); err != nil {
					t.Errorf("test %d by hash: headers mismatch: %v", i, err)
				}
			}
		}
	}
}

// Tests that block contents can be retrieved from a remote chain based on their hashes.
func TestGetBlockBodies68(t *testing.T) { testGetBlockBodies(t, ETH68) }

func testGetBlockBodies(t *testing.T, protocol uint) {
	t.Parallel()

	gen := func(n int, g *core.BlockGen) {
		if n%2 == 0 {
			w := &types.Withdrawal{
				Address: common.Address{0xaa},
				Amount:  42,
			}
			g.AddWithdrawal(w)
		}
	}

	backend := newTestBackendWithGenerator(maxBodiesServe+15, true, false, gen)
	defer backend.close()

	peer, _ := newTestPeer("peer", protocol, backend)
	defer peer.close()

	// Create a batch of tests for various scenarios
	limit := maxBodiesServe
	tests := []struct {
		random    int           // Number of blocks to fetch randomly from the chain
		explicit  []common.Hash // Explicitly requested blocks
		available []bool        // Availability of explicitly requested blocks
		expected  int           // Total number of existing blocks to expect
	}{
		{1, nil, nil, 1},             // A single random block should be retrievable
		{10, nil, nil, 10},           // Multiple random blocks should be retrievable
		{limit, nil, nil, limit},     // The maximum possible blocks should be retrievable
		{limit + 1, nil, nil, limit}, // No more than the possible block count should be returned
		{0, []common.Hash{backend.chain.Genesis().Hash()}, []bool{true}, 1},      // The genesis block should be retrievable
		{0, []common.Hash{backend.chain.CurrentBlock().Hash()}, []bool{true}, 1}, // The chains head block should be retrievable
		{0, []common.Hash{{}}, []bool{false}, 0},                                 // A non existent block should not be returned

		// Existing and non-existing blocks interleaved should not cause problems
		{0, []common.Hash{
			{},
			backend.chain.GetBlockByNumber(1).Hash(),
			{},
			backend.chain.GetBlockByNumber(10).Hash(),
			{},
			backend.chain.GetBlockByNumber(100).Hash(),
			{},
		}, []bool{false, true, false, true, false, true, false}, 3},
	}
	// Run each of the tests and verify the results against the chain
	for i, tt := range tests {
		// Collect the hashes to request, and the response to expect
		var (
			hashes []common.Hash
			bodies []*BlockBody
			seen   = make(map[int64]bool)
		)
		for j := 0; j < tt.random; j++ {
			for {
				num := rand.Int63n(int64(backend.chain.CurrentBlock().Number.Uint64()))
				if !seen[num] {
					seen[num] = true

					block := backend.chain.GetBlockByNumber(uint64(num))
					hashes = append(hashes, block.Hash())
					if len(bodies) < tt.expected {
						bodies = append(bodies, &BlockBody{Transactions: block.Transactions(), Uncles: block.Uncles(), Withdrawals: block.Withdrawals()})
					}
					break
				}
			}
		}
		for j, hash := range tt.explicit {
			hashes = append(hashes, hash)
			if tt.available[j] && len(bodies) < tt.expected {
				block := backend.chain.GetBlockByHash(hash)
				bodies = append(bodies, &BlockBody{Transactions: block.Transactions(), Uncles: block.Uncles(), Withdrawals: block.Withdrawals()})
			}
		}

		// Send the hash request and verify the response
		p2p.Send(peer.app, GetBlockBodiesMsg, &GetBlockBodiesPacket{
			RequestId:             123,
			GetBlockBodiesRequest: hashes,
		})
		if err := p2p.ExpectMsg(peer.app, BlockBodiesMsg, &BlockBodiesPacket{
			RequestId:           123,
			BlockBodiesResponse: bodies,
		}); err != nil {
			t.Fatalf("test %d: bodies mismatch: %v", i, err)
		}
	}
}

// Tests that the transaction receipts can be retrieved based on hashes.
func TestGetBlockReceipts68(t *testing.T) { testGetBlockReceipts(t, ETH68) }

func testGetBlockReceipts(t *testing.T, protocol uint) {
	t.Parallel()

	// Define three accounts to simulate transactions with
	acc1Key, _ := crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	acc2Key, _ := crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
	acc1Addr := crypto.PubkeyToAddress(acc1Key.PublicKey)
	acc2Addr := crypto.PubkeyToAddress(acc2Key.PublicKey)

	signer := types.HomesteadSigner{}
	// Create a chain generator with some simple transactions (blatantly stolen from @fjl/chain_markets_test)
	generator := func(i int, block *core.BlockGen) {
		switch i {
		case 0:
			// In block 1, the test bank sends account #1 some ether.
			tx, _ := types.SignTx(types.NewTransaction(block.TxNonce(testAddr), acc1Addr, big.NewInt(10_000_000_000_000_000), params.TxGas, block.BaseFee(), nil), signer, testKey)
			block.AddTx(tx)
		case 1:
			// In block 2, the test bank sends some more ether to account #1.
			// acc1Addr passes it on to account #2.
			tx1, _ := types.SignTx(types.NewTransaction(block.TxNonce(testAddr), acc1Addr, big.NewInt(1_000_000_000_000_000), params.TxGas, block.BaseFee(), nil), signer, testKey)
			tx2, _ := types.SignTx(types.NewTransaction(block.TxNonce(acc1Addr), acc2Addr, big.NewInt(1_000_000_000_000_000), params.TxGas, block.BaseFee(), nil), signer, acc1Key)
			block.AddTx(tx1)
			block.AddTx(tx2)
		case 2:
			// Block 3 is empty but was mined by account #2.
			block.SetCoinbase(acc2Addr)
			block.SetExtra([]byte("yeehaw"))
		case 3:
			// Block 4 includes blocks 2 and 3 as uncle headers (with modified extra data).
			b2 := block.PrevBlock(1).Header()
			b2.Extra = []byte("foo")
			block.AddUncle(b2)
			b3 := block.PrevBlock(2).Header()
			b3.Extra = []byte("foo")
			block.AddUncle(b3)
		}
	}
	// Assemble the test environment
	backend := newTestBackendWithGenerator(4, false, false, generator)
	defer backend.close()

	peer, _ := newTestPeer("peer", protocol, backend)
	defer peer.close()

	// Collect the hashes to request, and the response to expect
	var (
		hashes   []common.Hash
		receipts []*ReceiptList68
	)
	for i := uint64(0); i <= backend.chain.CurrentBlock().Number.Uint64(); i++ {
		block := backend.chain.GetBlockByNumber(i)
		hashes = append(hashes, block.Hash())
		trs := backend.chain.GetReceiptsByHash(block.Hash())
		receipts = append(receipts, NewReceiptList68(trs))
	}

	// Send the hash request and verify the response
	p2p.Send(peer.app, GetReceiptsMsg, &GetReceiptsPacket{
		RequestId:          123,
		GetReceiptsRequest: hashes,
	})
	if err := p2p.ExpectMsg(peer.app, ReceiptsMsg, &ReceiptsPacket[*ReceiptList68]{
		RequestId: 123,
		List:      receipts,
	}); err != nil {
		t.Errorf("receipts mismatch: %v", err)
	}
}

type decoder struct {
	msg []byte
}

func (d decoder) Decode(val interface{}) error {
	buffer := bytes.NewBuffer(d.msg)
	s := rlp.NewStream(buffer, uint64(len(d.msg)))
	return s.Decode(val)
}

func (d decoder) Time() time.Time {
	return time.Now()
}

func setup() (*testBackend, *testPeer) {
	// Generate some transactions etc.
	acc1Key, _ := crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	acc2Key, _ := crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
	acc1Addr := crypto.PubkeyToAddress(acc1Key.PublicKey)
	acc2Addr := crypto.PubkeyToAddress(acc2Key.PublicKey)
	signer := types.HomesteadSigner{}
	gen := func(n int, block *core.BlockGen) {
		if n%2 == 0 {
			w := &types.Withdrawal{
				Address: common.Address{0xaa},
				Amount:  42,
			}
			block.AddWithdrawal(w)
		}
		switch n {
		case 0:
			// In block 1, the test bank sends account #1 some ether.
			tx, _ := types.SignTx(types.NewTransaction(block.TxNonce(testAddr), acc1Addr, big.NewInt(10_000_000_000_000_000), params.TxGas, block.BaseFee(), nil), signer, testKey)
			block.AddTx(tx)
		case 1:
			// In block 2, the test bank sends some more ether to account #1.
			// acc1Addr passes it on to account #2.
			tx1, _ := types.SignTx(types.NewTransaction(block.TxNonce(testAddr), acc1Addr, big.NewInt(1_000_000_000_000_000), params.TxGas, block.BaseFee(), nil), signer, testKey)
			tx2, _ := types.SignTx(types.NewTransaction(block.TxNonce(acc1Addr), acc2Addr, big.NewInt(1_000_000_000_000_000), params.TxGas, block.BaseFee(), nil), signer, acc1Key)
			block.AddTx(tx1)
			block.AddTx(tx2)
		case 2:
			// Block 3 is empty but was mined by account #2.
			block.SetCoinbase(acc2Addr)
			block.SetExtra([]byte("yeehaw"))
		}
	}
	backend := newTestBackendWithGenerator(maxBodiesServe+15, true, false, gen)
	peer, _ := newTestPeer("peer", ETH68, backend)
	// Discard all messages
	go func() {
		for {
			msg, err := peer.app.ReadMsg()
			if err == nil {
				msg.Discard()
			}
		}
	}()
	return backend, peer
}

func FuzzEthProtocolHandlers(f *testing.F) {
	handlers := eth69
	backend, peer := setup()
	f.Fuzz(func(t *testing.T, code byte, msg []byte) {
		handler := handlers[uint64(code)%protocolLengths[ETH69]]
		if handler == nil {
			return
		}
		handler(backend, decoder{msg: msg}, peer.Peer)
	})
}

func TestGetPooledTransaction(t *testing.T) {
	t.Run("blobTx", func(t *testing.T) {
		testGetPooledTransaction(t, true)
	})
	t.Run("legacyTx", func(t *testing.T) {
		testGetPooledTransaction(t, false)
	})
}

func testGetPooledTransaction(t *testing.T, blobTx bool) {
	var (
		emptyBlob          = kzg4844.Blob{}
		emptyBlobs         = []kzg4844.Blob{emptyBlob}
		emptyBlobCommit, _ = kzg4844.BlobToCommitment(&emptyBlob)
		emptyBlobProof, _  = kzg4844.ComputeBlobProof(&emptyBlob, emptyBlobCommit)
		emptyBlobHash      = kzg4844.CalcBlobHashV1(sha256.New(), &emptyBlobCommit)
	)
	backend := newTestBackendWithGenerator(0, true, true, nil)
	defer backend.close()

	peer, _ := newTestPeer("peer", ETH68, backend)
	defer peer.close()

	var (
		tx     *types.Transaction
		err    error
		signer = types.NewCancunSigner(params.TestChainConfig.ChainID)
	)
	if blobTx {
		tx, err = types.SignNewTx(testKey, signer, &types.BlobTx{
			ChainID:    uint256.MustFromBig(params.TestChainConfig.ChainID),
			Nonce:      0,
			GasTipCap:  uint256.NewInt(20_000_000_000),
			GasFeeCap:  uint256.NewInt(21_000_000_000),
			Gas:        21000,
			To:         testAddr,
			BlobHashes: []common.Hash{emptyBlobHash},
			BlobFeeCap: uint256.MustFromBig(common.Big1),
			Sidecar:    types.NewBlobTxSidecar(types.BlobSidecarVersion0, emptyBlobs, []kzg4844.Commitment{emptyBlobCommit}, []kzg4844.Proof{emptyBlobProof}),
		})
		if err != nil {
			t.Fatal(err)
		}
	} else {
		tx, err = types.SignTx(
			types.NewTransaction(0, testAddr, big.NewInt(10_000), params.TxGas, big.NewInt(1_000_000_000), nil),
			signer,
			testKey,
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	errs := backend.txpool.Add([]*types.Transaction{tx}, true)
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	// Send the hash request and verify the response
	p2p.Send(peer.app, GetPooledTransactionsMsg, GetPooledTransactionsPacket{
		RequestId:                    123,
		GetPooledTransactionsRequest: []common.Hash{tx.Hash()},
	})
	if err := p2p.ExpectMsg(peer.app, PooledTransactionsMsg, PooledTransactionsPacket{
		RequestId:                  123,
		PooledTransactionsResponse: []*types.Transaction{tx},
	}); err != nil {
		t.Errorf("pooled transaction mismatch: %v", err)
	}
}
