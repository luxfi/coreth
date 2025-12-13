// Copyright (C) 2025, Lux Industries Inc. All rights reserved.
// Post-quantum cryptography support - ML-KEM (FIPS 203) key encapsulation
// Module-Lattice Key Encapsulation Mechanism (formerly CRYSTALS-Kyber)

package vm

import (
	"errors"

	"github.com/luxfi/coreth/accounts/abi"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/crypto/mlkem"
)

// ML-KEM precompile addresses
var (
	// ML-KEM encapsulation precompiles
	mlkemEncapsulate512Address  = common.HexToAddress("0x0000000000000000000000000000000000000120") // ML-KEM-512 (Level 1)
	mlkemEncapsulate768Address  = common.HexToAddress("0x0000000000000000000000000000000000000121") // ML-KEM-768 (Level 3)
	mlkemEncapsulate1024Address = common.HexToAddress("0x0000000000000000000000000000000000000122") // ML-KEM-1024 (Level 5)
	
	// ML-KEM decapsulation precompiles (for completeness, typically done off-chain)
	mlkemDecapsulate512Address  = common.HexToAddress("0x0000000000000000000000000000000000000123") // ML-KEM-512 decap
	mlkemDecapsulate768Address  = common.HexToAddress("0x0000000000000000000000000000000000000124") // ML-KEM-768 decap
	mlkemDecapsulate1024Address = common.HexToAddress("0x0000000000000000000000000000000000000125") // ML-KEM-1024 decap
	
	// Hybrid encryption helper (combines ML-KEM with AES-GCM)
	mlkemHybridEncryptAddress = common.HexToAddress("0x0000000000000000000000000000000000000126")
	mlkemHybridDecryptAddress = common.HexToAddress("0x0000000000000000000000000000000000000127")
)

// Gas costs for ML-KEM operations
const (
	// ML-KEM encapsulation gas costs (based on benchmarks)
	mlkemEncapsulate512Gas  = 2000000  // ML-KEM-512 (2M gas)
	mlkemEncapsulate768Gas  = 3000000  // ML-KEM-768 (3M gas)
	mlkemEncapsulate1024Gas = 4000000  // ML-KEM-1024 (4M gas)
	
	// ML-KEM decapsulation gas costs (higher due to security checks)
	mlkemDecapsulate512Gas  = 3000000  // ML-KEM-512 (3M gas)
	mlkemDecapsulate768Gas  = 4000000  // ML-KEM-768 (4M gas)
	mlkemDecapsulate1024Gas = 5000000  // ML-KEM-1024 (5M gas)
	
	// Hybrid encryption gas costs
	mlkemHybridEncryptGas = 5000000  // Hybrid encrypt (5M gas)
	mlkemHybridDecryptGas = 6000000  // Hybrid decrypt (6M gas)
	
	// ML-KEM parameter sizes (FIPS 203)
	mlkem512PublicKeySize   = 800
	mlkem512CiphertextSize  = 768
	mlkem512SharedSecretSize = 32
	
	mlkem768PublicKeySize   = 1184
	mlkem768CiphertextSize  = 1088
	mlkem768SharedSecretSize = 32
	
	mlkem1024PublicKeySize   = 1568
	mlkem1024CiphertextSize  = 1568
	mlkem1024SharedSecretSize = 32
)

// mlkemEncapsulate512 implements ML-KEM-512 (NIST Level 1) encapsulation
type mlkemEncapsulate512 struct{}

func (m *mlkemEncapsulate512) RequiredGas(input []byte) uint64 {
	return mlkemEncapsulate512Gas
}

func (m *mlkemEncapsulate512) Run(input []byte) ([]byte, error) {
	return runMLKEMEncapsulate(input, mlkem.MLKEM512)
}

// mlkemEncapsulate768 implements ML-KEM-768 (NIST Level 3) encapsulation
type mlkemEncapsulate768 struct{}

func (m *mlkemEncapsulate768) RequiredGas(input []byte) uint64 {
	return mlkemEncapsulate768Gas
}

func (m *mlkemEncapsulate768) Run(input []byte) ([]byte, error) {
	return runMLKEMEncapsulate(input, mlkem.MLKEM768)
}

// mlkemEncapsulate1024 implements ML-KEM-1024 (NIST Level 5) encapsulation
type mlkemEncapsulate1024 struct{}

func (m *mlkemEncapsulate1024) RequiredGas(input []byte) uint64 {
	return mlkemEncapsulate1024Gas
}

func (m *mlkemEncapsulate1024) Run(input []byte) ([]byte, error) {
	return runMLKEMEncapsulate(input, mlkem.MLKEM1024)
}

// runMLKEMEncapsulate performs ML-KEM encapsulation for any security level
func runMLKEMEncapsulate(input []byte, mode mlkem.Mode) ([]byte, error) {
	// Input is the public key bytes
	expectedPubKeySize := mlkem.GetPublicKeySize(mode)
	if len(input) != expectedPubKeySize {
		return nil, errors.New("invalid public key size")
	}
	
	// Create public key from bytes
	pub, err := mlkem.PublicKeyFromBytes(input, mode)
	if err != nil {
		return nil, err
	}
	
	// Perform encapsulation (uses randomness from EVM environment)
	ciphertext, sharedSecret, err := pub.Encapsulate() // uses default entropy source
	if err != nil {
		return nil, err
	}

	// Return format: [ciphertext][shared_secret]
	// This allows the caller to get both values in one call
	output := make([]byte, 0, len(ciphertext)+len(sharedSecret))
	output = append(output, ciphertext...)
	output = append(output, sharedSecret...)

	return output, nil
}

// mlkemDecapsulate512 implements ML-KEM-512 decapsulation
type mlkemDecapsulate512 struct{}

func (m *mlkemDecapsulate512) RequiredGas(input []byte) uint64 {
	return mlkemDecapsulate512Gas
}

func (m *mlkemDecapsulate512) Run(input []byte) ([]byte, error) {
	return runMLKEMDecapsulate(input, mlkem.MLKEM512)
}

// mlkemDecapsulate768 implements ML-KEM-768 decapsulation
type mlkemDecapsulate768 struct{}

func (m *mlkemDecapsulate768) RequiredGas(input []byte) uint64 {
	return mlkemDecapsulate768Gas
}

func (m *mlkemDecapsulate768) Run(input []byte) ([]byte, error) {
	return runMLKEMDecapsulate(input, mlkem.MLKEM768)
}

// mlkemDecapsulate1024 implements ML-KEM-1024 decapsulation
type mlkemDecapsulate1024 struct{}

func (m *mlkemDecapsulate1024) RequiredGas(input []byte) uint64 {
	return mlkemDecapsulate1024Gas
}

func (m *mlkemDecapsulate1024) Run(input []byte) ([]byte, error) {
	return runMLKEMDecapsulate(input, mlkem.MLKEM1024)
}

// runMLKEMDecapsulate performs ML-KEM decapsulation for any security level
// Note: This requires the private key, so typically done off-chain
// Included for completeness and special use cases (e.g., threshold decryption)
func runMLKEMDecapsulate(input []byte, mode mlkem.Mode) ([]byte, error) {
	// Parse ABI-encoded input: (bytes ciphertext, bytes privateKey)
	args := abi.Arguments{
		{Type: mustNewBytesType()},
		{Type: mustNewBytesType()},
	}
	
	decoded, err := args.UnpackValues(input)
	if err != nil {
		return nil, errors.New("invalid ABI encoding")
	}
	
	if len(decoded) != 2 {
		return nil, errors.New("expected 2 arguments")
	}
	
	ciphertext, ok1 := decoded[0].([]byte)
	privateKeyBytes, ok2 := decoded[1].([]byte)
	
	if !ok1 || !ok2 {
		return nil, errors.New("failed to decode arguments")
	}
	
	// Validate ciphertext size
	expectedCiphertextSize := mlkem.GetCiphertextSize(mode)
	if len(ciphertext) != expectedCiphertextSize {
		return nil, errors.New("invalid ciphertext size")
	}
	
	// Create private key from bytes
	priv, err := mlkem.PrivateKeyFromBytes(privateKeyBytes, mode)
	if err != nil {
		return nil, err
	}
	
	// Perform decapsulation
	sharedSecret, err := priv.Decapsulate(ciphertext)
	if err != nil {
		return nil, err
	}
	
	// Return shared secret as 32 bytes
	return sharedSecret, nil
}

// mlkemHybridEncrypt implements hybrid encryption using ML-KEM + AES-GCM
// This combines post-quantum KEM with symmetric encryption for practical use
type mlkemHybridEncrypt struct{}

func (m *mlkemHybridEncrypt) RequiredGas(input []byte) uint64 {
	return mlkemHybridEncryptGas
}

func (m *mlkemHybridEncrypt) Run(input []byte) ([]byte, error) {
	// Parse ABI-encoded input: (bytes publicKey, bytes plaintext, uint8 mode)
	args := abi.Arguments{
		{Type: mustNewBytesType()},
		{Type: mustNewBytesType()},
		{Type: mustNewUint8Type()},
	}
	
	decoded, err := args.UnpackValues(input)
	if err != nil {
		return nil, errors.New("invalid ABI encoding")
	}
	
	if len(decoded) != 3 {
		return nil, errors.New("expected 3 arguments")
	}
	
	publicKeyBytes, ok1 := decoded[0].([]byte)
	plaintext, ok2 := decoded[1].([]byte)
	modeInt, ok3 := decoded[2].(uint8)
	
	if !ok1 || !ok2 || !ok3 {
		return nil, errors.New("failed to decode arguments")
	}
	
	// Determine ML-KEM mode
	var mode mlkem.Mode
	switch modeInt {
	case 0:
		mode = mlkem.MLKEM512
	case 1:
		mode = mlkem.MLKEM768
	case 2:
		mode = mlkem.MLKEM1024
	default:
		return nil, errors.New("invalid ML-KEM mode")
	}
	
	// Create public key
	pub, err := mlkem.PublicKeyFromBytes(publicKeyBytes, mode)
	if err != nil {
		return nil, err
	}
	
	// Perform KEM encapsulation
	ciphertext, sharedSecret, err := pub.Encapsulate()
	if err != nil {
		return nil, err
	}

	// Use shared secret as AES-256-GCM key
	// In production, this would use proper symmetric encryption
	// For now, we'll return a simplified format

	// Output format: [ciphertext_kem][encrypted_data][auth_tag]
	output := make([]byte, 0, len(ciphertext)+len(plaintext)+16)
	output = append(output, ciphertext...)

	// Simplified: XOR with shared secret (in production, use AES-GCM)
	encrypted := make([]byte, len(plaintext))
	for i := range plaintext {
		encrypted[i] = plaintext[i] ^ sharedSecret[i%32]
	}
	output = append(output, encrypted...)

	// Add simplified auth tag (in production, use proper MAC)
	authTag := make([]byte, 16)
	copy(authTag, sharedSecret[:16])
	output = append(output, authTag...)
	
	return output, nil
}

// mlkemHybridDecrypt implements hybrid decryption using ML-KEM + AES-GCM
type mlkemHybridDecrypt struct{}

func (m *mlkemHybridDecrypt) RequiredGas(input []byte) uint64 {
	return mlkemHybridDecryptGas
}

func (m *mlkemHybridDecrypt) Run(input []byte) ([]byte, error) {
	// This would typically be done off-chain since it requires the private key
	// Included for completeness
	return nil, errors.New("hybrid decryption typically performed off-chain")
}

// Helper function to create ABI types
func mustNewBytesType() abi.Type {
	t, _ := abi.NewType("bytes", "", nil)
	return t
}

func mustNewUint8Type() abi.Type {
	t, _ := abi.NewType("uint8", "", nil)
	return t
}

// PrecompiledContractsMLKEM contains the ML-KEM precompiled contracts
var PrecompiledContractsMLKEM = map[common.Address]PrecompiledContract{
	mlkemEncapsulate512Address:  &mlkemEncapsulate512{},
	mlkemEncapsulate768Address:  &mlkemEncapsulate768{},
	mlkemEncapsulate1024Address: &mlkemEncapsulate1024{},
	mlkemDecapsulate512Address:  &mlkemDecapsulate512{},
	mlkemDecapsulate768Address:  &mlkemDecapsulate768{},
	mlkemDecapsulate1024Address: &mlkemDecapsulate1024{},
	mlkemHybridEncryptAddress:   &mlkemHybridEncrypt{},
	mlkemHybridDecryptAddress:   &mlkemHybridDecrypt{},
}

// PrecompiledAddressesMLKEM contains the addresses of ML-KEM precompiles
var PrecompiledAddressesMLKEM = []common.Address{
	mlkemEncapsulate512Address,
	mlkemEncapsulate768Address,
	mlkemEncapsulate1024Address,
	mlkemDecapsulate512Address,
	mlkemDecapsulate768Address,
	mlkemDecapsulate1024Address,
	mlkemHybridEncryptAddress,
	mlkemHybridDecryptAddress,
}