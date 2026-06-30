# DEPRECATED: coreth

**This package is deprecated. Use `github.com/luxfi/evm` instead.**

## Migration Status

As of January 2025, the Lux C-Chain and subnet EVM implementation has been consolidated into a single canonical package:

- **Canonical package**: `github.com/luxfi/evm`
- **Deprecated package**: `github.com/luxfi/coreth` (this repo)

## Why the Change

1. **Single implementation**: Eliminates divergence between C-Chain (coreth) and Subnet EVM implementations
2. **Simplified maintenance**: One codebase to maintain, test, and audit
3. **Consistent behavior**: All EVM chains on Lux use identical code paths

## Migration Guide

Replace imports:
```go
// OLD (deprecated)
import "github.com/luxfi/coreth/..."

// NEW (canonical)
import "github.com/luxfi/evm/..."
```

Update go.mod:
```
// Remove
require github.com/luxfi/coreth v1.x.x

// Add
require github.com/luxfi/evm v0.8.x
```

## Package Mapping

| coreth path | evm path |
|-------------|----------|
| `github.com/luxfi/coreth/core` | `github.com/luxfi/evm/core` |
| `github.com/luxfi/coreth/params` | `github.com/luxfi/evm/params` |
| `github.com/luxfi/coreth/plugin/evm` | `github.com/luxfi/evm/plugin/evm` |
| `github.com/luxfi/coreth/eth` | `github.com/luxfi/evm/eth` |

## Historic Chain Compatibility

The `github.com/luxfi/evm` package maintains full compatibility with historic Lux C-Chain blocks:
- Genesis hash: `0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e`
- Chain ID: 96369
- SubnetEVM gas accounting rules supported via `subnetEVMTimestamp` config

## Timeline

- **2025-01**: Deprecation announced
- **2025-Q2**: No new features in coreth; security fixes only
- **2025-Q3**: Archive-only; no maintenance

## Questions

For migration assistance, open an issue in `github.com/luxfi/evm`.
