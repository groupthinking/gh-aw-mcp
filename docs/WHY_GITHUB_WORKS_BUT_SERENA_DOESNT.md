# Why GitHub MCP Works Through Gateway But Serena MCP Doesn't

## TL;DR

**GitHub MCP Server** works through the HTTP gateway because it's a **stateless HTTP-native server**.  
**Serena MCP Server** doesn't work through the HTTP gateway because it's a **stateful stdio-based server**.

This is not a bug - it's an architectural difference between two valid MCP server design patterns.

---

## Quick Comparison

| Aspect | GitHub MCP Server | Serena MCP Server |
|--------|-------------------|-------------------|
| **Server Type** | `"http"` | `"stdio"` |
| **Architecture** | Stateless | Stateful |
| **Connection Model** | Independent HTTP requests | Persistent stdio stream |
| **Session State** | None (each request is self-contained) | In-memory (tied to connection) |
| **Gateway Compatible** | ✅ Yes | ❌ No (without enhancement) |
| **Direct Connection** | ✅ Yes | ✅ Yes |
| **Best For** | Cloud, serverless, scalable deployments | CLI tools, local development |

---

## Configuration Comparison

### GitHub MCP Server Configuration

```json
{
  "mcpServers": {
    "github": {
      "type": "http",
      "url": "http://localhost:3000",
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "..."
      }
    }
  }
}
```

**Key Point:** `"type": "http"` means it's an HTTP-native server that accepts HTTP requests directly.

### Serena MCP Server Configuration

```json
{
  "mcpServers": {
    "serena": {
      "type": "stdio",
      "container": "ghcr.io/githubnext/serena-mcp-server:latest",
      "mounts": ["${PWD}:/workspace:ro"]
    }
  }
}
```

**Key Point:** `"type": "stdio"` means it's a stdio-based server that communicates via stdin/stdout pipes.

---

## How They Work Differently

### GitHub MCP Server: Stateless HTTP

```
┌─────────────────────────────────────────────────────────┐
│ Request 1: Initialize                                    │
├─────────────────────────────────────────────────────────┤
│ Client → Gateway → GitHub Server (HTTP)                 │
│                                                          │
│ POST /mcp/github                                         │
│ {"method": "initialize", ...}                           │
│                                                          │
│ GitHub Server:                                           │
│   - Processes request immediately                       │
│   - NO state stored                                     │
│   - Returns initialization response                     │
│   - Request complete ✅                                  │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│ Request 2: List Tools (SEPARATE HTTP REQUEST)           │
├─────────────────────────────────────────────────────────┤
│ Client → Gateway → GitHub Server (HTTP)                 │
│                                                          │
│ POST /mcp/github                                         │
│ {"method": "tools/list", ...}                           │
│                                                          │
│ GitHub Server:                                           │
│   - Processes request immediately                       │
│   - NO initialization check needed                      │
│   - Returns list of tools                               │
│   - Request complete ✅                                  │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│ Request 3: Call Tool (SEPARATE HTTP REQUEST)            │
├─────────────────────────────────────────────────────────┤
│ Client → Gateway → GitHub Server (HTTP)                 │
│                                                          │
│ POST /mcp/github                                         │
│ {"method": "tools/call", "params": {...}}              │
│                                                          │
│ GitHub Server:                                           │
│   - Processes request immediately                       │
│   - Executes tool without session state                 │
│   - Returns tool result                                 │
│   - Request complete ✅                                  │
└─────────────────────────────────────────────────────────┘
```

**Why it works:** Each HTTP request is independent. The server doesn't need or expect session state from previous requests.

### Serena MCP Server: Stateful Stdio

```
┌─────────────────────────────────────────────────────────┐
│ Request 1: Initialize                                    │
├─────────────────────────────────────────────────────────┤
│ Client → Gateway → NEW Serena Process (stdio)           │
│                                                          │
│ Gateway launches: docker run -i serena-mcp-server       │
│ Sends to stdin: {"method": "initialize", ...}          │
│                                                          │
│ Serena Server:                                           │
│   - Creates session state: "initializing"               │
│   - Starts language servers                             │
│   - Returns initialization response                     │
│                                                          │
│ Gateway receives response                                │
│ Connection CLOSES ❌ (session state LOST!)               │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│ Request 2: List Tools (SEPARATE HTTP REQUEST)           │
├─────────────────────────────────────────────────────────┤
│ Client → Gateway → NEW Serena Process (stdio)           │
│                                                          │
│ Gateway launches: docker run -i serena-mcp-server       │
│ (This is a FRESH process - no memory of Request 1!)    │
│ Sends to stdin: {"method": "tools/list", ...}          │
│                                                          │
│ Serena Server:                                           │
│   - Session state: "uninitialized" (NEW PROCESS!)       │
│   - Checks: "Am I initialized?" → NO                    │
│   - Returns ERROR ❌                                     │
│                                                          │
│ Error: "method 'tools/list' is invalid during           │
│         session initialization"                         │
└─────────────────────────────────────────────────────────┘
```

**Why it fails:** Each HTTP request creates a new stdio process with fresh state. The server expects initialization to happen on the same connection as tool calls.

---

## Code Examples

### GitHub MCP Server (TypeScript - Stateless)

```typescript
import { Server } from '@modelcontextprotocol/sdk/server/index.js';

const server = new Server({
  name: 'github-mcp-server',
  version: '1.0.0',
}, {
  capabilities: {
    tools: {},
  },
});

// Each request is handled independently - no session state
server.setRequestHandler('initialize', async (request) => {
  // No state stored
  return {
    protocolVersion: '2024-11-05',
    capabilities: { tools: {} },
    serverInfo: { name: 'github-mcp-server', version: '1.0.0' }
  };
});

server.setRequestHandler('tools/list', async () => {
  // No initialization check - just return tools
  return {
    tools: [
      { name: 'list_branches', description: '...' },
      { name: 'create_issue', description: '...' }
    ]
  };
});

server.setRequestHandler('tools/call', async (request) => {
  // No session state needed - execute tool immediately
  const { name, arguments: args } = request.params;
  return await executeTool(name, args);
});
```

**Key:** No session state, no initialization checks, each request is self-contained.

### Serena MCP Server (Python - Stateful)

```python
class SerenaMCPServer:
    def __init__(self):
        self.session_state = "uninitialized"
        self.language_servers = {}
        self.workspace_context = None
    
    async def handle_message(self, message):
        method = message.get("method")
        
        if method == "initialize":
            # Create and store session state
            self.session_state = "initializing"
            self.workspace_context = message["params"]["workspaceFolder"]
            self.language_servers = await self.start_language_servers()
            
            return {
                "protocolVersion": "2024-11-05",
                "capabilities": { "tools": {} },
                "serverInfo": { "name": "serena-mcp-server", "version": "1.0.0" }
            }
        
        elif method == "notifications/initialized":
            # Mark session as fully initialized
            self.session_state = "ready"
            return None
        
        elif method == "tools/list":
            # CHECK: Are we initialized?
            if self.session_state != "ready":
                return {
                    "error": {
                        "code": 0,
                        "message": "method 'tools/list' is invalid during session initialization"
                    }
                }
            
            return { "tools": self.get_available_tools() }
        
        elif method == "tools/call":
            # CHECK: Are we initialized?
            if self.session_state != "ready":
                return {
                    "error": {
                        "code": 0,
                        "message": "method 'tools/call' is invalid during session initialization"
                    }
                }
            
            # Use session state (language servers, workspace context)
            return await self.execute_tool(message["params"])
```

**Key:** Session state is required and checked for every tool operation. State is lost when connection closes.

---

## Test Evidence

### GitHub MCP Server Tests (ALL PASS ✅)

From `test/integration/github_test.go`:

```go
// Request 1: Initialize (separate HTTP request)
resp1 := sendRequest(t, gatewayURL+"/mcp/github", initializeRequest)
assert.NoError(t, resp1.Error)
✅ PASS

// Request 2: Send notification (separate HTTP request)
resp2 := sendRequest(t, gatewayURL+"/mcp/github", initializedNotification)
assert.NoError(t, resp2.Error)
✅ PASS

// Request 3: List tools (separate HTTP request)
resp3 := sendRequest(t, gatewayURL+"/mcp/github", toolsListRequest)
assert.NoError(t, resp3.Error)
assert.NotEmpty(t, resp3.Result.Tools)
✅ PASS - Returns 23 tools

// Request 4: Call tool (separate HTTP request)
resp4 := sendRequest(t, gatewayURL+"/mcp/github", toolCallRequest)
assert.NoError(t, resp4.Error)
assert.NotEmpty(t, resp4.Result)
✅ PASS - Tool executes successfully
```

**Result:** 100% success rate through HTTP gateway

### Serena MCP Server Tests (PARTIAL ⚠️)

From `test/serena-mcp-tests/test_serena_via_gateway.sh`:

```bash
# Test 6: Initialize (works!)
curl -X POST http://localhost:18080/mcp/serena \
  -H "Authorization: test-session-123" \
  -d '{"method":"initialize",...}'
✅ PASS

# Test 7: List tools (fails - new connection)
curl -X POST http://localhost:18080/mcp/serena \
  -H "Authorization: test-session-123" \
  -d '{"method":"tools/list",...}'
❌ FAIL - Error: "method 'tools/list' is invalid during session initialization"

# Test 8: Call tool (fails - new connection)
curl -X POST http://localhost:18080/mcp/serena \
  -H "Authorization: test-session-123" \
  -d '{"method":"tools/call",...}'
❌ FAIL - Error: "method 'tools/call' is invalid during session initialization"
```

**Result:** 7/23 tests pass (30%) - All tool operations fail

### Serena Direct Connection Tests (ALL PASS ✅)

From `test/serena-mcp-tests/test_serena.sh`:

```bash
# Single stdio connection - all messages on same stream
docker run -i serena-mcp-server <<EOF
{"method":"initialize",...}
{"method":"notifications/initialized"}
{"method":"tools/list",...}
{"method":"tools/call","params":{"name":"analyze_go_symbol",...}}
EOF

✅ PASS - 68/68 tests pass
```

**Result:** 100% success rate with direct stdio connection

---

## Why This Matters

### For Users

**If you're using GitHub MCP Server (or similar HTTP-native servers):**
- ✅ Use the HTTP gateway - it works perfectly
- ✅ Get all benefits of HTTP: load balancing, scaling, cloud deployment

**If you're using Serena MCP Server (or similar stdio servers):**
- ✅ Use direct stdio connection - full functionality
- ❌ Don't use HTTP gateway - tool operations will fail
- ℹ️ This is not a bug - it's an architectural mismatch

### For Developers

**Building a new MCP server? Choose your architecture:**

**Stateless HTTP (like GitHub):**
- ✅ Works through HTTP gateways
- ✅ Horizontally scalable
- ✅ Cloud-native friendly
- ⚠️ More complex to implement conversational context
- **Use when:** Building cloud services, APIs, serverless functions

**Stateful stdio (like Serena):**
- ✅ Simple session management
- ✅ Natural conversational context
- ✅ Perfect for CLI tools
- ⚠️ Single-client, local-only
- **Use when:** Building CLI tools, desktop integrations, local development tools

---

## Solution Paths

### For Serena Users (Today)

Use direct stdio connection:

```json
{
  "mcpServers": {
    "serena": {
      "type": "stdio",
      "container": "ghcr.io/githubnext/serena-mcp-server:latest",
      "mounts": ["${PWD}:/workspace:ro"]
    }
  }
}
```

Connect directly without HTTP gateway. Full functionality available.

### For Gateway Enhancement (Future)

The gateway could be enhanced to support stateful backends:

1. **Connection Pooling:** Maintain persistent stdio connections to backends
2. **Session Mapping:** Map HTTP Authorization headers to persistent backend connections
3. **State Preservation:** Keep backend connections alive between HTTP requests

This would allow stateful servers like Serena to work through the HTTP gateway.

**Status:** Not yet implemented (see connection pooling work in progress)

---

## How to Identify Your Server Type

Check your MCP server configuration:

```json
{
  "mcpServers": {
    "my-server": {
      "type": "???"  // ← Look here
    }
  }
}
```

| Type | Architecture | Gateway Compatible |
|------|--------------|-------------------|
| `"http"` | Stateless HTTP-native | ✅ Yes |
| `"stdio"` | Stateful stdio-based | ❌ No (without enhancement) |
| `"local"` | Alias for stdio | ❌ No (without enhancement) |

Or check the server documentation for:
- "Requires persistent connection" → Stateful
- "Stateless operation" → Stateless
- "HTTP-native" → Stateless
- "stdio transport" → Stateful

---

## Conclusion

The gateway session persistence issue affects **Serena** but not **GitHub MCP** because:

1. **GitHub MCP is stateless** - each request is independent, no session state needed
2. **Serena is stateful** - requires persistent connection with session state

This is **not a bug** - both are valid MCP server architectures:
- GitHub's approach is optimal for cloud/serverless deployments
- Serena's approach is optimal for CLI/local tool integrations

Choose the right architecture for your use case, or design hybrid servers that support both modes.

---

## References

- [MCP Server Architecture Analysis](../test/serena-mcp-tests/MCP_SERVER_ARCHITECTURE_ANALYSIS.md) - Comprehensive analysis
- [Gateway Test Findings](../test/serena-mcp-tests/GATEWAY_TEST_FINDINGS.md) - Detailed test results
- [Serena Test Results](../SERENA_TEST_RESULTS.md) - Test summary
- [GitHub MCP Server Integration Tests](../test/integration/github_test.go) - Working examples
