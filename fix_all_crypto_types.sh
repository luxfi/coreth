#!/bin/bash

# Find all files with crypto.Keccak256Hash that need conversion
echo "Fixing crypto.Keccak256Hash conversions..."

# Fix patterns like: functionCall(crypto.Keccak256Hash(...))
find . -name "*.go" -type f -exec perl -i -pe 's/([a-zA-Z_][a-zA-Z0-9_]*)\((crypto\.Keccak256Hash\([^)]+\))\)/$1(common.Hash($2))/g' {} \;

# Fix patterns like: variable = crypto.Keccak256Hash(...)
find . -name "*.go" -type f -exec perl -i -pe 's/^(\s*)([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*(crypto\.Keccak256Hash\([^)]+\))$/$1h := $3\n$1$2 = common.Hash(h)/g' {} \;

# Fix patterns like: field: crypto.Keccak256Hash(...)
find . -name "*.go" -type f -exec perl -i -pe 's/^(\s*)([a-zA-Z_][a-zA-Z0-9_]*):\s*(crypto\.Keccak256Hash\([^)]+\)),?$/$1$2: common.Hash($3),/g' {} \;

# Fix map index patterns like: map[hash] where hash is crypto.Hash
find . -name "*.go" -type f -exec perl -i -pe 's/\[([a-zA-Z_][a-zA-Z0-9_]*Hash)\]/[common.Hash($1)]/g if /crypto\.Hash/' {} \;

echo "Done!"
