# Quick Reference: MCP Server Configuration

## MCP Server Support

The MCP Gateway supports MCP servers via stdio transport using Docker containers. All properly configured MCP servers work with the gateway.

### Verified Servers

| Server | Transport | Direct Tests | Gateway Tests | Configuration |
|--------|-----------|--------------|---------------|---------------|
| **GitHub MCP** | Stdio (Docker) | ✅ All passed | ✅ All passed | Production ready |
| **Serena MCP** | Stdio (Docker) | ✅ 68/68 passed | ✅ All passed | Production ready |

---

## Configuration Examples

### GitHub MCP Server

**JSON Configuration:**
```json
{
  "mcpServers": {
    "github": {
      "type": "stdio",
      "container": "ghcr.io/github/github-mcp-server:latest",
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": ""
      }
    }
  }
}
```

**TOML Configuration:**
```toml
[servers.github]
command = "docker"
args = ["run", "--rm", "-i", "ghcr.io/github/github-mcp-server:latest"]
```

### Serena MCP Server

**JSON Configuration:**
```json
{
  "mcpServers": {
    "serena": {
      "type": "stdio",
      "container": "ghcr.io/githubnext/serena-mcp-server:latest",
      "env": {
        "SERENA_CONFIG": "/path/to/config"
      }
    }
  }
}
```

**TOML Configuration:**
```toml
[servers.serena]
command = "docker"
args = ["run", "--rm", "-i", "ghcr.io/githubnext/serena-mcp-server:latest"]
```

---

## How It Works

**Backend Connection Management:**
- The gateway launches MCP servers as Docker containers
- Each session maintains a persistent connection pool
- Backend processes are reused across multiple requests
- Stdio pipes remain open for the lifetime of the session

**Example Flow:**
```
Client Request 1 (session abc):
  → Gateway launches: docker run -i github-mcp-server
  → Stores connection in pool["github"]["abc"]
  → Sends initialize via stdio
  → Returns response

Client Request 2 (session abc):
  → Gateway retrieves existing connection from pool
  → SAME Docker process, SAME stdio connection
  → Sends tools/list via same connection
  → Returns response
```

---

## Test Results

### GitHub MCP Server
- ✅ Full test suite validation (direct and gateway)
- ✅ Repository operations tested
- ✅ Issue management validated
- ✅ Production deployment confirmed

### Serena MCP Server
- ✅ **Direct Connection:** 68 comprehensive tests (100% pass rate)
- ✅ **Gateway Connection:** All integration tests passed via `make test-serena-gateway`
- ✅ Multi-language support (Go, Java, JavaScript, Python)
- ✅ All 29 tools tested and validated
- ✅ File operations, symbol operations, memory management
- ✅ See [SERENA_TEST_RESULTS.md](../SERENA_TEST_RESULTS.md) for details

---

## For More Details

📖 **Configuration Specification:** [MCP Gateway Configuration Reference](https://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md)

📊 **Test Results:** [Serena Test Results](../SERENA_TEST_RESULTS.md)

🏗️ **Architecture:** See README.md for session pooling and backend management details
