#!/bin/bash

# HomeX Backend - Fix Import Paths Script
# This script fixes any incorrect import paths in Go files

echo "🔧 Fixing import paths in HomeX backend..."

# The correct module path from go.mod
CORRECT_MODULE="homexai"

# Find all Go files and fix incorrect imports
find . -name "*.go" -type f | while read -r file; do
    # Check if file contains incorrect import paths
    if grep -q "\"homexai/" "$file" 2>/dev/null; then
        echo "Fixing: $file"
        # Replace homexai with correct module path
        sed -i.bak 's|"homexai/|"homexai/|g' "$file"
        rm -f "$file.bak"
    fi
done

echo "✅ Import paths fixed!"
echo ""
echo "Now run:"
echo "  go mod tidy"
echo "  make build"
