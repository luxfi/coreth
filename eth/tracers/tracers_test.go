// Copyright 2017 The go-ethereum Authors
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

package tracers

import (
	"math/big"
	"testing"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/core/vm"
	"github.com/luxfi/geth/crypto"
	"github.com/luxfi/geth/eth/tracers/logger"
	"github.com/luxfi/geth/params"
	"github.com/luxfi/geth/tests"
)

func BenchmarkTransactionTraceV2(b *testing.B) {
	key, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	from := crypto.PubkeyToAddress(key.PublicKey)
	gas := uint64(1000000) // 1M gas
	to := common.HexToAddress("0x00000000000000000000000000000000deadbeef")
	signer := types.LatestSignerForChainID(big.NewInt(1337))
	tx, err := types.SignNewTx(key, signer,
		&types.LegacyTx{
			Nonce:    1,
			GasPrice: big.NewInt(500),
			Gas:      gas,
			To:       &to,
		})
	if err != nil {
		b.Fatal(err)
	}
	context := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		Coinbase:    common.Address{},
		BlockNumber: new(big.Int).SetUint64(uint64(5)),
		Time:        5,
		Difficulty:  big.NewInt(0xffffffff),
		GasLimit:    gas,
		BaseFee:     big.NewInt(8),
	}
	alloc := types.GenesisAlloc{}
	// The code pushes 'deadbeef' into memory, then the other params, and calls CREATE2, then returns
	// the address
	loop := []byte{
		byte(vm.JUMPDEST), //  [ count ]
		byte(vm.PUSH1), 0, // jumpdestination
		byte(vm.JUMP),
	}
	alloc[common.HexToAddress("0x00000000000000000000000000000000deadbeef")] = types.Account{
		Nonce:   1,
		Code:    loop,
		Balance: big.NewInt(1),
	}
	alloc[from] = types.Account{
		Nonce:   1,
		Code:    []byte{},
		Balance: big.NewInt(500000000000000),
	}
	state := tests.MakePreState(rawdb.NewMemoryDatabase(), alloc, false, rawdb.HashScheme)
	defer state.Close()

	evm := vm.NewEVM(context, state.StateDB, params.AllEthashProtocolChanges, vm.Config{})

	msg, err := core.TransactionToMessage(tx, signer, context.BaseFee)
	if err != nil {
		b.Fatalf("failed to prepare transaction for tracing: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tracer := logger.NewStructLogger(&logger.Config{}).Hooks()
		tracer.OnTxStart(evm.GetVMContext(), tx, msg.From)
		evm.Config.Tracer = tracer

		snap := state.StateDB.Snapshot()
		_, err := core.ApplyMessage(evm, msg, new(core.GasPool).AddGas(tx.Gas()))
		if err != nil {
			b.Fatal(err)
		}
		state.StateDB.RevertToSnapshot(snap)
	}
}
