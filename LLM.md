# coreth

## Overview

Each blockchain is an instance of a Virtual Machine (VM), much like an object in an object-oriented language is an instance of a class. That is, the VM defines the behavior of the blockchain. 

## Package Information

- **Type**: go
- **Module**: github.com/luxfi/coreth
- **Repository**: github.com/luxfi/coreth

## Directory Structure

```
.
cmd
cmd/abigen
cmd/simulator
cmd/utils
metrics
metrics/metricstest
metrics/gatherer
precompile
precompile/contract
precompile/contracts
precompile/modules
precompile/precompileconfig
precompile/precompiletest
precompile/registry
```

## Key Files

- go.mod
- tools.go

## Development

### Prerequisites

- Go 1.21+

### Build

```bash
go build ./...
```

### Test

```bash
go test -v ./...
```

## Known Issues & Fixes

### `Dockerfile` is dead and is deliberately not maintained

The root `Dockerfile` cannot build, for three independent reasons, and no CI
workflow references it (`.github/workflows/` builds and tests only — it never
builds an image). It was left untouched during the fleet-wide Go 1.26.5 builder
bump precisely so this file does not read as current:

1. `FROM golang:1.26.4-bullseye` — that tag has never existed. The official
   `golang` images dropped Debian 11; the last bullseye tag published is
   `golang:1.24.6-bullseye`. Any Go >= 1.25 builder must be `bookworm` (or
   `trixie`/`alpine`), which is what the rest of the fleet uses.
2. `RUN ./scripts/build_lux.sh` — that script no longer exists anywhere in
   `luxfi/node`, on `main` or on any recent `v1.36.x` tag. The current entry
   point is `scripts/build.sh`.
3. `FROM debian:11-slim` as the runtime — EOL, and glibc-incompatible with any
   builder new enough to compile this module.

Fixing only the base image would leave the build failing at (2) while making
the file look maintained. Either repair all three against a real `LUX_VERSION`
and wire it to CI, or delete it.

### Block Import Persistence Fix (2025-12-31)

**Issue**: After importing blocks via `admin_importChain`, blocks were stored in the database but the consensus layer didn't recognize them as the chain head. This meant:
- `eth_blockNumber` returned the correct height
- But new blocks couldn't be produced (no validators thought they were building on top of imported blocks)
- The chain appeared "frozen" at the imported height

**Root Cause**: The `PostImportCallback` in `plugin/evm/vm.go` only updated `acceptedBlockDB` (persistence layer) but did NOT update `chain.State.lastAcceptedBlock` (consensus layer).

**Fix**: Updated `PostImportCallback` (vm.go:710-758) to also update the consensus state:
1. Get the eth block by hash from blockchain
2. Wrap it using `wrapBlock()` to create a `block.Block` interface
3. Call `v.SetLastAcceptedBlock(wrappedBlock)` to update consensus

**Key Code Change** (vm.go):
```go
// After updating acceptedBlockDB...
ethBlock := v.blockChain.GetBlockByHash(lastBlockHash)
wrappedBlock, err := wrapBlock(ethBlock, v)
if err := v.SetLastAcceptedBlock(wrappedBlock); err != nil {
    return fmt.Errorf("failed to set last accepted block in chain.State: %w", err)
}
```

**Impact**:
- Imported blocks are now recognized by consensus as the canonical chain head
- New blocks can be produced on top of imported blocks
- Essential for disaster recovery and chain migration scenarios

## Integration with Lux Ecosystem

This package is part of the Lux blockchain ecosystem. See the main documentation at:
- GitHub: https://github.com/luxfi
- Docs: https://docs.lux.network

---

*Last Updated: 2025-12-31*
