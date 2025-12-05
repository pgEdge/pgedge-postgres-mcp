#!/bin/bash

# Test script for GoReleaser configuration
# This script tests the GoReleaser build locally without creating a release

set -e

echo "========================================="
echo "Testing GoReleaser Configuration"
echo "========================================="
echo ""

# Check if goreleaser is installed
if ! command -v goreleaser &> /dev/null; then
    echo "Error: goreleaser is not installed"
    echo "Install with: go install github.com/goreleaser/goreleaser@latest"
    echo "Or visit: https://goreleaser.com/install/"
    exit 1
fi

echo "✓ GoReleaser found: $(goreleaser --version | head -n1)"
echo ""

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "Error: Node.js is not installed"
    echo "Install from: https://nodejs.org/"
    exit 1
fi

echo "✓ Node.js found: $(node --version)"
echo ""

# Check if npm is installed
if ! command -v npm &> /dev/null; then
    echo "Error: npm is not installed"
    exit 1
fi

echo "✓ npm found: $(npm --version)"
echo ""

# Clean previous test builds
echo "Cleaning previous builds..."
rm -rf dist/
rm -rf web/dist/
echo "✓ Cleanup complete"
echo ""

# Install web dependencies
echo "Installing web dependencies..."
cd web
npm ci
cd ..
echo "✓ Web dependencies installed"
echo ""

# Build web UI
echo "Building Web UI..."
cd web
npm run build
cd ..
echo "✓ Web UI built"
echo ""

# Verify web build
if [ ! -d "web/dist" ]; then
    echo "✗ Error: web/dist directory not found"
    exit 1
fi

if [ ! -f "web/dist/index.html" ]; then
    echo "✗ Error: web/dist/index.html not found"
    exit 1
fi

echo "✓ Web UI build verified"
echo ""

# Run tests
echo "Running Go tests..."
go test ./... || {
    echo "✗ Tests failed"
    exit 1
}
echo "✓ Tests passed"
echo ""

# Test goreleaser build
echo "Testing GoReleaser build (snapshot mode)..."
goreleaser release --snapshot --clean --skip=publish

echo ""
echo "========================================="
echo "Build Test Complete!"
echo "========================================="
echo ""
echo "Generated artifacts in dist/:"
ls -lh dist/*.tar.gz dist/*.zip 2>/dev/null || true
echo ""
echo "Checksums:"
cat dist/checksums.txt 2>/dev/null || echo "No checksums file found"
echo ""
echo "To test a specific archive, extract it and test the binary:"
echo "  tar -xzf dist/pgedge-nla-server_*_linux_x86_64.tar.gz"
echo "  ./pgedge-mcp-server --version"
echo ""
