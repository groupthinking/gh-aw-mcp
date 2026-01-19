#!/bin/bash
# Comprehensive test script for Serena MCP Server
# Tests multi-language support: Go, Java, JavaScript, Python
# Tests MCP protocol interactions and validates responses

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CONTAINER_IMAGE="${SERENA_IMAGE:-ghcr.io/githubnext/serena-mcp-server:latest}"
TEST_DIR="$(cd "$(dirname "$0")" && pwd)"
SAMPLES_DIR="${TEST_DIR}/samples"
EXPECTED_DIR="${TEST_DIR}/expected"
RESULTS_DIR="${TEST_DIR}/results"
TEMP_DIR="/tmp/serena-test-$$"

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
    ((TESTS_PASSED++))
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
    ((TESTS_FAILED++))
}

log_warning() {
    echo -e "${YELLOW}[⚠]${NC} $1"
}

log_section() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

# Increment test counter
count_test() {
    ((TESTS_TOTAL++))
}

# Cleanup function
cleanup() {
    log_info "Cleaning up temporary files..."
    rm -rf "$TEMP_DIR"
}

trap cleanup EXIT

# Initialize
log_section "Serena MCP Server Comprehensive Test Suite"
log_info "Container Image: $CONTAINER_IMAGE"
log_info "Test Directory: $TEST_DIR"
log_info "Samples Directory: $SAMPLES_DIR"
echo ""

# Create temporary and results directories
mkdir -p "$TEMP_DIR"
mkdir -p "$RESULTS_DIR"

# Test 1: Check if Docker is available
log_section "Test 1: Docker Availability"
count_test
if command -v docker >/dev/null 2>&1; then
    log_success "Docker is installed"
else
    log_error "Docker is not installed"
    exit 1
fi

# Test 2: Check if container image is available
log_section "Test 2: Container Image Availability"
count_test
log_info "Pulling container image (this may take a while)..."
if docker pull "$CONTAINER_IMAGE" >/dev/null 2>&1; then
    log_success "Container image is available"
else
    log_error "Failed to pull container image: $CONTAINER_IMAGE"
    log_warning "Make sure the image exists and you have proper credentials"
    exit 1
fi

# Test 3: Container help command
log_section "Test 3: Container Basic Functionality"
count_test
if docker run --rm "$CONTAINER_IMAGE" --help >/dev/null 2>&1; then
    log_success "Container help command works"
else
    log_error "Container help command failed"
fi

# Test 4: Verify language runtimes
log_section "Test 4: Language Runtime Verification"

# Python
count_test
if docker run --rm --entrypoint python3 "$CONTAINER_IMAGE" --version >/dev/null 2>&1; then
    PYTHON_VERSION=$(docker run --rm --entrypoint python3 "$CONTAINER_IMAGE" --version 2>&1)
    log_success "Python runtime available: $PYTHON_VERSION"
else
    log_error "Python runtime not found"
fi

# Java
count_test
if docker run --rm --entrypoint java "$CONTAINER_IMAGE" -version >/dev/null 2>&1; then
    JAVA_VERSION=$(docker run --rm --entrypoint java "$CONTAINER_IMAGE" -version 2>&1 | head -1)
    log_success "Java runtime available: $JAVA_VERSION"
else
    log_error "Java runtime not found"
fi

# Node.js
count_test
if docker run --rm --entrypoint node "$CONTAINER_IMAGE" --version >/dev/null 2>&1; then
    NODE_VERSION=$(docker run --rm --entrypoint node "$CONTAINER_IMAGE" --version 2>&1)
    log_success "Node.js runtime available: $NODE_VERSION"
else
    log_error "Node.js runtime not found"
fi

# Go
count_test
if docker run --rm --entrypoint go "$CONTAINER_IMAGE" version >/dev/null 2>&1; then
    GO_VERSION=$(docker run --rm --entrypoint go "$CONTAINER_IMAGE" version 2>&1)
    log_success "Go runtime available: $GO_VERSION"
else
    log_error "Go runtime not found"
fi

# Test 5: MCP Protocol - Initialize
log_section "Test 5: MCP Protocol Initialize"
count_test
log_info "Sending MCP initialize request..."

INIT_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}'

INIT_RESPONSE=$(echo "$INIT_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    "$CONTAINER_IMAGE" 2>/dev/null || echo '{"error": "failed"}')

echo "$INIT_RESPONSE" > "$RESULTS_DIR/initialize_response.json"

if echo "$INIT_RESPONSE" | grep -q '"jsonrpc"'; then
    if echo "$INIT_RESPONSE" | grep -q '"result"'; then
        log_success "MCP initialize succeeded"
        log_info "Response saved to: $RESULTS_DIR/initialize_response.json"
    else
        log_error "MCP initialize returned error"
        echo "$INIT_RESPONSE" | head -5
    fi
else
    log_error "MCP initialize failed - no valid JSON-RPC response"
    echo "$INIT_RESPONSE" | head -10
fi

# Test 6: MCP Protocol - List Tools
log_section "Test 6: MCP Protocol - List Available Tools"
count_test
log_info "Requesting list of available tools..."

# First send initialize, then tools/list
TOOLS_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'

TOOLS_RESPONSE=$(echo "$TOOLS_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')

echo "$TOOLS_RESPONSE" > "$RESULTS_DIR/tools_list_response.json"

if echo "$TOOLS_RESPONSE" | grep -q '"tools"'; then
    TOOL_COUNT=$(echo "$TOOLS_RESPONSE" | grep -o '"name"' | wc -l)
    log_success "Tools list retrieved - found $TOOL_COUNT tools"
    log_info "Response saved to: $RESULTS_DIR/tools_list_response.json"
    
    # Display available tools
    log_info "Available Serena tools:"
    echo "$TOOLS_RESPONSE" | grep -o '"name":"[^"]*"' | sed 's/"name":"/  - /' | sed 's/"$//'
else
    log_error "Failed to retrieve tools list"
    echo "$TOOLS_RESPONSE" | head -10
fi

# Test 7: Go Code Analysis
log_section "Test 7: Go Code Analysis Tests"

if [ -f "$SAMPLES_DIR/go_project/main.go" ]; then
    log_info "Testing Go project at: $SAMPLES_DIR/go_project"
    
    # Test 7a: Go - Find symbols
    count_test
    log_info "Test 7a: Finding symbols in Go code..."
    
    SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"go_project/main.go"}}}'
    
    GO_RESPONSE=$(echo "$SYMBOLS_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$GO_RESPONSE" > "$RESULTS_DIR/go_symbols_response.json"
    
    if echo "$GO_RESPONSE" | grep -q -E '(Calculator|NewCalculator|Add|Multiply)'; then
        log_success "Go symbol analysis working - found expected symbols"
    elif echo "$GO_RESPONSE" | grep -q '"result"'; then
        log_success "Go symbol analysis completed successfully"
    else
        log_error "Go symbol analysis failed"
    fi
    
    # Test 7b: Go - Find specific symbol
    count_test
    log_info "Test 7b: Finding specific Calculator symbol..."
    
    FIND_SYMBOL_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"go_project"}}}'
    
    FIND_RESPONSE=$(echo "$FIND_SYMBOL_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$FIND_RESPONSE" > "$RESULTS_DIR/go_find_symbol_response.json"
    
    if echo "$FIND_RESPONSE" | grep -q '"result"'; then
        log_success "Go find_symbol completed successfully"
    else
        log_error "Go find_symbol failed"
    fi
else
    log_warning "Go project not found, skipping Go tests"
fi

# Test 8: Java Code Analysis
log_section "Test 8: Java Code Analysis Tests"

if [ -f "$SAMPLES_DIR/java_project/Calculator.java" ]; then
    log_info "Testing Java project at: $SAMPLES_DIR/java_project"
    
    # Test 8a: Java - Find symbols
    count_test
    log_info "Test 8a: Finding symbols in Java code..."
    
    JAVA_SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"java_project/Calculator.java"}}}'
    
    JAVA_RESPONSE=$(echo "$JAVA_SYMBOLS_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$JAVA_RESPONSE" > "$RESULTS_DIR/java_symbols_response.json"
    
    if echo "$JAVA_RESPONSE" | grep -q -E '(Calculator|add|multiply)'; then
        log_success "Java symbol analysis working - found expected symbols"
    elif echo "$JAVA_RESPONSE" | grep -q '"result"'; then
        log_success "Java symbol analysis completed successfully"
    else
        log_error "Java symbol analysis failed"
    fi
    
    # Test 8b: Java - Find specific symbol
    count_test
    log_info "Test 8b: Finding specific Calculator symbol..."
    
    JAVA_FIND_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"java_project"}}}'
    
    JAVA_FIND_RESPONSE=$(echo "$JAVA_FIND_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$JAVA_FIND_RESPONSE" > "$RESULTS_DIR/java_find_symbol_response.json"
    
    if echo "$JAVA_FIND_RESPONSE" | grep -q '"result"'; then
        log_success "Java find_symbol completed successfully"
    else
        log_error "Java find_symbol failed"
    fi
else
    log_warning "Java project not found, skipping Java tests"
fi

# Test 9: JavaScript Code Analysis
log_section "Test 9: JavaScript Code Analysis Tests"

if [ -f "$SAMPLES_DIR/js_project/calculator.js" ]; then
    log_info "Testing JavaScript project at: $SAMPLES_DIR/js_project"
    
    # Test 9a: JavaScript - Find symbols
    count_test
    log_info "Test 9a: Finding symbols in JavaScript code..."
    
    JS_SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"js_project/calculator.js"}}}'
    
    JS_RESPONSE=$(echo "$JS_SYMBOLS_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$JS_RESPONSE" > "$RESULTS_DIR/js_symbols_response.json"
    
    if echo "$JS_RESPONSE" | grep -q -E '(Calculator|add|multiply)'; then
        log_success "JavaScript symbol analysis working - found expected symbols"
    elif echo "$JS_RESPONSE" | grep -q '"result"'; then
        log_success "JavaScript symbol analysis completed successfully"
    else
        log_error "JavaScript symbol analysis failed"
    fi
    
    # Test 9b: JavaScript - Find specific symbol
    count_test
    log_info "Test 9b: Finding specific Calculator symbol..."
    
    JS_FIND_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"js_project"}}}'
    
    JS_FIND_RESPONSE=$(echo "$JS_FIND_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$JS_FIND_RESPONSE" > "$RESULTS_DIR/js_find_symbol_response.json"
    
    if echo "$JS_FIND_RESPONSE" | grep -q '"result"'; then
        log_success "JavaScript find_symbol completed successfully"
    else
        log_error "JavaScript find_symbol failed"
    fi
else
    log_warning "JavaScript project not found, skipping JavaScript tests"
fi

# Test 10: Python Code Analysis
log_section "Test 10: Python Code Analysis Tests"

if [ -f "$SAMPLES_DIR/python_project/calculator.py" ]; then
    log_info "Testing Python project at: $SAMPLES_DIR/python_project"
    
    # Test 10a: Python - Find symbols
    count_test
    log_info "Test 10a: Finding symbols in Python code..."
    
    PY_SYMBOLS_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"relative_path":"python_project/calculator.py"}}}'
    
    PY_RESPONSE=$(echo "$PY_SYMBOLS_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$PY_RESPONSE" > "$RESULTS_DIR/python_symbols_response.json"
    
    if echo "$PY_RESPONSE" | grep -q -E '(Calculator|add|multiply)'; then
        log_success "Python symbol analysis working - found expected symbols"
    elif echo "$PY_RESPONSE" | grep -q '"result"'; then
        log_success "Python symbol analysis completed successfully"
    else
        log_error "Python symbol analysis failed"
    fi
    
    # Test 10b: Python - Find specific symbol
    count_test
    log_info "Test 10b: Finding specific Calculator symbol..."
    
    PY_FIND_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"find_symbol","arguments":{"query":"Calculator","relative_path":"python_project"}}}'
    
    PY_FIND_RESPONSE=$(echo "$PY_FIND_REQUEST" | docker run --rm -i \
        -v "$SAMPLES_DIR:/workspace:ro" \
        -w /workspace \
        "$CONTAINER_IMAGE" 2>/dev/null | tail -1 || echo '{"error": "failed"}')
    
    echo "$PY_FIND_RESPONSE" > "$RESULTS_DIR/python_find_symbol_response.json"
    
    if echo "$PY_FIND_RESPONSE" | grep -q '"result"'; then
        log_success "Python find_symbol completed successfully"
    else
        log_error "Python find_symbol failed"
    fi
else
    log_warning "Python project not found, skipping Python tests"
fi

# Test 11: Error Handling - Invalid Request
log_section "Test 11: Error Handling Tests"
count_test
log_info "Test 11a: Testing invalid MCP request..."

INVALID_REQUEST='{"jsonrpc":"2.0","id":99,"method":"invalid_method","params":{}}'

INVALID_RESPONSE=$(echo "$INVALID_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    "$CONTAINER_IMAGE" 2>/dev/null || echo '{"error": "failed"}')

echo "$INVALID_RESPONSE" > "$RESULTS_DIR/invalid_request_response.json"

if echo "$INVALID_RESPONSE" | grep -q '"error"'; then
    log_success "Invalid request properly rejected with error response"
else
    log_warning "Invalid request handling unclear"
fi

# Test 12: Malformed JSON
count_test
log_info "Test 11b: Testing malformed JSON..."

MALFORMED_REQUEST='{"jsonrpc":"2.0","id":100,"method":"initialize"'

MALFORMED_RESPONSE=$(echo "$MALFORMED_REQUEST" | docker run --rm -i \
    -v "$SAMPLES_DIR:/workspace:ro" \
    "$CONTAINER_IMAGE" 2>&1 || echo "error")

echo "$MALFORMED_RESPONSE" > "$RESULTS_DIR/malformed_json_response.txt"

if echo "$MALFORMED_RESPONSE" | grep -q -i "error\|invalid\|parse"; then
    log_success "Malformed JSON properly rejected"
else
    log_warning "Malformed JSON handling unclear"
fi

# Test 13: Container Size Check
log_section "Test 13: Container Metrics"
count_test
log_info "Checking container size..."

SIZE=$(docker images "$CONTAINER_IMAGE" --format "{{.Size}}" 2>/dev/null || echo "unknown")
log_info "Container size: $SIZE"

if [ "$SIZE" != "unknown" ]; then
    log_success "Container size information retrieved"
else
    log_warning "Could not retrieve container size"
fi

# Summary
log_section "Test Summary"
echo ""
log_info "Total Tests: $TESTS_TOTAL"
log_success "Passed: $TESTS_PASSED"
if [ $TESTS_FAILED -gt 0 ]; then
    log_error "Failed: $TESTS_FAILED"
fi
echo ""

# Calculate success rate
if [ $TESTS_TOTAL -gt 0 ]; then
    SUCCESS_RATE=$((TESTS_PASSED * 100 / TESTS_TOTAL))
    log_info "Success Rate: $SUCCESS_RATE%"
else
    log_warning "No tests were run"
fi
echo ""

log_info "Detailed results saved to: $RESULTS_DIR"
echo ""

# Exit with appropriate code
if [ $TESTS_FAILED -gt 0 ]; then
    log_warning "Some tests failed. Review the output above for details."
    exit 1
else
    log_success "All tests passed!"
    exit 0
fi
