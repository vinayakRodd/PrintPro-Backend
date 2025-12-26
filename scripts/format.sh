#!/bin/bash

# Format script for Print Pro Backend
# Formats both Go code and non-Go files (markdown, JSON, etc.)

echo "🔧 Formatting code..."

# Format Go files
echo "📝 Formatting Go files..."
go fmt ./...

# Format non-Go files with Prettier (if pnpm is available)
if command -v pnpm &> /dev/null; then
    echo "✨ Formatting markdown, JSON, and other files with Prettier..."
    pnpm format
else
    echo "⚠️  pnpm not found. Install pnpm to format non-Go files."
    echo "   Run: npm install -g pnpm"
fi

echo "✅ Formatting complete!"

