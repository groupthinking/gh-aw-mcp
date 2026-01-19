# Serena MCP Server Test Suite

Comprehensive shell script tests for the Serena MCP Server (`ghcr.io/githubnext/serena-mcp-server:latest`).

## Quick Start

### Run Tests with Make

The easiest way to run the tests:

```bash
make test-serena
```

### Run Tests Directly

From the repository root:
```bash
./test/serena-mcp-tests/test_serena.sh
```

Or from this directory:
```bash
cd test/serena-mcp-tests
./test_serena.sh
```

## Overview

This test suite validates that the Serena MCP Server correctly supports multiple programming languages (Go, Java, JavaScript, and Python) through the Model Context Protocol (MCP). The tests include sample codebases with known structures and verify that the server provides correct responses for various code analysis operations.

## Features

- **Multi-language Testing**: Tests Go, Java, JavaScript, and Python support
- **MCP Protocol Validation**: Tests initialization, tool listing, and tool invocation
- **Code Analysis Tests**: Validates symbol finding, diagnostics, and semantic analysis
- **Error Handling**: Tests invalid requests and malformed JSON handling
- **Detailed Reporting**: Color-coded output with pass/fail status and JSON response logs
- **Sample Codebases**: Includes realistic code samples for each language

## Directory Structure

```
test/serena-mcp-tests/
├── README.md                    # This file
├── test_serena.sh              # Main test script
├── samples/                    # Sample codebases for testing
│   ├── go_project/            # Go calculator sample
│   │   ├── main.go
│   │   ├── go.mod
│   │   └── utils.go
│   ├── java_project/          # Java calculator sample
│   │   ├── Calculator.java
│   │   └── Utils.java
│   ├── js_project/            # JavaScript calculator sample
│   │   ├── calculator.js
│   │   ├── utils.js
│   │   └── package.json
│   └── python_project/        # Python calculator sample
│       ├── calculator.py
│       └── utils.py
├── expected/                   # Directory for expected results (future use)
└── results/                   # Generated test results (JSON responses)
```

## Requirements

- **Docker**: The test script requires Docker to be installed and running
- **Bash**: Shell script tested with Bash 4.0+
- **Network Access**: Ability to pull the Serena MCP Server Docker image

## Usage

### Basic Usage

Run the test script from this directory:

```bash
./test_serena.sh
```

### Custom Docker Image

To test a different version or local build:

```bash
SERENA_IMAGE="serena-mcp-server:local" ./test_serena.sh
```

### Running from Repository Root

```bash
./test/serena-mcp-tests/test_serena.sh
```

## Test Categories

### 1. Infrastructure Tests
- Docker availability
- Container image availability
- Container basic functionality

### 2. Runtime Verification
- Python runtime (3.11+)
- Java runtime (JDK 21)
- Node.js runtime
- Go runtime

### 3. MCP Protocol Tests
- Initialize connection
- List available tools
- Tool invocation

### 4. Language-Specific Analysis

#### Go Tests
- Symbol finding (functions, types, methods)
- Code diagnostics
- Language server integration

#### Java Tests
- Symbol finding (classes, methods)
- Code diagnostics
- Language server integration

#### JavaScript Tests
- Symbol finding (classes, functions)
- Code diagnostics
- Language server integration

#### Python Tests
- Symbol finding (classes, functions)
- Type checking
- Code diagnostics

### 5. Error Handling
- Invalid MCP method requests
- Malformed JSON handling
- Proper error responses

## Sample Codebases

Each sample codebase contains a simple Calculator implementation with:
- Basic arithmetic operations (add, multiply)
- Properly documented code
- Utility functions for testing cross-file references
- Language-specific idioms and patterns

These samples provide:
- **Known symbols**: Functions/methods with predictable names for validation
- **Type information**: Proper typing for language server analysis
- **Documentation**: Comments and docstrings for documentation tests
- **Realistic structure**: Multiple files to test cross-file analysis

## Test Output

The script provides:
- **Color-coded results**: Green for pass, red for fail, yellow for warnings
- **Progress indicators**: Real-time test execution feedback
- **JSON response logs**: All MCP responses saved to `results/` directory
- **Summary statistics**: Total tests, pass/fail counts, success rate

### Example Output

```
========================================
Test 5: MCP Protocol Initialize
========================================
[INFO] Sending MCP initialize request...
[✓] MCP initialize succeeded
[INFO] Response saved to: results/initialize_response.json

========================================
Test Summary
========================================

[INFO] Total Tests: 20
[✓] Passed: 18
[✗] Failed: 2

[INFO] Success Rate: 90%
[INFO] Detailed results saved to: results/
```

## Response Files

All MCP responses are saved to `results/` directory:
- `initialize_response.json` - MCP initialization response
- `tools_list_response.json` - Available tools listing
- `go_symbols_response.json` - Go symbol analysis results
- `go_diagnostics_response.json` - Go diagnostics results
- `java_symbols_response.json` - Java symbol analysis results
- `java_diagnostics_response.json` - Java diagnostics results
- `js_symbols_response.json` - JavaScript symbol analysis results
- `js_diagnostics_response.json` - JavaScript diagnostics results
- `python_symbols_response.json` - Python symbol analysis results
- `python_diagnostics_response.json` - Python diagnostics results
- `invalid_request_response.json` - Error handling test results
- `malformed_json_response.txt` - Malformed JSON test results

## Interpreting Results

### Successful Test
```
[✓] Go symbol analysis working - found expected symbols
```

### Failed Test
```
[✗] Go symbol analysis failed
```

### Warning
```
[⚠] Go symbol analysis returned result but symbols not as expected
```

## Troubleshooting

### Container Pull Fails
- Verify network connectivity
- Check Docker registry access
- Ensure proper authentication for private registries

### All Language Tests Fail
- Check that sample files exist in `samples/` directory
- Verify workspace is properly mounted (`/workspace`)
- Review container logs for language server initialization errors

### Specific Language Fails
- Verify the language runtime is installed in the container
- Check that language-specific tools are in the tools list
- Review the response JSON files in `results/` for error details

### Tests Hang
- Some language servers may take time to initialize on first run
- Container may need more resources (CPU/memory)
- Check Docker daemon health

## Extending the Tests

To add new tests:

1. Add new sample code to `samples/` directory
2. Add test cases to `test_serena.sh` following the existing pattern
3. Document expected behavior in `expected/` directory (optional)
4. Update this README with new test descriptions

## CI/CD Integration

The test script returns:
- Exit code `0` on success (all tests pass)
- Exit code `1` on failure (one or more tests fail)

Example GitHub Actions usage:

```yaml
- name: Run Serena MCP Tests
  run: |
    cd test/serena-mcp-tests
    ./test_serena.sh
```

## Known Limitations

- Tests require Docker (no native binary testing)
- First run may be slow due to language server initialization
- Some language servers cache data - results may vary between runs
- Tool names and arguments depend on Serena MCP Server version

## References

- [Serena MCP Server Documentation](https://github.com/oraios/serena)
- [Model Context Protocol Specification](https://github.com/modelcontextprotocol)
- [MCP Gateway Configuration](https://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md)

## Contributing

To improve these tests:
1. Add more comprehensive sample codebases
2. Add expected response validation
3. Add performance benchmarks
4. Test additional language features (refactoring, code completion, etc.)
5. Add integration tests with the MCP Gateway

## License

Part of the gh-aw-mcpg project. See repository root for license information.
