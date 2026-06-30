// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package eth

import (
	"encoding/binary"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/ethdb"
)

// bloomBitsPrefix is the database prefix for bloom bits data
var bloomBitsPrefix = []byte("B") // bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash -> bloom bits

// ReadBloomBits retrieves the compressed bloom bit vector belonging to the given
// bit index from the given section and block hash.
func ReadBloomBits(db ethdb.KeyValueReader, bit uint, section uint64, head common.Hash) ([]byte, error) {
	// Construct the bloom bits key
	key := make([]byte, len(bloomBitsPrefix)+2+8+common.HashLength)
	copy(key, bloomBitsPrefix)

	// Add bit index (uint16 big endian)
	binary.BigEndian.PutUint16(key[len(bloomBitsPrefix):], uint16(bit))

	// Add section index (uint64 big endian)
	binary.BigEndian.PutUint64(key[len(bloomBitsPrefix)+2:], section)

	// Add block hash
	copy(key[len(bloomBitsPrefix)+2+8:], head.Bytes())

	// Read the bloom bits from the database
	return db.Get(key)
}
