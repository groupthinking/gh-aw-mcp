# Quick Reference: MCP Server Gateway Compatibility

## Is My MCP Server Compatible with the HTTP Gateway?

**Key Point:** Compatibility depends on **architecture** (stateless vs stateful), not transport (HTTP vs stdio).

### Check Your Gateway Configuration

Look at the `"type"` field - this tells the gateway HOW to connect to your server:

```json
{
  "mcpServers": {
    "my-server": {
      "type": "???"  ← This is the gateway's connection method
    }
  }
}
```

### Compatibility Chart

| Gateway Config | Server Architecture | Backend Connection | Gateway Compatible? | Notes |
|----------------|--------------------|--------------------|---------------------|-------|
| **`"http"`** | Stateless | Single persistent HTTP connection per backend | ✅ **YES** | Server processes each request independently |
| **`"stdio"`** | Stateless | Single persistent stdio connection per backend | ✅ **YES** | Would work if SDK supported it |
| **`"stdio"`** | Stateful | Session pool (one connection per session) | ❌ **NO*** | Backend connection reuse works, but SDK creates new protocol state per HTTP request |

\* Connection pooling infrastructure implemented but SDK limitation prevents it from working

**Important Clarification:**
- **HTTP backends**: Gateway maintains ONE persistent connection per backend, reused across ALL frontend HTTP requests
- **Stdio backends**: Gateway implements session connection pool (one per backend+session), but SDK's `StreamableHTTPHandler` creates new protocol state per request
- The issue is NOT backend connection management - that works correctly
- The issue is SDK protocol session state not persisting across HTTP requests

---

## Real-World Examples

### ✅ Works Through Gateway

**GitHub MCP Server** (Stateless, multi-transport):
```json
{
  "github": {
    "type": "http",
    "url": "http://localhost:3000"
  }
}
```
- **Architecture:** Stateless
- **Supports:** Both stdio AND HTTP transports
- **Gateway uses:** HTTP transport (`"type": "http"`)
- **Backend connection:** Single persistent HTTP connection, reused for ALL requests
- **Why it works:** Stateless design - no session state needed between requests
- **Result:** 100% gateway compatible

### ❌ Doesn't Work Through Gateway (Yet)

**Serena MCP Server** (Stateful, stdio-only):
```json
{
  "serena": {
    "type": "stdio",
    "container": "ghcr.io/githubnext/serena-mcp-server:latest"
  }
}
```
- **Architecture:** Stateful
- **Supports:** Stdio transport only
- **Gateway uses:** Stdio transport (`"type": "stdio"`)
- **Backend connection:** Session pool (one per backend+session), connection IS reused
- **Why it fails:** Backend connection reuse works, but SDK creates new protocol state per HTTP request
- **Workaround:** Use direct stdio connection instead

---

## Quick Decision Guide

```
┌─────────────────────────────────────────┐
│ Do you need to deploy in the cloud?    │
│ Do you need horizontal scaling?        │
│ Do you need load balancing?            │
└────────────┬────────────────────────────┘
             │
             ├─ YES → Use HTTP-native server (type: "http")
             │        ✅ Gateway compatible
             │
             └─ NO  → Use stdio server (type: "stdio")
                      ✅ Direct connection only
                      ℹ️  Perfect for CLI/local tools
```

---

## Error Signatures

### Stateful Server Through Gateway (Will Fail)

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": 0,
    "message": "method 'tools/list' is invalid during session initialization"
  }
}
```

**Cause:** SDK creates new protocol session state per HTTP request, even though backend connection is reused  
**Technical Detail:** Backend stdio connection IS reused from SessionConnectionPool, but SDK's StreamableHTTPHandler creates fresh protocol state  
**Solution:** Use direct stdio connection to bypass HTTP gateway layer

### Stateless Server (Will Work)

```json
{
  "jsonrpc": "2.0",
  "result": {
    "tools": [
      {"name": "tool1", "description": "..."},
      {"name": "tool2", "description": "..."}
    ]
  }
}
```

**Cause:** Server doesn't need session state  
**Result:** Works perfectly through gateway ✅

---

## For More Details

📖 **Full Explanation:** [Why GitHub Works But Serena Doesn't](./WHY_GITHUB_WORKS_BUT_SERENA_DOESNT.md)

📊 **Architecture Analysis:** [MCP Server Architecture Patterns](../test/serena-mcp-tests/MCP_SERVER_ARCHITECTURE_ANALYSIS.md)

🧪 **Test Results:** [Serena Test Results Summary](../SERENA_TEST_RESULTS.md)
