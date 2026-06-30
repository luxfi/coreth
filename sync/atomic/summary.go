// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomic

import (
	"context"
	"errors"
	"fmt"

	"github.com/luxfi/coreth/plugin/evm/message"
	"github.com/luxfi/ids"

	"github.com/luxfi/consensus/engine/chain/block"

	"github.com/luxfi/crypto"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/utils/wrappers"
)

var (
	_ message.Syncable = (*Summary)(nil)

	errSummaryShort = errors.New("syncable atomic summary truncated")
)

// Summary provides the information necessary to sync a node starting at
// the given block. The wire shape mirrors message.BlockSyncSummary with
// an AtomicRoot suffix:
//
//	[u16 version=0]
//	[u64 BlockNumber]
//	[32 BlockHash]
//	[32 BlockRoot]
//	[32 AtomicRoot]
//
// The summary wire format is owned here (not by plugin/evm/message)
// because plugin/evm/message must not depend on sync/atomic. Marshal /
// Unmarshal are local; the message codec is only used to spell the
// version constant via message.Version.
type Summary struct {
	*message.BlockSyncSummary `serialize:"true"`
	AtomicRoot                common.Hash `serialize:"true"`

	summaryID  ids.ID
	bytes      []byte
	acceptImpl message.AcceptImplFn
}

func NewSummary(blockHash common.Hash, blockNumber uint64, blockRoot common.Hash, atomicRoot common.Hash) (*Summary, error) {
	summary := Summary{
		BlockSyncSummary: &message.BlockSyncSummary{
			BlockNumber: blockNumber,
			BlockHash:   blockHash,
			BlockRoot:   blockRoot,
		},
		AtomicRoot: atomicRoot,
	}
	bytes, err := summary.marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal syncable summary: %w", err)
	}
	summary.bytes = bytes
	summaryID, err := ids.ToID(crypto.Keccak256(bytes))
	if err != nil {
		return nil, fmt.Errorf("failed to compute summary ID: %w", err)
	}
	summary.summaryID = summaryID
	return &summary, nil
}

func (a *Summary) Bytes() []byte {
	return a.bytes
}

func (a *Summary) ID() ids.ID {
	return a.summaryID
}

func (a *Summary) String() string {
	return fmt.Sprintf("Summary(BlockHash=%s, BlockNumber=%d, BlockRoot=%s, AtomicRoot=%s)", a.BlockHash, a.BlockNumber, a.BlockRoot, a.AtomicRoot)
}

func (a *Summary) Accept(context.Context) (block.StateSyncMode, error) {
	if a.acceptImpl == nil {
		return block.StateSyncSkipped, fmt.Errorf("accept implementation not specified for summary: %s", a)
	}
	return a.acceptImpl(a)
}

// marshal emits the [u16 version][BlockSyncSummary body][AtomicRoot]
// shape. Big-endian, byte-equal to legacy reflectcodec.
func (a *Summary) marshal() ([]byte, error) {
	const summarySize = 2 + 8 + 32 + 32 + 32
	p := &wrappers.Packer{MaxSize: summarySize, Bytes: make([]byte, 0, summarySize)}
	p.PackShort(message.Version)
	p.PackLong(a.BlockNumber)
	p.PackFixedBytes(a.BlockHash[:])
	p.PackFixedBytes(a.BlockRoot[:])
	p.PackFixedBytes(a.AtomicRoot[:])
	if p.Errored() {
		return nil, p.Err
	}
	return p.Bytes, nil
}

// unmarshalSummary parses a wire blob in the format produced by
// [Summary.marshal] and populates [a]. Returns ErrUnknownVersion if the
// leading version is anything other than [message.Version].
func unmarshalSummary(blob []byte, a *Summary) error {
	const summarySize = 2 + 8 + 32 + 32 + 32
	if len(blob) != summarySize {
		return fmt.Errorf("%w: got %d bytes, want %d", errSummaryShort, len(blob), summarySize)
	}
	p := &wrappers.Packer{Bytes: blob, MaxSize: summarySize}
	v := p.UnpackShort()
	if v != message.Version {
		return fmt.Errorf("%w: %d", message.ErrUnknownVersion, v)
	}
	a.BlockSyncSummary = &message.BlockSyncSummary{}
	a.BlockNumber = p.UnpackLong()
	copy(a.BlockHash[:], p.UnpackFixedBytes(common.HashLength))
	copy(a.BlockRoot[:], p.UnpackFixedBytes(common.HashLength))
	copy(a.AtomicRoot[:], p.UnpackFixedBytes(common.HashLength))
	if p.Errored() {
		return p.Err
	}
	return nil
}
