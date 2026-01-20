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

| Gateway Config | Server Architecture | Gateway Compatible? | Notes |
|----------------|--------------------|--------------------|-------|
| **`"http"`** | Stateless | ✅ **YES** | Server processes each request independently |
| **`"stdio"`** | Stateless | ✅ **YES** | Would work if accessed directly, but gateway creates new connections |
| **`"stdio"`** | Stateful | ❌ **NO*** | Requires persistent connection, gateway creates new ones |

\* Without gateway enhancement (connection pooling not yet implemented)

**Important:** `"type": "http"` means the gateway connects via HTTP, but the server may support multiple transports. The real question is whether the server's **architecture** is stateless or stateful.

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
- **Why it works:** Stateless design - each request is independent
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
- **Why it fails:** Stateful design - requires persistent connection
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

**Cause:** Gateway creates new connection per request, loses session state  
**Solution:** Use direct stdio connection instead

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
