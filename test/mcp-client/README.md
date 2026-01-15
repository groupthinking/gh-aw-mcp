# MCP Test Client

This directory contains a Python-based MCP test client for verifying MCP server behavior.

## Overview

The MCP test client provides:
- Simple Python library for testing MCP servers via stdio transport
- Docker container for isolated testing
- Pytest-based test suite for Serena containers
- Integration with Go test suite

## Components

### MCP Client Library (`mcp_client.py`)

A Python library that provides:
- `MCPClient` class for interacting with MCP servers
- Support for stdio-based MCP servers in Docker containers
- Helper methods for common MCP operations (initialize, list_tools, call_tool)
- Context manager support for easy resource cleanup

**Example usage:**

```python
from mcp_client import MCPClient

# Test a Serena container
with MCPClient("ghcr.io/githubnext/aw-serena:latest", "/path/to/project") as client:
    # Initialize the server
    response = client.initialize()
    print(f"Server: {response['result']['serverInfo']['name']}")
    
    # List available tools
    tools = client.list_tools()
    print(f"Found {len(tools)} tools")
```

### Test Suite (`test_serena.py`)

Pytest-based tests for Serena containers:
- `test_serena_initialize`: Verify container initialization
- `test_serena_list_tools`: Verify tool listing
- `test_unified_serena_multi_language`: Test unified container with multiple languages
- Language-specific project tests

### Test Client Container (`Dockerfile.test-client`)

A Docker container with all testing dependencies:
- Python 3.12
- pytest and testing libraries
- MCP client library
- Docker CLI for testing containers

## Running Tests

### Locally with Python

```bash
# Install dependencies
pip install pytest pytest-asyncio pytest-timeout httpx

# Run tests
cd test/mcp-client
pytest -v test_serena.py
```

### Using the Test Client Container

```bash
# Build the test client container
docker build -f serena/Dockerfile.test-client -t mcp-test-client:local .

# Run tests in container
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd):/workspace \
  -e USE_LOCAL_IMAGES=1 \
  mcp-test-client:local \
  /workspace/test/mcp-client/test_serena.py
```

### With Go Integration Tests

The Go test suite includes integration tests that use the MCP client:

```bash
# Run Serena integration tests
go test -v ./test/integration -run TestSerena

# Run all integration tests
make test-integration
```

## Writing New Tests

### Python Tests

Add new tests to `test_serena.py` or create new test files:

```python
import pytest
from mcp_client import MCPClient

@pytest.mark.timeout(60)
def test_my_feature():
    """Test a specific Serena feature."""
    with MCPClient("aw-serena:local", "/tmp/workspace") as client:
        client.initialize()
        
        # Test your feature
        result = client.call_tool("my_tool", {"arg": "value"})
        assert result is not None
```

### Go Integration Tests

Add tests to `test/integration/serena_test.go`:

```go
func TestSerenaNewFeature(t *testing.T) {
    // Use testSerenaContainer helper
    success := testSerenaContainer(t, "aw-serena:local", workspacePath)
    assert.True(t, success)
}
```

## Environment Variables

- `USE_LOCAL_IMAGES`: Set to "1" to use locally built images (e.g., `aw-serena:local`)
- `TEST_DIR`: Override test directory location (default: `/test`)

## Container Images Tested

The test suite tests these containers:

**Unified:**
- `ghcr.io/githubnext/aw-serena:latest` (or `aw-serena:local`)

**Language-Specific:**
- `ghcr.io/githubnext/serena-go:latest` (or `serena-go:local`)
- `ghcr.io/githubnext/serena-typescript:latest` (or `serena-typescript:local`)
- `ghcr.io/githubnext/serena-python:latest` (or `serena-python:local`)
- `ghcr.io/githubnext/serena-java:latest` (or `serena-java:local`)
- `ghcr.io/githubnext/serena-rust:latest` (or `serena-rust:local`)
- `ghcr.io/githubnext/serena-csharp:latest` (or `serena-csharp:local`)

## Troubleshooting

### Docker Socket Access

The test client needs access to the Docker socket to test containers:

```bash
# Linux/Mac
docker run -v /var/run/docker.sock:/var/run/docker.sock ...

# Windows (WSL2)
docker run -v //var/run/docker.sock:/var/run/docker.sock ...
```

### Container Not Found

If tests skip with "container not built", build the containers first:

```bash
# Build unified container
docker build -f serena/Dockerfile -t aw-serena:local .

# Build language-specific containers
docker build -f serena/Dockerfile-go -t serena-go:local .
```

### Timeouts

If tests timeout, increase the timeout decorator:

```python
@pytest.mark.timeout(120)  # Increase to 120 seconds
def test_slow_operation():
    ...
```

## CI/CD Integration

The test suite can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Build test client
  run: docker build -f serena/Dockerfile.test-client -t mcp-test-client .

- name: Build Serena containers
  run: |
    docker build -f serena/Dockerfile -t aw-serena:local .
    docker build -f serena/Dockerfile-go -t serena-go:local .

- name: Run MCP tests
  run: |
    docker run --rm \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -v $(pwd):/workspace \
      -e USE_LOCAL_IMAGES=1 \
      mcp-test-client \
      /workspace/test/mcp-client/test_serena.py
```

## Future Enhancements

Ideas for extending the test suite:

1. **Tool Execution Tests**: Test actual tool invocations
2. **Performance Tests**: Measure startup time and response times
3. **Multi-Language Tests**: Verify language detection in monorepos
4. **Error Handling Tests**: Test error conditions and recovery
5. **Security Tests**: Verify container isolation and permissions
6. **Load Tests**: Test with multiple concurrent requests

## Contributing

When adding new Serena features:

1. Add corresponding tests to `test_serena.py`
2. Update the Go integration tests if needed
3. Document any new test requirements
4. Ensure tests pass in CI/CD

## References

- [Model Context Protocol Specification](https://spec.modelcontextprotocol.io/)
- [Serena Documentation](https://github.com/oraios/serena)
- [pytest Documentation](https://docs.pytest.org/)
- [Docker Python SDK](https://docker-py.readthedocs.io/)
