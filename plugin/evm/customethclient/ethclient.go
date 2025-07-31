// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package customethclient

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/luxfi/geth/common/hexutil"
	"github.com/luxfi/geth/core/types"

	"github.com/luxfi/coreth/ethclient"
	"github.com/luxfi/coreth/plugin/evm/customtypes"
	"github.com/luxfi/coreth/rpc"
)

var _ ethclient.BlockHook = (*extBlockHook)(nil)

// Client wraps the ethclient.Client interface to provide extra data types (in header, block body).
// If you want to use the standardized Ethereum RPC functionality without extra types, use [ethclient.Client] instead.
type Client struct {
	*ethclient.Client
}

// New creates a client that uses the given RPC client.
func New(c *rpc.Client) *Client {
	return &Client{
		Client: ethclient.NewClientWithHook(c, &extBlockHook{}),
	}
}

// Dial connects a client to the given URL.
func Dial(rawurl string) (*Client, error) {
	return DialContext(context.Background(), rawurl)
}

// DialContext connects a client to the given URL with context.
func DialContext(ctx context.Context, rawurl string) (*Client, error) {
	c, err := rpc.DialContext(ctx, rawurl)
	if err != nil {
		return nil, err
	}
	return New(c), nil
}

// extBlockHook is a hook that is called when a block is decoded.
type extBlockHook struct{}

// extRpcBlock is the structure of the block extra data in the RPC response.
// It contains the version and the block extra data.
type extRpcBlock struct {
	Version        uint32         `json:"version"`
	BlockExtraData *hexutil.Bytes `json:"blockExtraData"`
}

// OnBlockDecoded is called when a block is decoded. It unmarshals the
// extra data from the RPC response and sets it in the block with geth extras.
func (h *extBlockHook) OnBlockDecoded(raw json.RawMessage, block *types.Block) error {
	var body extRpcBlock
	if err := json.Unmarshal(raw, &body); err != nil {
		return fmt.Errorf("failed to unmarshal block extra data: %w", err)
	}
	extra := &customtypes.BlockBodyExtra{
		Version: body.Version,
		ExtData: (*[]byte)(body.BlockExtraData),
	}
	customtypes.SetBlockExtra(block, extra)
	return nil
}
