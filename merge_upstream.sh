#!/bin/bash
# Script to merge upstream changes while preserving Lux branding and imports

echo "Creating backup branch..."
git checkout -b backup-lux-$(date +%Y%m%d-%H%M%S)
git checkout main

echo "Starting merge from upstream..."
git merge origin/master --no-commit --allow-unrelated-histories || true

echo "Preserving Lux-specific changes..."
# Reset files that should keep Lux branding
git checkout HEAD -- README.md
git checkout HEAD -- LICENSE

# Find and preserve Lux imports
find . -name "*.go" -type f | while read file; do
    # Check if file has conflicts
    if grep -q "<<<<<<< HEAD" "$file" 2>/dev/null; then
        echo "Resolving conflicts in $file..."
        # Backup the file
        cp "$file" "$file.merge"
        
        # Try to auto-resolve by keeping our imports
        sed -i '' '/"github.com\/luxfi\//,/^[[:space:]]*$/{
            /<<<<<<< HEAD/,/=======/d
            />>>>>>> /d
        }' "$file"
        
        # Remove remaining conflict markers if any
        sed -i '' '/<<<<<<< HEAD/,/>>>>>>>/d' "$file"
    fi
done

echo "Checking for Avalanche imports that need to be replaced..."
grep -r "github.com/ava-labs/avalanchego" . --include="*.go" | grep -v "vendor" | grep -v ".git" || true

echo "Done! Please review changes before committing."