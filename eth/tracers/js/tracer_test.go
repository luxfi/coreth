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

package js

import (
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/core/vm"
	"github.com/luxfi/geth/eth/tracers"
	"github.com/luxfi/geth/params"
	"github.com/holiman/uint256"
)

type dummyStatedb struct {
	state.StateDB
}

func (*dummyStatedb) GetRefund() uint64                           { return 1337 }
func (*dummyStatedb) GetBalance(addr common.Address) *uint256.Int { return new(uint256.Int) }

type vmContext struct {
	blockCtx vm.BlockContext
	txCtx    vm.TxContext
}

func testCtx() *vmContext {
	return &vmContext{blockCtx: vm.BlockContext{BlockNumber: big.NewInt(1), BaseFee: big.NewInt(0)}, txCtx: vm.TxContext{GasPrice: big.NewInt(100000)}}
}

func runTrace(tracer *tracers.Tracer, vmctx *vmContext, chaincfg *params.ChainConfig, contractCode []byte) (json.RawMessage, error) {
	var (
		evm             = vm.NewEVM(vmctx.blockCtx, &dummyStatedb{}, chaincfg, vm.Config{Tracer: tracer.Hooks})
		gasLimit uint64 = 31000
		startGas uint64 = 10000
		value           = uint256.NewInt(0)
		contract        = vm.NewContract(common.Address{}, common.Address{}, value, startGas, nil)
	)
	evm.SetTxContext(vmctx.txCtx)
	contract.Code = []byte{byte(vm.PUSH1), 0x1, byte(vm.PUSH1), 0x1, 0x0}
	if contractCode != nil {
		contract.Code = contractCode
	}

	tracer.OnTxStart(evm.GetVMContext(), types.NewTx(&types.LegacyTx{Gas: gasLimit, GasPrice: vmctx.txCtx.GasPrice}), contract.Caller())
	tracer.OnEnter(0, byte(vm.CALL), contract.Caller(), contract.Address(), []byte{}, startGas, value.ToBig())
	ret, err := evm.Interpreter().Run(contract, []byte{}, false)
	tracer.OnExit(0, ret, startGas-contract.Gas, err, true)
	// Rest gas assumes no refund
	tracer.OnTxEnd(&types.Receipt{GasUsed: gasLimit - contract.Gas}, nil)
	if err != nil {
		return nil, err
	}
	return tracer.GetResult()
}

func TestTracer(t *testing.T) {
	execTracer := func(code string, contract []byte) ([]byte, string) {
		t.Helper()
		chainConfig := params.TestChainConfig
		tracer, err := newJsTracer(code, nil, nil, chainConfig)
		if err != nil {
			t.Fatal(err)
		}
		ret, err := runTrace(tracer, testCtx(), chainConfig, contract)
		if err != nil {
			return nil, err.Error() // Stringify to allow comparison without nil checks
		}
		return ret, ""
	}
	for i, tt := range []struct {
		code     string
		want     string
		fail     string
		contract []byte
	}{
		{ // tests that we don't panic on bad arguments to memory access
			code: "{depths: [], step: function(log) { this.depths.push(log.memory.slice(-1,-2)); }, fault: function() {}, result: function() { return this.depths; }}",
			want: ``,
			fail: "tracer accessed out of bound memory: offset -1, end -2 at step (<eval>:1:53(13))    in server-side tracer function 'step'",
		}, { // tests that we don't panic on bad arguments to stack peeks
			code: "{depths: [], step: function(log) { this.depths.push(log.stack.peek(-1)); }, fault: function() {}, result: function() { return this.depths; }}",
			want: ``,
			fail: "tracer accessed out of bound stack: size 0, index -1 at step (<eval>:1:53(11))    in server-side tracer function 'step'",
		}, { //  tests that we don't panic on bad arguments to memory getUint
			code: "{ depths: [], step: function(log, db) { this.depths.push(log.memory.getUint(-64));}, fault: function() {}, result: function() { return this.depths; }}",
			want: ``,
			fail: "tracer accessed out of bound memory: available 0, offset -64, size 32 at step (<eval>:1:58(11))    in server-side tracer function 'step'",
		}, { // tests some general counting
			code: "{count: 0, step: function() { this.count += 1; }, fault: function() {}, result: function() { return this.count; }}",
			want: `3`,
		}, { // tests that depth is reported correctly
			code: "{depths: [], step: function(log) { this.depths.push(log.stack.length()); }, fault: function() {}, result: function() { return this.depths; }}",
			want: `[0,1,2]`,
		}, { // tests memory length
			code: "{lengths: [], step: function(log) { this.lengths.push(log.memory.length()); }, fault: function() {}, result: function() { return this.lengths; }}",
			want: `[0,0,0]`,
		}, { // tests to-string of opcodes
			code: "{opcodes: [], step: function(log) { this.opcodes.push(log.op.toString()); }, fault: function() {}, result: function() { return this.opcodes; }}",
			want: `["PUSH1","PUSH1","STOP"]`,
		}, { // tests gasUsed
			code: "{depths: [], step: function() {}, fault: function() {}, result: function(ctx) { return ctx.gasPrice+'.'+ctx.gasUsed; }}",
			want: `"100000.21006"`,
		}, { // tests toWord with byte array length < 32
			code: "{res: null, step: function(log) {}, fault: function() {}, result: function() { return toWord('0xffaa') }}",
			want: `{"0":0,"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0,"10":0,"11":0,"12":0,"13":0,"14":0,"15":0,"16":0,"17":0,"18":0,"19":0,"20":0,"21":0,"22":0,"23":0,"24":0,"25":0,"26":0,"27":0,"28":0,"29":0,"30":255,"31":170}`,
		}, { // tests toWord with byte array length = 32
			code: "{step: function() {}, fault: function() {}, result: function() { return toWord('0x1234567890123456789012345678901234567890123456789012345678901234'); }}",
			want: `{"0":18,"1":52,"2":86,"3":120,"4":144,"5":18,"6":52,"7":86,"8":120,"9":144,"10":18,"11":52,"12":86,"13":120,"14":144,"15":18,"16":52,"17":86,"18":120,"19":144,"20":18,"21":52,"22":86,"23":120,"24":144,"25":18,"26":52,"27":86,"28":120,"29":144,"30":18,"31":52}`,
		}, { // tests toWord with byte array length > 32
			code: "{step: function() {}, fault: function() {}, result: function() { return toWord('0x1234567890123456789012345678901234567890123456789012345678901234567890'); }}",
			want: `{"0":120,"1":144,"2":18,"3":52,"4":86,"5":120,"6":144,"7":18,"8":52,"9":86,"10":120,"11":144,"12":18,"13":52,"14":86,"15":120,"16":144,"17":18,"18":52,"19":86,"20":120,"21":144,"22":18,"23":52,"24":86,"25":120,"26":144,"27":18,"28":52,"29":86,"30":120,"31":144}`,
		}, { // test feeding a buffer back into go
			code: "{res: null, step: function(log) { var address = log.contract.getAddress(); this.res = toAddress(address); }, fault: function() {}, result: function() { return this.res }}",
			want: `{"0":0,"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0,"10":0,"11":0,"12":0,"13":0,"14":0,"15":0,"16":0,"17":0,"18":0,"19":0}`,
		}, {
			code: "{res: null, step: function(log) { var address = '0x0000000000000000000000000000000000000000'; this.res = toAddress(address); }, fault: function() {}, result: function() { return this.res }}",
			want: `{"0":0,"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0,"10":0,"11":0,"12":0,"13":0,"14":0,"15":0,"16":0,"17":0,"18":0,"19":0}`,
		}, {
			code: "{res: null, step: function(log) { var address = Array.prototype.slice.call(log.contract.getAddress()); this.res = toAddress(address); }, fault: function() {}, result: function() { return this.res }}",
			want: `{"0":0,"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0,"10":0,"11":0,"12":0,"13":0,"14":0,"15":0,"16":0,"17":0,"18":0,"19":0}`,
		}, {
			code:     "{res: [], step: function(log) { var op = log.op.toString(); if (op === 'MSTORE8' || op === 'STOP') { this.res.push(log.memory.slice(0, 2)) } }, fault: function() {}, result: function() { return this.res }}",
			want:     `[{"0":0,"1":0},{"0":255,"1":0}]`,
			contract: []byte{byte(vm.PUSH1), byte(0xff), byte(vm.PUSH1), byte(0x00), byte(vm.MSTORE8), byte(vm.STOP)},
		}, {
			code:     "{res: [], step: function(log) { if (log.op.toString() === 'STOP') { this.res.push(log.memory.slice(5, 1025 * 1024)) } }, fault: function() {}, result: function() { return this.res }}",
			want:     "",
			fail:     "reached limit for padding memory slice: 1049568 at step (<eval>:1:83(20))    in server-side tracer function 'step'",
			contract: []byte{byte(vm.PUSH1), byte(0xff), byte(vm.PUSH1), byte(0x00), byte(vm.MSTORE8), byte(vm.STOP)},
		}, { // tests ctx.coinbase
			code: "{lengths: [], step: function(log) { }, fault: function() {}, result: function(ctx) { var coinbase = ctx.coinbase; return toAddress(coinbase); }}",
			want: `{"0":0,"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0,"10":0,"11":0,"12":0,"13":0,"14":0,"15":0,"16":0,"17":0,"18":0,"19":0}`,
		},
	} {
		if have, err := execTracer(tt.code, tt.contract); tt.want != string(have) || tt.fail != err {
			t.Errorf("testcase %d: expected return value to be \n'%s'\n\tgot\n'%s'\nerror to be\n'%s'\n\tgot\n'%s'\n\tcode: %v", i, tt.want, string(have), tt.fail, err, tt.code)
		}
	}
}

func TestHalt(t *testing.T) {
	timeout := errors.New("stahp")
	chainConfig := params.TestChainConfig
	tracer, err := newJsTracer("{step: function() { while(1); }, result: function() { return null; }, fault: function(){}}", nil, nil, chainConfig)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(1 * time.Second)
		tracer.Stop(timeout)
	}()
	if _, err = runTrace(tracer, testCtx(), chainConfig, nil); !strings.Contains(err.Error(), "stahp") {
		t.Errorf("Expected timeout error, got %v", err)
	}
}

func TestHaltBetweenSteps(t *testing.T) {
	chainConfig := params.TestChainConfig
	tracer, err := newJsTracer("{step: function() {}, fault: function() {}, result: function() { return null; }}", nil, nil, chainConfig)
	if err != nil {
		t.Fatal(err)
	}
	scope := &vm.ScopeContext{
		Contract: vm.NewContract(common.Address{}, common.Address{}, uint256.NewInt(0), 0, nil),
	}
	evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(1)}, &dummyStatedb{}, chainConfig, vm.Config{Tracer: tracer.Hooks})
	evm.SetTxContext(vm.TxContext{GasPrice: big.NewInt(1)})
	tracer.OnTxStart(evm.GetVMContext(), types.NewTx(&types.LegacyTx{}), common.Address{})
	tracer.OnEnter(0, byte(vm.CALL), common.Address{}, common.Address{}, []byte{}, 0, big.NewInt(0))
	tracer.OnOpcode(0, 0, 0, 0, scope, nil, 0, nil)
	timeout := errors.New("stahp")
	tracer.Stop(timeout)
	tracer.OnOpcode(0, 0, 0, 0, scope, nil, 0, nil)

	if _, err := tracer.GetResult(); !strings.Contains(err.Error(), timeout.Error()) {
		t.Errorf("Expected timeout error, got %v", err)
	}
}

// TestNoStepExec tests a regular value transfer (no exec), and accessing the statedb
// in 'result'
func TestNoStepExec(t *testing.T) {
	execTracer := func(code string) []byte {
		t.Helper()
		chainConfig := params.TestChainConfig
		tracer, err := newJsTracer(code, nil, nil, chainConfig)
		if err != nil {
			t.Fatal(err)
		}
		evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(1)}, &dummyStatedb{}, chainConfig, vm.Config{Tracer: tracer.Hooks})
		evm.SetTxContext(vm.TxContext{GasPrice: big.NewInt(100)})
		tracer.OnTxStart(evm.GetVMContext(), types.NewTx(&types.LegacyTx{}), common.Address{})
		tracer.OnEnter(0, byte(vm.CALL), common.Address{}, common.Address{}, []byte{}, 1000, big.NewInt(0))
		tracer.OnExit(0, nil, 0, nil, false)
		ret, err := tracer.GetResult()
		if err != nil {
			t.Fatal(err)
		}
		return ret
	}
	for i, tt := range []struct {
		code string
		want string
	}{
		{ // tests that we don't panic on accessing the db methods
			code: "{depths: [], step: function() {}, fault: function() {},  result: function(ctx, db){ return db.getBalance(ctx.to)} }",
			want: `"0"`,
		},
	} {
		if have := execTracer(tt.code); tt.want != string(have) {
			t.Errorf("testcase %d: expected return value to be %s got %s\n\tcode: %v", i, tt.want, string(have), tt.code)
		}
	}
}

func TestIsPrecompile(t *testing.T) {
	chaincfg := &params.ChainConfig{ChainID: big.NewInt(1), HomesteadBlock: big.NewInt(0), DAOForkBlock: nil, DAOForkSupport: false, EIP150Block: big.NewInt(0), EIP155Block: big.NewInt(0), EIP158Block: big.NewInt(0), ByzantiumBlock: big.NewInt(100), ConstantinopleBlock: big.NewInt(0), PetersburgBlock: big.NewInt(0), IstanbulBlock: big.NewInt(200), MuirGlacierBlock: big.NewInt(0), BerlinBlock: big.NewInt(300), LondonBlock: big.NewInt(0), TerminalTotalDifficulty: nil, Ethash: new(params.EthashConfig), Clique: nil}
	chaincfg.ByzantiumBlock = big.NewInt(100)
	chaincfg.IstanbulBlock = big.NewInt(200)
	chaincfg.BerlinBlock = big.NewInt(300)
	txCtx := vm.TxContext{GasPrice: big.NewInt(100000)}
	tracer, err := newJsTracer("{addr: toAddress('0000000000000000000000000000000000000009'), res: null, step: function() { this.res = isPrecompiled(this.addr); }, fault: function() {}, result: function() { return this.res; }}", nil, nil, chaincfg)
	if err != nil {
		t.Fatal(err)
	}

	blockCtx := vm.BlockContext{BlockNumber: big.NewInt(150)}
	res, err := runTrace(tracer, &vmContext{blockCtx, txCtx}, chaincfg, nil)
	if err != nil {
		t.Error(err)
	}
	if string(res) != "false" {
		t.Errorf("tracer should not consider blake2f as precompile in byzantium")
	}

	tracer, _ = newJsTracer("{addr: toAddress('0000000000000000000000000000000000000009'), res: null, step: function() { this.res = isPrecompiled(this.addr); }, fault: function() {}, result: function() { return this.res; }}", nil, nil, chaincfg)
	blockCtx = vm.BlockContext{BlockNumber: big.NewInt(250)}
	res, err = runTrace(tracer, &vmContext{blockCtx, txCtx}, chaincfg, nil)
	if err != nil {
		t.Error(err)
	}
	if string(res) != "true" {
		t.Errorf("tracer should consider blake2f as precompile in istanbul")
	}
}

func TestEnterExit(t *testing.T) {
	chainConfig := params.TestChainConfig
	// test that either both or none of enter() and exit() are defined
	if _, err := newJsTracer("{step: function() {}, fault: function() {}, result: function() { return null; }, enter: function() {}}", new(tracers.Context), nil, chainConfig); err == nil {
		t.Fatal("tracer creation should've failed without exit() definition")
	}
	if _, err := newJsTracer("{step: function() {}, fault: function() {}, result: function() { return null; }, enter: function() {}, exit: function() {}}", new(tracers.Context), nil, chainConfig); err != nil {
		t.Fatal(err)
	}
	// test that the enter and exit method are correctly invoked and the values passed
	tracer, err := newJsTracer("{enters: 0, exits: 0, enterGas: 0, gasUsed: 0, step: function() {}, fault: function() {}, result: function() { return {enters: this.enters, exits: this.exits, enterGas: this.enterGas, gasUsed: this.gasUsed} }, enter: function(frame) { this.enters++; this.enterGas = frame.getGas(); }, exit: function(res) { this.exits++; this.gasUsed = res.getGasUsed(); }}", new(tracers.Context), nil, chainConfig)
	if err != nil {
		t.Fatal(err)
	}
	scope := &vm.ScopeContext{
		Contract: vm.NewContract(common.Address{}, common.Address{}, uint256.NewInt(0), 0, nil),
	}
	tracer.OnEnter(1, byte(vm.CALL), scope.Contract.Caller(), scope.Contract.Address(), []byte{}, 1000, new(big.Int))
	tracer.OnExit(1, []byte{}, 400, nil, false)

	have, err := tracer.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"enters":1,"exits":1,"enterGas":1000,"gasUsed":400}`
	if string(have) != want {
		t.Errorf("Number of invocations of enter() and exit() is wrong. Have %s, want %s\n", have, want)
	}
}

func TestSetup(t *testing.T) {
	// Test empty config
	chainConfig := params.TestChainConfig
	_, err := newJsTracer(`{setup: function(cfg) { if (cfg !== "{}") { throw("invalid empty config") } }, fault: function() {}, result: function() {}}`, new(tracers.Context), nil, chainConfig)
	if err != nil {
		t.Error(err)
	}

	cfg, err := json.Marshal(map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatal(err)
	}
	// Test no setup func
	_, err = newJsTracer(`{fault: function() {}, result: function() {}}`, new(tracers.Context), cfg, chainConfig)
	if err != nil {
		t.Fatal(err)
	}
	// Test config value
	tracer, err := newJsTracer("{config: null, setup: function(cfg) { this.config = JSON.parse(cfg) }, step: function() {}, fault: function() {}, result: function() { return this.config.foo }}", new(tracers.Context), cfg, chainConfig)
	if err != nil {
		t.Fatal(err)
	}
	have, err := tracer.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if string(have) != `"bar"` {
		t.Errorf("tracer returned wrong result. have: %s, want: \"bar\"\n", string(have))
	}
}
