#!/bin/bash
# Test runner script for MCP client tests
# This script can be used inside the test client container

set -e

echo "=== MCP Client Test Runner ==="
echo ""

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not available"
    exit 1
fi

# Check if pytest is available
if ! command -v pytest &> /dev/null; then
    echo "Error: pytest is not installed"
    exit 1
fi

# Set defaults
TEST_DIR="${TEST_DIR:-/test}"
USE_LOCAL_IMAGES="${USE_LOCAL_IMAGES:-}"

echo "Test directory: $TEST_DIR"
echo "Use local images: ${USE_LOCAL_IMAGES:-no}"
echo ""

# Run pytest
cd "$TEST_DIR"
pytest -v "$@"

echo ""
echo "=== Tests Complete ==="
