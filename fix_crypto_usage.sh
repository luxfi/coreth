#!/bin/bash

# Fix all crypto.Keccak256Hash direct returns
find triedb/pathdb -name "*.go" -exec sed -i 's/return crypto.Keccak256Hash(\(.*\)), nil/h := crypto.Keccak256Hash(\1); return common.Hash(h), nil/g' {} \;

# Fix all crypto.Keccak256Hash assignments
find triedb/pathdb -name "*.go" -exec sed -i 's/\([[:space:]]*\)\([a-zA-Z_][a-zA-Z0-9_]*\) = crypto.Keccak256Hash(\(.*\))$/\1h := crypto.Keccak256Hash(\3)\n\1\2 = common.Hash(h)/g' {} \;

# Fix addrHash usage
find triedb/pathdb -name "*.go" -exec sed -i 's/\(trie\.StorageTrieID.*\)addrHash/\1common.Hash(addrHash)/g' {} \;

# Fix map index usage  
find triedb/pathdb -name "*.go" -exec sed -i 's/\[addrHash\]/[common.Hash(addrHash)]/g' {} \;
find triedb/pathdb -name "*.go" -exec sed -i 's/\[crypto.Keccak256Hash(\(.*\))\]/[common.Hash(crypto.Keccak256Hash(\1))]/g' {} \;

