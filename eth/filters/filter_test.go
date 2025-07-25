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

package filters

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/luxfi/geth/accounts/abi"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/consensus/ethash"
	"github.com/luxfi/geth/core"
	"github.com/luxfi/geth/core/filtermaps"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/crypto"
	"github.com/luxfi/geth/params"
	"github.com/luxfi/geth/rpc"
	"github.com/luxfi/geth/triedb"
)

func makeReceipt(addr common.Address) *types.Receipt {
	receipt := types.NewReceipt(nil, false, 0)
	receipt.Logs = []*types.Log{
		{Address: addr},
	}
	receipt.Bloom = types.CreateBloom(receipt)
	return receipt
}

func BenchmarkFiltersIndexed(b *testing.B) {
	benchmarkFilters(b, 0, false)
}

func BenchmarkFiltersHalfIndexed(b *testing.B) {
	benchmarkFilters(b, 50000, false)
}

func BenchmarkFiltersUnindexed(b *testing.B) {
	benchmarkFilters(b, 0, true)
}

func benchmarkFilters(b *testing.B, history uint64, noHistory bool) {
	var (
		db           = rawdb.NewMemoryDatabase()
		backend, sys = newTestFilterSystem(db, Config{})
		key1, _      = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr1        = crypto.PubkeyToAddress(key1.PublicKey)
		addr2        = common.BytesToAddress([]byte("jeff"))
		addr3        = common.BytesToAddress([]byte("ethereum"))
		addr4        = common.BytesToAddress([]byte("random addresses please"))

		gspec = &core.Genesis{
			Alloc:   types.GenesisAlloc{addr1: {Balance: big.NewInt(1000000)}},
			BaseFee: big.NewInt(params.InitialBaseFee),
			Config:  params.TestChainConfig,
		}
	)
	defer db.Close()
	_, chain, receipts := core.GenerateChainWithGenesis(gspec, ethash.NewFaker(), 100010, func(i int, gen *core.BlockGen) {
		switch i {
		case 2403:
			receipt := makeReceipt(addr1)
			gen.AddUncheckedReceipt(receipt)
			gen.AddUncheckedTx(types.NewTransaction(999, common.HexToAddress("0x999"), big.NewInt(999), 999, gen.BaseFee(), nil))
		case 1034:
			receipt := makeReceipt(addr2)
			gen.AddUncheckedReceipt(receipt)
			gen.AddUncheckedTx(types.NewTransaction(999, common.HexToAddress("0x999"), big.NewInt(999), 999, gen.BaseFee(), nil))
		case 34:
			receipt := makeReceipt(addr3)
			gen.AddUncheckedReceipt(receipt)
			gen.AddUncheckedTx(types.NewTransaction(999, common.HexToAddress("0x999"), big.NewInt(999), 999, gen.BaseFee(), nil))
		case 99999:
			receipt := makeReceipt(addr4)
			gen.AddUncheckedReceipt(receipt)
			gen.AddUncheckedTx(types.NewTransaction(999, common.HexToAddress("0x999"), big.NewInt(999), 999, gen.BaseFee(), nil))
		}
	})
	// The test txs are not properly signed, can't simply create a chain
	// and then import blocks. TODO(rjl493456442) try to get rid of the
	// manual database writes.
	gspec.MustCommit(db, triedb.NewDatabase(db, triedb.HashDefaults))

	for i, block := range chain {
		rawdb.WriteBlock(db, block)
		rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64())
		rawdb.WriteHeadBlockHash(db, block.Hash())
		rawdb.WriteReceipts(db, block.Hash(), block.NumberU64(), receipts[i])
	}
	backend.startFilterMaps(history, noHistory, filtermaps.DefaultParams)
	defer backend.stopFilterMaps()

	b.ResetTimer()

	filter := sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), []common.Address{addr1, addr2, addr3, addr4}, nil)

	for i := 0; i < b.N; i++ {
		filter.begin = 0
		logs, _ := filter.Logs(context.Background())
		if len(logs) != 4 {
			b.Fatal("expected 4 logs, got", len(logs))
		}
	}
}

func TestFiltersIndexed(t *testing.T) {
	testFilters(t, 0, false)
}

func TestFiltersHalfIndexed(t *testing.T) {
	testFilters(t, 500, false)
}

func TestFiltersUnindexed(t *testing.T) {
	testFilters(t, 0, true)
}

func testFilters(t *testing.T, history uint64, noHistory bool) {
	var (
		db           = rawdb.NewMemoryDatabase()
		backend, sys = newTestFilterSystem(db, Config{})
		// Sender account
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr    = crypto.PubkeyToAddress(key1.PublicKey)
		signer  = types.NewLondonSigner(big.NewInt(1))
		// Logging contract
		contract  = common.Address{0xfe}
		contract2 = common.Address{0xff}
		abiStr    = `[{"inputs":[],"name":"log0","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"}],"name":"log1","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"},{"internalType":"uint256","name":"t2","type":"uint256"}],"name":"log2","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"},{"internalType":"uint256","name":"t2","type":"uint256"},{"internalType":"uint256","name":"t3","type":"uint256"}],"name":"log3","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"},{"internalType":"uint256","name":"t2","type":"uint256"},{"internalType":"uint256","name":"t3","type":"uint256"},{"internalType":"uint256","name":"t4","type":"uint256"}],"name":"log4","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
		/*
			// SPDX-License-Identifier: GPL-3.0
			pragma solidity >=0.7.0 <0.9.0;

			contract Logger {
				function log0() external {
					assembly {
						log0(0, 0)
					}
				}

				function log1(uint t1) external {
					assembly {
						log1(0, 0, t1)
					}
				}

				function log2(uint t1, uint t2) external {
					assembly {
						log2(0, 0, t1, t2)
					}
				}

				function log3(uint t1, uint t2, uint t3) external {
					assembly {
						log3(0, 0, t1, t2, t3)
					}
				}

				function log4(uint t1, uint t2, uint t3, uint t4) external {
					assembly {
						log4(0, 0, t1, t2, t3, t4)
					}
				}
			}
		*/
		bytecode = common.FromHex("608060405234801561001057600080fd5b50600436106100575760003560e01c80630aa731851461005c5780632a4c08961461006657806378b9a1f314610082578063c670f8641461009e578063c683d6a3146100ba575b600080fd5b6100646100d6565b005b610080600480360381019061007b9190610143565b6100dc565b005b61009c60048036038101906100979190610196565b6100e8565b005b6100b860048036038101906100b391906101d6565b6100f2565b005b6100d460048036038101906100cf9190610203565b6100fa565b005b600080a0565b808284600080a3505050565b8082600080a25050565b80600080a150565b80828486600080a450505050565b600080fd5b6000819050919050565b6101208161010d565b811461012b57600080fd5b50565b60008135905061013d81610117565b92915050565b60008060006060848603121561015c5761015b610108565b5b600061016a8682870161012e565b935050602061017b8682870161012e565b925050604061018c8682870161012e565b9150509250925092565b600080604083850312156101ad576101ac610108565b5b60006101bb8582860161012e565b92505060206101cc8582860161012e565b9150509250929050565b6000602082840312156101ec576101eb610108565b5b60006101fa8482850161012e565b91505092915050565b6000806000806080858703121561021d5761021c610108565b5b600061022b8782880161012e565b945050602061023c8782880161012e565b935050604061024d8782880161012e565b925050606061025e8782880161012e565b9150509295919450925056fea264697066735822122073a4b156f487e59970dc1ef449cc0d51467268f676033a17188edafcee861f9864736f6c63430008110033")

		hash1 = common.BytesToHash([]byte("topic1"))
		hash2 = common.BytesToHash([]byte("topic2"))
		hash3 = common.BytesToHash([]byte("topic3"))
		hash4 = common.BytesToHash([]byte("topic4"))
		hash5 = common.BytesToHash([]byte("topic5"))

		gspec = &core.Genesis{
			Config: params.TestChainConfig,
			Alloc: types.GenesisAlloc{
				addr:      {Balance: big.NewInt(0).Mul(big.NewInt(100), big.NewInt(params.Ether))},
				contract:  {Balance: big.NewInt(0), Code: bytecode},
				contract2: {Balance: big.NewInt(0), Code: bytecode},
			},
			BaseFee: big.NewInt(params.InitialBaseFee),
		}
	)

	contractABI, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		t.Fatal(err)
	}

	// Hack: GenerateChainWithGenesis creates a new db.
	// Commit the genesis manually and use GenerateChain.
	_, err = gspec.Commit(db, triedb.NewDatabase(db, nil))
	if err != nil {
		t.Fatal(err)
	}
	chain, _ := core.GenerateChain(gspec.Config, gspec.ToBlock(), ethash.NewFaker(), db, 1000, func(i int, gen *core.BlockGen) {
		switch i {
		case 1:
			data, err := contractABI.Pack("log1", hash1.Big())
			if err != nil {
				t.Fatal(err)
			}
			tx, _ := types.SignTx(types.NewTx(&types.LegacyTx{
				Nonce:    0,
				GasPrice: gen.BaseFee(),
				Gas:      30000,
				To:       &contract,
				Data:     data,
			}), signer, key1)
			gen.AddTx(tx)
			tx2, _ := types.SignTx(types.NewTx(&types.LegacyTx{
				Nonce:    1,
				GasPrice: gen.BaseFee(),
				Gas:      30000,
				To:       &contract2,
				Data:     data,
			}), signer, key1)
			gen.AddTx(tx2)
		case 2:
			data, err := contractABI.Pack("log2", hash2.Big(), hash1.Big())
			if err != nil {
				t.Fatal(err)
			}
			tx, _ := types.SignTx(types.NewTx(&types.LegacyTx{
				Nonce:    2,
				GasPrice: gen.BaseFee(),
				Gas:      30000,
				To:       &contract,
				Data:     data,
			}), signer, key1)
			gen.AddTx(tx)
		case 998:
			data, err := contractABI.Pack("log1", hash3.Big())
			if err != nil {
				t.Fatal(err)
			}
			tx, _ := types.SignTx(types.NewTx(&types.LegacyTx{
				Nonce:    3,
				GasPrice: gen.BaseFee(),
				Gas:      30000,
				To:       &contract2,
				Data:     data,
			}), signer, key1)
			gen.AddTx(tx)
		case 999:
			data, err := contractABI.Pack("log1", hash4.Big())
			if err != nil {
				t.Fatal(err)
			}
			tx, _ := types.SignTx(types.NewTx(&types.LegacyTx{
				Nonce:    4,
				GasPrice: gen.BaseFee(),
				Gas:      30000,
				To:       &contract,
				Data:     data,
			}), signer, key1)
			gen.AddTx(tx)
		}
	})
	options := core.DefaultConfig().WithStateScheme(rawdb.HashScheme)
	options.TxLookupLimit = 0 // index all txs
	bc, err := core.NewBlockChain(db, gspec, ethash.NewFaker(), options)
	if err != nil {
		t.Fatal(err)
	}
	_, err = bc.InsertChain(chain)
	if err != nil {
		t.Fatal(err)
	}

	// Set block 998 as Finalized (-3)
	bc.SetFinalized(chain[998].Header())

	// Generate pending block
	pchain, preceipts := core.GenerateChain(gspec.Config, chain[len(chain)-1], ethash.NewFaker(), db, 1, func(i int, gen *core.BlockGen) {
		data, err := contractABI.Pack("log1", hash5.Big())
		if err != nil {
			t.Fatal(err)
		}
		tx, _ := types.SignTx(types.NewTx(&types.LegacyTx{
			Nonce:    5,
			GasPrice: gen.BaseFee(),
			Gas:      30000,
			To:       &contract,
			Data:     data,
		}), signer, key1)
		gen.AddTx(tx)
	})
	backend.setPending(pchain[0], preceipts[0])

	backend.startFilterMaps(history, noHistory, filtermaps.DefaultParams)
	defer backend.stopFilterMaps()

	for i, tc := range []struct {
		f    *Filter
		want string
		err  string
	}{
		{
			f:    sys.NewBlockFilter(chain[2].Hash(), []common.Address{contract}, nil),
			want: `[{"address":"0xfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696332","0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x3","transactionHash":"0xdefe471992a07a02acdfbe33edaae22fbb86d7d3cec3f1b8e4e77702fb3acc1d","transactionIndex":"0x0","blockHash":"0x7a7556792ca7d37882882e2b001fe14833eaf81c2c7f865c9c771ec37a024f6b","blockTimestamp":"0x1e","logIndex":"0x0","removed":false}]`,
		},
		{
			f:    sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), []common.Address{contract}, [][]common.Hash{{hash1, hash2, hash3, hash4}}),
			want: `[{"address":"0xfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x2","transactionHash":"0xa8028c655b6423204c8edfbc339f57b042d6bec2b6a61145d76b7c08b4cccd42","transactionIndex":"0x0","blockHash":"0x24417bb49ce44cfad65da68f33b510bf2a129c0d89ccf06acb6958b8585ccf34","blockTimestamp":"0x14","logIndex":"0x0","removed":false},{"address":"0xfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696332","0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x3","transactionHash":"0xdefe471992a07a02acdfbe33edaae22fbb86d7d3cec3f1b8e4e77702fb3acc1d","transactionIndex":"0x0","blockHash":"0x7a7556792ca7d37882882e2b001fe14833eaf81c2c7f865c9c771ec37a024f6b","blockTimestamp":"0x1e","logIndex":"0x0","removed":false},{"address":"0xfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696334"],"data":"0x","blockNumber":"0x3e8","transactionHash":"0x9a87842100a638dfa5da8842b4beda691d2fd77b0c84b57f24ecfa9fb208f747","transactionIndex":"0x0","blockHash":"0xb360bad5265261c075ece02d3bf0e39498a6a76310482cdfd90588748e6c5ee0","blockTimestamp":"0x2710","logIndex":"0x0","removed":false}]`,
		},
		{
			f: sys.NewRangeFilter(900, 999, []common.Address{contract}, [][]common.Hash{{hash3}}),
		},
		{
			f:    sys.NewRangeFilter(990, int64(rpc.LatestBlockNumber), []common.Address{contract2}, [][]common.Hash{{hash3}}),
			want: `[{"address":"0xff00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696333"],"data":"0x","blockNumber":"0x3e7","transactionHash":"0x53e3675800c6908424b61b35a44e51ca4c73ca603e58a65b32c67968b4f42200","transactionIndex":"0x0","blockHash":"0x2e4620a2b426b0612ec6cad9603f466723edaed87f98c9137405dd4f7a2409ff","blockTimestamp":"0x2706","logIndex":"0x0","removed":false}]`,
		},
		{
			f:    sys.NewRangeFilter(1, 10, []common.Address{contract}, [][]common.Hash{{hash2}, {hash1}}),
			want: `[{"address":"0xfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696332","0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x3","transactionHash":"0xdefe471992a07a02acdfbe33edaae22fbb86d7d3cec3f1b8e4e77702fb3acc1d","transactionIndex":"0x0","blockHash":"0x7a7556792ca7d37882882e2b001fe14833eaf81c2c7f865c9c771ec37a024f6b","blockTimestamp":"0x1e","logIndex":"0x0","removed":false}]`,
		},
		{
			f:    sys.NewRangeFilter(1, 10, nil, [][]common.Hash{{hash1, hash2}}),
			want: `[{"address":"0xfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x2","transactionHash":"0xa8028c655b6423204c8edfbc339f57b042d6bec2b6a61145d76b7c08b4cccd42","transactionIndex":"0x0","blockHash":"0x24417bb49ce44cfad65da68f33b510bf2a129c0d89ccf06acb6958b8585ccf34","blockTimestamp":"0x14","logIndex":"0x0","removed":false},{"address":"0xff00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x2","transactionHash":"0xdba3e2ea9a7d690b722d70ee605fd67ba4c00d1d3aecd5cf187a7b92ad8eb3df","transactionIndex":"0x1","blockHash":"0x24417bb49ce44cfad65da68f33b510bf2a129c0d89ccf06acb6958b8585ccf34","blockTimestamp":"0x14","logIndex":"0x1","removed":false},{"address":"0xfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696332","0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x3","transactionHash":"0xdefe471992a07a02acdfbe33edaae22fbb86d7d3cec3f1b8e4e77702fb3acc1d","transactionIndex":"0x0","blockHash":"0x7a7556792ca7d37882882e2b001fe14833eaf81c2c7f865c9c771ec37a024f6b","blockTimestamp":"0x1e","logIndex":"0x0","removed":false}]`,
		},
		{
			f: sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), nil, [][]common.Hash{{common.BytesToHash([]byte("fail"))}}),
		},
		{
			f: sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), []common.Address{common.BytesToAddress([]byte("failmenow"))}, nil),
		},
		{
			f: sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), nil, [][]common.Hash{{common.BytesToHash([]byte("fail"))}, {hash1}}),
		},
		{
			f:    sys.NewRangeFilter(int64(rpc.LatestBlockNumber), int64(rpc.LatestBlockNumber), nil, nil),
			want: `[{"address":"0xfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696334"],"data":"0x","blockNumber":"0x3e8","transactionHash":"0x9a87842100a638dfa5da8842b4beda691d2fd77b0c84b57f24ecfa9fb208f747","transactionIndex":"0x0","blockHash":"0xb360bad5265261c075ece02d3bf0e39498a6a76310482cdfd90588748e6c5ee0","blockTimestamp":"0x2710","logIndex":"0x0","removed":false}]`,
		},
		{
			f:    sys.NewRangeFilter(int64(rpc.FinalizedBlockNumber), int64(rpc.LatestBlockNumber), nil, nil),
			want: `[{"address":"0xff00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696333"],"data":"0x","blockNumber":"0x3e7","transactionHash":"0x53e3675800c6908424b61b35a44e51ca4c73ca603e58a65b32c67968b4f42200","transactionIndex":"0x0","blockHash":"0x2e4620a2b426b0612ec6cad9603f466723edaed87f98c9137405dd4f7a2409ff","blockTimestamp":"0x2706","logIndex":"0x0","removed":false},{"address":"0xfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696334"],"data":"0x","blockNumber":"0x3e8","transactionHash":"0x9a87842100a638dfa5da8842b4beda691d2fd77b0c84b57f24ecfa9fb208f747","transactionIndex":"0x0","blockHash":"0xb360bad5265261c075ece02d3bf0e39498a6a76310482cdfd90588748e6c5ee0","blockTimestamp":"0x2710","logIndex":"0x0","removed":false}]`,
		},
		{
			f:    sys.NewRangeFilter(int64(rpc.FinalizedBlockNumber), int64(rpc.FinalizedBlockNumber), nil, nil),
			want: `[{"address":"0xff00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696333"],"data":"0x","blockNumber":"0x3e7","transactionHash":"0x53e3675800c6908424b61b35a44e51ca4c73ca603e58a65b32c67968b4f42200","transactionIndex":"0x0","blockHash":"0x2e4620a2b426b0612ec6cad9603f466723edaed87f98c9137405dd4f7a2409ff","blockTimestamp":"0x2706","logIndex":"0x0","removed":false}]`,
		},
		{
			f: sys.NewRangeFilter(int64(rpc.LatestBlockNumber), int64(rpc.FinalizedBlockNumber), nil, nil),
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.SafeBlockNumber), int64(rpc.LatestBlockNumber), nil, nil),
			err: "safe header not found",
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.SafeBlockNumber), int64(rpc.SafeBlockNumber), nil, nil),
			err: "safe header not found",
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.LatestBlockNumber), int64(rpc.SafeBlockNumber), nil, nil),
			err: "safe header not found",
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.PendingBlockNumber), int64(rpc.PendingBlockNumber), nil, nil),
			err: errPendingLogsUnsupported.Error(),
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.LatestBlockNumber), int64(rpc.PendingBlockNumber), nil, nil),
			err: errPendingLogsUnsupported.Error(),
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.PendingBlockNumber), int64(rpc.LatestBlockNumber), nil, nil),
			err: errPendingLogsUnsupported.Error(),
		},
	} {
		logs, err := tc.f.Logs(context.Background())
		if err == nil && tc.err != "" {
			t.Fatalf("test %d, expected error %q, got nil", i, tc.err)
		} else if err != nil && err.Error() != tc.err {
			t.Fatalf("test %d, expected error %q, got %q", i, tc.err, err.Error())
		}
		if tc.want == "" && len(logs) == 0 {
			continue
		}
		have, err := json.Marshal(logs)
		if err != nil {
			t.Fatal(err)
		}
		if string(have) != tc.want {
			t.Fatalf("test %d, have:\n%s\nwant:\n%s", i, have, tc.want)
		}
	}

	t.Run("timeout", func(t *testing.T) {
		f := sys.NewRangeFilter(0, rpc.LatestBlockNumber.Int64(), nil, nil)
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
		defer cancel()
		_, err := f.Logs(ctx)
		if err == nil {
			t.Fatal("expected error")
		}
		if err != context.DeadlineExceeded {
			t.Fatalf("expected context.DeadlineExceeded, got %v", err)
		}
	})
}

func TestRangeLogs(t *testing.T) {
	var (
		db           = rawdb.NewMemoryDatabase()
		backend, sys = newTestFilterSystem(db, Config{})
		gspec        = &core.Genesis{
			Config:  params.TestChainConfig,
			Alloc:   types.GenesisAlloc{},
			BaseFee: big.NewInt(params.InitialBaseFee),
		}
	)
	_, err := gspec.Commit(db, triedb.NewDatabase(db, nil))
	if err != nil {
		t.Fatal(err)
	}
	chain, _ := core.GenerateChain(gspec.Config, gspec.ToBlock(), ethash.NewFaker(), db, 1000, func(i int, gen *core.BlockGen) {})

	options := core.DefaultConfig().WithStateScheme(rawdb.HashScheme)
	options.TxLookupLimit = 0 // index all txs
	bc, err := core.NewBlockChain(db, gspec, ethash.NewFaker(), options)
	if err != nil {
		t.Fatal(err)
	}
	_, err = bc.InsertChain(chain[:600])
	if err != nil {
		t.Fatal(err)
	}

	backend.startFilterMaps(200, false, filtermaps.RangeTestParams)
	defer backend.stopFilterMaps()

	var (
		testCase, event int
		filter          *Filter
		addresses       = []common.Address{{}}
	)

	expEvent := func(expEvent int, expFirst, expAfterLast uint64) {
		exp := rangeLogsTestEvent{expEvent, common.NewRange[uint64](expFirst, expAfterLast-expFirst)}
		event++
		ev := <-filter.rangeLogsTestHook
		if ev != exp {
			t.Fatalf("Test case #%d: wrong test event #%d received (got %v, expected %v)", testCase, event, ev, exp)
		}
	}

	newFilter := func(begin, end int64) {
		testCase++
		event = 0
		filter = sys.NewRangeFilter(begin, end, addresses, nil)
		filter.rangeLogsTestHook = make(chan rangeLogsTestEvent)
		go func(filter *Filter) {
			filter.Logs(context.Background())
			// ensure that filter will not be blocked if we exit early
			for range filter.rangeLogsTestHook {
			}
		}(filter)
	}

	updateHead := func() {
		head := bc.CurrentBlock()
		backend.fm.SetTarget(filtermaps.NewChainView(backend, head.Number.Uint64(), head.Hash()), 0, 0)
		backend.fm.WaitIdle()
	}

	// test case #1
	newFilter(300, 500)
	expEvent(rangeLogsTestIndexed, 401, 501)
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 401, 601)
	expEvent(rangeLogsTestResults, 401, 501)
	expEvent(rangeLogsTestUnindexed, 300, 401)
	if _, err := bc.InsertChain(chain[600:700]); err != nil {
		t.Fatal(err)
	}
	updateHead()
	expEvent(rangeLogsTestResults, 300, 501)
	expEvent(rangeLogsTestDone, 0, 0)

	// test case #2
	newFilter(400, int64(rpc.LatestBlockNumber))
	expEvent(rangeLogsTestIndexed, 501, 701)
	if _, err := bc.InsertChain(chain[700:800]); err != nil {
		t.Fatal(err)
	}
	updateHead()
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 601, 699)
	expEvent(rangeLogsTestResults, 601, 699)
	expEvent(rangeLogsTestUnindexed, 400, 601)
	expEvent(rangeLogsTestResults, 400, 699)
	expEvent(rangeLogsTestIndexed, 699, 801)
	if _, err := bc.SetCanonical(chain[749]); err != nil { // set head to block 750
		t.Fatal(err)
	}
	updateHead()
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 601, 749)
	expEvent(rangeLogsTestResults, 400, 749)
	expEvent(rangeLogsTestIndexed, 749, 751)
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 551, 751)
	expEvent(rangeLogsTestResults, 400, 751)
	expEvent(rangeLogsTestDone, 0, 0)

	// test case #3
	newFilter(int64(rpc.LatestBlockNumber), int64(rpc.LatestBlockNumber))
	expEvent(rangeLogsTestIndexed, 750, 751)
	if _, err := bc.SetCanonical(chain[739]); err != nil {
		t.Fatal(err)
	}
	updateHead()
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 551, 739)
	expEvent(rangeLogsTestResults, 0, 0)
	expEvent(rangeLogsTestIndexed, 740, 741)
	if _, err := bc.InsertChain(chain[740:750]); err != nil {
		t.Fatal(err)
	}
	updateHead()
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 551, 739)
	expEvent(rangeLogsTestResults, 0, 0)
	expEvent(rangeLogsTestIndexed, 750, 751)
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 551, 751)
	expEvent(rangeLogsTestResults, 750, 751)
	expEvent(rangeLogsTestDone, 0, 0)

	// test case #4
	if _, err := bc.SetCanonical(chain[499]); err != nil {
		t.Fatal(err)
	}
	updateHead()
	newFilter(400, int64(rpc.LatestBlockNumber))
	expEvent(rangeLogsTestIndexed, 400, 501)
	if _, err := bc.InsertChain(chain[500:650]); err != nil {
		t.Fatal(err)
	}
	updateHead()
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 451, 499)
	expEvent(rangeLogsTestResults, 451, 499)
	expEvent(rangeLogsTestUnindexed, 400, 451)
	expEvent(rangeLogsTestResults, 400, 499)
	// indexed head extension seems possible
	expEvent(rangeLogsTestIndexed, 499, 651)
	// further head extension causes tail unindexing in searched range
	if _, err := bc.InsertChain(chain[650:750]); err != nil {
		t.Fatal(err)
	}
	updateHead()
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 551, 649)
	// tail trimmed to 551; cannot merge with existing results
	expEvent(rangeLogsTestResults, 551, 649)
	expEvent(rangeLogsTestUnindexed, 400, 551)
	expEvent(rangeLogsTestResults, 400, 649)
	expEvent(rangeLogsTestIndexed, 649, 751)
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 551, 751)
	expEvent(rangeLogsTestResults, 400, 751)
	expEvent(rangeLogsTestDone, 0, 0)

	// test case #5
	newFilter(400, int64(rpc.LatestBlockNumber))
	expEvent(rangeLogsTestIndexed, 551, 751)
	expEvent(rangeLogsTestSync, 0, 0)
	expEvent(rangeLogsTestSynced, 551, 751)
	expEvent(rangeLogsTestResults, 551, 751)
	expEvent(rangeLogsTestUnindexed, 400, 551)
	if _, err := bc.InsertChain(chain[750:1000]); err != nil {
		t.Fatal(err)
	}
	updateHead()
	expEvent(rangeLogsTestResults, 400, 751)
	// indexed tail already beyond results head; revert to unindexed head search
	expEvent(rangeLogsTestUnindexed, 751, 1001)
	if _, err := bc.SetCanonical(chain[899]); err != nil {
		t.Fatal(err)
	}
	updateHead()
	expEvent(rangeLogsTestResults, 400, 1001)
	expEvent(rangeLogsTestReorg, 400, 901)
	expEvent(rangeLogsTestDone, 0, 0)
}
