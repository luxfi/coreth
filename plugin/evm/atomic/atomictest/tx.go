// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomictest

import (
	"math/rand"

	"github.com/luxfi/coreth/plugin/evm/atomic"

	"github.com/luxfi/codec"
	"github.com/luxfi/codec/linearcodec"
	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/codec/wrappers"
	"github.com/luxfi/crypto"
	"github.com/luxfi/ids"
	"github.com/luxfi/math/set"
	luxatomic "github.com/luxfi/vm/chains/atomic"
)

// TODO: Remove this and use actual codec and transactions (export, import)
var (
	_           atomic.UnsignedAtomicTx = (*TestUnsignedTx)(nil)
	TestTxCodec codec.Manager
)

func init() {
	TestTxCodec = codec.NewDefaultManager()
	c := linearcodec.NewDefault()

	errs := wrappers.Errs{}
	errs.Add(
		c.RegisterType(&TestUnsignedTx{}),
		TestTxCodec.RegisterCodec(0, c),
	)

	if errs.Errored() {
		panic(errs.Err)
	}
}

type TestUnsignedTx struct {
	GasUsedV                    uint64              `serialize:"true"`
	AcceptRequestsBlockchainIDV ids.ID              `serialize:"true"`
	AcceptRequestsV             *luxatomic.Requests `serialize:"true"`
	VerifyV                     error               `serialize:"-"`
	IDV                         ids.ID              `serialize:"true" json:"id"`
	BurnedV                     uint64              `serialize:"true"`
	UnsignedBytesV              []byte              `serialize:"-"`
	SignedBytesV                []byte              `serialize:"-"`
	InputUTXOsV                 set.Set[ids.ID]     `serialize:"-"`
	VisitV                      error               `serialize:"-"`
	EVMStateTransferV           error               `serialize:"-"`
}

// GasUsed implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) GasUsed(fixedFee bool) (uint64, error) { return t.GasUsedV, nil }

// Verify implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) Verify(ctx *consensusctx.Context, rules extras.Rules) error {
	return t.VerifyV
}

// AtomicOps implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) AtomicOps() (ids.ID, *luxatomic.Requests, error) {
	return t.AcceptRequestsBlockchainIDV, t.AcceptRequestsV, nil
}

// Initialize implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) Initialize(unsignedBytes, signedBytes []byte) {}

// ID implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) ID() ids.ID { return t.IDV }

// Burned implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) Burned(assetID ids.ID) (uint64, error) { return t.BurnedV, nil }

// Bytes implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) Bytes() []byte { return t.UnsignedBytesV }

// SignedBytes implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) SignedBytes() []byte { return t.SignedBytesV }

// InputUTXOs implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) InputUTXOs() set.Set[ids.ID] { return t.InputUTXOsV }

// Visit implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) Visit(v atomic.Visitor) error {
	return t.VisitV
}

// EVMStateTransfer implements the UnsignedAtomicTx interface
func (t *TestUnsignedTx) EVMStateTransfer(ctx *consensusctx.Context, state atomic.StateDB) error {
	return t.EVMStateTransferV
}

var TestBlockchainID = ids.GenerateTestID()

func GenerateTestImportTxWithGas(gasUsed uint64, burned uint64) *atomic.Tx {
	return &atomic.Tx{
		UnsignedAtomicTx: &TestUnsignedTx{
			IDV:                         ids.GenerateTestID(),
			GasUsedV:                    gasUsed,
			BurnedV:                     burned,
			AcceptRequestsBlockchainIDV: TestBlockchainID,
			AcceptRequestsV: &luxatomic.Requests{
				RemoveRequests: [][]byte{
					crypto.RandomBytes(32),
					crypto.RandomBytes(32),
				},
			},
		},
	}
}

func GenerateTestImportTx() *atomic.Tx {
	return &atomic.Tx{
		UnsignedAtomicTx: &TestUnsignedTx{
			IDV:                         ids.GenerateTestID(),
			AcceptRequestsBlockchainIDV: TestBlockchainID,
			AcceptRequestsV: &luxatomic.Requests{
				RemoveRequests: [][]byte{
					crypto.RandomBytes(32),
					crypto.RandomBytes(32),
				},
			},
		},
	}
}

func GenerateTestExportTx() *atomic.Tx {
	return &atomic.Tx{
		UnsignedAtomicTx: &TestUnsignedTx{
			IDV:                         ids.GenerateTestID(),
			AcceptRequestsBlockchainIDV: TestBlockchainID,
			AcceptRequestsV: &luxatomic.Requests{
				PutRequests: []*luxatomic.Element{
					{
						Key:   crypto.RandomBytes(16),
						Value: crypto.RandomBytes(24),
						Traits: [][]byte{
							crypto.RandomBytes(32),
							crypto.RandomBytes(32),
						},
					},
				},
			},
		},
	}
}

func NewTestTx() *atomic.Tx {
	txType := rand.Intn(2)
	switch txType {
	case 0:
		return GenerateTestImportTx()
	case 1:
		return GenerateTestExportTx()
	default:
		panic("rng generated unexpected value for tx type")
	}
}

func NewTestTxs(numTxs int) []*atomic.Tx {
	txs := make([]*atomic.Tx, 0, numTxs)
	for i := 0; i < numTxs; i++ {
		txs = append(txs, NewTestTx())
	}

	return txs
}
