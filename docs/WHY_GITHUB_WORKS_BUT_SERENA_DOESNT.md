# Why GitHub MCP Works Through Gateway But Serena MCP Doesn't

## TL;DR

**GitHub MCP Server** works through the HTTP gateway because it has a **stateless architecture**.  
**Serena MCP Server** doesn't work through the HTTP gateway because it has a **stateful architecture**.

This is not a bug - it's an architectural difference between two valid MCP server design patterns.

**Critical Fact:** 
- **BOTH servers use stdio transport via Docker containers in production**
- **BOTH use the same backend connection management (session pool)**
- **The ONLY difference is stateless vs stateful architecture**
- The transport layer (stdio) is identical for both

---

## The Real Story

### What's Actually Happening

**Production Deployment (Both Servers):**
```
Gateway → docker run -i ghcr.io/github/github-mcp-server (stdio)
Gateway → docker run -i ghcr.io/githubnext/serena-mcp-server (stdio)
```

**Both servers:**
- Run as Docker containers
- Communicate via stdin/stdout (stdio transport)
- Use the same session connection pool in the gateway
- Backend stdio connections are reused for same session

**So why does one work and not the other?**

---

## Architecture: Stateless vs Stateful

### GitHub MCP Server: Stateless Architecture

**Each request is independent:**
```typescript
// GitHub MCP Server (simplified)
server.setRequestHandler(ListToolsRequestSchema, async () => {
  // NO session state needed
  // Just return the tools list
  return {
    tools: [
      { name: "search_repositories", ... },
      { name: "create_issue", ... }
    ]
  };
});

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  // NO session state needed
  // Just execute the tool with provided parameters
  const result = await executeTool(request.params.name, request.params.arguments);
  return { result };
});
```

**Why it works:**
- Server doesn't care if it was initialized before
- Each request is processed independently
- No memory of previous requests needed
- SDK protocol state recreation doesn't break anything

### Serena MCP Server: Stateful Architecture

**Requires session state:**
```python
# Serena MCP Server (simplified)
class SerenaServer:
    def __init__(self):
        self.state = "uninitialized"  # Session state!
        self.language_servers = {}     # Session state!
    
    async def initialize(self, params):
        self.state = "initializing"
        # Start language servers
        self.language_servers = await start_all_language_servers()
        self.state = "ready"
    
    async def list_tools(self):
        if self.state != "ready":
            raise Error("invalid during session initialization")
        return self.tools
    
    async def call_tool(self, name, args):
        if self.state != "ready":
            raise Error("invalid during session initialization")
        # Use language servers from session state
        return await self.language_servers[name].execute(args)
```

**Why it fails:**
- Server REQUIRES initialization before tool calls
- SDK creates new protocol state per HTTP request
- Backend process is still running (reused correctly)
- But SDK protocol layer is fresh and uninitialized
- Server rejects tool calls because SDK says "not initialized"

---

## Gateway Backend Connection Management

### How It Actually Works

**Session Connection Pool (for stdio backends):**
```go
// SessionConnectionPool manages connections by (backend, session)
type SessionConnectionPool struct {
    connections map[string]map[string]*Connection
    // Key 1: backendID (e.g., "github", "serena")
    // Key 2: sessionID (from Authorization header)
}
```

**Connection Reuse:**
```
Frontend Request 1 (session abc):
  → Gateway: GetOrLaunchForSession("github", "abc")
  → Launches: docker run -i github-mcp-server
  → Stores connection in pool["github"]["abc"]
  → Sends initialize via stdio
  → Response returned

Frontend Request 2 (session abc):
  → Gateway: GetOrLaunchForSession("github", "abc")
  → Retrieves SAME connection from pool["github"]["abc"]
  → SAME Docker process, SAME stdio pipes
  → Sends tools/list via SAME stdio connection
  → Response returned
```

**This works correctly for both GitHub and Serena!**
- ✅ Backend Docker process is reused
- ✅ Stdio pipes are reused
- ✅ Same connection for all requests in a session

---

## The SDK Problem

### What the SDK Does

**For each incoming HTTP request:**
```go
// SDK's StreamableHTTPHandler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Creates NEW protocol session state
    session := NewProtocolSession()  // Fresh state!
    session.state = "uninitialized"
    
    // Even though we reuse backend connection:
    backend := getBackendConnection()  // Reused ✅
    
    // The protocol layer is fresh
    jsonrpcRequest := parseRequest(r)
    
    if jsonrpcRequest.method != "initialize" && session.state == "uninitialized" {
        return Error("invalid during session initialization")
    }
}
```

**The layers:**
```
HTTP Request
    ↓
SDK StreamableHTTPHandler (NEW protocol state) ❌
    ↓
Backend Stdio Connection (REUSED) ✅
    ↓
MCP Server Process (REUSED, has state) ✅
```

### Why GitHub Works Despite This

**GitHub doesn't check protocol state:**
```
HTTP Request → tools/list
    ↓
SDK: "I'm uninitialized, but I'll pass it through"
    ↓
Backend GitHub Server: "I don't care about initialization, here are the tools"
    ↓
Success ✅
```

**Serena checks protocol state:**
```
HTTP Request → tools/list
    ↓
SDK: "I'm uninitialized, reject this"
    ↓
Error: "invalid during session initialization" ❌
(Backend Serena never even receives the request!)
```

---

## Configuration Examples

### GitHub MCP Server (Production)

**config.toml:**
```toml
[servers.github]
command = "docker"
args = ["run", "--rm", "-i", "ghcr.io/github/github-mcp-server:latest"]
```

**config.json:**
```json
{
  "github": {
    "type": "local",
    "container": "ghcr.io/github/github-mcp-server:latest"
  }
}
```

**Note:** `"type": "local"` is an alias for stdio. Both configs use stdio transport.

### Serena MCP Server (Production)

**config.toml:**
```toml
[servers.serena]
command = "docker"
args = ["run", "--rm", "-i", "ghcr.io/githubnext/serena-mcp-server:latest"]
```

**config.json:**
```json
{
  "serena": {
    "type": "stdio",
    "container": "ghcr.io/githubnext/serena-mcp-server:latest"
  }
}
```

**SAME transport as GitHub!**

---

## Comparison Table

| Aspect | GitHub MCP | Serena MCP |
|--------|------------|------------|
| **Production Transport** | Stdio (Docker) | Stdio (Docker) |
| **Backend Connection** | Session pool | Session pool |
| **Connection Reuse** | ✅ Yes | ✅ Yes |
| **Architecture** | Stateless | Stateful |
| **Checks Initialization** | ❌ No | ✅ Yes |
| **SDK Protocol State Issue** | Doesn't matter | Breaks it |
| **Gateway Compatible** | ✅ Yes | ❌ No |
| **Direct Connection** | ✅ Yes | ✅ Yes |

---

## Test Results

### GitHub MCP Through Gateway: 100% Pass ✅

```
Request 1 (initialize):
  → SDK creates protocol state (uninitialized)
  → Backend process launched
  → Initialize sent
  → SDK state: initialized
  → Success ✅

Request 2 (tools/list):
  → SDK creates NEW protocol state (uninitialized) ❌
  → Backend process REUSED ✅
  → GitHub doesn't care about SDK state ✅
  → Returns tools list
  → Success ✅

Request 3 (tools/call):
  → SDK creates NEW protocol state (uninitialized) ❌
  → Backend process REUSED ✅
  → GitHub doesn't care about SDK state ✅
  → Executes tool
  → Success ✅
```

### Serena MCP Through Gateway: 30% Pass ⚠️

```
Request 1 (initialize):
  → SDK creates protocol state (uninitialized)
  → Backend process launched
  → Initialize sent
  → Serena starts language servers
  → SDK state: initialized
  → Success ✅

Request 2 (tools/list):
  → SDK creates NEW protocol state (uninitialized) ❌
  → Backend process REUSED ✅
  → Backend Serena state: ready ✅
  → BUT: SDK rejects before sending to backend ❌
  → Error: "invalid during session initialization"
  → Failure ❌
```

### Serena MCP Direct: 100% Pass ✅

```
Single persistent stdio connection (no SDK HTTP layer):
  → Send initialize → Success
  → Send tools/list → Success (same connection)
  → Send tools/call → Success (same connection)
All 68 tests pass ✅
```

---

## Summary

### What Works
- ✅ Backend connection management (both servers)
- ✅ Session connection pooling (both servers)
- ✅ Docker container reuse (both servers)
- ✅ Stdio pipe reuse (both servers)
- ✅ GitHub MCP stateless architecture
- ✅ Serena MCP with direct stdio connection

### What Doesn't Work
- ❌ SDK StreamableHTTPHandler protocol state persistence
- ❌ Stateful servers through HTTP gateway

### The Root Cause
The SDK's `StreamableHTTPHandler` creates fresh protocol session state for each HTTP request, treating each request as a new session. This is fine for stateless servers (GitHub) but breaks stateful servers (Serena) because the SDK rejects tool calls before they reach the backend.

### The Fix Needed
Either:
1. Modify SDK to support protocol state persistence across HTTP requests
2. Bypass SDK's StreamableHTTPHandler for stateful servers
3. Accept this as a known limitation and use direct connections for stateful servers

---

## For Users

**If your MCP server is:**
- **Stateless** (like GitHub MCP): Use gateway ✅
- **Stateful** (like Serena MCP): Use direct stdio connection ✅

**How to tell:**
- Does your server validate initialization state before handling requests?
- Does your server maintain in-memory state between requests?
- If yes to either: You have a stateful server

Both patterns are valid and serve different use cases. The gateway is optimized for stateless servers, while stateful servers work best with direct connections.
