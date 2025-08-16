// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title MLDSAVerifier
 * @notice Contract for verifying ML-DSA (FIPS 204) post-quantum signatures
 * @dev Uses Lux network's ML-DSA precompiles for quantum-resistant verification
 */
contract MLDSAVerifier {
    
    // Precompile addresses for ML-DSA signatures
    address constant MLDSA_VERIFY_44 = 0x0000000000000000000000000000000000000110;  // Level 2
    address constant MLDSA_VERIFY_65 = 0x0000000000000000000000000000000000000111;  // Level 3 (recommended)
    address constant MLDSA_VERIFY_87 = 0x0000000000000000000000000000000000000112;  // Level 5
    address constant ETH_MLDSA_VERIFY = 0x0000000000000000000000000000000000000113; // ETH-optimized
    
    // Events
    event MLDSAVerified(address indexed verifier, bytes32 messageHash, uint8 level, bool valid);
    
    // Errors
    error InvalidSignatureLength();
    error InvalidPublicKeyLength();
    error InvalidSecurityLevel();
    error VerificationFailed();
    
    // ML-DSA parameter sizes
    uint256 constant MLDSA44_PUBKEY_SIZE = 1312;
    uint256 constant MLDSA44_MAX_SIG_SIZE = 2420;
    
    uint256 constant MLDSA65_PUBKEY_SIZE = 1952;
    uint256 constant MLDSA65_MAX_SIG_SIZE = 3309;
    
    uint256 constant MLDSA87_PUBKEY_SIZE = 2592;
    uint256 constant MLDSA87_MAX_SIG_SIZE = 4627;
    
    /**
     * @notice Verify an ML-DSA-44 signature (NIST Level 2)
     * @param signature The ML-DSA signature
     * @param message The message that was signed
     * @param publicKey The ML-DSA public key (1312 bytes)
     * @return valid True if signature is valid
     */
    function verifyMLDSA44(
        bytes memory signature,
        bytes memory message,
        bytes memory publicKey
    ) public returns (bool valid) {
        if (publicKey.length != MLDSA44_PUBKEY_SIZE) {
            revert InvalidPublicKeyLength();
        }
        if (signature.length > MLDSA44_MAX_SIG_SIZE) {
            revert InvalidSignatureLength();
        }
        
        // ABI encode the parameters for the precompile
        bytes memory input = abi.encode(signature, message, publicKey);
        
        // Call the ML-DSA-44 verify precompile
        (bool success, bytes memory result) = MLDSA_VERIFY_44.staticcall(input);
        
        if (success && result.length == 32) {
            valid = uint256(bytes32(result)) == 1;
            emit MLDSAVerified(msg.sender, keccak256(message), 44, valid);
        }
        
        return valid;
    }
    
    /**
     * @notice Verify an ML-DSA-65 signature (NIST Level 3 - Recommended)
     * @param signature The ML-DSA signature (up to 3309 bytes)
     * @param message The message that was signed
     * @param publicKey The ML-DSA public key (1952 bytes)
     * @return valid True if signature is valid
     */
    function verifyMLDSA65(
        bytes memory signature,
        bytes memory message,
        bytes memory publicKey
    ) public returns (bool valid) {
        if (publicKey.length != MLDSA65_PUBKEY_SIZE) {
            revert InvalidPublicKeyLength();
        }
        if (signature.length > MLDSA65_MAX_SIG_SIZE) {
            revert InvalidSignatureLength();
        }
        
        // ABI encode the parameters for the precompile
        bytes memory input = abi.encode(signature, message, publicKey);
        
        // Call the ML-DSA-65 verify precompile
        (bool success, bytes memory result) = MLDSA_VERIFY_65.staticcall(input);
        
        if (success && result.length == 32) {
            valid = uint256(bytes32(result)) == 1;
            emit MLDSAVerified(msg.sender, keccak256(message), 65, valid);
        }
        
        return valid;
    }
    
    /**
     * @notice Verify an ML-DSA-87 signature (NIST Level 5)
     * @param signature The ML-DSA signature (up to 4627 bytes)
     * @param message The message that was signed
     * @param publicKey The ML-DSA public key (2592 bytes)
     * @return valid True if signature is valid
     */
    function verifyMLDSA87(
        bytes memory signature,
        bytes memory message,
        bytes memory publicKey
    ) public returns (bool valid) {
        if (publicKey.length != MLDSA87_PUBKEY_SIZE) {
            revert InvalidPublicKeyLength();
        }
        if (signature.length > MLDSA87_MAX_SIG_SIZE) {
            revert InvalidSignatureLength();
        }
        
        // ABI encode the parameters for the precompile
        bytes memory input = abi.encode(signature, message, publicKey);
        
        // Call the ML-DSA-87 verify precompile
        (bool success, bytes memory result) = MLDSA_VERIFY_87.staticcall(input);
        
        if (success && result.length == 32) {
            valid = uint256(bytes32(result)) == 1;
            emit MLDSAVerified(msg.sender, keccak256(message), 87, valid);
        }
        
        return valid;
    }
    
    /**
     * @notice Verify an ETH-optimized ML-DSA signature (uses Keccak)
     * @param messageHash The hash of the message
     * @param level The security level (44, 65, or 87)
     * @param signature The ML-DSA signature
     * @param publicKey The ML-DSA public key
     * @return valid True if signature is valid
     */
    function verifyETHMLDSA(
        bytes32 messageHash,
        uint8 level,
        bytes memory signature,
        bytes memory publicKey
    ) public returns (bool valid) {
        // Validate security level and key sizes
        if (level == 44) {
            if (publicKey.length != MLDSA44_PUBKEY_SIZE) revert InvalidPublicKeyLength();
            if (signature.length > MLDSA44_MAX_SIG_SIZE) revert InvalidSignatureLength();
        } else if (level == 65) {
            if (publicKey.length != MLDSA65_PUBKEY_SIZE) revert InvalidPublicKeyLength();
            if (signature.length > MLDSA65_MAX_SIG_SIZE) revert InvalidSignatureLength();
        } else if (level == 87) {
            if (publicKey.length != MLDSA87_PUBKEY_SIZE) revert InvalidPublicKeyLength();
            if (signature.length > MLDSA87_MAX_SIG_SIZE) revert InvalidSignatureLength();
        } else {
            revert InvalidSecurityLevel();
        }
        
        // Pack the input: [32 bytes hash][1 byte mode][signature][public key]
        bytes memory input = abi.encodePacked(
            messageHash,
            bytes1(level),
            signature,
            publicKey
        );
        
        // Call the ETH-optimized ML-DSA verify precompile
        (bool success, bytes memory result) = ETH_MLDSA_VERIFY.staticcall(input);
        
        if (success && result.length == 32) {
            valid = uint256(bytes32(result)) == 1;
            emit MLDSAVerified(msg.sender, messageHash, level, valid);
        }
        
        return valid;
    }
    
    /**
     * @notice Check if an address has been quantum-secured with ML-DSA
     * @dev This would integrate with a registry of quantum-secured addresses
     */
    mapping(address => bool) public quantumSecured;
    mapping(address => bytes) public mldsaPublicKeys;
    mapping(address => uint8) public securityLevels;
    
    /**
     * @notice Register an ML-DSA public key for an address
     * @param publicKey The ML-DSA public key to register
     * @param level The security level (44, 65, or 87)
     */
    function registerMLDSAKey(bytes memory publicKey, uint8 level) public {
        if (level == 44) {
            require(publicKey.length == MLDSA44_PUBKEY_SIZE, "Invalid ML-DSA-44 key");
        } else if (level == 65) {
            require(publicKey.length == MLDSA65_PUBKEY_SIZE, "Invalid ML-DSA-65 key");
        } else if (level == 87) {
            require(publicKey.length == MLDSA87_PUBKEY_SIZE, "Invalid ML-DSA-87 key");
        } else {
            revert InvalidSecurityLevel();
        }
        
        mldsaPublicKeys[msg.sender] = publicKey;
        securityLevels[msg.sender] = level;
        quantumSecured[msg.sender] = true;
    }
    
    /**
     * @notice Execute a transaction with ML-DSA signature verification
     * @param target The target contract
     * @param data The transaction data
     * @param signature The ML-DSA signature
     * @param signer The address that should have signed
     */
    function executeWithMLDSA(
        address target,
        bytes memory data,
        bytes memory signature,
        address signer
    ) public returns (bool success, bytes memory returnData) {
        // Get the registered ML-DSA public key
        bytes memory publicKey = mldsaPublicKeys[signer];
        require(publicKey.length > 0, "No ML-DSA key registered");
        
        uint8 level = securityLevels[signer];
        
        // Create message hash from transaction data
        bytes32 messageHash = keccak256(abi.encodePacked(target, data, block.number));
        
        // Verify the signature
        bool valid;
        if (level == 44) {
            valid = verifyMLDSA44(signature, abi.encodePacked(messageHash), publicKey);
        } else if (level == 65) {
            valid = verifyMLDSA65(signature, abi.encodePacked(messageHash), publicKey);
        } else if (level == 87) {
            valid = verifyMLDSA87(signature, abi.encodePacked(messageHash), publicKey);
        } else {
            revert InvalidSecurityLevel();
        }
        
        require(valid, "Invalid ML-DSA signature");
        
        // Execute the transaction
        (success, returnData) = target.call(data);
    }
}