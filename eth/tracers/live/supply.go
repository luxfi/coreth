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

package live

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/common/hexutil"
	"github.com/luxfi/geth/consensus/misc/eip4844"
	"github.com/luxfi/geth/core/tracing"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/core/vm"
	"github.com/luxfi/geth/eth/tracers"
	"github.com/luxfi/geth/log"
	"github.com/luxfi/geth/params"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
	tracers.LiveDirectory.Register("supply", newSupplyTracer)
}

type supplyInfoIssuance struct {
	GenesisAlloc *big.Int `json:"genesisAlloc,omitempty"`
	Reward       *big.Int `json:"reward,omitempty"`
	Withdrawals  *big.Int `json:"withdrawals,omitempty"`
}

//go:generate go run github.com/fjl/gencodec -type supplyInfoIssuance -field-override supplyInfoIssuanceMarshaling -out gen_supplyinfoissuance.go
type supplyInfoIssuanceMarshaling struct {
	GenesisAlloc *hexutil.Big
	Reward       *hexutil.Big
	Withdrawals  *hexutil.Big
}

type supplyInfoBurn struct {
	EIP1559 *big.Int `json:"1559,omitempty"`
	Blob    *big.Int `json:"blob,omitempty"`
	Misc    *big.Int `json:"misc,omitempty"`
}

//go:generate go run github.com/fjl/gencodec -type supplyInfoBurn -field-override supplyInfoBurnMarshaling -out gen_supplyinfoburn.go
type supplyInfoBurnMarshaling struct {
	EIP1559 *hexutil.Big
	Blob    *hexutil.Big
	Misc    *hexutil.Big
}

type supplyInfo struct {
	Issuance *supplyInfoIssuance `json:"issuance,omitempty"`
	Burn     *supplyInfoBurn     `json:"burn,omitempty"`

	// Block info
	Number     uint64      `json:"blockNumber"`
	Hash       common.Hash `json:"hash"`
	ParentHash common.Hash `json:"parentHash"`
}

type supplyTxCallstack struct {
	calls []supplyTxCallstack
	burn  *big.Int
}

type supplyTracer struct {
	delta       supplyInfo
	txCallstack []supplyTxCallstack // Callstack for current transaction
	logger      *lumberjack.Logger
	chainConfig *params.ChainConfig
}

type supplyTracerConfig struct {
	Path    string `json:"path"`    // Path to the directory where the tracer logs will be stored
	MaxSize int    `json:"maxSize"` // MaxSize is the maximum size in megabytes of the tracer log file before it gets rotated. It defaults to 100 megabytes.
}

func newSupplyTracer(cfg json.RawMessage) (*tracing.Hooks, error) {
	var config supplyTracerConfig
	if err := json.Unmarshal(cfg, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}
	if config.Path == "" {
		return nil, errors.New("supply tracer output path is required")
	}

	// Store traces in a rotating file
	logger := &lumberjack.Logger{
		Filename: filepath.Join(config.Path, "supply.jsonl"),
	}
	if config.MaxSize > 0 {
		logger.MaxSize = config.MaxSize
	}

	t := &supplyTracer{
		delta:  newSupplyInfo(),
		logger: logger,
	}
	return &tracing.Hooks{
		OnBlockchainInit: t.onBlockchainInit,
		OnBlockStart:     t.onBlockStart,
		OnBlockEnd:       t.onBlockEnd,
		OnGenesisBlock:   t.onGenesisBlock,
		OnTxStart:        t.onTxStart,
		OnBalanceChange:  t.onBalanceChange,
		OnEnter:          t.onEnter,
		OnExit:           t.onExit,
		OnClose:          t.onClose,
	}, nil
}

func newSupplyInfo() supplyInfo {
	return supplyInfo{
		Issuance: &supplyInfoIssuance{
			GenesisAlloc: big.NewInt(0),
			Reward:       big.NewInt(0),
			Withdrawals:  big.NewInt(0),
		},
		Burn: &supplyInfoBurn{
			EIP1559: big.NewInt(0),
			Blob:    big.NewInt(0),
			Misc:    big.NewInt(0),
		},

		Number:     0,
		Hash:       common.Hash{},
		ParentHash: common.Hash{},
	}
}

func (s *supplyTracer) resetDelta() {
	s.delta = newSupplyInfo()
}

func (s *supplyTracer) onBlockchainInit(chainConfig *params.ChainConfig) {
	s.chainConfig = chainConfig
}

func (s *supplyTracer) onBlockStart(ev tracing.BlockEvent) {
	s.resetDelta()

	s.delta.Number = ev.Block.NumberU64()
	s.delta.Hash = ev.Block.Hash()
	s.delta.ParentHash = ev.Block.ParentHash()

	// Calculate Burn for this block
	if ev.Block.BaseFee() != nil {
		burn := new(big.Int).Mul(new(big.Int).SetUint64(ev.Block.GasUsed()), ev.Block.BaseFee())
		s.delta.Burn.EIP1559 = burn
	}
	// Blob burnt gas
	if blobGas := ev.Block.BlobGasUsed(); blobGas != nil && *blobGas > 0 && ev.Block.ExcessBlobGas() != nil {
		var (
			baseFee = eip4844.CalcBlobFee(s.chainConfig, ev.Block.Header())
			burn    = new(big.Int).Mul(new(big.Int).SetUint64(*blobGas), baseFee)
		)
		s.delta.Burn.Blob = burn
	}
}

func (s *supplyTracer) onBlockEnd(err error) {
	s.write(s.delta)
}

func (s *supplyTracer) onGenesisBlock(b *types.Block, alloc types.GenesisAlloc) {
	s.resetDelta()

	s.delta.Number = b.NumberU64()
	s.delta.Hash = b.Hash()
	s.delta.ParentHash = b.ParentHash()

	// Initialize supply with total allocation in genesis block
	for _, account := range alloc {
		s.delta.Issuance.GenesisAlloc.Add(s.delta.Issuance.GenesisAlloc, account.Balance)
	}

	s.write(s.delta)
}

func (s *supplyTracer) onBalanceChange(a common.Address, prevBalance, newBalance *big.Int, reason tracing.BalanceChangeReason) {
	diff := new(big.Int).Sub(newBalance, prevBalance)

	// NOTE: don't handle "BalanceIncreaseGenesisBalance" because it is handled in OnGenesisBlock
	switch reason {
	case tracing.BalanceIncreaseRewardMineUncle:
	case tracing.BalanceIncreaseRewardMineBlock:
		s.delta.Issuance.Reward.Add(s.delta.Issuance.Reward, diff)
	case tracing.BalanceIncreaseWithdrawal:
		s.delta.Issuance.Withdrawals.Add(s.delta.Issuance.Withdrawals, diff)
	case tracing.BalanceDecreaseSelfdestructBurn:
		// BalanceDecreaseSelfdestructBurn is non-reversible as it happens
		// at the end of the transaction.
		s.delta.Burn.Misc.Sub(s.delta.Burn.Misc, diff)
	default:
		return
	}
}

func (s *supplyTracer) onTxStart(vm *tracing.VMContext, tx *types.Transaction, from common.Address) {
	s.txCallstack = make([]supplyTxCallstack, 0, 1)
}

// internalTxsHandler handles internal transactions burned amount
func (s *supplyTracer) internalTxsHandler(call *supplyTxCallstack) {
	// Handle Burned amount
	if call.burn != nil {
		s.delta.Burn.Misc.Add(s.delta.Burn.Misc, call.burn)
	}

	// Recursively handle internal calls
	for _, call := range call.calls {
		callCopy := call
		s.internalTxsHandler(&callCopy)
	}
}

func (s *supplyTracer) onEnter(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	call := supplyTxCallstack{
		calls: make([]supplyTxCallstack, 0),
	}

	// This is a special case of burned amount which has to be handled here
	// which happens when type == selfdestruct and from == to.
	if vm.OpCode(typ) == vm.SELFDESTRUCT && from == to && value.Cmp(common.Big0) == 1 {
		call.burn = value
	}

	// Append call to the callstack, so we can fill the details in CaptureExit
	s.txCallstack = append(s.txCallstack, call)
}

func (s *supplyTracer) onExit(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
	if depth == 0 {
		// No need to handle Burned amount if transaction is reverted
		if !reverted {
			s.internalTxsHandler(&s.txCallstack[0])
		}
		return
	}

	size := len(s.txCallstack)
	if size <= 1 {
		return
	}
	// Pop call
	call := s.txCallstack[size-1]
	s.txCallstack = s.txCallstack[:size-1]
	size -= 1

	// In case of a revert, we can drop the call and all its subcalls.
	// Caution, that this has to happen after popping the call from the stack.
	if reverted {
		return
	}
	s.txCallstack[size-1].calls = append(s.txCallstack[size-1].calls, call)
}

func (s *supplyTracer) onClose() {
	if err := s.logger.Close(); err != nil {
		log.Warn("failed to close supply tracer log file", "error", err)
	}
}

func (s *supplyTracer) write(data any) {
	supply, ok := data.(supplyInfo)
	if !ok {
		log.Warn("failed to cast supply tracer data on write to log file")
		return
	}

	// Remove empty fields
	if supply.Issuance.GenesisAlloc.Sign() == 0 {
		supply.Issuance.GenesisAlloc = nil
	}

	if supply.Issuance.Reward.Sign() == 0 {
		supply.Issuance.Reward = nil
	}

	if supply.Issuance.Withdrawals.Sign() == 0 {
		supply.Issuance.Withdrawals = nil
	}

	if supply.Issuance.GenesisAlloc == nil && supply.Issuance.Reward == nil && supply.Issuance.Withdrawals == nil {
		supply.Issuance = nil
	}

	if supply.Burn.EIP1559.Sign() == 0 {
		supply.Burn.EIP1559 = nil
	}

	if supply.Burn.Blob.Sign() == 0 {
		supply.Burn.Blob = nil
	}

	if supply.Burn.Misc.Sign() == 0 {
		supply.Burn.Misc = nil
	}

	if supply.Burn.EIP1559 == nil && supply.Burn.Blob == nil && supply.Burn.Misc == nil {
		supply.Burn = nil
	}

	out, _ := json.Marshal(supply)
	if _, err := s.logger.Write(out); err != nil {
		log.Warn("failed to write to supply tracer log file", "error", err)
	}
	if _, err := s.logger.Write([]byte{'\n'}); err != nil {
		log.Warn("failed to write to supply tracer log file", "error", err)
	}
}
