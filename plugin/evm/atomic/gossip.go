// (c) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomic

import (
	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/network/p2p/gossip"
)

var (
	_ gossip.Gossipable                  = (*GossipAtomicTx)(nil)
	_ gossip.Marshaller[*GossipAtomicTx] = (*GossipAtomicTxMarshaller)(nil)
)

type GossipAtomicTxMarshaller struct{}

func (g GossipAtomicTxMarshaller) MarshalGossip(tx *GossipAtomicTx) ([]byte, error) {
	return tx.Tx.SignedBytes(), nil
}

func (g GossipAtomicTxMarshaller) UnmarshalGossip(bytes []byte) (*GossipAtomicTx, error) {
	tx, err := ExtractAtomicTx(bytes, Codec)
	return &GossipAtomicTx{
		Tx: tx,
	}, err
}

type GossipAtomicTx struct {
	Tx *Tx
}

func (tx *GossipAtomicTx) GossipID() ids.ID {
	return tx.Tx.ID()
}
