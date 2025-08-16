// Copyright (C) 2025, Lux Industries Inc. All rights reserved.
// Tests for post-quantum cryptography precompiles

package vm

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/luxfi/geth/common"
)

// TestFalconVerifyPrecompile tests the FALCON signature verification precompile
func TestFalconVerifyPrecompile(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		wantError bool
	}{
		{
			name:      "empty input",
			input:     "",
			expected:  "",
			wantError: true,
		},
		{
			name:      "invalid input length",
			input:     "0x1234",
			expected:  "",
			wantError: true,
		},
		// TODO: Add test cases with actual FALCON test vectors
		// These would come from the NIST KAT (Known Answer Tests)
	}

	falcon := &falconVerify{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := hex.DecodeString(tt.input)
			if err != nil && !tt.wantError {
				t.Fatalf("failed to decode input: %v", err)
			}

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

			expected, _ := hex.DecodeString(tt.expected)
			if !bytes.Equal(output, expected) {
				t.Errorf("output mismatch: got %x, want %x", output, expected)
			}
		})
	}
}

// TestFalconRecoverPrecompile tests the FALCON recovery precompile
func TestFalconRecoverPrecompile(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		wantError bool
	}{
		{
			name:      "empty input",
			input:     "",
			expected:  "",
			wantError: true,
		},
		{
			name:      "invalid input length",
			input:     "0x1234",
			expected:  "",
			wantError: true,
		},
		// TODO: Add test cases with EPERVIER variant test vectors
	}

	falconRecover := &falconRecover{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := hex.DecodeString(tt.input)
			if err != nil && !tt.wantError {
				t.Fatalf("failed to decode input: %v", err)
			}

			output, err := falconRecover.Run(input)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expected, _ := hex.DecodeString(tt.expected)
			if !bytes.Equal(output, expected) {
				t.Errorf("output mismatch: got %x, want %x", output, expected)
			}
		})
	}
}

// TestDilithiumVerifyPrecompile tests the Dilithium signature verification precompile
func TestDilithiumVerifyPrecompile(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		wantError bool
	}{
		{
			name:      "empty input",
			input:     "",
			expected:  "",
			wantError: true,
		},
		{
			name:      "invalid input length",
			input:     "0x1234",
			expected:  "",
			wantError: true,
		},
		// TODO: Add test cases with actual Dilithium test vectors
		// These would come from the NIST KAT (Known Answer Tests)
	}

	dilithium := &dilithiumVerify{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := hex.DecodeString(tt.input)
			if err != nil && !tt.wantError {
				t.Fatalf("failed to decode input: %v", err)
			}

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

			expected, _ := hex.DecodeString(tt.expected)
			if !bytes.Equal(output, expected) {
				t.Errorf("output mismatch: got %x, want %x", output, expected)
			}
		})
	}
}

// TestFalconGasCost tests the gas cost calculation for FALCON operations
func TestFalconGasCost(t *testing.T) {
	falcon := &falconVerify{}
	
	// Test with minimal valid input size
	minInput := make([]byte, 32+40+897) // hash + salt + pubkey
	gas := falcon.RequiredGas(minInput)
	
	if gas != falconVerifyBaseGas {
		t.Errorf("expected gas cost %d, got %d", falconVerifyBaseGas, gas)
	}
}

// TestDilithiumGasCost tests the gas cost calculation for Dilithium operations
func TestDilithiumGasCost(t *testing.T) {
	dilithium := &dilithiumVerify{}
	
	// Test with exact valid input size
	input := make([]byte, 32+3293+1952) // hash + signature + pubkey
	gas := dilithium.RequiredGas(input)
	
	if gas != dilithiumVerifyBaseGas {
		t.Errorf("expected gas cost %d, got %d", dilithiumVerifyBaseGas, gas)
	}
}

// TestPostQuantumPrecompilesRegistered tests that post-quantum precompiles are registered
func TestPostQuantumPrecompilesRegistered(t *testing.T) {
	// Check that Lux precompiles include post-quantum support
	if _, ok := PrecompiledContractsLux[common.HexToAddress("0x0100")]; !ok {
		t.Error("FALCON verify precompile not registered")
	}
	
	if _, ok := PrecompiledContractsLux[common.HexToAddress("0x0101")]; !ok {
		t.Error("FALCON recover precompile not registered")
	}
	
	if _, ok := PrecompiledContractsLux[common.HexToAddress("0x0102")]; !ok {
		t.Error("Dilithium verify precompile not registered")
	}
}

// BenchmarkFalconVerify benchmarks FALCON signature verification
func BenchmarkFalconVerify(b *testing.B) {
	falcon := &falconVerify{}
	
	// Create a dummy input of the expected size
	input := make([]byte, 32+40+512+897) // hash + salt + avg sig + pubkey
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = falcon.Run(input)
	}
}

// BenchmarkDilithiumVerify benchmarks Dilithium signature verification
func BenchmarkDilithiumVerify(b *testing.B) {
	dilithium := &dilithiumVerify{}
	
	// Create a dummy input of the expected size
	input := make([]byte, 32+3293+1952) // hash + signature + pubkey
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dilithium.Run(input)
	}
}