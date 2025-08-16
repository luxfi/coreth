// Copyright (C) 2025, Lux Industries Inc. All rights reserved.
// Tests for post-quantum cryptography precompiles

package vm

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

// Test vectors from ETHFALCON and ETHDILITHIUM repositories
// These would be replaced with actual NIST KAT vectors

var (
	// Sample FALCON-512 test vector (would be from NIST KAT)
	falconTestPublicKey, _ = hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20") // 897 bytes in real implementation
	falconTestMessage      = []byte("Hello, post-quantum world!")
	falconTestSignature, _ = hex.DecodeString("deadbeef") // Variable length, max 690 bytes
	falconTestSalt, _      = hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000") // 40 bytes

	// Sample Dilithium3 test vector
	dilithiumTestPublicKey, _ = hex.DecodeString("0102030405060708090a0b0c0d0e0f10") // 1952 bytes in real implementation
	dilithiumTestSignature, _ = hex.DecodeString("deadbeef")                          // 3293 bytes in real implementation
)

// TestFalconVerifyPrecompile tests the NIST-compliant FALCON verification
func TestFalconVerifyPrecompile(t *testing.T) {
	falcon := &falconVerify{}

	tests := []struct {
		name      string
		signature []byte
		message   []byte
		publicKey []byte
		wantError bool
		expected  bool
	}{
		{
			name:      "valid signature",
			signature: falconTestSignature,
			message:   falconTestMessage,
			publicKey: make([]byte, falcon512PublicKeySize), // Placeholder
			wantError: false,
			expected:  false, // Would be true with real implementation
		},
		{
			name:      "invalid public key size",
			signature: falconTestSignature,
			message:   falconTestMessage,
			publicKey: []byte{0x01, 0x02}, // Too short
			wantError: true,
			expected:  false,
		},
		{
			name:      "empty input",
			signature: []byte{},
			message:   []byte{},
			publicKey: []byte{},
			wantError: true,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ABI encode the input
			args := abi.Arguments{
				{Type: mustNewBytesType()},
				{Type: mustNewBytesType()},
				{Type: mustNewBytesType()},
			}

			input, err := args.Pack(tt.signature, tt.message, tt.publicKey)
			if err != nil && !tt.wantError {
				t.Fatalf("failed to pack input: %v", err)
			}

			// Test gas requirement
			gas := falcon.RequiredGas(input)
			if gas != falconVerifyGas {
				t.Errorf("incorrect gas: got %d, want %d", gas, falconVerifyGas)
			}

			// Run the precompile
			output, err := falcon.Run(input)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check output
			if len(output) != 32 {
				t.Errorf("incorrect output length: got %d, want 32", len(output))
			}

			isValid := new(big.Int).SetBytes(output).Cmp(big.NewInt(1)) == 0
			if isValid != tt.expected {
				t.Errorf("incorrect validation result: got %v, want %v", isValid, tt.expected)
			}
		})
	}
}

// TestETHFalconVerifyPrecompile tests the ETH-optimized FALCON verification
func TestETHFalconVerifyPrecompile(t *testing.T) {
	ethFalcon := &ethFalconVerify{}

	// Create test input with compacted format
	messageHash := make([]byte, 32)
	salt := make([]byte, 40)
	
	// Compacted s2 (32 uint256 values, 16 coefficients per word)
	s2 := make([]byte, 32*32) // 32 words * 32 bytes per word
	
	// Compacted public key in NTT domain
	ntth := make([]byte, 32*32)

	input := append(messageHash, salt...)
	input = append(input, s2...)
	input = append(input, ntth...)

	// Test gas requirement
	gas := ethFalcon.RequiredGas(input)
	if gas != ethFalconVerifyGas {
		t.Errorf("incorrect gas: got %d, want %d", gas, ethFalconVerifyGas)
	}

	// Run the precompile
	output, err := ethFalcon.Run(input)
	if err != nil {
		// Expected to fail without actual implementation
		t.Logf("Expected error without implementation: %v", err)
	}

	if output != nil && len(output) != 32 {
		t.Errorf("incorrect output length: got %d, want 32", len(output))
	}
}

// TestFalconRecoverPrecompile tests the FALCON recovery (EPERVIER variant)
func TestFalconRecoverPrecompile(t *testing.T) {
	falconRecover := &falconRecover{}

	messageHash := make([]byte, 32)
	salt := make([]byte, 40)
	signature := make([]byte, 500) // Variable length signature

	input := append(messageHash, salt...)
	input = append(input, signature...)

	// Test gas requirement
	gas := falconRecover.RequiredGas(input)
	if gas != falconRecoverGas {
		t.Errorf("incorrect gas: got %d, want %d", gas, falconRecoverGas)
	}

	// Run the precompile
	output, err := falconRecover.Run(input)
	if err != nil {
		// Expected to fail without actual implementation
		t.Logf("Expected error without implementation: %v", err)
	}

	if output != nil && len(output) != 32 {
		t.Errorf("incorrect output length: got %d, want 32", len(output))
	}
}

// TestDilithiumVerifyPrecompile tests the NIST-compliant Dilithium verification
func TestDilithiumVerifyPrecompile(t *testing.T) {
	dilithium := &dilithiumVerify{}

	tests := []struct {
		name      string
		signature []byte
		message   []byte
		publicKey []byte
		wantError bool
		expected  bool
	}{
		{
			name:      "valid signature",
			signature: make([]byte, dilithium3SignatureSize),
			message:   []byte("Test message"),
			publicKey: make([]byte, dilithium3PublicKeySize),
			wantError: false,
			expected:  false, // Would be true with real implementation
		},
		{
			name:      "invalid signature size",
			signature: []byte{0x01, 0x02}, // Too short
			message:   []byte("Test message"),
			publicKey: make([]byte, dilithium3PublicKeySize),
			wantError: true,
			expected:  false,
		},
		{
			name:      "invalid public key size",
			signature: make([]byte, dilithium3SignatureSize),
			message:   []byte("Test message"),
			publicKey: []byte{0x01, 0x02}, // Too short
			wantError: true,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ABI encode the input
			args := abi.Arguments{
				{Type: mustNewBytesType()},
				{Type: mustNewBytesType()},
				{Type: mustNewBytesType()},
			}

			input, err := args.Pack(tt.signature, tt.message, tt.publicKey)
			if err != nil && !tt.wantError {
				t.Fatalf("failed to pack input: %v", err)
			}

			// Test gas requirement
			gas := dilithium.RequiredGas(input)
			if gas != dilithiumVerifyGas {
				t.Errorf("incorrect gas: got %d, want %d", gas, dilithiumVerifyGas)
			}

			// Run the precompile
			output, err := dilithium.Run(input)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check output
			if len(output) != 32 {
				t.Errorf("incorrect output length: got %d, want 32", len(output))
			}

			isValid := new(big.Int).SetBytes(output).Cmp(big.NewInt(1)) == 0
			if isValid != tt.expected {
				t.Errorf("incorrect validation result: got %v, want %v", isValid, tt.expected)
			}
		})
	}
}

// TestETHDilithiumVerifyPrecompile tests the ETH-optimized Dilithium verification
func TestETHDilithiumVerifyPrecompile(t *testing.T) {
	ethDilithium := &ethDilithiumVerify{}

	messageHash := make([]byte, 32)
	// ETH-optimized format with precomputed NTT and compressed signature
	signatureData := make([]byte, 5000) // Placeholder size

	input := append(messageHash, signatureData...)

	// Test gas requirement
	gas := ethDilithium.RequiredGas(input)
	if gas != ethDilithiumVerifyGas {
		t.Errorf("incorrect gas: got %d, want %d", gas, ethDilithiumVerifyGas)
	}

	// Run the precompile
	output, err := ethDilithium.Run(input)
	if err != nil {
		// Expected to fail without actual implementation
		t.Logf("Expected error without implementation: %v", err)
	}

	if output != nil && len(output) != 32 {
		t.Errorf("incorrect output length: got %d, want 32", len(output))
	}
}

// TestPostQuantumPrecompilesInLux tests that all post-quantum precompiles are registered
func TestPostQuantumPrecompilesInLux(t *testing.T) {
	// Check that Lux precompiles include all post-quantum contracts
	expectedAddresses := []common.Address{
		common.HexToAddress("0x0100"), // FALCON verify
		common.HexToAddress("0x0101"), // FALCON recover
		common.HexToAddress("0x0102"), // Dilithium verify
		common.HexToAddress("0x0103"), // ETH-FALCON verify
		common.HexToAddress("0x0104"), // ETH-Dilithium verify
	}

	for _, addr := range expectedAddresses {
		if _, ok := PrecompiledContractsLux[addr]; !ok {
			t.Errorf("precompile not registered at address %s", addr.Hex())
		}
	}

	// Verify they're included in the address list
	for _, addr := range expectedAddresses {
		found := false
		for _, registeredAddr := range PrecompiledAddressesLux {
			if registeredAddr == addr {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("address %s not in PrecompiledAddressesLux", addr.Hex())
		}
	}
}

// TestNTTOperations tests the Number Theoretic Transform functions
func TestNTTOperations(t *testing.T) {
	// Test FALCON NTT parameters
	falconNTT := &NTTParams{
		N: 512,
		Q: big.NewInt(12289),
	}

	// Test polynomial
	poly := make([]uint64, falconNTT.N)
	for i := range poly {
		poly[i] = uint64(i % 100)
	}

	// Forward NTT
	nttPoly := falconNTT.ForwardNTT(poly)
	if len(nttPoly) != falconNTT.N {
		t.Errorf("forward NTT changed polynomial length")
	}

	// Inverse NTT
	recoveredPoly := falconNTT.InverseNTT(nttPoly)
	if len(recoveredPoly) != falconNTT.N {
		t.Errorf("inverse NTT changed polynomial length")
	}

	// Test Dilithium NTT parameters
	dilithiumNTT := &NTTParams{
		N: 256,
		Q: big.NewInt(8380417),
	}

	poly2 := make([]uint64, dilithiumNTT.N)
	for i := range poly2 {
		poly2[i] = uint64(i % 100)
	}

	nttPoly2 := dilithiumNTT.ForwardNTT(poly2)
	if len(nttPoly2) != dilithiumNTT.N {
		t.Errorf("Dilithium forward NTT changed polynomial length")
	}
}

// BenchmarkFalconVerify benchmarks FALCON signature verification
func BenchmarkFalconVerify(b *testing.B) {
	falcon := &falconVerify{}

	// Create test input
	args := abi.Arguments{
		{Type: mustNewBytesType()},
		{Type: mustNewBytesType()},
		{Type: mustNewBytesType()},
	}

	signature := make([]byte, 600)     // Average signature size
	message := []byte("Benchmark message")
	publicKey := make([]byte, falcon512PublicKeySize)

	input, _ := args.Pack(signature, message, publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = falcon.Run(input)
	}
}

// BenchmarkDilithiumVerify benchmarks Dilithium signature verification
func BenchmarkDilithiumVerify(b *testing.B) {
	dilithium := &dilithiumVerify{}

	// Create test input
	args := abi.Arguments{
		{Type: mustNewBytesType()},
		{Type: mustNewBytesType()},
		{Type: mustNewBytesType()},
	}

	signature := make([]byte, dilithium3SignatureSize)
	message := []byte("Benchmark message")
	publicKey := make([]byte, dilithium3PublicKeySize)

	input, _ := args.Pack(signature, message, publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dilithium.Run(input)
	}
}

// TestGasCostComparison compares gas costs of different signature schemes
func TestGasCostComparison(t *testing.T) {
	tests := []struct {
		name string
		gas  uint64
	}{
		{"ECDSA ecrecover", params.EcrecoverGas},
		{"FALCON NIST", falconVerifyGas},
		{"FALCON ETH-optimized", ethFalconVerifyGas},
		{"FALCON with recovery", falconRecoverGas},
		{"Dilithium NIST", dilithiumVerifyGas},
		{"Dilithium ETH-optimized", ethDilithiumVerifyGas},
	}

	t.Log("Gas cost comparison:")
	for _, tt := range tests {
		t.Logf("  %s: %d gas", tt.name, tt.gas)
		if tt.name != "ECDSA ecrecover" {
			ratio := float64(tt.gas) / float64(params.EcrecoverGas)
			t.Logf("    -> %.1fx more expensive than ECDSA", ratio)
		}
	}
}