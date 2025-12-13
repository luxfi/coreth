# Lux Coreth - C-Chain Implementation

## Overview

Lux Coreth is the C-Chain (Contract Chain) implementation for Lux Network, providing EVM compatibility. This document tracks the modernization effort to slim down the codebase.

**Key Principle**: Use `github.com/luxfi/geth` for shared ethereum types. Zero ava-labs packages.

## Current vs Target Structure

### Directories to REMOVE (duplicated from luxfi/geth or not needed)

```bash
# Top-level directories - NOT NEEDED for C-chain VM
beacon/          # Eth2 beacon chain - lux doesn't use this
common/          # Already in luxfi/geth
console/         # Interactive REPL - not needed for VM
crypto/          # Already in luxfi/geth (or use luxfi/crypto)
ethdb/           # Already in luxfi/geth (5 backends - bloat)
ethstats/        # Ethereum network stats service
event/           # Already in luxfi/geth
graphql/         # GraphQL API - not essential
metrics/         # Already in luxfi/geth (use prometheus)
p2p/             # Full P2P stack - lux node handles this
rlp/             # Already in luxfi/geth
signer/          # Clef/external signer support
trie/            # Already in luxfi/geth (use triedb)
version/         # Move version info to params/

# Legacy CI - use GitHub Actions only
appveyor.yml
circle.yml

# cmd/ subdirectories to remove
cmd/blsync/      # Beacon light sync
cmd/clef/        # External signer
cmd/devp2p/      # P2P development tools
cmd/era/         # Era/history format
cmd/geth/        # Full geth node (using lux node instead)

# internal/ subdirectories to remove
internal/cmdtest/    # Testing for removed commands
internal/download/   # Geth binary downloads
internal/era/        # Era format support
internal/guide/      # Geth documentation guide
internal/jsre/       # JavaScript runtime for console
internal/reexec/     # Re-execution utilities
internal/web3ext/    # Web3 extensions for console
```

### Directories to KEEP

```bash
# Essential for C-chain VM
accounts/abi/     # ABI bindings (keep abigen, bind)
contracts/        # Lux contracts (PostQuantumVerifier, etc.)
core/             # Core blockchain logic
coreth/           # Lux coreth adapter
eth/              # Ethereum service (includes api_lux.go)
ethclient/        # Client library
log/              # Logging
miner/            # Block production
node/             # Node configuration
params/           # Chain parameters
precompile/       # Precompiled contracts
rpc/              # RPC handling
tests/            # Test utilities
triedb/           # Trie database

# cmd/ subdirectories to keep
cmd/abidump/      # ABI debugging
cmd/abigen/       # Contract binding generator
cmd/evm/          # EVM execution tool
cmd/utils/        # CLI utilities
cmd/workload/     # Workload testing

# Maybe keep (evaluate usage)
cmd/ethkey/       # Key management CLI
cmd/rlpdump/      # RLP debugging

# internal/ subdirectories to keep
internal/blocktest/      # Block testing
internal/debug/          # Debug utilities
internal/ethapi/         # Ethereum API implementation
internal/flags/          # CLI flags
internal/shutdowncheck/  # Shutdown checking
internal/syncx/          # Sync extensions
internal/testlog/        # Test logging
internal/testrand/       # Test randomness
internal/utesting/       # Unit testing
internal/version/        # Version info
```

### Directories to ADD (Lux-specific structure)

```bash
# Create these matching Ava's architecture
constants/        # Clean constants package
interfaces/       # Clean interfaces (vs interfaces.go file)
network/          # Simplified C-chain networking layer
plugin/           # VM plugin architecture
scripts/          # Build/test/lint scripts
sync/             # State and block sync handlers
utils/            # Utility functions
```

## go.mod Cleanup

### Dependencies to REMOVE
```go
// Cloud SDKs - not needed
github.com/Azure/azure-sdk-for-go/sdk/storage/azblob
github.com/aws/aws-sdk-go-v2/*
github.com/cloudflare/cloudflare-go

// P2P related - lux node handles networking
github.com/pion/stun/v2

// Eth2/Beacon related
github.com/ferranbt/fastssz
github.com/protolambda/bls12-381-util
github.com/protolambda/zrnt
github.com/protolambda/ztyp

// GraphQL - if removing graphql/
github.com/graph-gophers/graphql-go

// Hardware wallet - not needed for VM
github.com/karalabe/hid
github.com/status-im/keycard-go
github.com/gballet/go-libpcsclite

// Console related
github.com/dop251/goja
github.com/peterh/liner
github.com/naoina/toml

// DNS/NAT - lux node handles
github.com/huin/goupnp
github.com/jackpal/go-nat-pmp
github.com/pion/*
```

### Dependencies to KEEP
```go
github.com/luxfi/geth           // Core EVM types
github.com/luxfi/node           // Lux node integration
github.com/luxfi/consensus      // Consensus
github.com/luxfi/crypto         // Cryptography
github.com/luxfi/database       // Database
github.com/luxfi/ids            // ID types

github.com/VictoriaMetrics/fastcache  // Caching
github.com/cockroachdb/pebble         // Database
github.com/holiman/uint256            // Big integers
github.com/prometheus/client_golang   // Metrics
github.com/stretchr/testify           // Testing
github.com/urfave/cli/v2              // CLI
golang.org/x/*                        // Standard extensions
```

## Implementation Script

```bash
#!/bin/bash
# slim-coreth.sh - Modernize lux/coreth
set -e

CORETH_DIR="/Users/z/work/lux/coreth"
cd "$CORETH_DIR"

echo "=== Phase 1: Remove unused top-level directories ==="
# These are duplicated in luxfi/geth or not needed for C-chain
trash beacon/ 2>/dev/null || true
trash common/ 2>/dev/null || true
trash console/ 2>/dev/null || true
trash crypto/ 2>/dev/null || true
trash ethdb/ 2>/dev/null || true
trash ethstats/ 2>/dev/null || true
trash event/ 2>/dev/null || true
trash graphql/ 2>/dev/null || true
trash metrics/ 2>/dev/null || true
trash p2p/ 2>/dev/null || true
trash rlp/ 2>/dev/null || true
trash signer/ 2>/dev/null || true
trash trie/ 2>/dev/null || true
trash version/ 2>/dev/null || true

echo "=== Phase 2: Remove unused cmd/ subdirectories ==="
trash cmd/blsync/ 2>/dev/null || true
trash cmd/clef/ 2>/dev/null || true
trash cmd/devp2p/ 2>/dev/null || true
trash cmd/era/ 2>/dev/null || true
trash cmd/geth/ 2>/dev/null || true

echo "=== Phase 3: Remove unused internal/ subdirectories ==="
trash internal/cmdtest/ 2>/dev/null || true
trash internal/download/ 2>/dev/null || true
trash internal/era/ 2>/dev/null || true
trash internal/guide/ 2>/dev/null || true
trash internal/jsre/ 2>/dev/null || true
trash internal/reexec/ 2>/dev/null || true
trash internal/web3ext/ 2>/dev/null || true

echo "=== Phase 4: Remove legacy CI files ==="
trash appveyor.yml 2>/dev/null || true
trash circle.yml 2>/dev/null || true

echo "=== Phase 5: Remove unused Dockerfiles ==="
# Keep only Dockerfile, remove alltools variant
trash Dockerfile.alltools 2>/dev/null || true

echo "=== Phase 6: Create new structure ==="
mkdir -p constants
mkdir -p interfaces
mkdir -p network
mkdir -p plugin
mkdir -p scripts
mkdir -p sync
mkdir -p utils

echo "=== Phase 7: Move interfaces.go to interfaces/ ==="
if [ -f interfaces.go ]; then
    mv interfaces.go interfaces/interfaces.go
fi

echo "=== Done! Next steps: ==="
echo "1. Update go.mod - remove unused dependencies"
echo "2. Update imports in remaining files"
echo "3. go mod tidy"
echo "4. go build ./..."
echo "5. go test ./..."
```

## Key Files to Preserve

### Lux-Specific Code
- `eth/api_lux.go` - Lux RPC API (ImportJSONL, ImportBlock, Status)
- `contracts/PostQuantumVerifier.sol` - Post-quantum cryptography
- `contracts/MLDSAVerifier.sol` - ML-DSA verification
- `precompile/` - Lux precompiled contracts
- `coreth/unified_adapter.go` - Node integration

### Configuration
- `.github/workflows/` - CI/CD
- `Makefile` - Build system
- `go.mod`, `go.sum` - Dependencies

## Testing Strategy

After each phase:
```bash
go mod tidy
go build ./...
go test ./... -short
```

Full validation:
```bash
go test ./... -race -cover
```

## File Count Comparison

| State | Directories | Files |
|-------|-------------|-------|
| Current (lux/coreth) | ~220 | ~354 |
| Target (after cleanup) | ~100 | ~180 |
| Reference (ava/coreth) | ~118 | ~191 |

## Notes

- All imports should use `github.com/luxfi/geth` for EVM types
- No `ava-labs` packages allowed
- Keep post-quantum cryptography contracts
- Keep Lux-specific precompiles
- The `api_lux.go` contains essential Lux APIs
