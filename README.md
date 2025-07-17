# Geth and the C-Chain

[Lux](https://www.lux.network/) is a network composed of multiple blockchains.
Each blockchain is an instance of a Virtual Machine (VM), much like an object in an object-oriented language is an instance of a class.
That is, the VM defines the behavior of the blockchain.
Geth (from core Ethereum) is the [Virtual Machine (VM)](https://docs.lux.network/learn/virtual-machines) that defines the Contract Chain (C-Chain).
This chain implements the Ethereum Virtual Machine and supports Solidity smart contracts as well as most other Ethereum client functionality.

## Building

Geth is a dependency of Lux which is used to implement the EVM based Virtual Machine for the Lux C-Chain. In order to run with a local version of Geth, users must update their Geth dependency within Lux to point to their local Geth directory. If Geth and Lux are at the standard location within your GOPATH, this will look like the following:

```bash
cd $GOPATH/src/github.com/luxfi/node
go mod edit -replace github.com/luxfi/geth=../geth
```

Now that Lux depends on the local version of Geth, we can build with the normal build script:

```bash
./scripts/build.sh
./build/node
```

Note: the C-Chain originally ran in a separate process from the main Lux process and communicated with it over a local gRPC connection. When this was the case, Lux's build script would download Geth, compile it, and place the binary into the `node/build/plugins` directory.

## API

The C-Chain supports the following API namespaces:

- `eth`
- `personal`
- `txpool`
- `debug`

Only the `eth` namespace is enabled by default. 
To enable the other namespaces see the instructions for passing the C-Chain config to Lux [here.](https://docs.lux.network/nodes/configure/chain-config-flags#enabling-evm-apis)
Full documentation for the C-Chain's API can be found [here.](https://docs.lux.network/reference/node/c-chain/api)

## Compatibility

The C-Chain is compatible with almost all Ethereum tooling, including [Core,](https://docs.lux.network/build/dapp/launch-dapp#through-core) [Metamask,](https://docs.lux.network/build/dapp/launch-dapp#through-metamask) [Remix](https://docs.lux.network/dapps/smart-contract-dev/deploy-with-remix-ide) and [Truffle.](https://docs.lux.network/build/tutorials/smart-contracts/using-truffle-with-the-lux-c-chain)

## Differences Between Lux C-Chain and Ethereum

### Atomic Transactions

As a network composed of multiple blockchains, Lux uses *atomic transactions* to move assets between chains. Geth modifies the Ethereum block format by adding an *ExtraData* field, which contains the atomic transactions.

### Block Timing

Blocks are produced asynchronously in Snowman Consensus, so the timing assumptions that apply to Ethereum do not apply to Geth. To support block production in an async environment, a block is permitted to have the same timestamp as its parent. Since there is no general assumption that a block will be produced every 10 seconds, smart contracts built on Lux should use the block timestamp instead of the block number for their timing assumptions.

A block with a timestamp more than 10 seconds in the future will not be considered valid. However, a block with a timestamp more than 10 seconds in the past will still be considered valid as long as its timestamp is greater than or equal to the timestamp of its parent block.

## Difficulty and Random OpCode

Snowman consensus does not use difficulty in any way, so the difficulty of every block is required to be set to 1. This means that the DIFFICULTY opcode should not be used as a source of randomness.

Additionally, with the change from the DIFFICULTY OpCode to the RANDOM OpCode (RANDOM replaces DIFFICULTY directly), there is no planned change to provide a stronger source of randomness. The RANDOM OpCode relies on the Eth2.0 Randomness Beacon, which has no direct parallel within the context of either Geth or Snowman consensus. Therefore, instead of providing a weaker source of randomness that may be manipulated, the RANDOM OpCode will not be supported. Instead, it will continue the behavior of the DIFFICULTY OpCode of returning the block's difficulty, such that it will always return 1.

## Block Format

To support these changes, there have been a number of changes to the C-Chain block format compared to what exists on Ethereum.

### Block Body

* `Version`: provides version of the `ExtData` in the block. Currently, this field is always 0.
* `ExtData`: extra data field within the block body to store atomic transaction bytes.

### Block Header

* `ExtDataHash`: the hash of the bytes in the `ExtDataHash` field
* `BaseFee`: Added by EIP-1559 to represent the base fee of the block (present in Ethereum as of EIP-1559)
* `ExtDataGasUsed`: amount of gas consumed by the atomic transactions in the block
* `BlockGasCost`: surcharge for producing a block faster than the target rate
