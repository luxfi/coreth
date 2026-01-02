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
metrics/prometheus
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
