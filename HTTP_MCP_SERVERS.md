# HTTP MCP Server Support

The MCP Gateway now supports routing requests to HTTP-based MCP servers that are already running. This is useful for integrating with externally managed MCP services.

## Configuration

To configure an HTTP MCP server, use the following format in your configuration:

```json
{
  "mcpServers": {
    "safeinputs": {
      "type": "http",
      "url": "http://host.docker.internal:3000/",
      "headers": {
        "Authorization": "your-auth-secret"
      }
    }
  },
  "gateway": {
    "port": 3001,
    "domain": "localhost",
    "apiKey": "gateway-api-key"
  }
}
```

### Configuration Fields

- `type`: Must be `"http"` for HTTP-based MCP servers
- `url`: The HTTP endpoint URL for the MCP server (required)
- `headers`: Optional HTTP headers to include in requests (commonly used for authentication)

### Environment Variable Expansion

You can use environment variable expansion in header values:

```json
{
  "mcpServers": {
    "safeinputs": {
      "type": "http",
      "url": "http://host.docker.internal:3000/",
      "headers": {
        "Authorization": "${SAFEINPUTS_AUTH_TOKEN}"
      }
    }
  }
}
```

Set the environment variable before starting the gateway:

```bash
export SAFEINPUTS_AUTH_TOKEN="your-secret-token"
cat config.json | ./awmg --config-stdin --routed
```

## Routing

In routed mode, HTTP MCP servers are accessible at:

```
http://gateway-host:gateway-port/mcp/<server-name>
```

For example, with the configuration above:
- Gateway running on `localhost:3001`
- HTTP server configured as `safeinputs`
- Access via: `http://localhost:3001/mcp/safeinputs`

## How It Works

1. When the gateway receives a configuration for an HTTP MCP server:
   - It stores the URL and headers
   - It creates an HTTP connection (no process launch required)

2. When a client sends a request to `/mcp/safeinputs`:
   - The gateway validates the client's authorization
   - It forwards the JSON-RPC request to the HTTP backend
   - It includes the configured headers (e.g., Authorization)
   - It returns the response from the HTTP backend to the client

3. The HTTP backend never sees the gateway's API key - only its own configured headers.

## Example: Safeinputs Service

If you have a safeinputs MCP server already running at `http://host.docker.internal:3000/`:

1. Create a config file:

```json
{
  "mcpServers": {
    "safeinputs": {
      "type": "http",
      "url": "http://host.docker.internal:3000/",
      "headers": {
        "Authorization": "safeinputs-secret-key"
      }
    }
  },
  "gateway": {
    "port": 3001,
    "domain": "localhost",
    "apiKey": "gateway-api-key"
  }
}
```

2. Start the gateway:

```bash
cat config.json | ./awmg --config-stdin --routed
```

3. Send requests to the gateway:

```bash
curl -X POST http://localhost:3001/mcp/safeinputs \
  -H "Authorization: gateway-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'
```

The gateway will:
- Validate your `gateway-api-key`
- Forward the request to `http://host.docker.internal:3000/`
- Include `Authorization: safeinputs-secret-key` header
- Return the tools list from the safeinputs backend

## Mixed Configuration

You can configure both HTTP and stdio (Docker) servers in the same gateway:

```json
{
  "mcpServers": {
    "safeinputs": {
      "type": "http",
      "url": "http://host.docker.internal:3000/",
      "headers": {
        "Authorization": "safeinputs-auth"
      }
    },
    "github": {
      "type": "stdio",
      "container": "ghcr.io/github/github-mcp-server:latest",
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": ""
      }
    }
  },
  "gateway": {
    "port": 3001,
    "domain": "localhost",
    "apiKey": "gateway-api-key"
  }
}
```

Both servers will be available at:
- `http://localhost:3001/mcp/safeinputs` (HTTP backend)
- `http://localhost:3001/mcp/github` (stdio/Docker backend)
