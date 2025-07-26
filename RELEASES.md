# Release Notes

## Historical Timeline

### Private Testing Phase (January 2021)
- Initial private alpha testing and benchmarking
- Based on early Coreth implementation
- Used for internal evaluation and performance testing
- Testing C-Chain functionality

### Mainnet Beta Launch (January 1, 2022)
- Lux C-Chain launched as primary EVM chain
- Based on coreth v0.8.x (contemporary with avalanchego v1.7.x)
- Initial production deployment
- Full EVM compatibility established

### Second Major Update (January 2023)
- Network upgrade to coreth v0.11.x (contemporary with avalanchego v1.9.x)
- Enhanced stability and performance
- Improved transaction processing
- Added new RPC methods

### Public Mainnet (November 2024)
- Upgraded to coreth v0.13.x (contemporary with avalanchego v1.11.x)
- Public mainnet launch
- Enhanced state management
- Running stable through July 2025

### Current Sync (July 26, 2025)
- Full synchronization with latest upstream versions
- Now at parity with coreth v0.15.2
- All tests passing with upstream compatibility
- Final compatibility build for Lux network

---

## Current Releases

## [v0.15.2](https://github.com/luxfi/geth/releases/tag/v0.15.2)

**First Official Tagged Release of Lux Geth**

This is the first officially tagged release of Lux Geth (C-Chain implementation), fully synchronized with coreth v0.15.2.

### Key Features

- Full Ethereum compatibility for the C-Chain
- Support for Lux mainnet C-Chain operations
- Optimized for Lux network consensus
- Built with Go 1.24.5
- All upstream tests passing

### Major Components

- **EVM Engine**: Full Ethereum Virtual Machine
- **State Management**: Efficient trie and database layer
- **RPC Server**: Complete JSON-RPC API
- **Transaction Pool**: Optimized mempool management
- **Consensus Integration**: Seamless integration with Lux consensus

### Configuration

- Integrated with Lux node as C-Chain
- Support for all standard Ethereum RPC methods
- Compatible with MetaMask and Web3 tooling
- Optimized gas pricing for Lux network

### Network Support

- **C-Chain**: Primary EVM chain on Lux network
- **Cross-Chain**: Atomic swaps with X-Chain and P-Chain
- **Bridge Support**: Compatible with bridge infrastructure
- **Smart Contracts**: Full Solidity support

### Development

- Extensive test suite with full upstream compatibility
- Docker support
- Comprehensive documentation
- Active development and community support