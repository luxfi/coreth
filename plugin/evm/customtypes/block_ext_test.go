// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customtypes

import (
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"
	"unsafe"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/luxfi/geth/common"
	ethtypes "github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newBlockTest wraps ethtypes.NewBlock with the new 4-arg signature.
// Named differently to avoid collision with any dot-imported NewBlock.
func newBlockTest(header *ethtypes.Header, body *ethtypes.Body, receipts []*ethtypes.Receipt, hasher ethtypes.ListHasher) *ethtypes.Block {
	return ethtypes.NewBlock(header, body, receipts, hasher)
}

func TestCopyHeader(t *testing.T) {
	t.Parallel()

	t.Run("empty_header", func(t *testing.T) {
		t.Parallel()

		empty := &ethtypes.Header{}

		headerExtra := &HeaderExtra{}
		extras.Header.Set(empty, headerExtra)

		cpy := ethtypes.CopyHeader(empty)

		want := &ethtypes.Header{
			Difficulty: new(big.Int),
			Number:     new(big.Int),
		}

		headerExtra = &HeaderExtra{}
		extras.Header.Set(want, headerExtra)

		assert.Equal(t, want, cpy)
	})

	t.Run("filled_header", func(t *testing.T) {
		t.Parallel()

		header, _ := headerWithNonZeroFields() // the header carries the [HeaderExtra] so we can ignore it

		gotHeader := ethtypes.CopyHeader(header)
		gotExtra := GetHeaderExtra(gotHeader)

		wantHeader, wantExtra := headerWithNonZeroFields()
		assert.Equal(t, wantHeader, gotHeader)
		assert.Equal(t, wantExtra, gotExtra)

		exportedFieldsPointToDifferentMemory(t, header, gotHeader)
		exportedFieldsPointToDifferentMemory(t, GetHeaderExtra(header), gotExtra)
	})
}

func exportedFieldsPointToDifferentMemory[T interface {
	ethtypes.Header | HeaderExtra | BlockBodyExtra
}](t *testing.T, original, cpy *T) {
	t.Helper()

	v := reflect.ValueOf(*original)
	typ := v.Type()
	cp := reflect.ValueOf(*cpy)
	for i := range v.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		switch field.Type.Kind() {
		case reflect.Array, reflect.Uint64, reflect.Uint32:
			// Not pointers, but using explicit Kinds for safety
			continue
		}

		t.Run(field.Name, func(t *testing.T) {
			fieldCp := cp.Field(i).Interface()
			switch f := v.Field(i).Interface().(type) {
			case *big.Int:
				assertDifferentPointers(t, f, fieldCp)
			case *common.Hash:
				assertDifferentPointers(t, f, fieldCp)
			case *uint64:
				assertDifferentPointers(t, f, fieldCp)
			case *[]uint8:
				assertDifferentPointers(t, f, fieldCp)
			case []uint8:
				assertDifferentPointers(t, unsafe.SliceData(f), unsafe.SliceData(fieldCp.([]uint8)))
			default:
				t.Errorf("field %q type %T needs to be added to switch cases of exportedFieldsDeepCopied", field.Name, f)
			}
		})
	}
}

// assertDifferentPointers asserts that `a` and `b` are both non-nil
// pointers pointing to different memory locations.
func assertDifferentPointers[T any](t *testing.T, a *T, b any) {
	t.Helper()
	switch {
	case a == nil:
		t.Errorf("a (%T) cannot be nil", a)
	case b == nil:
		t.Errorf("b (%T) cannot be nil", b)
	case a == b:
		t.Errorf("pointers to same memory")
	}
	// Note: no need to check `b` is of the same type as `a`, otherwise
	// the memory address would be different as well.
}

// blockWithNonZeroFields returns a [Block] and a [BlockBodyExtra],
// each with all fields set to non-zero values.
// The [BlockBodyExtra] extra payload is set in the [Block] via `extras.Block.Set`.
//
// NOTE: They can be used to demonstrate that RLP round-trip encoding
// can recover all fields, but not that the encoded format is correct. This is
// very important as the RLP encoding of a [Block] defines its hash.
func blockWithNonZeroFields() (*ethtypes.Block, *BlockBodyExtra) {
	header := WithHeaderExtra(
		&ethtypes.Header{
			ParentHash: common.Hash{1},
		},
		&HeaderExtra{
			ExtDataHash: common.Hash{2},
		},
	)

	tx := ethtypes.NewTransaction(1, common.Address{2}, big.NewInt(3), 4, big.NewInt(5), []byte{6})
	txs := []*ethtypes.Transaction{tx}

	uncle := WithHeaderExtra(
		&ethtypes.Header{
			Difficulty: big.NewInt(7),
			Number:     big.NewInt(8),
			ParentHash: common.Hash{9},
		},
		&HeaderExtra{
			ExtDataHash: common.Hash{10},
		},
	)
	uncles := []*ethtypes.Header{uncle}

	receipts := []*ethtypes.Receipt{{PostState: []byte{11}}}
	withdrawals := []*ethtypes.Withdrawal{{Index: 12}}

	body := &ethtypes.Body{
		Transactions: txs,
		Uncles:       uncles,
		Withdrawals:  withdrawals,
	}
	block := newBlockTest(header, body, receipts, stubHasher{})
	extra := &BlockBodyExtra{
		Version: 13,
		ExtData: &[]byte{14},
	}
	SetBlockExtra(block, extra)
	return block, extra
}

func TestBlockWithNonZeroFields(t *testing.T) {
	t.Parallel()

	block, extra := blockWithNonZeroFields()
	t.Run("Block", func(t *testing.T) {
		ignoreFields := []string{"extra", "hash", "size", "ReceivedAt", "ReceivedFrom"}
		allFieldsSet(t, block, ignoreFields...)
	})
	t.Run("BlockExtra", func(t *testing.T) { allFieldsSet(t, extra) })
}

// bodyWithNonZeroFields returns a [Body] and a [BlockBodyExtra],
// each with all fields set to non-zero values.
// The [BlockBodyExtra] extra payload is set in the [Body] via `extras.Block.Set`
// and the extra copying done in `Block.Body()`.
//
// NOTE: They can be used to demonstrate that RLP round-trip encoding
// can recover all fields, but not that the encoded format is correct. This is
// very important as the RLP encoding of a [Body] defines its hash.
func bodyWithNonZeroFields() (*ethtypes.Body, *BlockBodyExtra) {
	block, extra := blockWithNonZeroFields()
	return block.Body(), extra
}

func TestBodyWithNonZeroFields(t *testing.T) {
	t.Parallel()

	body, extra := bodyWithNonZeroFields()
	t.Run("Body", func(t *testing.T) {
		ignoredFields := []string{"extra"}
		allFieldsSet(t, body, ignoredFields...)
	})
	t.Run("BodyExtra", func(t *testing.T) { allFieldsSet(t, extra) })
}

func txHashComparer() cmp.Option {
	return cmp.Comparer(func(a, b *ethtypes.Transaction) bool {
		return a.Hash() == b.Hash()
	})
}

func headerHashComparer() cmp.Option {
	return cmp.Comparer(func(a, b *ethtypes.Header) bool {
		return a.Hash() == b.Hash()
	})
}

func TestBodyExtraRLP(t *testing.T) {
	t.Parallel()

	body, _ := bodyWithNonZeroFields() // the body carries the [BlockBodyExtra] so we can ignore it

	encoded, err := rlp.EncodeToBytes(body)
	require.NoError(t, err)

	gotBody := new(ethtypes.Body)
	require.NoError(t, rlp.DecodeBytes(encoded, gotBody))

	wantBody, wantExtra := bodyWithNonZeroFields()
	wantBody.Withdrawals = nil

	opts := cmp.Options{
		txHashComparer(),
		headerHashComparer(),
		cmpopts.IgnoreUnexported(ethtypes.Body{}),
	}
	if diff := cmp.Diff(wantBody, gotBody, opts); diff != "" {
		t.Errorf("%T diff after RLP round-trip (-want +got):\n%s", wantBody, diff)
	}

	gotExtra := extras.Body.Get(gotBody)
	if diff := cmp.Diff(wantExtra, gotExtra); diff != "" {
		t.Errorf("%T diff after RLP round-trip of %T (-want +got):\n%s", wantExtra, wantBody, diff)
	}

	// Golden data from original coreth implementation, before integration of
	// geth. WARNING: changing these values can break backwards compatibility
	// with extreme consequences.
	const wantHex = "f90235dedd0105049402000000000000000000000000000000000000000306808080f90211f9020ea00900000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000940000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000070880808080a00000000000000000000000000000000000000000000000000000000000000000880000000000000000a00a000000000000000000000000000000000000000000000000000000000000000d0e"

	assert.Equal(t, wantHex, hex.EncodeToString(encoded), "golden data")
}

func TestBlockExtraRLP(t *testing.T) {
	t.Parallel()

	block, _ := blockWithNonZeroFields() // the block carries the [BlockBodyExtra] so we can ignore it

	encoded, err := rlp.EncodeToBytes(block)
	require.NoError(t, err)

	gotBlock := new(ethtypes.Block)
	require.NoError(t, rlp.DecodeBytes(encoded, gotBlock))

	// Create expected block without withdrawals for comparison
	// (withdrawals are not encoded in the standard RLP format)
	wantBlock, wantExtra := blockWithNonZeroFields()
	_ = wantBlock // block comparison uses hash comparer

	opts := cmp.Options{
		txHashComparer(),
		headerHashComparer(),
		cmpopts.IgnoreUnexported(ethtypes.Block{}),
	}
	if diff := cmp.Diff(wantBlock, gotBlock, opts); diff != "" {
		t.Errorf("%T diff after RLP round-trip (-want +got):\n%s", gotBlock, diff)
	}

	gotExtra := extras.Block.Get(gotBlock)
	if diff := cmp.Diff(wantExtra, gotExtra); diff != "" {
		t.Errorf("%T diff after RLP round-trip of %T (-want +got):\n%s", wantExtra, wantBlock, diff)
	}

	// Golden data from original coreth implementation, before integration of
	// geth. WARNING: changing these values can break backwards compatibility
	// with extreme consequences.
	const wantHex = "f90446f9020ea00100000000000000000000000000000000000000000000000000000000000000a008539331084089cedbaf7771d0f5f69847f246e0676e4d96091a49c53c89360b940000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000808080808080a00000000000000000000000000000000000000000000000000000000000000000880000000000000000a00200000000000000000000000000000000000000000000000000000000000000dedd0105049402000000000000000000000000000000000000000306808080f90211f9020ea00900000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000940000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000070880808080a00000000000000000000000000000000000000000000000000000000000000000880000000000000000a00a000000000000000000000000000000000000000000000000000000000000000d0e"

	assert.Equal(t, wantHex, hex.EncodeToString(encoded), "golden data")
}

// TestBlockBody tests the [BlockBodyExtra.Copy] method is implemented correctly.
func TestBlockBody(t *testing.T) {
	t.Parallel()

	const version = 1
	extData := &[]byte{2}

	blockExtras := &BlockBodyExtra{
		Version: version,
		ExtData: extData,
	}
	allFieldsSet(t, blockExtras) // make sure each field is checked
	block := newBlockTest(&ethtypes.Header{}, nil, nil, stubHasher{})
	extras.Block.Set(block, blockExtras)

	wantExtra := &BlockBodyExtra{
		Version: version,
		ExtData: extData,
	}
	gotExtra := extras.Body.Get(block.Body()) // [types.Block.Body] invokes [BlockBodyExtra.Copy]
	assert.Equal(t, wantExtra, gotExtra)

	exportedFieldsPointToDifferentMemory(t, blockExtras, gotExtra)
}

func TestBlockGetters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		headerExtra        *HeaderExtra
		blockExtra         *BlockBodyExtra
		wantExtDataGasUsed *big.Int
		wantBlockGasCost   *big.Int
		wantVersion        uint32
		wantExtData        []byte
	}{
		{
			name:        "empty",
			headerExtra: &HeaderExtra{},
			blockExtra:  &BlockBodyExtra{},
		},
		{
			name: "fields_set",
			headerExtra: &HeaderExtra{
				ExtDataGasUsed: big.NewInt(1),
				BlockGasCost:   big.NewInt(2),
			},
			blockExtra: &BlockBodyExtra{
				Version: 3,
				ExtData: &[]byte{4},
			},
			wantExtDataGasUsed: big.NewInt(1),
			wantBlockGasCost:   big.NewInt(2),
			wantVersion:        3,
			wantExtData:        []byte{4},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			header := WithHeaderExtra(&ethtypes.Header{}, test.headerExtra)

			block := newBlockTest(header, nil, nil, stubHasher{})
			extras.Block.Set(block, test.blockExtra)

			extData := BlockExtData(block)
			assert.Equal(t, test.wantExtData, extData, "BlockExtData()")

			version := GetBlockVersion(block)
			assert.Equal(t, test.wantVersion, version, "GetBlockVersion()")

			extDataGasUsed := BlockExtDataGasUsed(block)
			assert.Equal(t, test.wantExtDataGasUsed, extDataGasUsed, "BlockExtDataGasUsed()")

			blockGasCost := BlockGasCost(block)
			assert.Equal(t, test.wantBlockGasCost, blockGasCost, "BlockGasCost()")
		})
	}
}

func TestNewBlockWithExtData(t *testing.T) {
	t.Parallel()

	// This transaction is generated beforehand because of its unexported time field being set
	// on creation.
	testTx := ethtypes.NewTransaction(0, common.Address{1}, big.NewInt(2), 3, big.NewInt(4), []byte{5})

	tests := []struct {
		name      string
		header    *ethtypes.Header
		txs       []*ethtypes.Transaction
		uncles    []*ethtypes.Header
		receipts  []*ethtypes.Receipt
		extdata   []byte
		recalc    bool
		wantBlock func() *ethtypes.Block
	}{
		{
			name:   "empty",
			header: WithHeaderExtra(&ethtypes.Header{}, &HeaderExtra{}),
			wantBlock: func() *ethtypes.Block {
				header := WithHeaderExtra(&ethtypes.Header{}, &HeaderExtra{})
				block := newBlockTest(header, nil, nil, stubHasher{})
				blockExtra := &BlockBodyExtra{ExtData: &[]byte{}}
				extras.Block.Set(block, blockExtra)
				return block
			},
		},
		{
			name:   "header_nil_extra",
			header: &ethtypes.Header{},
			wantBlock: func() *ethtypes.Block {
				header := WithHeaderExtra(&ethtypes.Header{}, &HeaderExtra{})
				block := newBlockTest(header, nil, nil, stubHasher{})
				blockExtra := &BlockBodyExtra{ExtData: &[]byte{}}
				extras.Block.Set(block, blockExtra)
				return block
			},
		},
		{
			name: "with_recalc",
			header: WithHeaderExtra(
				&ethtypes.Header{},
				&HeaderExtra{
					ExtDataHash: common.Hash{1}, // should be overwritten
				},
			),
			extdata: []byte{2},
			recalc:  true,
			wantBlock: func() *ethtypes.Block {
				header := WithHeaderExtra(
					&ethtypes.Header{},
					&HeaderExtra{ExtDataHash: CalcExtDataHash([]byte{2})},
				)
				block := newBlockTest(header, nil, nil, stubHasher{})
				blockExtra := &BlockBodyExtra{ExtData: &[]byte{2}}
				extras.Block.Set(block, blockExtra)
				return block
			},
		},
		{
			name: "filled_no_recalc",
			header: WithHeaderExtra(
				&ethtypes.Header{GasLimit: 1},
				&HeaderExtra{
					ExtDataHash:    common.Hash{2},
					ExtDataGasUsed: big.NewInt(3),
					BlockGasCost:   big.NewInt(4),
				},
			),
			txs: []*ethtypes.Transaction{testTx},
			uncles: []*ethtypes.Header{
				WithHeaderExtra(
					&ethtypes.Header{GasLimit: 5},
					&HeaderExtra{BlockGasCost: big.NewInt(6)},
				),
			},
			receipts: []*ethtypes.Receipt{{PostState: []byte{7}}},
			extdata:  []byte{8},
			wantBlock: func() *ethtypes.Block {
				header := WithHeaderExtra(
					&ethtypes.Header{GasLimit: 1},
					&HeaderExtra{
						ExtDataHash:    common.Hash{2},
						ExtDataGasUsed: big.NewInt(3),
						BlockGasCost:   big.NewInt(4),
					},
				)
				uncle := WithHeaderExtra(
					&ethtypes.Header{GasLimit: 5},
					&HeaderExtra{BlockGasCost: big.NewInt(6)},
				)
				uncles := []*ethtypes.Header{uncle}
				body := &ethtypes.Body{
					Transactions: []*ethtypes.Transaction{testTx},
					Uncles:       uncles,
				}
				block := newBlockTest(header, body, []*ethtypes.Receipt{{PostState: []byte{7}}}, stubHasher{})
				blockExtra := &BlockBodyExtra{ExtData: &[]byte{8}}
				extras.Block.Set(block, blockExtra)
				return block
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			block := NewBlockWithExtData(
				test.header,
				test.txs,
				test.uncles,
				test.receipts,
				stubHasher{},
				test.extdata,
				test.recalc,
			)

			assert.Equal(t, test.wantBlock(), block)
		})
	}
}

type stubHasher struct{}

func (h stubHasher) Reset()                         {}
func (h stubHasher) Update(key, value []byte) error { return nil }
func (h stubHasher) Hash() common.Hash              { return common.Hash{} }
