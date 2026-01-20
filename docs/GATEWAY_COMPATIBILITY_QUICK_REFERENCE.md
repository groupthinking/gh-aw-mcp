# Quick Reference: MCP Server Gateway Compatibility

## Is My MCP Server Compatible with the HTTP Gateway?

**Key Point:** Compatibility depends on **architecture** (stateless vs stateful). In production, most MCP servers use stdio transport via Docker containers.

### Critical Fact

**Both GitHub and Serena use stdio in production:**
```json
{
  "github": {
    "type": "local",  // Alias for stdio
    "container": "ghcr.io/github/github-mcp-server:latest"
  },
  "serena": {
    "type": "stdio",
    "container": "ghcr.io/githubnext/serena-mcp-server:latest"
  }
}
```

Both use Docker containers with stdio transport. The difference is architecture, not transport.

---

## Compatibility Chart

| Server | Transport | Architecture | Backend Connection | Compatible? | Why? |
|--------|-----------|--------------|-------------------|-------------|------|
| **GitHub MCP** | Stdio (Docker) | Stateless | Session pool | ✅ **YES** | Doesn't validate initialization state |
| **Serena MCP** | Stdio (Docker) | Stateful | Session pool | ❌ **NO*** | Validates initialization, SDK breaks it |

\* Backend connection reuse works correctly. Issue is SDK's StreamableHTTPHandler creates new protocol state per HTTP request.

**Both servers use identical backend infrastructure:**
- ✅ Stdio transport via Docker containers
- ✅ Session connection pool (one per backend+session)
- ✅ Backend process reuse
- ✅ Stdio pipe reuse

**The difference:**
- GitHub: Stateless architecture → doesn't care about SDK protocol state
- Serena: Stateful architecture → SDK protocol state recreation breaks it

---

## Real-World Examples

### ✅ Works Through Gateway

**GitHub MCP Server** (Stateless, stdio via Docker):
```json
{
  "github": {
    "type": "local",
    "container": "ghcr.io/github/github-mcp-server:latest"
  }
}
```
- **Transport:** Stdio via Docker (same as Serena!)
- **Architecture:** Stateless
- **Backend connection:** Session pool, reused per session
- **Why it works:** Doesn't validate initialization state - SDK protocol state recreation doesn't matter
- **Result:** 100% gateway compatible

### ❌ Doesn't Work Through Gateway (Yet)

**Serena MCP Server** (Stateful, stdio via Docker):
```json
{
  "serena": {
    "type": "stdio",
    "container": "ghcr.io/githubnext/serena-mcp-server:latest"
  }
}
```
- **Transport:** Stdio via Docker (same as GitHub!)
- **Architecture:** Stateful
- **Backend connection:** Session pool, reused per session
- **Why it fails:** Validates initialization state - SDK protocol state recreation breaks it
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
