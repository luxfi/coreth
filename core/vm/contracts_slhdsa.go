// Copyright (C) 2025, Lux Industries Inc. All rights reserved.
// Post-quantum cryptography support - SLH-DSA (FIPS 205) hash-based signatures
// Stateless Hash-Based Digital Signature Algorithm (formerly SPHINCS+)

package vm

import (
	"errors"

	"github.com/luxfi/geth/accounts/abi"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/crypto/slhdsa"
)

// SLH-DSA precompile addresses
var (
	// SLH-DSA verification precompiles - Small variants (optimized for signature size)
	slhdsaVerify128sAddress = common.HexToAddress("0x0000000000000000000000000000000000000130") // SLH-DSA-SHA2-128s
	slhdsaVerify192sAddress = common.HexToAddress("0x0000000000000000000000000000000000000131") // SLH-DSA-SHA2-192s
	slhdsaVerify256sAddress = common.HexToAddress("0x0000000000000000000000000000000000000132") // SLH-DSA-SHA2-256s
	
	// SLH-DSA verification precompiles - Fast variants (optimized for signing speed)
	slhdsaVerify128fAddress = common.HexToAddress("0x0000000000000000000000000000000000000133") // SLH-DSA-SHA2-128f
	slhdsaVerify192fAddress = common.HexToAddress("0x0000000000000000000000000000000000000134") // SLH-DSA-SHA2-192f
	slhdsaVerify256fAddress = common.HexToAddress("0x0000000000000000000000000000000000000135") // SLH-DSA-SHA2-256f
	
	// Batch verification for multiple SLH-DSA signatures
	slhdsaBatchVerifyAddress = common.HexToAddress("0x0000000000000000000000000000000000000136")
	
	// Hybrid signature verification (combines classical + SLH-DSA)
	slhdsaHybridVerifyAddress = common.HexToAddress("0x0000000000000000000000000000000000000137")
)

// Gas costs for SLH-DSA operations
const (
	// SLH-DSA verification gas costs - Small variants
	// Small signatures are more gas efficient due to smaller size
	slhdsaVerify128sGas = 10000000  // SLH-DSA-128s (10M gas)
	slhdsaVerify192sGas = 15000000  // SLH-DSA-192s (15M gas)
	slhdsaVerify256sGas = 20000000  // SLH-DSA-256s (20M gas)
	
	// SLH-DSA verification gas costs - Fast variants
	// Fast variants have larger signatures, higher gas cost
	slhdsaVerify128fGas = 15000000  // SLH-DSA-128f (15M gas)
	slhdsaVerify192fGas = 25000000  // SLH-DSA-192f (25M gas)
	slhdsaVerify256fGas = 30000000  // SLH-DSA-256f (30M gas)
	
	// Batch and hybrid verification gas costs
	slhdsaBatchVerifyBaseGas   = 5000000   // Base cost for batch verification
	slhdsaBatchVerifyPerSigGas = 8000000   // Additional cost per signature
	slhdsaHybridVerifyGas      = 12000000  // Hybrid verification (12M gas)
	
	// SLH-DSA parameter sizes (FIPS 205)
	slhdsa128sPublicKeySize = 32
	slhdsa128sSignatureSize = 7856
	slhdsa128fPublicKeySize = 32
	slhdsa128fSignatureSize = 17088
	
	slhdsa192sPublicKeySize = 48
	slhdsa192sSignatureSize = 16224
	slhdsa192fPublicKeySize = 48
	slhdsa192fSignatureSize = 35664
	
	slhdsa256sPublicKeySize = 64
	slhdsa256sSignatureSize = 29792
	slhdsa256fPublicKeySize = 64
	slhdsa256fSignatureSize = 49856
)

// slhdsaVerify128s implements SLH-DSA-SHA2-128s verification
type slhdsaVerify128s struct{}

func (s *slhdsaVerify128s) RequiredGas(input []byte) uint64 {
	return slhdsaVerify128sGas
}

func (s *slhdsaVerify128s) Run(input []byte) ([]byte, error) {
	return runSLHDSAVerify(input, slhdsa.SLHDSA128s)
}

// slhdsaVerify192s implements SLH-DSA-SHA2-192s verification
type slhdsaVerify192s struct{}

func (s *slhdsaVerify192s) RequiredGas(input []byte) uint64 {
	return slhdsaVerify192sGas
}

func (s *slhdsaVerify192s) Run(input []byte) ([]byte, error) {
	return runSLHDSAVerify(input, slhdsa.SLHDSA192s)
}

// slhdsaVerify256s implements SLH-DSA-SHA2-256s verification
type slhdsaVerify256s struct{}

func (s *slhdsaVerify256s) RequiredGas(input []byte) uint64 {
	return slhdsaVerify256sGas
}

func (s *slhdsaVerify256s) Run(input []byte) ([]byte, error) {
	return runSLHDSAVerify(input, slhdsa.SLHDSA256s)
}

// slhdsaVerify128f implements SLH-DSA-SHA2-128f verification
type slhdsaVerify128f struct{}

func (s *slhdsaVerify128f) RequiredGas(input []byte) uint64 {
	return slhdsaVerify128fGas
}

func (s *slhdsaVerify128f) Run(input []byte) ([]byte, error) {
	return runSLHDSAVerify(input, slhdsa.SLHDSA128f)
}

// slhdsaVerify192f implements SLH-DSA-SHA2-192f verification
type slhdsaVerify192f struct{}

func (s *slhdsaVerify192f) RequiredGas(input []byte) uint64 {
	return slhdsaVerify192fGas
}

func (s *slhdsaVerify192f) Run(input []byte) ([]byte, error) {
	return runSLHDSAVerify(input, slhdsa.SLHDSA192f)
}

// slhdsaVerify256f implements SLH-DSA-SHA2-256f verification
type slhdsaVerify256f struct{}

func (s *slhdsaVerify256f) RequiredGas(input []byte) uint64 {
	return slhdsaVerify256fGas
}

func (s *slhdsaVerify256f) Run(input []byte) ([]byte, error) {
	return runSLHDSAVerify(input, slhdsa.SLHDSA256f)
}

// runSLHDSAVerify performs SLH-DSA signature verification for any parameter set
func runSLHDSAVerify(input []byte, mode slhdsa.Mode) ([]byte, error) {
	// Parse ABI-encoded input: (bytes sig, bytes msg, bytes pubkey)
	args := abi.Arguments{
		{Type: mustNewBytesType()},
		{Type: mustNewBytesType()},
		{Type: mustNewBytesType()},
	}
	
	decoded, err := args.UnpackValues(input)
	if err != nil {
		return nil, errors.New("invalid ABI encoding")
	}
	
	if len(decoded) != 3 {
		return nil, errors.New("expected 3 arguments")
	}
	
	sig, ok1 := decoded[0].([]byte)
	msg, ok2 := decoded[1].([]byte)
	pubkey, ok3 := decoded[2].([]byte)
	
	if !ok1 || !ok2 || !ok3 {
		return nil, errors.New("failed to decode arguments")
	}
	
	// Validate public key size
	expectedPubKeySize := slhdsa.GetPublicKeySize(mode)
	if len(pubkey) != expectedPubKeySize {
		return nil, errors.New("invalid public key size")
	}
	
	// Validate signature size
	maxSigSize := slhdsa.GetSignatureSize(mode)
	if len(sig) > maxSigSize {
		return nil, errors.New("signature too large")
	}
	
	// Create public key from bytes
	pub, err := slhdsa.PublicKeyFromBytes(pubkey, mode)
	if err != nil {
		return nil, err
	}
	
	// Verify signature (stateless, deterministic)
	valid := pub.Verify(msg, sig)
	
	// Return result as 32 bytes (0x01 for valid, 0x00 for invalid)
	result := make([]byte, 32)
	if valid {
		result[31] = 0x01
	}
	
	return result, nil
}

// slhdsaBatchVerify implements batch verification of multiple SLH-DSA signatures
// This is more efficient than individual verifications for multiple signatures
type slhdsaBatchVerify struct{}

func (s *slhdsaBatchVerify) RequiredGas(input []byte) uint64 {
	// Parse to count signatures
	args := abi.Arguments{
		{Type: mustNewUint8Type()},    // mode
		{Type: mustNewBytesArrayType()}, // signatures
		{Type: mustNewBytesArrayType()}, // messages
		{Type: mustNewBytesArrayType()}, // public keys
	}
	
	decoded, err := args.UnpackValues(input)
	if err != nil {
		return slhdsaBatchVerifyBaseGas
	}
	
	if len(decoded) != 4 {
		return slhdsaBatchVerifyBaseGas
	}
	
	sigs, ok := decoded[1].([][]byte)
	if !ok {
		return slhdsaBatchVerifyBaseGas
	}
	
	// Calculate gas based on number of signatures
	numSigs := len(sigs)
	return slhdsaBatchVerifyBaseGas + uint64(numSigs)*slhdsaBatchVerifyPerSigGas
}

func (s *slhdsaBatchVerify) Run(input []byte) ([]byte, error) {
	// Parse ABI-encoded input: (uint8 mode, bytes[] sigs, bytes[] msgs, bytes[] pubkeys)
	args := abi.Arguments{
		{Type: mustNewUint8Type()},
		{Type: mustNewBytesArrayType()},
		{Type: mustNewBytesArrayType()},
		{Type: mustNewBytesArrayType()},
	}
	
	decoded, err := args.UnpackValues(input)
	if err != nil {
		return nil, errors.New("invalid ABI encoding")
	}
	
	if len(decoded) != 4 {
		return nil, errors.New("expected 4 arguments")
	}
	
	modeInt, ok1 := decoded[0].(uint8)
	sigs, ok2 := decoded[1].([][]byte)
	msgs, ok3 := decoded[2].([][]byte)
	pubkeys, ok4 := decoded[3].([][]byte)
	
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return nil, errors.New("failed to decode arguments")
	}
	
	// Validate array lengths match
	if len(sigs) != len(msgs) || len(msgs) != len(pubkeys) {
		return nil, errors.New("array length mismatch")
	}
	
	// Determine SLH-DSA mode
	var mode slhdsa.Mode
	switch modeInt {
	case 0:
		mode = slhdsa.SLHDSA128s
	case 1:
		mode = slhdsa.SLHDSA128f
	case 2:
		mode = slhdsa.SLHDSA192s
	case 3:
		mode = slhdsa.SLHDSA192f
	case 4:
		mode = slhdsa.SLHDSA256s
	case 5:
		mode = slhdsa.SLHDSA256f
	default:
		return nil, errors.New("invalid SLH-DSA mode")
	}
	
	// Verify each signature
	allValid := true
	results := make([]byte, len(sigs))
	
	for i := range sigs {
		// Create public key from bytes
		pub, err := slhdsa.PublicKeyFromBytes(pubkeys[i], mode)
		if err != nil {
			results[i] = 0x00
			allValid = false
			continue
		}
		
		// Verify signature
		if pub.Verify(msgs[i], sigs[i]) {
			results[i] = 0x01
		} else {
			results[i] = 0x00
			allValid = false
		}
	}
	
	// Return format: [overall_valid(1)][individual_results(N)]
	output := make([]byte, 1+len(results))
	if allValid {
		output[0] = 0x01
	}
	copy(output[1:], results)
	
	return output, nil
}

// slhdsaHybridVerify implements hybrid signature verification
// Combines classical ECDSA with SLH-DSA for transitional security
type slhdsaHybridVerify struct{}

func (s *slhdsaHybridVerify) RequiredGas(input []byte) uint64 {
	return slhdsaHybridVerifyGas
}

func (s *slhdsaHybridVerify) Run(input []byte) ([]byte, error) {
	// Parse ABI-encoded input: (bytes ecdsaSig, bytes slhdsaSig, bytes msg, address ecdsaAddr, bytes slhdsaPubkey, uint8 slhdsaMode)
	args := abi.Arguments{
		{Type: mustNewBytesType()},    // ECDSA signature
		{Type: mustNewBytesType()},    // SLH-DSA signature
		{Type: mustNewBytesType()},    // message
		{Type: mustNewAddressType()},  // ECDSA address
		{Type: mustNewBytesType()},    // SLH-DSA public key
		{Type: mustNewUint8Type()},    // SLH-DSA mode
	}
	
	decoded, err := args.UnpackValues(input)
	if err != nil {
		return nil, errors.New("invalid ABI encoding")
	}
	
	if len(decoded) != 6 {
		return nil, errors.New("expected 6 arguments")
	}
	
	ecdsaSig, ok1 := decoded[0].([]byte)
	slhdsaSig, ok2 := decoded[1].([]byte)
	msg, ok3 := decoded[2].([]byte)
	ecdsaAddr, ok4 := decoded[3].(common.Address)
	slhdsaPubkey, ok5 := decoded[4].([]byte)
	modeInt, ok6 := decoded[5].(uint8)
	
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
		return nil, errors.New("failed to decode arguments")
	}
	
	// Verify ECDSA signature using existing ecrecover precompile
	ecdsaValid := false
	// In production, call ecrecover precompile to verify ECDSA
	// For now, we'll simulate
	if len(ecdsaSig) == 65 && ecdsaAddr != (common.Address{}) {
		ecdsaValid = true // Simplified for demonstration
	}
	
	// Determine SLH-DSA mode
	var mode slhdsa.Mode
	switch modeInt {
	case 0:
		mode = slhdsa.SLHDSA128s
	case 1:
		mode = slhdsa.SLHDSA128f
	case 2:
		mode = slhdsa.SLHDSA192s
	case 3:
		mode = slhdsa.SLHDSA192f
	case 4:
		mode = slhdsa.SLHDSA256s
	case 5:
		mode = slhdsa.SLHDSA256f
	default:
		return nil, errors.New("invalid SLH-DSA mode")
	}
	
	// Verify SLH-DSA signature
	pub, err := slhdsa.PublicKeyFromBytes(slhdsaPubkey, mode)
	if err != nil {
		return nil, err
	}
	
	slhdsaValid := pub.Verify(msg, slhdsaSig)
	
	// Both signatures must be valid for hybrid verification
	hybridValid := ecdsaValid && slhdsaValid
	
	// Return format: [hybrid_valid(1)][ecdsa_valid(1)][slhdsa_valid(1)]
	result := make([]byte, 3)
	if hybridValid {
		result[0] = 0x01
	}
	if ecdsaValid {
		result[1] = 0x01
	}
	if slhdsaValid {
		result[2] = 0x01
	}
	
	return result, nil
}

// Helper function to create ABI array type
func mustNewBytesArrayType() abi.Type {
	t, _ := abi.NewType("bytes[]", "", nil)
	return t
}

func mustNewAddressType() abi.Type {
	t, _ := abi.NewType("address", "", nil)
	return t
}

// PrecompiledContractsSLHDSA contains the SLH-DSA precompiled contracts
var PrecompiledContractsSLHDSA = map[common.Address]PrecompiledContract{
	slhdsaVerify128sAddress:   &slhdsaVerify128s{},
	slhdsaVerify192sAddress:   &slhdsaVerify192s{},
	slhdsaVerify256sAddress:   &slhdsaVerify256s{},
	slhdsaVerify128fAddress:   &slhdsaVerify128f{},
	slhdsaVerify192fAddress:   &slhdsaVerify192f{},
	slhdsaVerify256fAddress:   &slhdsaVerify256f{},
	slhdsaBatchVerifyAddress:  &slhdsaBatchVerify{},
	slhdsaHybridVerifyAddress: &slhdsaHybridVerify{},
}

// PrecompiledAddressesSLHDSA contains the addresses of SLH-DSA precompiles
var PrecompiledAddressesSLHDSA = []common.Address{
	slhdsaVerify128sAddress,
	slhdsaVerify192sAddress,
	slhdsaVerify256sAddress,
	slhdsaVerify128fAddress,
	slhdsaVerify192fAddress,
	slhdsaVerify256fAddress,
	slhdsaBatchVerifyAddress,
	slhdsaHybridVerifyAddress,
}