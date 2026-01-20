# Quick Reference: MCP Server Gateway Compatibility

## Is My MCP Server Compatible with the HTTP Gateway?

### Check Your Configuration

Look at the `"type"` field in your MCP server configuration:

```json
{
  "mcpServers": {
    "my-server": {
      "type": "???"  ← Check this
    }
  }
}
```

### Compatibility Chart

| Server Type | Gateway Compatible? | Direct Connection? | Best Use Case |
|-------------|--------------------|--------------------|---------------|
| **`"http"`** | ✅ **YES** | ✅ Yes | Cloud, serverless, scalable apps |
| **`"stdio"`** | ❌ **NO*** | ✅ Yes | CLI tools, local development |
| **`"local"`** | ❌ **NO*** | ✅ Yes | Same as stdio |

\* Without gateway enhancement (connection pooling not yet implemented)

---

## Real-World Examples

### ✅ Works Through Gateway

**GitHub MCP Server** (TypeScript, HTTP-native):
```json
{
  "github": {
    "type": "http",
    "url": "http://localhost:3000"
  }
}
```
- Stateless architecture
- Each request independent
- 100% gateway compatible

### ❌ Doesn't Work Through Gateway (Yet)

**Serena MCP Server** (Python, stdio-based):
```json
{
  "serena": {
    "type": "stdio",
    "container": "ghcr.io/githubnext/serena-mcp-server:latest"
  }
}
```
- Stateful architecture
- Requires persistent connection
- Use direct stdio connection instead

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
