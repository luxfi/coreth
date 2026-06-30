// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomictest

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/luxfi/coreth/plugin/evm/atomic"

	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/coreth/params/extras"
	"github.com/luxfi/crypto"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/ids"
	"github.com/luxfi/math/set"
	"github.com/luxfi/utils/wrappers"
	luxatomic "github.com/luxfi/vm/chains/atomic"
	"github.com/luxfi/vm/components/verify"
)

// TestTxCodec is an atomic.Manager whose unsigned-tx dispatch only knows
// about [TestUnsignedTx]. It exists so repo/trie tests can exercise the
// AtomicRepository code path without depending on production import/export
// transactions.
//
// The wire shape is identical to atomic.Codec's:
//
//	[u16 version][u32 typeID=0][TestUnsignedTx body][u32 nCreds][creds]
//
// TestUnsignedTx is the only unsigned-tx type registered at id=0; production
// Codec uses id=0 for *UnsignedImportTx. The two managers are intentionally
// disjoint — never cross blobs between them.
var TestTxCodec atomic.Manager = &testManager{}

var (
	_                       atomic.UnsignedAtomicTx = (*TestUnsignedTx)(nil)
	errTestUnknownVersion                           = errors.New("unknown test wire version")
	errTestCantUnpackVer                            = errors.New("couldn't unpack test version")
	errTestUnsupportedType                          = errors.New("test codec unsupported type")
	errTestUnknownTypeID                            = errors.New("test codec unknown type id")
)

type testManager struct{}

const testMaxSize = 1 * 1024 * 1024 // 1 MiB

func (testManager) Marshal(version uint16, source interface{}) ([]byte, error) {
	if version != atomic.CodecVersion {
		return nil, fmt.Errorf("%w: %d", errTestUnknownVersion, version)
	}
	p := &wrappers.Packer{MaxSize: testMaxSize}
	p.PackShort(version)
	if p.Errored() {
		return nil, errTestCantUnpackVer
	}
	if err := testMarshal(source, p); err != nil {
		return nil, err
	}
	return p.Bytes, nil
}

func (testManager) Unmarshal(bytes []byte, dest interface{}) (uint16, error) {
	if len(bytes) < 2 {
		return 0, errTestCantUnpackVer
	}
	p := &wrappers.Packer{Bytes: bytes, MaxSize: testMaxSize}
	version := p.UnpackShort()
	if p.Errored() {
		return 0, errTestCantUnpackVer
	}
	if version != atomic.CodecVersion {
		return version, fmt.Errorf("%w: %d", errTestUnknownVersion, version)
	}
	if err := testUnmarshal(dest, p); err != nil {
		return version, err
	}
	return version, nil
}

func testMarshal(source interface{}, p *wrappers.Packer) error {
	switch v := source.(type) {
	case *atomic.Tx:
		return testMarshalTx(v, p)
	case []*atomic.Tx:
		return testMarshalTxs(v, p)
	case *[]*atomic.Tx:
		return testMarshalTxs(*v, p)
	case *atomic.UnsignedAtomicTx:
		// Pointer-to-interface — emitted by Tx.initializeBytes / Tx.Sign to
		// produce the unsigned-bytes form used in the tx hash. Dispatch on
		// the concrete underlying type (TestUnsignedTx is the only one
		// reachable through TestTxCodec).
		utx, ok := (*v).(*TestUnsignedTx)
		if !ok {
			return fmt.Errorf("%w: %T", errTestUnsupportedType, *v)
		}
		p.PackInt(0)
		return testMarshalUnsignedBody(utx, p)
	case *luxatomic.Requests:
		return testMarshalRequests(v, p)
	case *TestUnsignedTx:
		return testMarshalUnsignedBody(v, p)
	default:
		return fmt.Errorf("%w: %T", errTestUnsupportedType, source)
	}
}

func testUnmarshal(dest interface{}, p *wrappers.Packer) error {
	switch v := dest.(type) {
	case *atomic.Tx:
		return testUnmarshalTx(v, p)
	case *[]*atomic.Tx:
		return testUnmarshalTxs(v, p)
	case *luxatomic.Requests:
		return testUnmarshalRequests(v, p)
	default:
		return fmt.Errorf("%w: %T", errTestUnsupportedType, dest)
	}
}

func testMarshalTx(tx *atomic.Tx, p *wrappers.Packer) error {
	if tx == nil {
		return errors.New("nil Tx")
	}
	utx, ok := tx.UnsignedAtomicTx.(*TestUnsignedTx)
	if !ok {
		return fmt.Errorf("%w: TestTxCodec only marshals *TestUnsignedTx, got %T", errTestUnsupportedType, tx.UnsignedAtomicTx)
	}
	// typeID 0 — matches the legacy linearcodec registration in
	// pre-rip atomictest.init() where TestUnsignedTx was the sole registered
	// type and landed at id=0.
	p.PackInt(0)
	if err := testMarshalUnsignedBody(utx, p); err != nil {
		return err
	}
	// Creds: the original codec serialized verify.Verifiable as an interface.
	// In tests no credential types are exercised besides nil, so a count-only
	// prefix preserves byte-equality with a no-cred Tx.
	p.PackInt(uint32(len(tx.Creds)))
	if len(tx.Creds) != 0 {
		return fmt.Errorf("%w: TestTxCodec does not register cred types", errTestUnsupportedType)
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

func testUnmarshalTx(tx *atomic.Tx, p *wrappers.Packer) error {
	typeID := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	if typeID != 0 {
		return fmt.Errorf("%w: %d", errTestUnknownTypeID, typeID)
	}
	utx := &TestUnsignedTx{}
	if err := testUnmarshalUnsignedBody(utx, p); err != nil {
		return err
	}
	tx.UnsignedAtomicTx = utx
	nCreds := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	tx.Creds = make([]verify.Verifiable, 0, nCreds)
	if nCreds != 0 {
		return fmt.Errorf("%w: TestTxCodec does not register cred types", errTestUnsupportedType)
	}
	return nil
}

func testMarshalTxs(txs []*atomic.Tx, p *wrappers.Packer) error {
	p.PackInt(uint32(len(txs)))
	for _, tx := range txs {
		if err := testMarshalTx(tx, p); err != nil {
			return err
		}
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

func testUnmarshalTxs(out *[]*atomic.Tx, p *wrappers.Packer) error {
	n := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	txs := make([]*atomic.Tx, 0, n)
	for i := uint32(0); i < n; i++ {
		tx := &atomic.Tx{}
		if err := testUnmarshalTx(tx, p); err != nil {
			return err
		}
		txs = append(txs, tx)
	}
	*out = txs
	return nil
}

// TestUnsignedTx wire body:
//   [u64 GasUsedV]
//   [32 AcceptRequestsBlockchainIDV]
//   [bool hasReq] (Requests if true)
//   [32 IDV]
//   [u64 BurnedV]

func testMarshalUnsignedBody(t *TestUnsignedTx, p *wrappers.Packer) error {
	p.PackLong(t.GasUsedV)
	p.PackFixedBytes(t.AcceptRequestsBlockchainIDV[:])
	if t.AcceptRequestsV == nil {
		p.PackBool(false)
	} else {
		p.PackBool(true)
		testMarshalRequestsBody(t.AcceptRequestsV, p)
	}
	p.PackFixedBytes(t.IDV[:])
	p.PackLong(t.BurnedV)
	if p.Errored() {
		return p.Err
	}
	return nil
}

func testUnmarshalUnsignedBody(t *TestUnsignedTx, p *wrappers.Packer) error {
	t.GasUsedV = p.UnpackLong()
	copy(t.AcceptRequestsBlockchainIDV[:], p.UnpackFixedBytes(ids.IDLen))
	has := p.UnpackBool()
	if p.Errored() {
		return p.Err
	}
	if has {
		t.AcceptRequestsV = &luxatomic.Requests{}
		if err := testUnmarshalRequestsBody(t.AcceptRequestsV, p); err != nil {
			return err
		}
	}
	copy(t.IDV[:], p.UnpackFixedBytes(ids.IDLen))
	t.BurnedV = p.UnpackLong()
	if p.Errored() {
		return p.Err
	}
	return nil
}

func testMarshalRequests(r *luxatomic.Requests, p *wrappers.Packer) error {
	testMarshalRequestsBody(r, p)
	if p.Errored() {
		return p.Err
	}
	return nil
}

func testUnmarshalRequests(r *luxatomic.Requests, p *wrappers.Packer) error {
	return testUnmarshalRequestsBody(r, p)
}

func testMarshalRequestsBody(r *luxatomic.Requests, p *wrappers.Packer) {
	p.PackInt(uint32(len(r.RemoveRequests)))
	for _, rr := range r.RemoveRequests {
		p.PackBytes(rr)
	}
	p.PackInt(uint32(len(r.PutRequests)))
	for _, el := range r.PutRequests {
		p.PackBytes(el.Key)
		p.PackBytes(el.Value)
		p.PackInt(uint32(len(el.Traits)))
		for _, t := range el.Traits {
			p.PackBytes(t)
		}
	}
}

func testUnmarshalRequestsBody(r *luxatomic.Requests, p *wrappers.Packer) error {
	nRm := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	r.RemoveRequests = make([][]byte, 0, nRm)
	for i := uint32(0); i < nRm; i++ {
		r.RemoveRequests = append(r.RemoveRequests, p.UnpackBytes())
	}
	nPut := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	r.PutRequests = make([]*luxatomic.Element, 0, nPut)
	for i := uint32(0); i < nPut; i++ {
		el := &luxatomic.Element{
			Key:   p.UnpackBytes(),
			Value: p.UnpackBytes(),
		}
		nTraits := p.UnpackInt()
		if p.Errored() {
			return p.Err
		}
		el.Traits = make([][]byte, 0, nTraits)
		for j := uint32(0); j < nTraits; j++ {
			el.Traits = append(el.Traits, p.UnpackBytes())
		}
		r.PutRequests = append(r.PutRequests, el)
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

type TestUnsignedTx struct {
	GasUsedV                    uint64
	AcceptRequestsBlockchainIDV ids.ID
	AcceptRequestsV             *luxatomic.Requests
	VerifyV                     error
	IDV                         ids.ID
	BurnedV                     uint64
	UnsignedBytesV              []byte
	SignedBytesV                []byte
	InputUTXOsV                 set.Set[ids.ID]
	VisitV                      error
	EVMStateTransferV           error
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

// Reference the secp256k1 package so signers carried by tests link cleanly
// when this package is the only atomic surface they import.
var _ = (*secp256k1.PrivateKey)(nil)
