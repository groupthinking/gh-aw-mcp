# Tool Filtering with MCP Gateway

This document explains how to filter tools from backend MCP servers using the MCP Gateway.

## Overview

The MCP Gateway supports passing environment variables and HTTP headers to backend servers, allowing you to use server-specific tool filtering mechanisms.

## Use Cases

Tool filtering is useful when:
- A backend server exposes tools with problematic schemas
- You want to limit which tools are available to clients
- You need to exclude specific tools for security or performance reasons

## Configuration Examples

### Filtering Tools for Stdio/Docker Backends

For stdio-based backends (including Docker containers), use the `env` field to pass environment variables:

**JSON Configuration:**
```json
{
  "mcpServers": {
    "github": {
      "type": "stdio",
      "container": "ghcr.io/github/github-mcp-server:latest",
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_PAT}",
        "GITHUB_TOOLS": "issue_read,list_issues,get_file_contents,get_repository"
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

[servers.github.env]
GITHUB_PERSONAL_ACCESS_TOKEN = "${GITHUB_PAT}"
GITHUB_TOOLS = "issue_read,list_issues,get_file_contents,get_repository"
```

### Filtering Tools for HTTP Backends

For HTTP-based backends, use the `headers` field to pass custom HTTP headers:

**JSON Configuration:**
```json
{
  "mcpServers": {
    "github": {
      "type": "http",
      "url": "https://example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${API_TOKEN}",
        "X-MCP-Tools": "issue_read,list_issues,get_file_contents,get_repository"
      }
    }
  }
}
```

**TOML Configuration:**
```toml
[servers.github]
type = "http"
url = "https://example.com/mcp"

[servers.github.headers]
Authorization = "Bearer ${API_TOKEN}"
X-MCP-Tools = "issue_read,list_issues,get_file_contents,get_repository"
```

## GitHub MCP Server Tool Filtering

The [GitHub MCP Server](https://github.com/github/github-mcp-server) supports tool filtering via:
- `GITHUB_TOOLS` environment variable (for stdio/Docker)
- `X-MCP-Tools` HTTP header (for HTTP deployments)

### Example: Excluding Problematic Tools

If a specific tool (e.g., `get_commit`) has schema issues, you can exclude it:

```json
{
  "mcpServers": {
    "github": {
      "type": "stdio",
      "container": "ghcr.io/github/github-mcp-server:latest",
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_PAT}",
        "GITHUB_TOOLS": "issue_read,list_issues,get_file_contents,get_repository,list_pull_requests,pull_request_read,search_repositories,search_code,search_issues,list_commits,list_branches,list_tags"
      }
    }
  }
}
```

### Default Tools (Toolsets)

The GitHub MCP Server also supports predefined toolsets. Consult the [GitHub MCP Server documentation](https://github.com/github/github-mcp-server) for available toolsets.

## How It Works

1. **Stdio Backends**: The gateway passes environment variables from the `env` configuration field to the backend process
2. **HTTP Backends**: The gateway includes headers from the `headers` configuration field in all HTTP requests to the backend
3. **Backend Processing**: The backend server reads these values and performs its own tool filtering based on the values provided
4. **Gateway Role**: The gateway acts as a conduit, passing filtering configuration to backends but not performing the filtering itself - the actual tool filtering logic resides in the backend servers

## Schema Handling

The MCP Gateway handles tool schemas as follows:

### Routed Mode (`/mcp/{serverID}`)
- Gateway forwards `tools/list` requests directly to backend servers
- Tool schemas pass through **unchanged** from backend to client
- No schema modification or validation occurs

### Unified Mode (`/mcp`)
- Gateway stores tool schemas internally for reference
- When registering tools with the SDK, InputSchema may be omitted to avoid JSON Schema version conflicts
- This prevents validation errors between different schema versions (e.g., draft-07 vs draft-2020-12)

## Important Notes

1. **Backend Responsibility**: Tool filtering happens at the backend server level, not in the gateway
2. **Schema Preservation**: The gateway does not modify or corrupt tool schemas from backends
3. **Variable Expansion**: Environment variables support `${VAR_NAME}` syntax for variable expansion
4. **Case Sensitivity**: Tool names are typically case-sensitive; check your backend's documentation

## Related Configuration

See the [MCP Gateway Configuration Reference](https://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md) for complete configuration options.

## Troubleshooting

### Tools Not Being Filtered

1. Verify the backend server supports the environment variable or header you're using
2. Check that tool names are spelled correctly and match exactly
3. Review backend server logs to confirm it received the filtering configuration

### Schema Validation Errors

If you see schema validation errors:
1. The issue is likely in the backend server's tool schema definition
2. Use tool filtering to exclude problematic tools
3. Report schema issues to the backend server's maintainers

### Connection Issues

If filtering causes connection problems:
1. Ensure the backend server supports the filtering mechanism
2. Verify the syntax matches the backend's expectations (comma-separated, etc.)
3. Try without filtering first to isolate the issue

## References

- [MCP Specification](https://github.com/modelcontextprotocol)
- [GitHub MCP Server](https://github.com/github/github-mcp-server)
- [MCP Gateway Configuration](https://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md)
