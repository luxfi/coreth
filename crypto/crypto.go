// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package crypto provides cryptographic functions compatible with luxfi/geth types
package crypto

import (
	"crypto/ecdsa"
	"fmt"
	"hash"

	lux "github.com/luxfi/crypto"
	"github.com/luxfi/geth/common"
)

// Re-export constants
const (
	SignatureLength  = lux.SignatureLength
	RecoveryIDOffset = lux.RecoveryIDOffset
	DigestLength     = lux.DigestLength
)

// Re-export types
type (
	KeccakState   = lux.KeccakState
	EllipticCurve = lux.EllipticCurve
	// Address is re-exported for compatibility but common.Address should be preferred
	Address = lux.Address
	// Hash is re-exported for compatibility but common.Hash should be preferred
	Hash = lux.Hash
)

// NewKeccakState creates a new KeccakState
var NewKeccakState = lux.NewKeccakState

// Keccak256 calculates and returns the Keccak256 hash of the input data.
func Keccak256(data ...[]byte) []byte {
	return lux.Keccak256(data...)
}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to the common.Hash type.
func Keccak256Hash(data ...[]byte) common.Hash {
	return common.Hash(lux.Keccak256Hash(data...))
}

// HashData hashes the provided data using the KeccakState and returns a 32 byte hash
func HashData(kh hash.Hash, data []byte) common.Hash {
	kh.Reset()
	kh.Write(data)
	var h common.Hash
	kh.Sum(h[:0])
	return h
}

// CreateAddress creates an ethereum address from the given bytes
func CreateAddress(b common.Address, nonce uint64) common.Address {
	return common.Address(lux.CreateAddress(lux.Address(b), nonce))
}

// CreateAddress2 creates an ethereum address from the given bytes (CREATE2)
func CreateAddress2(b common.Address, salt [32]byte, inithash []byte) common.Address {
	return common.Address(lux.CreateAddress2(lux.Address(b), salt, inithash))
}

// PubkeyToAddress returns the Ethereum address of the given public key
func PubkeyToAddress(p ecdsa.PublicKey) common.Address {
	return common.Address(lux.PubkeyToAddress(p))
}

// TextHash is a helper function that calculates a hash for the given message that can be
// safely used to calculate a signature from.
// This creates a hash with the Ethereum signed message prefix.
func TextHash(data []byte) []byte {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(data), data)
	return Keccak256([]byte(msg))
}

// Re-export commonly used functions that don't involve type conversion
var (
	GenerateKey             = lux.GenerateKey
	ToECDSA                 = lux.ToECDSA
	ToECDSAUnsafe           = lux.ToECDSAUnsafe
	FromECDSA               = lux.FromECDSA
	UnmarshalPubkey         = lux.UnmarshalPubkey
	FromECDSAPub            = lux.FromECDSAPub
	HexToECDSA              = lux.HexToECDSA
	LoadECDSA               = lux.LoadECDSA
	SaveECDSA               = lux.SaveECDSA
	Sign                    = lux.Sign
	SignCompact             = lux.Sign // Alias for compatibility
	Ecrecover               = lux.Ecrecover
	SigToPub                = lux.SigToPub
	VerifySignature         = lux.VerifySignature
	DecompressPubkey        = lux.DecompressPubkey
	CompressPubkey          = lux.CompressPubkey
	S256                    = lux.S256
	ValidateSignatureValues = lux.ValidateSignatureValues
)
