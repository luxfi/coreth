// (c) 2019-2022, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"github.com/luxfi/node/codec"
	"github.com/luxfi/node/codec/linearcodec"
	"github.com/luxfi/node/utils/units"
	"github.com/luxfi/node/utils/wrappers"
)

const (
	Version        = uint16(0)
	maxMessageSize = 2*units.MiB - 64*units.KiB // Subtract 64 KiB from p2p network cap to leave room for encoding overhead from Lux
)

var Codec codec.Manager

func init() {
	Codec = codec.NewManager(maxMessageSize)
	c := linearcodec.NewDefault()

	errs := wrappers.Errs{}
	// Gossip types removed from codec
	c.SkipRegistrations(2)
	errs.Add(
		// Types for state sync frontier consensus
		c.RegisterType(SyncSummary{}),

		// state sync types
		c.RegisterType(BlockRequest{}),
		c.RegisterType(BlockResponse{}),
		c.RegisterType(LeafsRequest{}),
		c.RegisterType(LeafsResponse{}),
		c.RegisterType(CodeRequest{}),
		c.RegisterType(CodeResponse{}),

		// Warp request types
		c.RegisterType(MessageSignatureRequest{}),
		c.RegisterType(BlockSignatureRequest{}),
		c.RegisterType(SignatureResponse{}),

		Codec.RegisterCodec(Version, c),
	)

	if errs.Errored() {
		panic(errs.Err)
	}
}
