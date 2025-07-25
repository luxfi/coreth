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

package runtime

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/luxfi/geth/accounts/abi"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/consensus"
	"github.com/luxfi/geth/core"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/core/tracing"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/core/vm"
	"github.com/luxfi/geth/core/vm/program"
	"github.com/luxfi/geth/eth/tracers"
	"github.com/luxfi/geth/eth/tracers/logger"
	"github.com/luxfi/geth/params"

	// force-load js tracers to trigger registration
	_ "github.com/luxfi/geth/eth/tracers/js"
)

func TestDefaults(t *testing.T) {
	cfg := new(Config)
	setDefaults(cfg)

	if cfg.Difficulty == nil {
		t.Error("expected difficulty to be non nil")
	}

	if cfg.GasLimit == 0 {
		t.Error("didn't expect gaslimit to be zero")
	}
	if cfg.GasPrice == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.Value == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.GetHashFn == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.BlockNumber == nil {
		t.Error("expected block number to be non nil")
	}
}

func TestEVM(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("crashed with: %v", r)
		}
	}()

	Execute([]byte{
		byte(vm.DIFFICULTY),
		byte(vm.TIMESTAMP),
		byte(vm.GASLIMIT),
		byte(vm.PUSH1),
		byte(vm.ORIGIN),
		byte(vm.BLOCKHASH),
		byte(vm.COINBASE),
	}, nil, nil)
}

func TestExecute(t *testing.T) {
	ret, _, err := Execute([]byte{
		byte(vm.PUSH1), 10,
		byte(vm.PUSH1), 0,
		byte(vm.MSTORE),
		byte(vm.PUSH1), 32,
		byte(vm.PUSH1), 0,
		byte(vm.RETURN),
	}, nil, nil)
	if err != nil {
		t.Fatal("didn't expect error", err)
	}

	num := new(big.Int).SetBytes(ret)
	if num.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", num)
	}
}

func TestCall(t *testing.T) {
	state, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	address := common.HexToAddress("0xaa")
	state.SetCode(address, []byte{
		byte(vm.PUSH1), 10,
		byte(vm.PUSH1), 0,
		byte(vm.MSTORE),
		byte(vm.PUSH1), 32,
		byte(vm.PUSH1), 0,
		byte(vm.RETURN),
	})

	ret, _, err := Call(address, nil, &Config{State: state})
	if err != nil {
		t.Fatal("didn't expect error", err)
	}

	num := new(big.Int).SetBytes(ret)
	if num.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", num)
	}
}

func BenchmarkCall(b *testing.B) {
	var definition = `[{"constant":true,"inputs":[],"name":"seller","outputs":[{"name":"","type":"address"}],"type":"function"},{"constant":false,"inputs":[],"name":"abort","outputs":[],"type":"function"},{"constant":true,"inputs":[],"name":"value","outputs":[{"name":"","type":"uint256"}],"type":"function"},{"constant":false,"inputs":[],"name":"refund","outputs":[],"type":"function"},{"constant":true,"inputs":[],"name":"buyer","outputs":[{"name":"","type":"address"}],"type":"function"},{"constant":false,"inputs":[],"name":"confirmReceived","outputs":[],"type":"function"},{"constant":true,"inputs":[],"name":"state","outputs":[{"name":"","type":"uint8"}],"type":"function"},{"constant":false,"inputs":[],"name":"confirmPurchase","outputs":[],"type":"function"},{"inputs":[],"type":"constructor"},{"anonymous":false,"inputs":[],"name":"Aborted","type":"event"},{"anonymous":false,"inputs":[],"name":"PurchaseConfirmed","type":"event"},{"anonymous":false,"inputs":[],"name":"ItemReceived","type":"event"},{"anonymous":false,"inputs":[],"name":"Refunded","type":"event"}]`

	var code = common.Hex2Bytes("6060604052361561006c5760e060020a600035046308551a53811461007457806335a063b4146100865780633fa4f245146100a6578063590e1ae3146100af5780637150d8ae146100cf57806373fac6f0146100e1578063c19d93fb146100fe578063d696069714610112575b610131610002565b610133600154600160a060020a031681565b610131600154600160a060020a0390811633919091161461015057610002565b61014660005481565b610131600154600160a060020a039081163391909116146102d557610002565b610133600254600160a060020a031681565b610131600254600160a060020a0333811691161461023757610002565b61014660025460ff60a060020a9091041681565b61013160025460009060ff60a060020a9091041681146101cc57610002565b005b600160a060020a03166060908152602090f35b6060908152602090f35b60025460009060a060020a900460ff16811461016b57610002565b600154600160a060020a03908116908290301631606082818181858883f150506002805460a060020a60ff02191660a160020a179055506040517f72c874aeff0b183a56e2b79c71b46e1aed4dee5e09862134b8821ba2fddbf8bf9250a150565b80546002023414806101dd57610002565b6002805460a060020a60ff021973ffffffffffffffffffffffffffffffffffffffff1990911633171660a060020a1790557fd5d55c8a68912e9a110618df8d5e2e83b8d83211c57a8ddd1203df92885dc881826060a15050565b60025460019060a060020a900460ff16811461025257610002565b60025460008054600160a060020a0390921691606082818181858883f150508354604051600160a060020a0391821694503090911631915082818181858883f150506002805460a060020a60ff02191660a160020a179055506040517fe89152acd703c9d8c7d28829d443260b411454d45394e7995815140c8cbcbcf79250a150565b60025460019060a060020a900460ff1681146102f057610002565b6002805460008054600160a060020a0390921692909102606082818181858883f150508354604051600160a060020a0391821694503090911631915082818181858883f150506002805460a060020a60ff02191660a160020a179055506040517f8616bbbbad963e4e65b1366f1d75dfb63f9e9704bbbf91fb01bec70849906cf79250a15056")

	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		b.Fatal(err)
	}

	cpurchase, err := abi.Pack("confirmPurchase")
	if err != nil {
		b.Fatal(err)
	}
	creceived, err := abi.Pack("confirmReceived")
	if err != nil {
		b.Fatal(err)
	}
	refund, err := abi.Pack("refund")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 400; j++ {
			Execute(code, cpurchase, nil)
			Execute(code, creceived, nil)
			Execute(code, refund, nil)
		}
	}
}
func benchmarkEVM_Create(bench *testing.B, code string) {
	var (
		statedb, _ = state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
		sender     = common.BytesToAddress([]byte("sender"))
		receiver   = common.BytesToAddress([]byte("receiver"))
	)

	statedb.CreateAccount(sender)
	statedb.SetCode(receiver, common.FromHex(code))
	runtimeConfig := Config{
		Origin:      sender,
		State:       statedb,
		GasLimit:    10000000,
		Difficulty:  big.NewInt(0x200000),
		Time:        0,
		Coinbase:    common.Address{},
		BlockNumber: new(big.Int).SetUint64(1),
		ChainConfig: &params.ChainConfig{
			ChainID:             big.NewInt(1),
			HomesteadBlock:      new(big.Int),
			ByzantiumBlock:      new(big.Int),
			ConstantinopleBlock: new(big.Int),
			DAOForkBlock:        new(big.Int),
			DAOForkSupport:      false,
			EIP150Block:         new(big.Int),
			EIP155Block:         new(big.Int),
			EIP158Block:         new(big.Int),
		},
		EVMConfig: vm.Config{},
	}
	// Warm up the intpools and stuff
	bench.ResetTimer()
	for i := 0; i < bench.N; i++ {
		Call(receiver, []byte{}, &runtimeConfig)
	}
	bench.StopTimer()
}

func BenchmarkEVM_CREATE_500(bench *testing.B) {
	// initcode size 500K, repeatedly calls CREATE and then modifies the mem contents
	benchmarkEVM_Create(bench, "5b6207a120600080f0600152600056")
}
func BenchmarkEVM_CREATE2_500(bench *testing.B) {
	// initcode size 500K, repeatedly calls CREATE2 and then modifies the mem contents
	benchmarkEVM_Create(bench, "5b586207a120600080f5600152600056")
}
func BenchmarkEVM_CREATE_1200(bench *testing.B) {
	// initcode size 1200K, repeatedly calls CREATE and then modifies the mem contents
	benchmarkEVM_Create(bench, "5b62124f80600080f0600152600056")
}
func BenchmarkEVM_CREATE2_1200(bench *testing.B) {
	// initcode size 1200K, repeatedly calls CREATE2 and then modifies the mem contents
	benchmarkEVM_Create(bench, "5b5862124f80600080f5600152600056")
}

func BenchmarkEVM_SWAP1(b *testing.B) {
	// returns a contract that does n swaps (SWAP1)
	swapContract := func(n uint64) []byte {
		contract := []byte{
			byte(vm.PUSH0), // PUSH0
			byte(vm.PUSH0), // PUSH0
		}
		for i := uint64(0); i < n; i++ {
			contract = append(contract, byte(vm.SWAP1))
		}
		return contract
	}

	state, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	contractAddr := common.BytesToAddress([]byte("contract"))

	b.Run("10k", func(b *testing.B) {
		contractCode := swapContract(10_000)
		state.SetCode(contractAddr, contractCode)

		for i := 0; i < b.N; i++ {
			_, _, err := Call(contractAddr, []byte{}, &Config{State: state})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEVM_RETURN(b *testing.B) {
	// returns a contract that returns a zero-byte slice of len size
	returnContract := func(size uint64) []byte {
		contract := []byte{
			byte(vm.PUSH8), 0, 0, 0, 0, 0, 0, 0, 0, // PUSH8 0xXXXXXXXXXXXXXXXX
			byte(vm.PUSH0),  // PUSH0
			byte(vm.RETURN), // RETURN
		}
		binary.BigEndian.PutUint64(contract[1:], size)
		return contract
	}

	state, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	contractAddr := common.BytesToAddress([]byte("contract"))

	for _, n := range []uint64{1_000, 10_000, 100_000, 1_000_000} {
		b.Run(strconv.FormatUint(n, 10), func(b *testing.B) {
			b.ReportAllocs()

			contractCode := returnContract(n)
			state.SetCode(contractAddr, contractCode)

			for i := 0; i < b.N; i++ {
				ret, _, err := Call(contractAddr, []byte{}, &Config{State: state})
				if err != nil {
					b.Fatal(err)
				}
				if uint64(len(ret)) != n {
					b.Fatalf("expected return size %d, got %d", n, len(ret))
				}
			}
		})
	}
}

func fakeHeader(n uint64, parentHash common.Hash) *types.Header {
	header := types.Header{
		Coinbase:   common.HexToAddress("0x00000000000000000000000000000000deadbeef"),
		Number:     big.NewInt(int64(n)),
		ParentHash: parentHash,
		Time:       1000,
		Nonce:      types.BlockNonce{0x1},
		Extra:      []byte{},
		Difficulty: big.NewInt(0),
		GasLimit:   100000,
	}
	return &header
}

type dummyChain struct {
	counter int
}

// Engine retrieves the chain's consensus engine.
func (d *dummyChain) Engine() consensus.Engine {
	return nil
}

// GetHeader returns the hash corresponding to their hash.
func (d *dummyChain) GetHeader(h common.Hash, n uint64) *types.Header {
	d.counter++
	parentHash := common.Hash{}
	s := common.LeftPadBytes(big.NewInt(int64(n-1)).Bytes(), 32)
	copy(parentHash[:], s)

	//parentHash := common.Hash{byte(n - 1)}
	//fmt.Printf("GetHeader(%x, %d) => header with parent %x\n", h, n, parentHash)
	return fakeHeader(n, parentHash)
}

func (d *dummyChain) Config() *params.ChainConfig {
	return nil
}

// TestBlockhash tests the blockhash operation. It's a bit special, since it internally
// requires access to a chain reader.
func TestBlockhash(t *testing.T) {
	// Current head
	n := uint64(1000)
	parentHash := common.Hash{}
	s := common.LeftPadBytes(big.NewInt(int64(n-1)).Bytes(), 32)
	copy(parentHash[:], s)
	header := fakeHeader(n, parentHash)

	// This is the contract we're using. It requests the blockhash for current num (should be all zeroes),
	// then iteratively fetches all blockhashes back to n-260.
	// It returns
	// 1. the first (should be zero)
	// 2. the second (should be the parent hash)
	// 3. the last non-zero hash
	// By making the chain reader return hashes which correlate to the number, we can
	// verify that it obtained the right hashes where it should

	/*

		pragma solidity ^0.5.3;
		contract Hasher{

			function test() public view returns (bytes32, bytes32, bytes32){
				uint256 x = block.number;
				bytes32 first;
				bytes32 last;
				bytes32 zero;
				zero = blockhash(x); // Should be zeroes
				first = blockhash(x-1);
				for(uint256 i = 2 ; i < 260; i++){
					bytes32 hash = blockhash(x - i);
					if (uint256(hash) != 0){
						last = hash;
					}
				}
				return (zero, first, last);
			}
		}

	*/
	// The contract above
	data := common.Hex2Bytes("6080604052348015600f57600080fd5b50600436106045576000357c010000000000000000000000000000000000000000000000000000000090048063f8a8fd6d14604a575b600080fd5b60506074565b60405180848152602001838152602001828152602001935050505060405180910390f35b600080600080439050600080600083409050600184034092506000600290505b61010481101560c35760008186034090506000816001900414151560b6578093505b5080806001019150506094565b508083839650965096505050505090919256fea165627a7a72305820462d71b510c1725ff35946c20b415b0d50b468ea157c8c77dff9466c9cb85f560029")
	// The method call to 'test()'
	input := common.Hex2Bytes("f8a8fd6d")
	chain := &dummyChain{}
	ret, _, err := Execute(data, input, &Config{
		GetHashFn:   core.GetHashFn(header, chain),
		BlockNumber: new(big.Int).Set(header.Number),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(ret) != 96 {
		t.Fatalf("expected returndata to be 96 bytes, got %d", len(ret))
	}

	zero := new(big.Int).SetBytes(ret[0:32])
	first := new(big.Int).SetBytes(ret[32:64])
	last := new(big.Int).SetBytes(ret[64:96])
	if zero.BitLen() != 0 {
		t.Fatalf("expected zeroes, got %x", ret[0:32])
	}
	if first.Uint64() != 999 {
		t.Fatalf("second block should be 999, got %d (%x)", first, ret[32:64])
	}
	if last.Uint64() != 744 {
		t.Fatalf("last block should be 744, got %d (%x)", last, ret[64:96])
	}
	if exp, got := 255, chain.counter; exp != got {
		t.Errorf("suboptimal; too much chain iteration, expected %d, got %d", exp, got)
	}
}

// benchmarkNonModifyingCode benchmarks code, but if the code modifies the
// state, this should not be used, since it does not reset the state between runs.
func benchmarkNonModifyingCode(gas uint64, code []byte, name string, tracerCode string, b *testing.B) {
	cfg := new(Config)
	setDefaults(cfg)
	cfg.State, _ = state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	cfg.GasLimit = gas
	if len(tracerCode) > 0 {
		tracer, err := tracers.DefaultDirectory.New(tracerCode, new(tracers.Context), nil, cfg.ChainConfig)
		if err != nil {
			b.Fatal(err)
		}
		cfg.EVMConfig = vm.Config{
			Tracer: tracer.Hooks,
		}
	}
	destination := common.BytesToAddress([]byte("contract"))
	cfg.State.CreateAccount(destination)
	eoa := common.HexToAddress("E0")
	{
		cfg.State.CreateAccount(eoa)
		cfg.State.SetNonce(eoa, 100, tracing.NonceChangeUnspecified)
	}
	reverting := common.HexToAddress("EE")
	{
		cfg.State.CreateAccount(reverting)
		cfg.State.SetCode(reverting, []byte{
			byte(vm.PUSH1), 0x00,
			byte(vm.PUSH1), 0x00,
			byte(vm.REVERT),
		})
	}

	//cfg.State.CreateAccount(cfg.Origin)
	// set the receiver's (the executing contract) code for execution.
	cfg.State.SetCode(destination, code)
	Call(destination, nil, cfg)

	b.Run(name, func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			Call(destination, nil, cfg)
		}
	})
}

// BenchmarkSimpleLoop test a pretty simple loop which loops until OOG
// 55 ms
func BenchmarkSimpleLoop(b *testing.B) {
	p, lbl := program.New().Jumpdest()
	// Call identity, and pop return value
	staticCallIdentity := p.
		StaticCall(nil, 0x4, 0, 0, 0, 0).
		Op(vm.POP).Jump(lbl).Bytes() // pop return value and jump to label

	p, lbl = program.New().Jumpdest()
	callIdentity := p.
		Call(nil, 0x4, 0, 0, 0, 0, 0).
		Op(vm.POP).Jump(lbl).Bytes() // pop return value and jump to label

	p, lbl = program.New().Jumpdest()
	callInexistant := p.
		Call(nil, 0xff, 0, 0, 0, 0, 0).
		Op(vm.POP).Jump(lbl).Bytes() // pop return value and jump to label

	p, lbl = program.New().Jumpdest()
	callEOA := p.
		Call(nil, 0xE0, 0, 0, 0, 0, 0). // call addr of EOA
		Op(vm.POP).Jump(lbl).Bytes()    // pop return value and jump to label

	p, lbl = program.New().Jumpdest()
	// Push as if we were making call, then pop it off again, and loop
	loopingCode := p.Push(0).
		Op(vm.DUP1, vm.DUP1, vm.DUP1).
		Push(0x4).
		Op(vm.GAS, vm.POP, vm.POP, vm.POP, vm.POP, vm.POP, vm.POP).
		Jump(lbl).Bytes()

	p, lbl = program.New().Jumpdest()
	loopingCode2 := p.
		Push(0x01020304).Push(uint64(0x0102030405)).
		Op(vm.POP, vm.POP).
		Op(vm.PUSH6).Append(make([]byte, 6)).Op(vm.JUMP). // Jumpdest zero expressed in 6 bytes
		Jump(lbl).Bytes()

	p, lbl = program.New().Jumpdest()
	callRevertingContractWithInput := p.
		Call(nil, 0xee, 0, 0, 0x20, 0x0, 0x0).
		Op(vm.POP).Jump(lbl).Bytes() // pop return value and jump to label

	//tracer := logger.NewJSONLogger(nil, os.Stdout)
	//Execute(loopingCode, nil, &Config{
	//	EVMConfig: vm.Config{
	//		Debug:  true,
	//		Tracer: tracer,
	//	}})
	// 100M gas
	benchmarkNonModifyingCode(100000000, staticCallIdentity, "staticcall-identity-100M", "", b)
	benchmarkNonModifyingCode(100000000, callIdentity, "call-identity-100M", "", b)
	benchmarkNonModifyingCode(100000000, loopingCode, "loop-100M", "", b)
	benchmarkNonModifyingCode(100000000, loopingCode2, "loop2-100M", "", b)
	benchmarkNonModifyingCode(100000000, callInexistant, "call-nonexist-100M", "", b)
	benchmarkNonModifyingCode(100000000, callEOA, "call-EOA-100M", "", b)
	benchmarkNonModifyingCode(100000000, callRevertingContractWithInput, "call-reverting-100M", "", b)

	//benchmarkNonModifyingCode(10000000, staticCallIdentity, "staticcall-identity-10M", b)
	//benchmarkNonModifyingCode(10000000, loopingCode, "loop-10M", b)
}

// TestEip2929Cases contains various testcases that are used for
// EIP-2929 about gas repricings
func TestEip2929Cases(t *testing.T) {
	t.Skip("Test only useful for generating documentation")
	id := 1
	prettyPrint := func(comment string, code []byte) {
		fmt.Printf("### Case %d\n\n", id)
		id++
		fmt.Printf("%v\n\nBytecode: \n```\n%#x\n```\n", comment, code)
		Execute(code, nil, &Config{
			EVMConfig: vm.Config{
				Tracer:    logger.NewMarkdownLogger(nil, os.Stdout).Hooks(),
				ExtraEips: []int{2929},
			},
		})
	}

	{ // First eip testcase
		code := []byte{
			// Three checks against a precompile
			byte(vm.PUSH1), 1, byte(vm.EXTCODEHASH), byte(vm.POP),
			byte(vm.PUSH1), 2, byte(vm.EXTCODESIZE), byte(vm.POP),
			byte(vm.PUSH1), 3, byte(vm.BALANCE), byte(vm.POP),
			// Three checks against a non-precompile
			byte(vm.PUSH1), 0xf1, byte(vm.EXTCODEHASH), byte(vm.POP),
			byte(vm.PUSH1), 0xf2, byte(vm.EXTCODESIZE), byte(vm.POP),
			byte(vm.PUSH1), 0xf3, byte(vm.BALANCE), byte(vm.POP),
			// Same three checks (should be cheaper)
			byte(vm.PUSH1), 0xf2, byte(vm.EXTCODEHASH), byte(vm.POP),
			byte(vm.PUSH1), 0xf3, byte(vm.EXTCODESIZE), byte(vm.POP),
			byte(vm.PUSH1), 0xf1, byte(vm.BALANCE), byte(vm.POP),
			// Check the origin, and the 'this'
			byte(vm.ORIGIN), byte(vm.BALANCE), byte(vm.POP),
			byte(vm.ADDRESS), byte(vm.BALANCE), byte(vm.POP),

			byte(vm.STOP),
		}
		prettyPrint("This checks `EXT`(codehash,codesize,balance) of precompiles, which should be `100`, "+
			"and later checks the same operations twice against some non-precompiles. "+
			"Those are cheaper second time they are accessed. Lastly, it checks the `BALANCE` of `origin` and `this`.", code)
	}

	{ // EXTCODECOPY
		code := []byte{
			// extcodecopy( 0xff,0,0,0,0)
			byte(vm.PUSH1), 0x00, byte(vm.PUSH1), 0x00, byte(vm.PUSH1), 0x00, //length, codeoffset, memoffset
			byte(vm.PUSH1), 0xff, byte(vm.EXTCODECOPY),
			// extcodecopy( 0xff,0,0,0,0)
			byte(vm.PUSH1), 0x00, byte(vm.PUSH1), 0x00, byte(vm.PUSH1), 0x00, //length, codeoffset, memoffset
			byte(vm.PUSH1), 0xff, byte(vm.EXTCODECOPY),
			// extcodecopy( this,0,0,0,0)
			byte(vm.PUSH1), 0x00, byte(vm.PUSH1), 0x00, byte(vm.PUSH1), 0x00, //length, codeoffset, memoffset
			byte(vm.ADDRESS), byte(vm.EXTCODECOPY),

			byte(vm.STOP),
		}
		prettyPrint("This checks `extcodecopy( 0xff,0,0,0,0)` twice, (should be expensive first time), "+
			"and then does `extcodecopy( this,0,0,0,0)`.", code)
	}

	{ // SLOAD + SSTORE
		code := []byte{

			// Add slot `0x1` to access list
			byte(vm.PUSH1), 0x01, byte(vm.SLOAD), byte(vm.POP), // SLOAD( 0x1) (add to access list)
			// Write to `0x1` which is already in access list
			byte(vm.PUSH1), 0x11, byte(vm.PUSH1), 0x01, byte(vm.SSTORE), // SSTORE( loc: 0x01, val: 0x11)
			// Write to `0x2` which is not in access list
			byte(vm.PUSH1), 0x11, byte(vm.PUSH1), 0x02, byte(vm.SSTORE), // SSTORE( loc: 0x02, val: 0x11)
			// Write again to `0x2`
			byte(vm.PUSH1), 0x11, byte(vm.PUSH1), 0x02, byte(vm.SSTORE), // SSTORE( loc: 0x02, val: 0x11)
			// Read slot in access list (0x2)
			byte(vm.PUSH1), 0x02, byte(vm.SLOAD), // SLOAD( 0x2)
			// Read slot in access list (0x1)
			byte(vm.PUSH1), 0x01, byte(vm.SLOAD), // SLOAD( 0x1)
		}
		prettyPrint("This checks `sload( 0x1)` followed by `sstore(loc: 0x01, val:0x11)`, then 'naked' sstore:"+
			"`sstore(loc: 0x02, val:0x11)` twice, and `sload(0x2)`, `sload(0x1)`. ", code)
	}
	{ // Call variants
		code := []byte{
			// identity precompile
			byte(vm.PUSH1), 0x0, byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
			byte(vm.PUSH1), 0x04, byte(vm.PUSH1), 0x0, byte(vm.CALL), byte(vm.POP),

			// random account - call 1
			byte(vm.PUSH1), 0x0, byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
			byte(vm.PUSH1), 0xff, byte(vm.PUSH1), 0x0, byte(vm.CALL), byte(vm.POP),

			// random account - call 2
			byte(vm.PUSH1), 0x0, byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
			byte(vm.PUSH1), 0xff, byte(vm.PUSH1), 0x0, byte(vm.STATICCALL), byte(vm.POP),
		}
		prettyPrint("This calls the `identity`-precompile (cheap), then calls an account (expensive) and `staticcall`s the same"+
			"account (cheap)", code)
	}
}

// TestColdAccountAccessCost test that the cold account access cost is reported
// correctly
// see: https://github.com/luxfi/geth/issues/22649
func TestColdAccountAccessCost(t *testing.T) {
	for i, tc := range []struct {
		code []byte
		step int
		want uint64
	}{
		{ // EXTCODEHASH(0xff)
			code: []byte{byte(vm.PUSH1), 0xFF, byte(vm.EXTCODEHASH), byte(vm.POP)},
			step: 1,
			want: 2600,
		},
		{ // BALANCE(0xff)
			code: []byte{byte(vm.PUSH1), 0xFF, byte(vm.BALANCE), byte(vm.POP)},
			step: 1,
			want: 2600,
		},
		{ // CALL(0xff)
			code: []byte{
				byte(vm.PUSH1), 0x0,
				byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
				byte(vm.PUSH1), 0xff, byte(vm.DUP1), byte(vm.CALL), byte(vm.POP),
			},
			step: 7,
			want: 2855,
		},
		{ // CALLCODE(0xff)
			code: []byte{
				byte(vm.PUSH1), 0x0,
				byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
				byte(vm.PUSH1), 0xff, byte(vm.DUP1), byte(vm.CALLCODE), byte(vm.POP),
			},
			step: 7,
			want: 2855,
		},
		{ // DELEGATECALL(0xff)
			code: []byte{
				byte(vm.PUSH1), 0x0,
				byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
				byte(vm.PUSH1), 0xff, byte(vm.DUP1), byte(vm.DELEGATECALL), byte(vm.POP),
			},
			step: 6,
			want: 2855,
		},
		{ // STATICCALL(0xff)
			code: []byte{
				byte(vm.PUSH1), 0x0,
				byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
				byte(vm.PUSH1), 0xff, byte(vm.DUP1), byte(vm.STATICCALL), byte(vm.POP),
			},
			step: 6,
			want: 2855,
		},
		{ // SELFDESTRUCT(0xff)
			code: []byte{
				byte(vm.PUSH1), 0xff, byte(vm.SELFDESTRUCT),
			},
			step: 1,
			want: 7600,
		},
	} {
		var step = 0
		var have = uint64(0)
		Execute(tc.code, nil, &Config{
			EVMConfig: vm.Config{
				Tracer: &tracing.Hooks{
					OnOpcode: func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
						// Uncomment to investigate failures:
						//t.Logf("%d: %v %d", step, vm.OpCode(op).String(), cost)
						if step == tc.step {
							have = cost
						}
						step++
					},
				},
			},
		})
		if want := tc.want; have != want {
			t.Fatalf("testcase %d, gas report wrong, step %d, have %d want %d", i, tc.step, have, want)
		}
	}
}

func TestRuntimeJSTracer(t *testing.T) {
	jsTracers := []string{
		`{enters: 0, exits: 0, enterGas: 0, gasUsed: 0, steps:0,
	step: function() { this.steps++},
	fault: function() {},
	result: function() {
		return [this.enters, this.exits,this.enterGas,this.gasUsed, this.steps].join(",")
	},
	enter: function(frame) {
		this.enters++;
		this.enterGas = frame.getGas();
	},
	exit: function(res) {
		this.exits++;
		this.gasUsed = res.getGasUsed();
	}}`,
		`{enters: 0, exits: 0, enterGas: 0, gasUsed: 0, steps:0,
	fault: function() {},
	result: function() {
		return [this.enters, this.exits,this.enterGas,this.gasUsed, this.steps].join(",")
	},
	enter: function(frame) {
		this.enters++;
		this.enterGas = frame.getGas();
	},
	exit: function(res) {
		this.exits++;
		this.gasUsed = res.getGasUsed();
	}}`}
	initcode := program.New().Return(0, 0).Bytes()
	tests := []struct {
		code []byte
		// One result per tracer
		results []string
	}{
		{ // CREATE
			code: program.New().MstoreSmall(initcode, 0).
				Push(len(initcode)).      // length
				Push(32 - len(initcode)). // offset
				Push(0).                  // value
				Op(vm.CREATE).
				Op(vm.POP).Bytes(),
			results: []string{`"1,1,952853,6,12"`, `"1,1,952853,6,0"`},
		},
		{ // CREATE2
			code: program.New().MstoreSmall(initcode, 0).
				Push(1).                  // salt
				Push(len(initcode)).      // length
				Push(32 - len(initcode)). // offset
				Push(0).                  // value
				Op(vm.CREATE2).
				Op(vm.POP).Bytes(),
			results: []string{`"1,1,952844,6,13"`, `"1,1,952844,6,0"`},
		},
		{ // CALL
			code:    program.New().Call(nil, 0xbb, 0, 0, 0, 0, 0).Op(vm.POP).Bytes(),
			results: []string{`"1,1,981796,6,13"`, `"1,1,981796,6,0"`},
		},
		{ // CALLCODE
			code:    program.New().CallCode(nil, 0xcc, 0, 0, 0, 0, 0).Op(vm.POP).Bytes(),
			results: []string{`"1,1,981796,6,13"`, `"1,1,981796,6,0"`},
		},
		{ // STATICCALL
			code:    program.New().StaticCall(nil, 0xdd, 0, 0, 0, 0).Op(vm.POP).Bytes(),
			results: []string{`"1,1,981799,6,12"`, `"1,1,981799,6,0"`},
		},
		{ // DELEGATECALL
			code:    program.New().DelegateCall(nil, 0xee, 0, 0, 0, 0).Op(vm.POP).Bytes(),
			results: []string{`"1,1,981799,6,12"`, `"1,1,981799,6,0"`},
		},
		{ // CALL self-destructing contract
			code:    program.New().Call(nil, 0xff, 0, 0, 0, 0, 0).Op(vm.POP).Bytes(),
			results: []string{`"2,2,0,5003,12"`, `"2,2,0,5003,0"`},
		},
	}
	calleeCode := []byte{
		byte(vm.PUSH1), 0,
		byte(vm.PUSH1), 0,
		byte(vm.RETURN),
	}
	suicideCode := []byte{
		byte(vm.PUSH1), 0xaa,
		byte(vm.SELFDESTRUCT),
	}
	main := common.HexToAddress("0xaa")
	for i, jsTracer := range jsTracers {
		for j, tc := range tests {
			statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
			statedb.SetCode(main, tc.code)
			statedb.SetCode(common.HexToAddress("0xbb"), calleeCode)
			statedb.SetCode(common.HexToAddress("0xcc"), calleeCode)
			statedb.SetCode(common.HexToAddress("0xdd"), calleeCode)
			statedb.SetCode(common.HexToAddress("0xee"), calleeCode)
			statedb.SetCode(common.HexToAddress("0xff"), suicideCode)

			tracer, err := tracers.DefaultDirectory.New(jsTracer, new(tracers.Context), nil, params.MergedTestChainConfig)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = Call(main, nil, &Config{
				GasLimit: 1000000,
				State:    statedb,
				EVMConfig: vm.Config{
					Tracer: tracer.Hooks,
				}})
			if err != nil {
				t.Fatal("didn't expect error", err)
			}
			res, err := tracer.GetResult()
			if err != nil {
				t.Fatal(err)
			}
			if have, want := string(res), tc.results[i]; have != want {
				t.Errorf("wrong result for tracer %d testcase %d, have \n%v\nwant\n%v\n", i, j, have, want)
			}
		}
	}
}

func TestJSTracerCreateTx(t *testing.T) {
	jsTracer := `
	{enters: 0, exits: 0,
	step: function() {},
	fault: function() {},
	result: function() { return [this.enters, this.exits].join(",") },
	enter: function(frame) { this.enters++ },
	exit: function(res) { this.exits++ }}`
	code := []byte{byte(vm.PUSH1), 0, byte(vm.PUSH1), 0, byte(vm.RETURN)}

	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	tracer, err := tracers.DefaultDirectory.New(jsTracer, new(tracers.Context), nil, params.MergedTestChainConfig)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = Create(code, &Config{
		State: statedb,
		EVMConfig: vm.Config{
			Tracer: tracer.Hooks,
		}})
	if err != nil {
		t.Fatal(err)
	}

	res, err := tracer.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if have, want := string(res), `"0,0"`; have != want {
		t.Errorf("wrong result for tracer, have \n%v\nwant\n%v\n", have, want)
	}
}

func BenchmarkTracerStepVsCallFrame(b *testing.B) {
	// Simply pushes and pops some values in a loop
	p, lbl := program.New().Jumpdest()
	code := p.Push(0).Push(0).Op(vm.POP, vm.POP).Jump(lbl).Bytes()
	stepTracer := `
	{
	step: function() {},
	fault: function() {},
	result: function() {},
	}`
	callFrameTracer := `
	{
	enter: function() {},
	exit: function() {},
	fault: function() {},
	result: function() {},
	}`

	benchmarkNonModifyingCode(10000000, code, "tracer-step-10M", stepTracer, b)
	benchmarkNonModifyingCode(10000000, code, "tracer-call-frame-10M", callFrameTracer, b)
}

// TestDelegatedAccountAccessCost tests that calling an account with an EIP-7702
// delegation designator incurs the correct amount of gas based on the tracer.
func TestDelegatedAccountAccessCost(t *testing.T) {
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	statedb.SetCode(common.HexToAddress("0xff"), types.AddressToDelegation(common.HexToAddress("0xaa")))
	statedb.SetCode(common.HexToAddress("0xaa"), program.New().Return(0, 0).Bytes())

	for i, tc := range []struct {
		code []byte
		step int
		want uint64
	}{
		{ // CALL(0xff)
			code: []byte{
				byte(vm.PUSH1), 0x0,
				byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
				byte(vm.PUSH1), 0xff, byte(vm.DUP1), byte(vm.CALL), byte(vm.POP),
			},
			step: 7,
			want: 5455,
		},
		{ // CALLCODE(0xff)
			code: []byte{
				byte(vm.PUSH1), 0x0,
				byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
				byte(vm.PUSH1), 0xff, byte(vm.DUP1), byte(vm.CALLCODE), byte(vm.POP),
			},
			step: 7,
			want: 5455,
		},
		{ // DELEGATECALL(0xff)
			code: []byte{
				byte(vm.PUSH1), 0x0,
				byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
				byte(vm.PUSH1), 0xff, byte(vm.DUP1), byte(vm.DELEGATECALL), byte(vm.POP),
			},
			step: 6,
			want: 5455,
		},
		{ // STATICCALL(0xff)
			code: []byte{
				byte(vm.PUSH1), 0x0,
				byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1),
				byte(vm.PUSH1), 0xff, byte(vm.DUP1), byte(vm.STATICCALL), byte(vm.POP),
			},
			step: 6,
			want: 5455,
		},
		{ // SELFDESTRUCT(0xff): should not be affected by resolution
			code: []byte{
				byte(vm.PUSH1), 0xff, byte(vm.SELFDESTRUCT),
			},
			step: 1,
			want: 7600,
		},
	} {
		var step = 0
		var have = uint64(0)
		Execute(tc.code, nil, &Config{
			ChainConfig: params.MergedTestChainConfig,
			State:       statedb,
			EVMConfig: vm.Config{
				Tracer: &tracing.Hooks{
					OnOpcode: func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
						// Uncomment to investigate failures:
						t.Logf("%d: %v %d", step, vm.OpCode(op).String(), cost)
						if step == tc.step {
							have = cost
						}
						step++
					},
				},
			},
		})
		if want := tc.want; have != want {
			t.Fatalf("testcase %d, gas report wrong, step %d, have %d want %d", i, tc.step, have, want)
		}
	}
}
