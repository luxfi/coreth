// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"context"
	"fmt"

	"github.com/luxfi/database"
	"github.com/luxfi/ids"
	"github.com/luxfi/p2p"
	"github.com/luxfi/warp"
	"github.com/luxfi/warp/payload"
)

const (
	ParseErrCode = iota + 1
	VerifyErrCode
)

// Verify verifies the signature of the message
// It also implements the lp118.Verifier interface
func (b *backend) Verify(ctx context.Context, unsignedMessage *warp.UnsignedMessage, _ []byte) error {
	messageID := unsignedMessage.ID()
	// Known on-chain messages should be signed
	if _, err := b.GetMessage(messageID); err == nil {
		return nil
	} else if err != database.ErrNotFound {
		return &p2p.Error{
			Code:    ParseErrCode,
			Message: fmt.Sprintf("failed to get message %s: %s", messageID, err.Error()),
		}
	}

	parsed, err := payload.ParsePayload(unsignedMessage.Payload)
	if err != nil {
		b.stats.IncMessageParseFail()
		return &p2p.Error{
			Code:    ParseErrCode,
			Message: "failed to parse payload: " + err.Error(),
		}
	}

	switch p := parsed.(type) {
	case *payload.Hash:
		return b.verifyBlockMessage(ctx, p)
	default:
		b.stats.IncMessageParseFail()
		return &p2p.Error{
			Code:    ParseErrCode,
			Message: fmt.Sprintf("unknown payload type: %T", p),
		}
	}
}

// verifyBlockMessage returns nil if blockHashPayload contains the ID
// of an accepted block indicating it should be signed by the VM.
func (b *backend) verifyBlockMessage(ctx context.Context, blockHashPayload *payload.Hash) error {
	blockID, err := ids.ToID(blockHashPayload.Hash)
	if err != nil {
		b.stats.IncBlockValidationFail()
		return &p2p.Error{
			Code:    ParseErrCode,
			Message: fmt.Sprintf("failed to parse block ID: %s", err.Error()),
		}
	}
	_, err = b.blockClient.GetAcceptedBlock(ctx, blockID)
	if err != nil {
		b.stats.IncBlockValidationFail()
		return &p2p.Error{
			Code:    VerifyErrCode,
			Message: fmt.Sprintf("failed to get block %s: %s", blockID, err.Error()),
		}
	}

	return nil
}
