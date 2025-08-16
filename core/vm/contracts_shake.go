// Copyright (C) 2025, Lux Industries Inc. All rights reserved.
// SHAKE (FIPS 202) - Secure Hash Algorithm Keccak
// High-performance precompiles for SHAKE128 and SHAKE256

package vm

import (
	"errors"

	"github.com/luxfi/geth/common"
	"golang.org/x/crypto/sha3"
)

// SHAKE precompile addresses
var (
	// SHAKE128 precompiles
	shake128Address    = common.HexToAddress("0x0000000000000000000000000000000000000140") // SHAKE128 variable output
	shake128_256Address = common.HexToAddress("0x0000000000000000000000000000000000000141") // SHAKE128 256-bit output
	shake128_512Address = common.HexToAddress("0x0000000000000000000000000000000000000142") // SHAKE128 512-bit output
	
	// SHAKE256 precompiles  
	shake256Address    = common.HexToAddress("0x0000000000000000000000000000000000000143") // SHAKE256 variable output
	shake256_256Address = common.HexToAddress("0x0000000000000000000000000000000000000144") // SHAKE256 256-bit output
	shake256_512Address = common.HexToAddress("0x0000000000000000000000000000000000000145") // SHAKE256 512-bit output
	shake256_1024Address = common.HexToAddress("0x0000000000000000000000000000000000000146") // SHAKE256 1024-bit output
	
	// cSHAKE (customizable SHAKE) precompiles
	cshake128Address = common.HexToAddress("0x0000000000000000000000000000000000000147") // cSHAKE128
	cshake256Address = common.HexToAddress("0x0000000000000000000000000000000000000148") // cSHAKE256
)

// Gas costs for SHAKE operations
const (
	// Base gas costs
	shake128BaseGas = 60    // Base cost for SHAKE128
	shake256BaseGas = 60    // Base cost for SHAKE256
	cshakeBaseGas   = 80    // Base cost for cSHAKE (includes customization)
	
	// Per-word gas costs (32 bytes)
	shakePerWordGas = 12    // Cost per 32-byte word of input
	shakeOutputGas  = 3     // Cost per 32-byte word of output
	
	// Fixed output gas costs
	shake128_256Gas  = 200  // SHAKE128 with 256-bit output
	shake128_512Gas  = 250  // SHAKE128 with 512-bit output
	shake256_256Gas  = 200  // SHAKE256 with 256-bit output
	shake256_512Gas  = 250  // SHAKE256 with 512-bit output
	shake256_1024Gas = 350  // SHAKE256 with 1024-bit output
	
	// Maximum output size (to prevent DoS)
	maxShakeOutput = 8192  // 8KB maximum output
)

// shake128 implements SHAKE128 with variable output length
type shake128 struct{}

func (s *shake128) RequiredGas(input []byte) uint64 {
	// Input format: [4 bytes output_len][data...]
	if len(input) < 4 {
		return shake128BaseGas
	}
	
	outputLen := uint32(input[0])<<24 | uint32(input[1])<<16 | uint32(input[2])<<8 | uint32(input[3])
	inputWords := uint64((len(input)-4+31) / 32)
	outputWords := uint64((outputLen+31) / 32)
	
	return shake128BaseGas + inputWords*shakePerWordGas + outputWords*shakeOutputGas
}

func (s *shake128) Run(input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, errors.New("input too short")
	}
	
	// Parse output length (first 4 bytes)
	outputLen := uint32(input[0])<<24 | uint32(input[1])<<16 | uint32(input[2])<<8 | uint32(input[3])
	
	if outputLen > maxShakeOutput {
		return nil, errors.New("output length exceeds maximum")
	}
	
	// Hash the remaining data
	data := input[4:]
	hash := sha3.NewShake128()
	hash.Write(data)
	
	// Generate output
	output := make([]byte, outputLen)
	hash.Read(output)
	
	return output, nil
}

// shake128_256 implements SHAKE128 with fixed 256-bit output
type shake128_256 struct{}

func (s *shake128_256) RequiredGas(input []byte) uint64 {
	return shake128_256Gas
}

func (s *shake128_256) Run(input []byte) ([]byte, error) {
	hash := sha3.NewShake128()
	hash.Write(input)
	
	output := make([]byte, 32) // 256 bits
	hash.Read(output)
	
	return output, nil
}

// shake128_512 implements SHAKE128 with fixed 512-bit output
type shake128_512 struct{}

func (s *shake128_512) RequiredGas(input []byte) uint64 {
	return shake128_512Gas
}

func (s *shake128_512) Run(input []byte) ([]byte, error) {
	hash := sha3.NewShake128()
	hash.Write(input)
	
	output := make([]byte, 64) // 512 bits
	hash.Read(output)
	
	return output, nil
}

// shake256 implements SHAKE256 with variable output length
type shake256 struct{}

func (s *shake256) RequiredGas(input []byte) uint64 {
	// Input format: [4 bytes output_len][data...]
	if len(input) < 4 {
		return shake256BaseGas
	}
	
	outputLen := uint32(input[0])<<24 | uint32(input[1])<<16 | uint32(input[2])<<8 | uint32(input[3])
	inputWords := uint64((len(input)-4+31) / 32)
	outputWords := uint64((outputLen+31) / 32)
	
	return shake256BaseGas + inputWords*shakePerWordGas + outputWords*shakeOutputGas
}

func (s *shake256) Run(input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, errors.New("input too short")
	}
	
	// Parse output length (first 4 bytes)
	outputLen := uint32(input[0])<<24 | uint32(input[1])<<16 | uint32(input[2])<<8 | uint32(input[3])
	
	if outputLen > maxShakeOutput {
		return nil, errors.New("output length exceeds maximum")
	}
	
	// Hash the remaining data
	data := input[4:]
	hash := sha3.NewShake256()
	hash.Write(data)
	
	// Generate output
	output := make([]byte, outputLen)
	hash.Read(output)
	
	return output, nil
}

// shake256_256 implements SHAKE256 with fixed 256-bit output
type shake256_256 struct{}

func (s *shake256_256) RequiredGas(input []byte) uint64 {
	return shake256_256Gas
}

func (s *shake256_256) Run(input []byte) ([]byte, error) {
	hash := sha3.NewShake256()
	hash.Write(input)
	
	output := make([]byte, 32) // 256 bits
	hash.Read(output)
	
	return output, nil
}

// shake256_512 implements SHAKE256 with fixed 512-bit output
type shake256_512 struct{}

func (s *shake256_512) RequiredGas(input []byte) uint64 {
	return shake256_512Gas
}

func (s *shake256_512) Run(input []byte) ([]byte, error) {
	hash := sha3.NewShake256()
	hash.Write(input)
	
	output := make([]byte, 64) // 512 bits
	hash.Read(output)
	
	return output, nil
}

// shake256_1024 implements SHAKE256 with fixed 1024-bit output
type shake256_1024 struct{}

func (s *shake256_1024) RequiredGas(input []byte) uint64 {
	return shake256_1024Gas
}

func (s *shake256_1024) Run(input []byte) ([]byte, error) {
	hash := sha3.NewShake256()
	hash.Write(input)
	
	output := make([]byte, 128) // 1024 bits
	hash.Read(output)
	
	return output, nil
}

// cshake128 implements cSHAKE128 with customization string
type cshake128 struct{}

func (s *cshake128) RequiredGas(input []byte) uint64 {
	// Input format: [4 bytes output_len][4 bytes custom_len][custom][data...]
	if len(input) < 8 {
		return cshakeBaseGas
	}
	
	outputLen := uint32(input[0])<<24 | uint32(input[1])<<16 | uint32(input[2])<<8 | uint32(input[3])
	customLen := uint32(input[4])<<24 | uint32(input[5])<<16 | uint32(input[6])<<8 | uint32(input[7])
	
	if len(input) < int(8+customLen) {
		return cshakeBaseGas
	}
	
	dataLen := len(input) - int(8+customLen)
	inputWords := uint64((dataLen+31) / 32)
	outputWords := uint64((outputLen+31) / 32)
	customWords := uint64((customLen+31) / 32)
	
	return cshakeBaseGas + (inputWords+customWords)*shakePerWordGas + outputWords*shakeOutputGas
}

func (s *cshake128) Run(input []byte) ([]byte, error) {
	if len(input) < 8 {
		return nil, errors.New("input too short")
	}
	
	// Parse output length and customization length
	outputLen := uint32(input[0])<<24 | uint32(input[1])<<16 | uint32(input[2])<<8 | uint32(input[3])
	customLen := uint32(input[4])<<24 | uint32(input[5])<<16 | uint32(input[6])<<8 | uint32(input[7])
	
	if outputLen > maxShakeOutput {
		return nil, errors.New("output length exceeds maximum")
	}
	
	if len(input) < int(8+customLen) {
		return nil, errors.New("input too short for customization")
	}
	
	// Extract customization string and data
	customization := input[8 : 8+customLen]
	data := input[8+customLen:]
	
	// Create cSHAKE128 with customization
	hash := sha3.NewCShake128(nil, customization)
	hash.Write(data)
	
	// Generate output
	output := make([]byte, outputLen)
	hash.Read(output)
	
	return output, nil
}

// cshake256 implements cSHAKE256 with customization string
type cshake256 struct{}

func (s *cshake256) RequiredGas(input []byte) uint64 {
	// Input format: [4 bytes output_len][4 bytes custom_len][custom][data...]
	if len(input) < 8 {
		return cshakeBaseGas
	}
	
	outputLen := uint32(input[0])<<24 | uint32(input[1])<<16 | uint32(input[2])<<8 | uint32(input[3])
	customLen := uint32(input[4])<<24 | uint32(input[5])<<16 | uint32(input[6])<<8 | uint32(input[7])
	
	if len(input) < int(8+customLen) {
		return cshakeBaseGas
	}
	
	dataLen := len(input) - int(8+customLen)
	inputWords := uint64((dataLen+31) / 32)
	outputWords := uint64((outputLen+31) / 32)
	customWords := uint64((customLen+31) / 32)
	
	return cshakeBaseGas + (inputWords+customWords)*shakePerWordGas + outputWords*shakeOutputGas
}

func (s *cshake256) Run(input []byte) ([]byte, error) {
	if len(input) < 8 {
		return nil, errors.New("input too short")
	}
	
	// Parse output length and customization length
	outputLen := uint32(input[0])<<24 | uint32(input[1])<<16 | uint32(input[2])<<8 | uint32(input[3])
	customLen := uint32(input[4])<<24 | uint32(input[5])<<16 | uint32(input[6])<<8 | uint32(input[7])
	
	if outputLen > maxShakeOutput {
		return nil, errors.New("output length exceeds maximum")
	}
	
	if len(input) < int(8+customLen) {
		return nil, errors.New("input too short for customization")
	}
	
	// Extract customization string and data
	customization := input[8 : 8+customLen]
	data := input[8+customLen:]
	
	// Create cSHAKE256 with customization
	hash := sha3.NewCShake256(nil, customization)
	hash.Write(data)
	
	// Generate output
	output := make([]byte, outputLen)
	hash.Read(output)
	
	return output, nil
}

// PrecompiledContractsSHAKE contains the SHAKE precompiled contracts
var PrecompiledContractsSHAKE = map[common.Address]PrecompiledContract{
	shake128Address:      &shake128{},
	shake128_256Address:  &shake128_256{},
	shake128_512Address:  &shake128_512{},
	shake256Address:      &shake256{},
	shake256_256Address:  &shake256_256{},
	shake256_512Address:  &shake256_512{},
	shake256_1024Address: &shake256_1024{},
	cshake128Address:     &cshake128{},
	cshake256Address:     &cshake256{},
}

// PrecompiledAddressesSHAKE contains the addresses of SHAKE precompiles
var PrecompiledAddressesSHAKE = []common.Address{
	shake128Address,
	shake128_256Address,
	shake128_512Address,
	shake256Address,
	shake256_256Address,
	shake256_512Address,
	shake256_1024Address,
	cshake128Address,
	cshake256Address,
}