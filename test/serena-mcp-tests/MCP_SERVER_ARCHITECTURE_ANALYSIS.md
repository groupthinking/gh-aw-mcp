# MCP Server Architecture: Stateless vs Stateful Design Patterns

## Executive Summary

The session persistence issue observed with Serena MCP Server through the gateway is **not unique to Serena**. It's a fundamental architectural difference between two MCP server design patterns:

1. **Stateless HTTP-Native Servers** (e.g., GitHub MCP Server) - Work seamlessly through HTTP gateways
2. **Stateful Stdio-Based Servers** (e.g., Serena MCP Server) - Require persistent connections

This document explains these patterns, provides evidence from testing, and offers guidance for MCP server developers and users.

---

## Table of Contents

- [Background](#background)
- [Stateless vs Stateful Architecture](#stateless-vs-stateful-architecture)
- [Evidence from Testing](#evidence-from-testing)
- [Why GitHub MCP Server Works](#why-github-mcp-server-works)
- [Why Serena MCP Server Doesn't Work](#why-serena-mcp-server-doesnt-work)
- [Other MCP Servers](#other-mcp-servers)
- [Recommendations](#recommendations)
- [Conclusion](#conclusion)

---

## Background

When testing MCP servers through the MCP Gateway, we discovered:

✅ **GitHub MCP Server**: Works perfectly through HTTP gateway  
❌ **Serena MCP Server**: Fails with "method is invalid during session initialization" errors

This raised the question: **Is this specific to Serena or a broader pattern?**

---

## Stateless vs Stateful Architecture

### Stateless HTTP-Native Servers

**Design Philosophy:**
- Each HTTP request is completely independent
- No session state maintained between requests
- Server processes each request as a fresh interaction
- Designed for cloud-native/serverless deployments

**How Initialization Works:**
```
Request 1 (initialize):
  Client → Gateway → Server: {"method":"initialize",...}
  Server processes, returns result
  Server: No state saved for next request
  
Request 2 (tools/list):
  Client → Gateway → Server: {"method":"tools/list",...}
  Server processes WITHOUT requiring prior initialization
  Server: Ready to serve from first request
```

**Key Characteristics:**
- ✅ Infinite horizontal scalability
- ✅ Works with load balancers
- ✅ Resilient to instance failures
- ✅ Perfect for serverless (AWS Lambda, Cloud Run, etc.)
- ⚠️ No conversational context without external storage

**Example Implementations:**
- GitHub MCP Server (TypeScript)
- Most HTTP-native MCP servers
- Serverless MCP implementations

### Stateful Stdio-Based Servers

**Design Philosophy:**
- Designed for persistent stdio connections
- Session state maintained in-memory throughout connection lifetime
- Initialization creates session context for subsequent operations
- Optimized for local CLI tools and long-running processes

**How Initialization Works:**
```
Single stdio stream:
  Client → Server: {"method":"initialize",...}
  Server: Creates session state in memory
  
  Client → Server: {"method":"notifications/initialized"}
  Server: Marks session as fully initialized
  
  Client → Server: {"method":"tools/list",...}
  Server: Uses session state, returns tools
  
  (All on same connection, session state persists)
```

**Key Characteristics:**
- ✅ Low latency for local operations
- ✅ Natural conversational context
- ✅ Simple session management
- ✅ Perfect for CLI integrations
- ⚠️ Single-client, local-only
- ⚠️ Cannot scale horizontally
- ⚠️ Session lost if process dies

**Example Implementations:**
- Serena MCP Server (Python)
- Many stdio-based MCP servers
- CLI tool integrations

---

## Evidence from Testing

### Test 1: GitHub MCP Server Through Gateway

**Test File:** `test/integration/github_test.go`

**Test Flow:**
```go
// Request 1: Initialize (separate HTTP request)
POST /mcp/github
{"method":"initialize",...}
Authorization: test-github-key
→ ✅ Success

// Request 2: Send notification (separate HTTP request) 
POST /mcp/github
{"method":"notifications/initialized"}
Authorization: test-github-key
→ ✅ Success

// Request 3: List tools (separate HTTP request)
POST /mcp/github
{"method":"tools/list",...}
Authorization: test-github-key
→ ✅ Success (Returns list of tools)

// Request 4: Call tool (separate HTTP request)
POST /mcp/github
{"method":"tools/call","params":{"name":"list_branches",...}}
Authorization: test-github-key
→ ✅ Success (Returns tool result)
```

**Result:** All tests pass! GitHub MCP server handles each HTTP request independently.

### Test 2: Serena MCP Server Through Gateway

**Test File:** `test/serena-mcp-tests/test_serena_via_gateway.sh`

**Test Flow:**
```bash
# Request 1: Initialize (separate HTTP request)
curl -X POST http://localhost:18080/mcp/serena \
  -H "Authorization: session-123" \
  -d '{"method":"initialize",...}'
→ ✅ Success

# Request 2: Send notification (separate HTTP request)
curl -X POST http://localhost:18080/mcp/serena \
  -H "Authorization: session-123" \
  -d '{"method":"notifications/initialized"}'
→ ✅ Success (but creates NEW backend connection)

# Request 3: List tools (separate HTTP request)
curl -X POST http://localhost:18080/mcp/serena \
  -H "Authorization: session-123" \
  -d '{"method":"tools/list",...}'
→ ❌ Error: "method 'tools/list' is invalid during session initialization"
```

**Result:** Fails because each HTTP request creates a new stdio connection to Serena, and that connection hasn't completed initialization.

### Test 3: Serena MCP Server Direct Connection

**Test File:** `test/serena-mcp-tests/test_serena.sh`

**Test Flow:**
```bash
# Single stdio connection to Serena container
docker run -i serena-mcp-server | {
  # Send initialize
  echo '{"method":"initialize",...}'
  # Send notification
  echo '{"method":"notifications/initialized"}'
  # Send tools/list
  echo '{"method":"tools/list",...}'
  # All on SAME connection
}
→ ✅ Success (68/68 tests pass)
```

**Result:** Works perfectly because all messages flow through the same connection.

---

## Why GitHub MCP Server Works

**Architecture:** Stateless HTTP-native design

**Implementation Details:**
1. **No Session State:** Server doesn't store initialization state between requests
2. **Request Independence:** Each request is self-contained and processable without prior context
3. **Immediate Readiness:** Server can handle `tools/list` and `tools/call` without seeing an `initialize` request first
4. **TypeScript SDK:** Built on `@modelcontextprotocol/sdk` with streamable HTTP transport designed for stateless operation

**Code Pattern (Simplified):**
```typescript
// GitHub MCP server handles each request independently
server.on('request', async (req) => {
  // No session state needed
  switch(req.method) {
    case 'initialize':
      return { serverInfo: {...}, capabilities: {...} };
    case 'tools/list':
      // Returns tools immediately, no initialization check
      return { tools: getAllTools() };
    case 'tools/call':
      // Executes tool immediately, no session state required
      return executeTool(req.params.name, req.params.arguments);
  }
});
```

**Gateway Behavior:**
```
HTTP Request 1 (Authorization: key-123) → New GitHub server instance
  ↳ Process initialize
  ↳ Return response
  ↳ Instance terminates

HTTP Request 2 (Authorization: key-123) → New GitHub server instance
  ↳ Process tools/list
  ↳ Return tools (no initialization needed)
  ↳ Instance terminates

HTTP Request 3 (Authorization: key-123) → New GitHub server instance
  ↳ Process tools/call
  ↳ Execute tool (no session state needed)
  ↳ Instance terminates
```

---

## Why Serena MCP Server Doesn't Work

**Architecture:** Stateful stdio-based design

**Implementation Details:**
1. **Session State:** Server creates in-memory session state during initialization
2. **Initialization Protocol:** Strictly enforces MCP protocol state machine:
   - State 1: Uninitialized (only accepts `initialize`)
   - State 2: Initializing (only accepts `notifications/initialized`)
   - State 3: Ready (accepts `tools/list`, `tools/call`, etc.)
3. **Connection-Based State:** Session state is tied to the stdio connection lifetime
4. **Python Implementation:** Uses language servers (Pylance, Jedi) that require initialization

**Code Pattern (Simplified):**
```python
# Serena MCP server maintains session state
class SerenaServer:
    def __init__(self):
        self.session_state = "uninitialized"
        self.language_servers = {}
    
    def handle_request(self, req):
        if req.method == 'initialize':
            # Create session state
            self.session_state = "initializing"
            self.language_servers = self.start_language_servers()
            return success_response
        
        elif req.method == 'notifications/initialized':
            # Mark session as ready
            self.session_state = "ready"
            return None
        
        elif req.method == 'tools/list':
            # Check session state
            if self.session_state != "ready":
                return error("method is invalid during session initialization")
            return { tools: self.get_all_tools() }
```

**Gateway Behavior:**
```
HTTP Request 1 (Authorization: key-123) → New Serena stdio connection
  ↳ Send: {"method":"initialize",...}
  ↳ Serena: session_state = "initializing"
  ↳ Return response
  ↳ Connection closes (session state lost!)

HTTP Request 2 (Authorization: key-123) → NEW Serena stdio connection
  ↳ New process: session_state = "uninitialized" (fresh instance!)
  ↳ Send: {"method":"notifications/initialized"}
  ↳ Serena: Error! Not in initializing state
  ↳ Connection closes

HTTP Request 3 (Authorization: key-123) → NEW Serena stdio connection
  ↳ New process: session_state = "uninitialized" (fresh instance!)
  ↳ Send: {"method":"tools/list",...}
  ↳ Serena: Error! "method is invalid during session initialization"
  ↳ Connection closes
```

---

## Other MCP Servers

### Expected Behavior by Server Type

| Server Type | Through Gateway | Direct Connection | Example Servers |
|-------------|-----------------|-------------------|-----------------|
| **Stateless HTTP** | ✅ Works | ✅ Works | GitHub, most TypeScript servers |
| **Stateful stdio** | ❌ Fails | ✅ Works | Serena, many Python servers |
| **Hybrid** | ⚠️ Depends | ✅ Works | Servers with optional stateless mode |

### Server Categories

#### 1. Stateless HTTP-Native (Gateway Compatible)
- **GitHub MCP Server** - Stateless TypeScript implementation
- **HTTP-based MCP servers** - Designed for serverless deployment
- **Stateless examples** - Reference implementations like `example-mcp-server-streamable-http-stateless`

**Characteristics:**
- Built with HTTP-first mindset
- No initialization state machine
- Can process any request at any time
- Perfect for cloud deployments

#### 2. Stateful Stdio-Based (Gateway Incompatible)
- **Serena MCP Server** - Stateful Python implementation with language servers
- **CLI tool MCP servers** - Designed for local subprocess execution
- **Conversational MCP servers** - Maintain chat history and context

**Characteristics:**
- Built for persistent connections
- Strict MCP protocol state machine
- Session state tied to connection lifetime
- Perfect for CLI/desktop integrations

#### 3. Hybrid Servers (Configurable)
Some servers support both modes with configuration flags:
- Stateless mode for HTTP gateway deployment
- Stateful mode for stdio/local execution

---

## Recommendations

### For MCP Server Developers

#### When Building Stateless HTTP-Native Servers

✅ **DO:**
- Design each request to be self-contained
- Avoid storing session state in memory
- Allow `tools/list` and `tools/call` without prior `initialize`
- Use external storage for any persistent state
- Support horizontal scaling
- Document as "Gateway Compatible"

❌ **DON'T:**
- Enforce strict initialization protocol for HTTP transport
- Store session data in server memory
- Assume requests come from same client instance

**Example:**
```typescript
// Good: Stateless design
export const server = new Server({
  capabilities: {
    tools: {}
  }
});

server.setRequestHandler(ListToolsRequestSchema, async () => ({
  tools: getAllTools() // No initialization check needed
}));

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  // Execute immediately, no session state
  return await executeTool(request.params);
});
```

#### When Building Stateful Stdio-Based Servers

✅ **DO:**
- Clearly document as "stdio-only" or "requires direct connection"
- Enforce MCP protocol state machine for correctness
- Maintain session state for optimal user experience
- Optimize for CLI and subprocess use cases

❌ **DON'T:**
- Claim HTTP gateway compatibility
- Recommend using through proxies
- Expect to work in serverless environments

**Example:**
```python
# Good: Clear session state management
class StdioMCPServer:
    """Stateful MCP server for stdio connections only.
    
    NOT compatible with HTTP gateways due to session state requirements.
    """
    
    def __init__(self):
        self.state = "uninitialized"
        self.context = None
    
    def handle_initialize(self, req):
        self.state = "initializing"
        self.context = create_session_context()
        return success
    
    def handle_tools_list(self, req):
        if self.state != "ready":
            raise ProtocolError("Invalid state")
        return self.get_tools()
```

#### Hybrid Approach (Best of Both Worlds)

```typescript
// Support both stateless and stateful modes
class HybridMCPServer {
  constructor(config: { mode: 'stateless' | 'stateful' }) {
    this.mode = config.mode;
    this.sessionState = this.mode === 'stateful' ? {} : null;
  }
  
  handleToolsList() {
    if (this.mode === 'stateful' && !this.isInitialized()) {
      throw new Error("Not initialized");
    }
    return this.getTools();
  }
}

// Usage:
// For HTTP gateway: new HybridMCPServer({ mode: 'stateless' })
// For stdio: new HybridMCPServer({ mode: 'stateful' })
```

### For MCP Server Users

#### Choosing the Right Server

**Use Stateless HTTP-Native Servers When:**
- Deploying through HTTP gateway
- Need horizontal scalability
- Running in serverless/cloud environments
- Serving multiple concurrent clients
- Want load balancer compatibility

**Use Stateful Stdio-Based Servers When:**
- Running as local CLI tool
- Need conversational context
- Using as subprocess from desktop app
- Want lowest latency
- Single-user scenarios

#### Checking Server Compatibility

Before using a server through gateway, check:

1. **Documentation:** Does it mention "HTTP gateway compatible" or "stateless"?
2. **Transport Type:** Does it use "streamable HTTP" transport?
3. **Implementation Language:** TypeScript servers more likely to be stateless
4. **Test First:** Try initialize → tools/list sequence through gateway

### For Gateway Operators

#### Current Limitations

The MCP Gateway currently:
- Creates new backend connections for each HTTP request
- Doesn't maintain persistent stdio connections
- Works perfectly with stateless HTTP-native servers
- Has limitations with stateful stdio-based servers

#### Future Enhancement Possibilities

To support stateful stdio-based servers:

1. **Connection Pooling by Session ID:**
   ```go
   // Maintain persistent stdio connections
   type ConnectionPool struct {
       connections map[string]*StdioConnection
       mutex       sync.RWMutex
   }
   
   func (p *ConnectionPool) GetOrCreate(sessionID string) *StdioConnection {
       p.mutex.Lock()
       defer p.mutex.Unlock()
       
       if conn, exists := p.connections[sessionID]; exists {
           return conn
       }
       
       conn := startNewStdioBackend()
       p.connections[sessionID] = conn
       return conn
   }
   ```

2. **Session Lifecycle Management:**
   - Track session initialization state
   - Reuse connections for same Authorization header
   - Implement session timeouts
   - Clean up idle connections

3. **Configuration Option:**
   ```toml
   [servers.serena]
   type = "stdio"
   container = "serena-mcp-server:latest"
   # New option for stateful backends
   connection_mode = "persistent"  # vs "per-request"
   session_timeout = "30m"
   ```

---

## Conclusion

### Key Takeaways

1. **Not Unique to Serena:** The session persistence issue affects **all stateful stdio-based MCP servers**, not just Serena.

2. **Two Design Patterns:**
   - **Stateless HTTP-Native** (e.g., GitHub) - Works through gateways
   - **Stateful Stdio-Based** (e.g., Serena) - Requires direct connections

3. **Both Patterns Are Valid:** Each has its use case:
   - Stateless: Cloud-native, scalable, gateway-compatible
   - Stateful: Local CLI, low latency, rich session context

4. **Gateway Compatibility:**
   - ✅ GitHub MCP Server: Designed for stateless HTTP
   - ❌ Serena MCP Server: Designed for stateful stdio
   - ⚠️ Gateway: Currently optimized for stateless servers

5. **Recommendations:**
   - **Developers:** Choose architecture based on deployment target
   - **Users:** Match server type to deployment environment
   - **Gateway:** Consider adding persistent connection support

### The Bottom Line

**Question:** Is the session persistence requirement unique to Serena?

**Answer:** **No.** It's an architectural pattern. Any MCP server designed as a stateful stdio application will have the same limitation when proxied through an HTTP gateway. GitHub MCP Server works because it's designed as a stateless HTTP-native application from the ground up.

Both patterns are valid and serve different use cases. The key is choosing the right architecture for your deployment model.

---

## References

- [MCP Protocol Specification - Transports](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)
- [Stateless vs Stateful MCP Architecture](https://deepwiki.com/aws-samples/sample-serverless-mcp-servers/1.1-stateful-vs.-stateless-architecture)
- [MCP Transport Protocols Comparison](https://mcpcat.io/guides/comparing-stdio-sse-streamablehttp/)
- [Building Stateless HTTP MCP Servers](https://peerlist.io/ruhan/articles/building-a-stateless-http-mcp-server-typescript-and-deploy)
- GitHub MCP Server Tests: `test/integration/github_test.go`
- Serena Gateway Tests: `test/serena-mcp-tests/test_serena_via_gateway.sh`
- Gateway Test Findings: `test/serena-mcp-tests/GATEWAY_TEST_FINDINGS.md`

---

**Document Version:** 1.0  
**Last Updated:** January 19, 2026  
**Author:** MCP Gateway Testing Team
