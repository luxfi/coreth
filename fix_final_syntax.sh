#!/bin/bash

# Fix double closing parentheses
find . -name "*.go" -type f -exec sed -i 's/common\.Hash(crypto\.Keccak256Hash(\([^)]*\)))/common.Hash(crypto.Keccak256Hash(\1))/g' {} \;

# Fix any remaining double common.Hash wrapping
find . -name "*.go" -type f -exec sed -i 's/common\.Hash(common\.Hash(/common.Hash(/g' {} \;

