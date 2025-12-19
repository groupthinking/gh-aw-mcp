# FlowGuard (Go Port)

A simplified Go port of FlowGuard - a proxy server for Model Context Protocol (MCP) servers.

## Features

- **Configuration Modes**: Supports both TOML files and JSON stdin configuration
- **Routing Modes**: 
  - **Routed**: Each backend server accessible at `/mcp/{serverID}`
  - **Unified**: Single endpoint `/mcp` that routes to configured servers
- **Docker Support**: Launch backend MCP servers as Docker containers
- **Stdio Transport**: JSON-RPC 2.0 over stdin/stdout for MCP communication

## Quick Start

### Prerequisites

1. **Docker** installed and running
2. **Go 1.23+** for building from source

### Setup Steps

1. **Build the binary**
   ```bash
   go build -o flowguard-go
   ```

2. **Create your environment file**
   ```bash
   cp example.env .env
   ```

3. **Create a GitHub Personal Access Token**
   - Go to https://github.com/settings/tokens
   - Click "Generate new token (classic)"
   - Select scopes as needed (e.g., `repo` for repository access)
   - Copy the generated token

4. **Add your token to `.env`**
   
   Replace the placeholder value with your actual token:
   ```bash
   sed -i '' 's/GITHUB_PERSONAL_ACCESS_TOKEN=.*/GITHUB_PERSONAL_ACCESS_TOKEN=your_token_here/' .env
   ```
   
   Or edit `.env` manually and replace the value of `GITHUB_PERSONAL_ACCESS_TOKEN`.

5. **Pull required Docker images**
   ```bash
   docker pull ghcr.io/github/github-mcp-server:latest
   docker pull mcp/fetch
   docker pull mcp/memory
   ```

6. **Start FlowGuard**
   
   In one terminal, run:
   ```bash
   ./run.sh
   ```
   
   This will start FlowGuard in routed mode on `http://127.0.0.1:8000`.

7. **Run Codex (in another terminal)**
   ```bash
   cp ~/.codex/config.toml ~/.codex/config.toml.bak && cp agent-configs/codex.config.toml ~/.codex/config.toml
   AGENT_ID=demo-agent codex
   ```
   
   You can use '/mcp' in codex to list the available tools. 

   That's it! FlowGuard is now proxying MCP requests to your configured backend servers.

   When you're done you can restore your old codex config file:

   ```bash
   cp ~/.codex/config.toml.bak ~/.codex/config.toml
   ```

### Manual Build & Run

If you prefer to run manually without the `run.sh` script:

```bash
# Run with TOML config
./flowguard-go --config config.toml

# Run with JSON stdin config
echo '{"mcpServers": {...}}' | ./flowguard-go --config-stdin
```

## Configuration

### TOML Format (`config.toml`)

```toml
[servers]

[servers.github]
command = "docker"
args = ["run", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "-i", "ghcr.io/github/github-mcp-server:latest"]

[servers.filesystem]
command = "node"
args = ["/path/to/filesystem-server.js"]
```

### JSON Stdin Format

```json
{
  "mcpServers": {
    "github": {
      "type": "local",
      "container": "ghcr.io/github/github-mcp-server:latest",
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": ""
      },
    }
  }
}
```

**Environment Variable Passthrough**: Set the value to an empty string (`""`) to pass through the variable from the host environment.

## Usage

```
FlowGuard is a proxy server for Model Context Protocol (MCP) servers.
It provides routing, aggregation, and management of multiple MCP backend servers.

Usage:
  flowguard-go [flags]

Flags:
  -c, --config string   Path to config file (default "config.toml")
      --config-stdin    Read MCP server configuration from stdin (JSON format). When enabled, overrides --config
      --env string      Path to .env file to load environment variables
  -h, --help            help for flowguard-go
  -l, --listen string   HTTP server listen address (default "127.0.0.1:3000")
      --routed          Run in routed mode (each backend at /mcp/<server>)
      --unified         Run in unified mode (all backends at /mcp)
```

## Docker

### Build Image

```bash
docker build -t flowguard-go .
```

### Run Container

```bash
docker run --rm -v $(pwd)/.env:/app/.env \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -p 8000:8000 \
  flowguard-go
```

The container uses `run.sh` as the entrypoint, which automatically:
- Loads environment variables from `.env`
- Starts FlowGuard in routed mode on port 8000
- Reads configuration from stdin (via heredoc in run.sh)

### Override with config file

To use a custom config file instead:

```bash
docker run --rm -v $(pwd)/config.toml:/app/config.toml \
  -v $(pwd)/.env:/app/.env \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -p 8000:8000 \
  flowguard-go /app/flowguard-go --config /app/config.toml --env /app/.env
```


## API Endpoints

### Routed Mode (default)

- `POST /mcp/{serverID}` - Send JSON-RPC request to specific server
  - Example: `POST /mcp/github` with body `{"jsonrpc": "2.0", "method": "tools/list", "id": 1}`

### Unified Mode

- `POST /mcp` - Send JSON-RPC request (routed to first configured server)

### Health Check

- `GET /health` - Returns `OK`

## MCP Methods

Supported JSON-RPC 2.0 methods:

- `tools/list` - List available tools
- `tools/call` - Call a tool with parameters
- Any other MCP method (forwarded as-is)

## Architecture Simplifications

This Go port focuses on core MCP proxy functionality:

- ✅ TOML and JSON stdin configuration
- ✅ Stdio transport for backend servers
- ✅ Docker container launching
- ✅ Routed and unified modes
- ✅ Basic request/response proxying
- ❌ DIFC enforcement (removed)
- ❌ Sub-agents (removed)
- ❌ Guards (removed)

## Development

### Project Structure

```
flowguard-go/
├── main.go              # Entry point
├── go.mod               # Dependencies
├── Dockerfile           # Container image
└── internal/
    ├── cmd/             # CLI commands (cobra)
    ├── config/          # Configuration loading
    ├── launcher/        # Backend server management
    ├── mcp/             # MCP protocol types & connection
    └── server/          # HTTP server
```

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/BurntSushi/toml` - TOML parser
- Standard library for JSON, HTTP, exec

## License

Same as original FlowGuard project.
