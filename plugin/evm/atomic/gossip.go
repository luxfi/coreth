// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package atomic

import "github.com/luxfi/p2p/gossip"

var _ gossip.Marshaller[*Tx] = (*TxMarshaller)(nil)

type TxMarshaller struct{}

func (g *TxMarshaller) MarshalGossip(tx *Tx) ([]byte, error) {
	return tx.SignedBytes(), nil
}

func (_ *TxMarshaller) UnmarshalGossip(bytes []byte) (*Tx, error) {
	return ExtractAtomicTx(bytes, Codec)
}
