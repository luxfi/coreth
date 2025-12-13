#!/bin/bash
# slim-coreth.sh - Modernize lux/coreth
set -e

CORETH_DIR="/Users/z/work/lux/coreth"
cd "$CORETH_DIR"

echo "=== Phase 1: Remove unused top-level directories ==="
# These are duplicated in luxfi/geth or not needed for C-chain
for dir in beacon common console crypto ethdb ethstats event graphql metrics p2p rlp signer trie version; do
    if [ -d "$dir" ]; then
        echo "Removing $dir/"
        trash "$dir" 2>/dev/null || rm -rf "$dir"
    fi
done

echo "=== Phase 2: Remove unused cmd/ subdirectories ==="
for dir in blsync clef devp2p era geth; do
    if [ -d "cmd/$dir" ]; then
        echo "Removing cmd/$dir/"
        trash "cmd/$dir" 2>/dev/null || rm -rf "cmd/$dir"
    fi
done

echo "=== Phase 3: Remove unused internal/ subdirectories ==="
for dir in cmdtest download era guide jsre reexec web3ext; do
    if [ -d "internal/$dir" ]; then
        echo "Removing internal/$dir/"
        trash "internal/$dir" 2>/dev/null || rm -rf "internal/$dir"
    fi
done

echo "=== Phase 4: Remove legacy CI files ==="
for file in appveyor.yml circle.yml Dockerfile.alltools; do
    if [ -f "$file" ]; then
        echo "Removing $file"
        trash "$file" 2>/dev/null || rm -f "$file"
    fi
done

echo "=== Phase 5: Create new structure ==="
mkdir -p constants
mkdir -p interfaces
mkdir -p network
mkdir -p plugin
mkdir -p scripts
mkdir -p sync
mkdir -p utils

echo "=== Phase 6: Move interfaces.go to interfaces/ ==="
if [ -f interfaces.go ]; then
    # Update package name and move
    sed 's/^package ethereum$/package interfaces/' interfaces.go > interfaces/interfaces.go
    rm interfaces.go
fi

echo "=== Complete! ==="
echo "Removed directories and created new structure."
echo ""
echo "Next steps:"
echo "1. Run: go mod tidy"
echo "2. Run: go build ./..."
echo "3. Run: go test ./..."
