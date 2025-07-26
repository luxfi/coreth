#!/bin/bash
# Simple build script for geth

set -e

# Create build directory if it doesn't exist
mkdir -p build/bin

# Build main executables
echo "Building geth..."
go build -o build/bin/geth ./cmd/geth

echo "Building evm..."
go build -o build/bin/evm ./cmd/evm

echo "Building abigen..."
go build -o build/bin/abigen ./cmd/abigen

echo "Building clef..."
go build -o build/bin/clef ./cmd/clef

echo "Building devp2p..."
go build -o build/bin/devp2p ./cmd/devp2p

echo "Building ethkey..."
go build -o build/bin/ethkey ./cmd/ethkey

echo "Building rlpdump..."
go build -o build/bin/rlpdump ./cmd/rlpdump

echo ""
echo "Build complete! Binaries are in build/bin/"
echo "Run './build/bin/geth --help' to see available options"