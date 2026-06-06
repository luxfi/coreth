// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomic

import (
	"errors"
	"fmt"

	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/ids"
	"github.com/luxfi/utils/wrappers"
	lux "github.com/luxfi/utxo"
	"github.com/luxfi/utxo/secp256k1fx"
	luxatomic "github.com/luxfi/vm/chains/atomic"
	"github.com/luxfi/vm/components/verify"
)

// Wire format
//
// All atomic-tx wire blobs are length-prefixed-and-versioned:
//
//	[u16 version][... type-specific big-endian fields ...]
//
// Version is currently 0. Interfaces (UnsignedAtomicTx, TransferableIn /
// TransferableOut feature-extension types, and verify.Verifiable credentials)
// are prefixed with their legacy linearcodec type IDs so the byte layout is
// preserved against any on-disk data or peer that already speaks the
// linearcodec encoding:
//
//	UnsignedImportTx          = 0
//	UnsignedExportTx          = 1
//	secp256k1fx.TransferInput = 5
//	secp256k1fx.TransferOutput = 7
//	secp256k1fx.Credential    = 9
//
// (Type IDs 2, 3, 4, 6, 8, 10, 11 were registered for other fx types or
// SkipRegistrations; none of them are reachable through the atomic-tx
// envelope, so they have no marshaler here. An interface with an unknown
// type ID surfaces as ErrUnknownTypeID at unmarshal time.)
//
// Slices and byte blobs are length-prefixed with a u32. Fixed-size byte
// arrays (ids.ID, secp256k1 signatures) are emitted as raw bytes. bool is
// a single byte (0/1). Big-endian throughout. Strings (none in this
// package, but used by the codec body) are u16-length-prefixed UTF-8.

// CodecVersion is the current wire version. Increment on any incompatible
// schema change. There is no "v0 read fallback" — hard cut, one shape.
const CodecVersion = uint16(0)

// Type IDs preserved from the legacy linearcodec registration order in the
// pre-rip init(). Kept as named constants so the wire layout is documented
// next to its dispatcher.
const (
	typeIDUnsignedImportTx     uint32 = 0
	typeIDUnsignedExportTx     uint32 = 1
	typeIDTransferInput        uint32 = 5
	typeIDTransferOutput       uint32 = 7
	typeIDSecp256k1Credential  uint32 = 9
)

// Manager is the surface every wire-touching peer of this package needs. It
// is the smallest method set that lets a caller serialize or parse an
// atomic-tx blob — no reflection registry, no codec.Codec sub-interface.
//
// The package singleton [Codec] implements [Manager]. Downstream packages
// (atomic/state, atomic/vm, plugin/evm/atomic/atomictest) accept [Manager]
// in their constructors rather than coupling to a specific concrete type.
type Manager interface {
	Marshal(version uint16, source interface{}) ([]byte, error)
	Unmarshal(bytes []byte, dest interface{}) (uint16, error)
}

const maxAtomicMessageSize = 1 * 1024 * 1024 // 1 MiB — matches legacy codec.DefaultMaxSize

// manager is the package-local marshal/unmarshal entry point.
type manager struct {
	maxSize int
}

// Codec is the singleton atomic-tx codec. Callers use Codec.Marshal /
// Codec.Unmarshal.
var Codec Manager = &manager{maxSize: maxAtomicMessageSize}

var (
	ErrUnknownVersion       = errors.New("unknown wire version")
	ErrCantUnpackVersion    = errors.New("couldn't unpack version")
	ErrCantPackVersion      = errors.New("couldn't pack version")
	ErrUnsupportedType      = errors.New("unsupported atomic type")
	ErrUnknownTypeID        = errors.New("unknown atomic type id")
	ErrMaxSizeExceeded      = errors.New("max atomic message size exceeded")
	ErrExtraSpace           = errors.New("trailing buffer space in atomic blob")
	ErrMissingAtomicTxs     = errors.New("cannot build a block with non-empty extra data and zero atomic transactions")
)

// Marshal serialises a source value with a u16 [version] prefix. Returns
// ErrUnsupportedType if the source isn't one of the supported atomic
// shapes (Tx, []*Tx, *UTXO, *Requests, *UnsignedImportTx, *UnsignedExportTx,
// or a pointer-to-UnsignedAtomicTx interface used for hashing).
func (m *manager) Marshal(version uint16, source interface{}) ([]byte, error) {
	if version != CodecVersion {
		return nil, ErrCantPackVersion
	}
	p := &wrappers.Packer{MaxSize: m.maxSize}
	p.PackShort(version)
	if p.Errored() {
		return nil, ErrCantPackVersion
	}
	if err := marshalValue(source, p); err != nil {
		return nil, err
	}
	return p.Bytes, nil
}

// Unmarshal parses bytes into dest and returns the leading version.
func (m *manager) Unmarshal(bytes []byte, dest interface{}) (uint16, error) {
	if len(bytes) < 2 {
		return 0, ErrCantUnpackVersion
	}
	if len(bytes) > m.maxSize {
		return 0, ErrMaxSizeExceeded
	}
	p := &wrappers.Packer{Bytes: bytes, MaxSize: m.maxSize}
	version := p.UnpackShort()
	if p.Errored() {
		return 0, ErrCantUnpackVersion
	}
	if version != CodecVersion {
		return version, fmt.Errorf("%w: %d", ErrUnknownVersion, version)
	}
	if err := unmarshalValue(dest, p); err != nil {
		return version, err
	}
	if p.Offset != len(bytes) {
		return version, ErrExtraSpace
	}
	return version, nil
}

// marshalValue dispatches on the concrete shape of [source] and writes
// its body to [p].
//
// The supported shapes mirror the call sites in this package:
//
//   - *Tx                       — full signed tx (UnsignedAtomicTx + creds)
//   - []*Tx                     — batched signed txs (block extra data)
//   - *[]*Tx                    — pointer form, accepted symmetrically
//   - *UnsignedAtomicTx         — pointer-to-interface, used by Tx.Sign for
//                                 the unsigned-bytes hash
//   - *UnsignedImportTx, *UnsignedExportTx — concrete forms (also used by
//                                 atomictest harnesses)
//   - *lux.UTXO                 — emitted into shared-memory put requests
//                                 from atomic export ops + LUX API
//   - *luxatomic.Requests       — value written into the atomic trie per
//                                 (height, blockchainID)
func marshalValue(source interface{}, p *wrappers.Packer) error {
	switch v := source.(type) {
	case *Tx:
		return marshalTx(v, p)
	case []*Tx:
		return marshalTxs(v, p)
	case *[]*Tx:
		return marshalTxs(*v, p)
	case *UnsignedAtomicTx:
		// Pointer-to-interface: legacy reflectcodec dereferenced the pointer
		// and emitted the interface (typeID + body). We do the same.
		return marshalUnsignedAtomicTxInterface(*v, p)
	case UnsignedAtomicTx:
		return marshalUnsignedAtomicTxInterface(v, p)
	case *UnsignedImportTx:
		// Direct shape — used by atomictest harnesses and the linearcodec
		// type-prefix path is not needed.
		marshalUnsignedImportTxBody(v, p)
	case *UnsignedExportTx:
		marshalUnsignedExportTxBody(v, p)
	case *lux.UTXO:
		marshalUTXO(v, p)
	case *luxatomic.Requests:
		marshalRequests(v, p)
	default:
		return fmt.Errorf("%w: %T", ErrUnsupportedType, source)
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

func unmarshalValue(dest interface{}, p *wrappers.Packer) error {
	switch v := dest.(type) {
	case *Tx:
		return unmarshalTx(v, p)
	case *[]*Tx:
		return unmarshalTxs(v, p)
	case *UnsignedImportTx:
		return unmarshalUnsignedImportTxBody(v, p)
	case *UnsignedExportTx:
		return unmarshalUnsignedExportTxBody(v, p)
	case *lux.UTXO:
		return unmarshalUTXO(v, p)
	case *luxatomic.Requests:
		return unmarshalRequests(v, p)
	default:
		return fmt.Errorf("%w: %T", ErrUnsupportedType, dest)
	}
}

// --- Tx envelope ---------------------------------------------------------

// marshalTx emits: [typeID + unsigned body] [u32 nCreds] [typeID + cred body]*
//
// Layout matches the legacy linearcodec wire shape so on-disk blobs and
// p2p gossip framing remain bit-equivalent.
func marshalTx(tx *Tx, p *wrappers.Packer) error {
	if tx == nil || tx.UnsignedAtomicTx == nil {
		return ErrNilTx
	}
	if err := marshalUnsignedAtomicTxInterface(tx.UnsignedAtomicTx, p); err != nil {
		return err
	}
	p.PackInt(uint32(len(tx.Creds)))
	for _, cred := range tx.Creds {
		if err := marshalCredentialInterface(cred, p); err != nil {
			return err
		}
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

func unmarshalTx(tx *Tx, p *wrappers.Packer) error {
	utx, err := unmarshalUnsignedAtomicTxInterface(p)
	if err != nil {
		return err
	}
	tx.UnsignedAtomicTx = utx
	nCreds := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	tx.Creds = make([]verify.Verifiable, 0, nCreds)
	for i := uint32(0); i < nCreds; i++ {
		cred, err := unmarshalCredentialInterface(p)
		if err != nil {
			return err
		}
		tx.Creds = append(tx.Creds, cred)
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

// marshalTxs / unmarshalTxs handle the block extra-data batch shape.
// The wire shape is: [u32 nTxs] [Tx body]*
func marshalTxs(txs []*Tx, p *wrappers.Packer) error {
	p.PackInt(uint32(len(txs)))
	for _, tx := range txs {
		if err := marshalTx(tx, p); err != nil {
			return err
		}
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

func unmarshalTxs(out *[]*Tx, p *wrappers.Packer) error {
	n := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	txs := make([]*Tx, 0, n)
	for i := uint32(0); i < n; i++ {
		tx := &Tx{}
		if err := unmarshalTx(tx, p); err != nil {
			return err
		}
		txs = append(txs, tx)
	}
	*out = txs
	return nil
}

// --- UnsignedAtomicTx interface dispatch --------------------------------

func marshalUnsignedAtomicTxInterface(utx UnsignedAtomicTx, p *wrappers.Packer) error {
	switch v := utx.(type) {
	case *UnsignedImportTx:
		p.PackInt(typeIDUnsignedImportTx)
		marshalUnsignedImportTxBody(v, p)
	case *UnsignedExportTx:
		p.PackInt(typeIDUnsignedExportTx)
		marshalUnsignedExportTxBody(v, p)
	default:
		return fmt.Errorf("%w: %T", ErrUnsupportedType, utx)
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

func unmarshalUnsignedAtomicTxInterface(p *wrappers.Packer) (UnsignedAtomicTx, error) {
	typeID := p.UnpackInt()
	if p.Errored() {
		return nil, p.Err
	}
	switch typeID {
	case typeIDUnsignedImportTx:
		utx := &UnsignedImportTx{}
		if err := unmarshalUnsignedImportTxBody(utx, p); err != nil {
			return nil, err
		}
		return utx, nil
	case typeIDUnsignedExportTx:
		utx := &UnsignedExportTx{}
		if err := unmarshalUnsignedExportTxBody(utx, p); err != nil {
			return nil, err
		}
		return utx, nil
	default:
		return nil, fmt.Errorf("%w: unsigned-tx type %d", ErrUnknownTypeID, typeID)
	}
}

// --- UnsignedImportTx body ----------------------------------------------
//
// Body layout (Metadata is `serialize:"-"` and skipped):
//   [u32 networkID]
//   [32 blockchainID]
//   [32 sourceChain]
//   [u32 nImportedInputs] (TransferableInput)*
//   [u32 nOuts] (EVMOutput)*

func marshalUnsignedImportTxBody(utx *UnsignedImportTx, p *wrappers.Packer) {
	p.PackInt(utx.NetworkID)
	p.PackFixedBytes(utx.BlockchainID[:])
	p.PackFixedBytes(utx.SourceChain[:])
	p.PackInt(uint32(len(utx.ImportedInputs)))
	for _, in := range utx.ImportedInputs {
		marshalTransferableInput(in, p)
	}
	p.PackInt(uint32(len(utx.Outs)))
	for i := range utx.Outs {
		marshalEVMOutput(&utx.Outs[i], p)
	}
}

func unmarshalUnsignedImportTxBody(utx *UnsignedImportTx, p *wrappers.Packer) error {
	utx.NetworkID = p.UnpackInt()
	copy(utx.BlockchainID[:], p.UnpackFixedBytes(ids.IDLen))
	copy(utx.SourceChain[:], p.UnpackFixedBytes(ids.IDLen))
	nIns := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	utx.ImportedInputs = make([]*lux.TransferableInput, 0, nIns)
	for i := uint32(0); i < nIns; i++ {
		in, err := unmarshalTransferableInput(p)
		if err != nil {
			return err
		}
		utx.ImportedInputs = append(utx.ImportedInputs, in)
	}
	nOuts := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	utx.Outs = make([]EVMOutput, 0, nOuts)
	for i := uint32(0); i < nOuts; i++ {
		var out EVMOutput
		if err := unmarshalEVMOutput(&out, p); err != nil {
			return err
		}
		utx.Outs = append(utx.Outs, out)
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

// --- UnsignedExportTx body ----------------------------------------------
//
// Body layout:
//   [u32 networkID]
//   [32 blockchainID]
//   [32 destinationChain]
//   [u32 nIns] (EVMInput)*
//   [u32 nExportedOutputs] (TransferableOutput)*

func marshalUnsignedExportTxBody(utx *UnsignedExportTx, p *wrappers.Packer) {
	p.PackInt(utx.NetworkID)
	p.PackFixedBytes(utx.BlockchainID[:])
	p.PackFixedBytes(utx.DestinationChain[:])
	p.PackInt(uint32(len(utx.Ins)))
	for i := range utx.Ins {
		marshalEVMInput(&utx.Ins[i], p)
	}
	p.PackInt(uint32(len(utx.ExportedOutputs)))
	for _, out := range utx.ExportedOutputs {
		marshalTransferableOutput(out, p)
	}
}

func unmarshalUnsignedExportTxBody(utx *UnsignedExportTx, p *wrappers.Packer) error {
	utx.NetworkID = p.UnpackInt()
	copy(utx.BlockchainID[:], p.UnpackFixedBytes(ids.IDLen))
	copy(utx.DestinationChain[:], p.UnpackFixedBytes(ids.IDLen))
	nIns := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	utx.Ins = make([]EVMInput, 0, nIns)
	for i := uint32(0); i < nIns; i++ {
		var in EVMInput
		if err := unmarshalEVMInput(&in, p); err != nil {
			return err
		}
		utx.Ins = append(utx.Ins, in)
	}
	nOuts := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	utx.ExportedOutputs = make([]*lux.TransferableOutput, 0, nOuts)
	for i := uint32(0); i < nOuts; i++ {
		out, err := unmarshalTransferableOutput(p)
		if err != nil {
			return err
		}
		utx.ExportedOutputs = append(utx.ExportedOutputs, out)
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

// --- EVMInput / EVMOutput -----------------------------------------------

func marshalEVMOutput(o *EVMOutput, p *wrappers.Packer) {
	p.PackFixedBytes(o.Address.Bytes())
	p.PackLong(o.Amount)
	p.PackFixedBytes(o.AssetID[:])
}

func unmarshalEVMOutput(o *EVMOutput, p *wrappers.Packer) error {
	addr := p.UnpackFixedBytes(20)
	if p.Errored() {
		return p.Err
	}
	copy(o.Address[:], addr)
	o.Amount = p.UnpackLong()
	copy(o.AssetID[:], p.UnpackFixedBytes(ids.IDLen))
	if p.Errored() {
		return p.Err
	}
	return nil
}

func marshalEVMInput(in *EVMInput, p *wrappers.Packer) {
	p.PackFixedBytes(in.Address.Bytes())
	p.PackLong(in.Amount)
	p.PackFixedBytes(in.AssetID[:])
	p.PackLong(in.Nonce)
}

func unmarshalEVMInput(in *EVMInput, p *wrappers.Packer) error {
	addr := p.UnpackFixedBytes(20)
	if p.Errored() {
		return p.Err
	}
	copy(in.Address[:], addr)
	in.Amount = p.UnpackLong()
	copy(in.AssetID[:], p.UnpackFixedBytes(ids.IDLen))
	in.Nonce = p.UnpackLong()
	if p.Errored() {
		return p.Err
	}
	return nil
}

// --- TransferableInput / TransferableOutput -----------------------------
//
// Body layout for TransferableInput:
//   [UTXOID: 32 TxID | u32 OutputIndex]
//   [Asset: 32 ID]
//   [In interface: u32 typeID | body]
//
// Body layout for TransferableOutput:
//   [Asset: 32 ID]
//   [Out interface: u32 typeID | body]

func marshalTransferableInput(in *lux.TransferableInput, p *wrappers.Packer) {
	p.PackFixedBytes(in.UTXOID.TxID[:])
	p.PackInt(in.UTXOID.OutputIndex)
	p.PackFixedBytes(in.Asset.ID[:])
	marshalTransferableInInterface(in.In, p)
}

func unmarshalTransferableInput(p *wrappers.Packer) (*lux.TransferableInput, error) {
	in := &lux.TransferableInput{}
	copy(in.UTXOID.TxID[:], p.UnpackFixedBytes(ids.IDLen))
	in.UTXOID.OutputIndex = p.UnpackInt()
	copy(in.Asset.ID[:], p.UnpackFixedBytes(ids.IDLen))
	if p.Errored() {
		return nil, p.Err
	}
	inner, err := unmarshalTransferableInInterface(p)
	if err != nil {
		return nil, err
	}
	in.In = inner
	return in, nil
}

func marshalTransferableOutput(out *lux.TransferableOutput, p *wrappers.Packer) {
	p.PackFixedBytes(out.Asset.ID[:])
	marshalTransferableOutInterface(out.Out, p)
}

func unmarshalTransferableOutput(p *wrappers.Packer) (*lux.TransferableOutput, error) {
	out := &lux.TransferableOutput{}
	copy(out.Asset.ID[:], p.UnpackFixedBytes(ids.IDLen))
	if p.Errored() {
		return nil, p.Err
	}
	inner, err := unmarshalTransferableOutInterface(p)
	if err != nil {
		return nil, err
	}
	out.Out = inner
	return out, nil
}

// --- TransferableIn / TransferableOut interface dispatch ----------------

func marshalTransferableInInterface(in lux.TransferableIn, p *wrappers.Packer) {
	switch v := in.(type) {
	case *secp256k1fx.TransferInput:
		p.PackInt(typeIDTransferInput)
		marshalSecp256k1TransferInput(v, p)
	default:
		p.Err = fmt.Errorf("%w: TransferableIn %T", ErrUnsupportedType, in)
	}
}

func unmarshalTransferableInInterface(p *wrappers.Packer) (lux.TransferableIn, error) {
	typeID := p.UnpackInt()
	if p.Errored() {
		return nil, p.Err
	}
	switch typeID {
	case typeIDTransferInput:
		v := &secp256k1fx.TransferInput{}
		if err := unmarshalSecp256k1TransferInput(v, p); err != nil {
			return nil, err
		}
		return v, nil
	default:
		return nil, fmt.Errorf("%w: TransferableIn typeID %d", ErrUnknownTypeID, typeID)
	}
}

func marshalTransferableOutInterface(out lux.TransferableOut, p *wrappers.Packer) {
	switch v := out.(type) {
	case *secp256k1fx.TransferOutput:
		p.PackInt(typeIDTransferOutput)
		marshalSecp256k1TransferOutput(v, p)
	default:
		p.Err = fmt.Errorf("%w: TransferableOut %T", ErrUnsupportedType, out)
	}
}

func unmarshalTransferableOutInterface(p *wrappers.Packer) (lux.TransferableOut, error) {
	typeID := p.UnpackInt()
	if p.Errored() {
		return nil, p.Err
	}
	switch typeID {
	case typeIDTransferOutput:
		v := &secp256k1fx.TransferOutput{}
		if err := unmarshalSecp256k1TransferOutput(v, p); err != nil {
			return nil, err
		}
		return v, nil
	default:
		return nil, fmt.Errorf("%w: TransferableOut typeID %d", ErrUnknownTypeID, typeID)
	}
}

// --- secp256k1fx.TransferInput ------------------------------------------
//
// Body layout (Amt + embedded Input):
//   [u64 Amt]
//   [u32 nSigIndices] [u32]*

func marshalSecp256k1TransferInput(in *secp256k1fx.TransferInput, p *wrappers.Packer) {
	p.PackLong(in.Amt)
	p.PackInt(uint32(len(in.SigIndices)))
	for _, idx := range in.SigIndices {
		p.PackInt(idx)
	}
}

func unmarshalSecp256k1TransferInput(in *secp256k1fx.TransferInput, p *wrappers.Packer) error {
	in.Amt = p.UnpackLong()
	n := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	in.SigIndices = make([]uint32, 0, n)
	for i := uint32(0); i < n; i++ {
		in.SigIndices = append(in.SigIndices, p.UnpackInt())
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

// --- secp256k1fx.TransferOutput -----------------------------------------
//
// Body layout (Amt + embedded OutputOwners):
//   [u64 Amt]
//   [u64 Locktime]
//   [u32 Threshold]
//   [u32 nAddrs] [20]*

func marshalSecp256k1TransferOutput(out *secp256k1fx.TransferOutput, p *wrappers.Packer) {
	p.PackLong(out.Amt)
	p.PackLong(out.OutputOwners.Locktime)
	p.PackInt(out.OutputOwners.Threshold)
	p.PackInt(uint32(len(out.OutputOwners.Addrs)))
	for _, a := range out.OutputOwners.Addrs {
		p.PackFixedBytes(a[:])
	}
}

func unmarshalSecp256k1TransferOutput(out *secp256k1fx.TransferOutput, p *wrappers.Packer) error {
	out.Amt = p.UnpackLong()
	out.OutputOwners.Locktime = p.UnpackLong()
	out.OutputOwners.Threshold = p.UnpackInt()
	n := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	out.OutputOwners.Addrs = make([]ids.ShortID, 0, n)
	for i := uint32(0); i < n; i++ {
		var sid ids.ShortID
		copy(sid[:], p.UnpackFixedBytes(ids.ShortIDLen))
		out.OutputOwners.Addrs = append(out.OutputOwners.Addrs, sid)
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

// --- verify.Verifiable credential dispatch ------------------------------
//
// secp256k1fx.Credential body:
//   [u32 nSigs] [65 sig]*

func marshalCredentialInterface(cred verify.Verifiable, p *wrappers.Packer) error {
	switch v := cred.(type) {
	case *secp256k1fx.Credential:
		p.PackInt(typeIDSecp256k1Credential)
		p.PackInt(uint32(len(v.Sigs)))
		for i := range v.Sigs {
			p.PackFixedBytes(v.Sigs[i][:])
		}
	default:
		return fmt.Errorf("%w: credential %T", ErrUnsupportedType, cred)
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

func unmarshalCredentialInterface(p *wrappers.Packer) (verify.Verifiable, error) {
	typeID := p.UnpackInt()
	if p.Errored() {
		return nil, p.Err
	}
	switch typeID {
	case typeIDSecp256k1Credential:
		nSigs := p.UnpackInt()
		if p.Errored() {
			return nil, p.Err
		}
		cred := &secp256k1fx.Credential{
			Sigs: make([][secp256k1.SignatureLen]byte, nSigs),
		}
		for i := uint32(0); i < nSigs; i++ {
			copy(cred.Sigs[i][:], p.UnpackFixedBytes(secp256k1.SignatureLen))
		}
		if p.Errored() {
			return nil, p.Err
		}
		return cred, nil
	default:
		return nil, fmt.Errorf("%w: credential typeID %d", ErrUnknownTypeID, typeID)
	}
}

// --- lux.UTXO -----------------------------------------------------------
//
// Body layout:
//   [UTXOID: 32 TxID | u32 OutputIndex]
//   [Asset:  32 ID]
//   [Out interface: u32 typeID | body]

func marshalUTXO(u *lux.UTXO, p *wrappers.Packer) {
	p.PackFixedBytes(u.UTXOID.TxID[:])
	p.PackInt(u.UTXOID.OutputIndex)
	p.PackFixedBytes(u.Asset.ID[:])
	// UTXO.Out is verify.State (not lux.TransferableOut), but the only
	// concrete reachable here is *secp256k1fx.TransferOutput which
	// satisfies both. Use a dedicated dispatcher so the type set is
	// exhaustive and explicit.
	switch v := u.Out.(type) {
	case *secp256k1fx.TransferOutput:
		p.PackInt(typeIDTransferOutput)
		marshalSecp256k1TransferOutput(v, p)
	default:
		p.Err = fmt.Errorf("%w: UTXO.Out %T", ErrUnsupportedType, u.Out)
	}
}

func unmarshalUTXO(u *lux.UTXO, p *wrappers.Packer) error {
	copy(u.UTXOID.TxID[:], p.UnpackFixedBytes(ids.IDLen))
	u.UTXOID.OutputIndex = p.UnpackInt()
	copy(u.Asset.ID[:], p.UnpackFixedBytes(ids.IDLen))
	if p.Errored() {
		return p.Err
	}
	typeID := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	switch typeID {
	case typeIDTransferOutput:
		out := &secp256k1fx.TransferOutput{}
		if err := unmarshalSecp256k1TransferOutput(out, p); err != nil {
			return err
		}
		u.Out = out
		return nil
	default:
		return fmt.Errorf("%w: UTXO.Out typeID %d", ErrUnknownTypeID, typeID)
	}
}

// --- luxatomic.Requests -------------------------------------------------
//
// Body layout:
//   [u32 nRemoveRequests] ([u32 len] [bytes])*
//   [u32 nPutRequests]    (Element)*
//
// Element:
//   [u32 keyLen][key bytes]
//   [u32 valLen][val bytes]
//   [u32 nTraits] ([u32 len][trait bytes])*

func marshalRequests(r *luxatomic.Requests, p *wrappers.Packer) {
	p.PackInt(uint32(len(r.RemoveRequests)))
	for _, rr := range r.RemoveRequests {
		p.PackBytes(rr)
	}
	p.PackInt(uint32(len(r.PutRequests)))
	for _, el := range r.PutRequests {
		marshalAtomicElement(el, p)
	}
}

func unmarshalRequests(r *luxatomic.Requests, p *wrappers.Packer) error {
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
		el := &luxatomic.Element{}
		if err := unmarshalAtomicElement(el, p); err != nil {
			return err
		}
		r.PutRequests = append(r.PutRequests, el)
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

func marshalAtomicElement(el *luxatomic.Element, p *wrappers.Packer) {
	p.PackBytes(el.Key)
	p.PackBytes(el.Value)
	p.PackInt(uint32(len(el.Traits)))
	for _, t := range el.Traits {
		p.PackBytes(t)
	}
}

func unmarshalAtomicElement(el *luxatomic.Element, p *wrappers.Packer) error {
	el.Key = p.UnpackBytes()
	el.Value = p.UnpackBytes()
	nTraits := p.UnpackInt()
	if p.Errored() {
		return p.Err
	}
	el.Traits = make([][]byte, 0, nTraits)
	for i := uint32(0); i < nTraits; i++ {
		el.Traits = append(el.Traits, p.UnpackBytes())
	}
	if p.Errored() {
		return p.Err
	}
	return nil
}

// --- ExtractAtomicTxs --------------------------------------------------

// ExtractAtomicTxs returns the atomic transactions in [atomicTxBytes] if
// they exist. If [batch] is true, [atomicTxBytes] is decoded as a batch;
// otherwise as a single atomic tx.
func ExtractAtomicTxs(atomicTxBytes []byte, batch bool, c Manager) ([]*Tx, error) {
	if len(atomicTxBytes) == 0 {
		return nil, nil
	}
	if !batch {
		tx, err := ExtractAtomicTx(atomicTxBytes, c)
		if err != nil {
			return nil, err
		}
		return []*Tx{tx}, nil
	}
	return ExtractAtomicTxsBatch(atomicTxBytes, c)
}

// ExtractAtomicTx extracts a singular atomic transaction from
// [atomicTxBytes]. Note: this function assumes [atomicTxBytes] is non-empty.
//
// The post-unmarshal step re-marshals via [c] to materialize the unsigned
// and signed byte forms on the Tx — these are stashed in Metadata and
// later consumed by ID(), Bytes(), SignedBytes(). We deliberately do not
// call Tx.Sign here because Sign is bound to the production [Codec] and
// would refuse non-production tx types (e.g. the atomictest harness type).
func ExtractAtomicTx(atomicTxBytes []byte, c Manager) (*Tx, error) {
	atomicTx := new(Tx)
	if _, err := c.Unmarshal(atomicTxBytes, atomicTx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal atomic transaction: %w", err)
	}
	if err := atomicTx.initializeBytes(c); err != nil {
		return nil, fmt.Errorf("failed to initialize singleton atomic tx: %w", err)
	}
	return atomicTx, nil
}

// ExtractAtomicTxsBatch extracts a slice of atomic transactions from
// [atomicTxBytes]. Note: this function assumes [atomicTxBytes] is non-empty.
func ExtractAtomicTxsBatch(atomicTxBytes []byte, c Manager) ([]*Tx, error) {
	var atomicTxs []*Tx
	if _, err := c.Unmarshal(atomicTxBytes, &atomicTxs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal atomic tx batch: %w", err)
	}
	// Do not allow non-empty extra data field to contain zero atomic
	// transactions. This would allow people to construct a block that
	// contains useless data.
	if len(atomicTxs) == 0 {
		return nil, ErrMissingAtomicTxs
	}
	for index, atx := range atomicTxs {
		if err := atx.initializeBytes(c); err != nil {
			return nil, fmt.Errorf("failed to initialize atomic tx at index %d: %w", index, err)
		}
	}
	return atomicTxs, nil
}
