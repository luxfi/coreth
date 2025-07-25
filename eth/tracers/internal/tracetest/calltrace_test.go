// Copyright 2021 The go-ethereum Authors
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

package tracetest

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/common/hexutil"
	"github.com/luxfi/geth/core"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/core/vm"
	"github.com/luxfi/geth/crypto"
	"github.com/luxfi/geth/eth/tracers"
	"github.com/luxfi/geth/params"
	"github.com/luxfi/geth/tests"
)

// callLog is the result of LOG opCode
type callLog struct {
	Address  common.Address `json:"address"`
	Topics   []common.Hash  `json:"topics"`
	Data     hexutil.Bytes  `json:"data"`
	Position hexutil.Uint   `json:"position"`
}

// callTrace is the result of a callTracer run.
type callTrace struct {
	From         common.Address  `json:"from"`
	Gas          *hexutil.Uint64 `json:"gas"`
	GasUsed      *hexutil.Uint64 `json:"gasUsed"`
	To           *common.Address `json:"to,omitempty"`
	Input        hexutil.Bytes   `json:"input"`
	Output       hexutil.Bytes   `json:"output,omitempty"`
	Error        string          `json:"error,omitempty"`
	RevertReason string          `json:"revertReason,omitempty"`
	Calls        []callTrace     `json:"calls,omitempty"`
	Logs         []callLog       `json:"logs,omitempty"`
	Value        *hexutil.Big    `json:"value,omitempty"`
	// Gencodec adds overridden fields at the end
	Type string `json:"type"`
}

// callTracerTest defines a single test to check the call tracer against.
type callTracerTest struct {
	tracerTestEnv
	Result *callTrace `json:"result"`
}

// Iterates over all the input-output datasets in the tracer test harness and
// runs the JavaScript tracers against them.
func TestCallTracerLegacy(t *testing.T) {
	testCallTracer("callTracerLegacy", "call_tracer_legacy", t)
}

func TestCallTracerNative(t *testing.T) {
	testCallTracer("callTracer", "call_tracer", t)
}

func TestCallTracerNativeWithLog(t *testing.T) {
	testCallTracer("callTracer", "call_tracer_withLog", t)
}

func testCallTracer(tracerName string, dirPath string, t *testing.T) {
	isLegacy := strings.HasSuffix(dirPath, "_legacy")
	files, err := os.ReadDir(filepath.Join("testdata", dirPath))
	if err != nil {
		t.Fatalf("failed to retrieve tracer test suite: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		t.Run(camel(strings.TrimSuffix(file.Name(), ".json")), func(t *testing.T) {
			t.Parallel()

			var (
				test = new(callTracerTest)
				tx   = new(types.Transaction)
			)
			// Call tracer test found, read if from disk
			if blob, err := os.ReadFile(filepath.Join("testdata", dirPath, file.Name())); err != nil {
				t.Fatalf("failed to read testcase: %v", err)
			} else if err := json.Unmarshal(blob, test); err != nil {
				t.Fatalf("failed to parse testcase: %v", err)
			}
			if err := tx.UnmarshalBinary(common.FromHex(test.Input)); err != nil {
				t.Fatalf("failed to parse testcase input: %v", err)
			}
			// Configure a blockchain with the given prestate
			var (
				signer  = types.MakeSigner(test.Genesis.Config, new(big.Int).SetUint64(uint64(test.Context.Number)), uint64(test.Context.Time))
				context = test.Context.toBlockContext(test.Genesis)
				st      = tests.MakePreState(rawdb.NewMemoryDatabase(), test.Genesis.Alloc, false, rawdb.HashScheme)
			)
			st.Close()

			tracer, err := tracers.DefaultDirectory.New(tracerName, new(tracers.Context), test.TracerConfig, test.Genesis.Config)
			if err != nil {
				t.Fatalf("failed to create call tracer: %v", err)
			}
			logState := vm.StateDB(st.StateDB)
			if tracer.Hooks != nil {
				logState = state.NewHookedState(st.StateDB, tracer.Hooks)
			}
			msg, err := core.TransactionToMessage(tx, signer, context.BaseFee)
			if err != nil {
				t.Fatalf("failed to prepare transaction for tracing: %v", err)
			}
			evm := vm.NewEVM(context, logState, test.Genesis.Config, vm.Config{Tracer: tracer.Hooks})
			tracer.OnTxStart(evm.GetVMContext(), tx, msg.From)
			vmRet, err := core.ApplyMessage(evm, msg, new(core.GasPool).AddGas(tx.Gas()))
			if err != nil {
				t.Fatalf("failed to execute transaction: %v", err)
			}
			tracer.OnTxEnd(&types.Receipt{GasUsed: vmRet.UsedGas}, nil)
			// Retrieve the trace result and compare against the expected.
			res, err := tracer.GetResult()
			if err != nil {
				t.Fatalf("failed to retrieve trace result: %v", err)
			}
			// The legacy javascript calltracer marshals json in js, which
			// is not deterministic (as opposed to the golang json encoder).
			if isLegacy {
				// This is a tweak to make it deterministic. Can be removed when
				// we remove the legacy tracer.
				var x callTrace
				json.Unmarshal(res, &x)
				res, _ = json.Marshal(x)
			}
			want, err := json.Marshal(test.Result)
			if err != nil {
				t.Fatalf("failed to marshal test: %v", err)
			}
			if string(want) != string(res) {
				t.Fatalf("trace mismatch\n have: %v\n want: %v\n", string(res), string(want))
			}
			// Sanity check: compare top call's gas used against vm result
			type simpleResult struct {
				GasUsed hexutil.Uint64
			}
			var topCall simpleResult
			if err := json.Unmarshal(res, &topCall); err != nil {
				t.Fatalf("failed to unmarshal top calls gasUsed: %v", err)
			}
			if uint64(topCall.GasUsed) != vmRet.UsedGas {
				t.Fatalf("top call has invalid gasUsed. have: %d want: %d", topCall.GasUsed, vmRet.UsedGas)
			}
		})
	}
}

func BenchmarkTracers(b *testing.B) {
	files, err := os.ReadDir(filepath.Join("testdata", "call_tracer"))
	if err != nil {
		b.Fatalf("failed to retrieve tracer test suite: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		b.Run(camel(strings.TrimSuffix(file.Name(), ".json")), func(b *testing.B) {
			blob, err := os.ReadFile(filepath.Join("testdata", "call_tracer", file.Name()))
			if err != nil {
				b.Fatalf("failed to read testcase: %v", err)
			}
			test := new(callTracerTest)
			if err := json.Unmarshal(blob, test); err != nil {
				b.Fatalf("failed to parse testcase: %v", err)
			}
			benchTracer("callTracer", test, b)
		})
	}
}

func benchTracer(tracerName string, test *callTracerTest, b *testing.B) {
	// Configure a blockchain with the given prestate
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(common.FromHex(test.Input)); err != nil {
		b.Fatalf("failed to parse testcase input: %v", err)
	}
	signer := types.MakeSigner(test.Genesis.Config, new(big.Int).SetUint64(uint64(test.Context.Number)), uint64(test.Context.Time))
	context := test.Context.toBlockContext(test.Genesis)
	msg, err := core.TransactionToMessage(tx, signer, context.BaseFee)
	if err != nil {
		b.Fatalf("failed to prepare transaction for tracing: %v", err)
	}
	state := tests.MakePreState(rawdb.NewMemoryDatabase(), test.Genesis.Alloc, false, rawdb.HashScheme)
	defer state.Close()

	b.ReportAllocs()
	b.ResetTimer()

	evm := vm.NewEVM(context, state.StateDB, test.Genesis.Config, vm.Config{})

	for i := 0; i < b.N; i++ {
		snap := state.StateDB.Snapshot()
		tracer, err := tracers.DefaultDirectory.New(tracerName, new(tracers.Context), nil, test.Genesis.Config)
		if err != nil {
			b.Fatalf("failed to create call tracer: %v", err)
		}
		evm.Config.Tracer = tracer.Hooks
		if tracer.OnTxStart != nil {
			tracer.OnTxStart(evm.GetVMContext(), tx, msg.From)
		}
		_, err = core.ApplyMessage(evm, msg, new(core.GasPool).AddGas(tx.Gas()))
		if err != nil {
			b.Fatalf("failed to execute transaction: %v", err)
		}
		if tracer.OnTxEnd != nil {
			tracer.OnTxEnd(&types.Receipt{GasUsed: tx.Gas()}, nil)
		}
		if _, err = tracer.GetResult(); err != nil {
			b.Fatal(err)
		}
		state.StateDB.RevertToSnapshot(snap)
	}
}

func TestInternals(t *testing.T) {
	var (
		config    = params.MainnetChainConfig
		to        = common.HexToAddress("0x00000000000000000000000000000000deadbeef")
		originHex = "0x71562b71999873db5b286df957af199ec94617f7"
		origin    = common.HexToAddress(originHex)
		signer    = types.LatestSigner(config)
		key, _    = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		context   = vm.BlockContext{
			CanTransfer: core.CanTransfer,
			Transfer:    core.Transfer,
			Coinbase:    common.Address{},
			BlockNumber: new(big.Int).SetUint64(8000000),
			Time:        5,
			Difficulty:  big.NewInt(0x30000),
			GasLimit:    uint64(6000000),
			BaseFee:     new(big.Int),
		}
	)
	mkTracer := func(name string, cfg json.RawMessage) *tracers.Tracer {
		tr, err := tracers.DefaultDirectory.New(name, nil, cfg, config)
		if err != nil {
			t.Fatalf("failed to create call tracer: %v", err)
		}
		return tr
	}

	for _, tc := range []struct {
		name   string
		code   []byte
		tracer *tracers.Tracer
		want   string
	}{
		{
			// TestZeroValueToNotExitCall tests the calltracer(s) on the following:
			// Tx to A, A calls B with zero value. B does not already exist.
			// Expected: that enter/exit is invoked and the inner call is shown in the result
			name: "ZeroValueToNotExitCall",
			code: []byte{
				byte(vm.PUSH1), 0x0, byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1), // in and outs zero
				byte(vm.DUP1), byte(vm.PUSH1), 0xff, byte(vm.GAS), // value=0,address=0xff, gas=GAS
				byte(vm.CALL),
			},
			tracer: mkTracer("callTracer", nil),
			want:   fmt.Sprintf(`{"from":"%s","gas":"0x13880","gasUsed":"0x54d8","to":"0x00000000000000000000000000000000deadbeef","input":"0x","calls":[{"from":"0x00000000000000000000000000000000deadbeef","gas":"0xe01a","gasUsed":"0x0","to":"0x00000000000000000000000000000000000000ff","input":"0x","value":"0x0","type":"CALL"}],"value":"0x0","type":"CALL"}`, originHex),
		},
		{
			name:   "Stack depletion in LOG0",
			code:   []byte{byte(vm.LOG3)},
			tracer: mkTracer("callTracer", json.RawMessage(`{ "withLog": true }`)),
			want:   fmt.Sprintf(`{"from":"%s","gas":"0x13880","gasUsed":"0x13880","to":"0x00000000000000000000000000000000deadbeef","input":"0x","error":"stack underflow (0 \u003c=\u003e 5)","value":"0x0","type":"CALL"}`, originHex),
		},
		{
			name: "Mem expansion in LOG0",
			code: []byte{
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0x0,
				byte(vm.MSTORE),
				byte(vm.PUSH1), 0xff,
				byte(vm.PUSH1), 0x0,
				byte(vm.LOG0),
			},
			tracer: mkTracer("callTracer", json.RawMessage(`{ "withLog": true }`)),
			want:   fmt.Sprintf(`{"from":"%s","gas":"0x13880","gasUsed":"0x5b9e","to":"0x00000000000000000000000000000000deadbeef","input":"0x","logs":[{"address":"0x00000000000000000000000000000000deadbeef","topics":[],"data":"0x000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","position":"0x0"}],"value":"0x0","type":"CALL"}`, originHex),
		},
		{
			// Leads to OOM on the prestate tracer
			name: "Prestate-tracer - CREATE2 OOM",
			code: []byte{
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0x0,
				byte(vm.MSTORE),
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH5), 0xff, 0xff, 0xff, 0xff, 0xff,
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0x0,
				byte(vm.CREATE2),
				byte(vm.PUSH1), 0xff,
				byte(vm.PUSH1), 0x0,
				byte(vm.LOG0),
			},
			tracer: mkTracer("prestateTracer", nil),
			want:   fmt.Sprintf(`{"0x00000000000000000000000000000000deadbeef":{"balance":"0x0","code":"0x6001600052600164ffffffffff60016000f560ff6000a0"},"%s":{"balance":"0x1c6bf52634000"}}`, originHex),
		},
		{
			// CREATE2 which requires padding memory by prestate tracer
			name: "Prestate-tracer - CREATE2 Memory padding",
			code: []byte{
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0x0,
				byte(vm.MSTORE),
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0xff,
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0x0,
				byte(vm.CREATE2),
				byte(vm.PUSH1), 0xff,
				byte(vm.PUSH1), 0x0,
				byte(vm.LOG0),
			},
			tracer: mkTracer("prestateTracer", nil),
			want:   fmt.Sprintf(`{"0x00000000000000000000000000000000deadbeef":{"balance":"0x0","code":"0x6001600052600160ff60016000f560ff6000a0"},"%s":{"balance":"0x1c6bf52634000"}}`, originHex),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := tests.MakePreState(rawdb.NewMemoryDatabase(),
				types.GenesisAlloc{
					to: types.Account{
						Code: tc.code,
					},
					origin: types.Account{
						Balance: big.NewInt(500000000000000),
					},
				}, false, rawdb.HashScheme)
			defer st.Close()

			logState := vm.StateDB(st.StateDB)
			if hooks := tc.tracer.Hooks; hooks != nil {
				logState = state.NewHookedState(st.StateDB, hooks)
			}

			tx, err := types.SignNewTx(key, signer, &types.LegacyTx{
				To:       &to,
				Value:    big.NewInt(0),
				Gas:      80000,
				GasPrice: big.NewInt(1),
			})
			if err != nil {
				t.Fatalf("test %v: failed to sign transaction: %v", tc.name, err)
			}
			evm := vm.NewEVM(context, logState, config, vm.Config{Tracer: tc.tracer.Hooks})
			msg, err := core.TransactionToMessage(tx, signer, big.NewInt(0))
			if err != nil {
				t.Fatalf("test %v: failed to create message: %v", tc.name, err)
			}
			tc.tracer.OnTxStart(evm.GetVMContext(), tx, msg.From)
			vmRet, err := core.ApplyMessage(evm, msg, new(core.GasPool).AddGas(tx.Gas()))
			if err != nil {
				t.Fatalf("test %v: failed to execute transaction: %v", tc.name, err)
			}
			tc.tracer.OnTxEnd(&types.Receipt{GasUsed: vmRet.UsedGas}, nil)
			// Retrieve the trace result and compare against the expected
			res, err := tc.tracer.GetResult()
			if err != nil {
				t.Fatalf("test %v: failed to retrieve trace result: %v", tc.name, err)
			}
			if string(res) != tc.want {
				t.Errorf("test %v: trace mismatch\n have: %v\n want: %v\n", tc.name, string(res), tc.want)
			}
		})
	}
}
