# MCP Gateway

A gateway for Model Context Protocol (MCP) servers.

This gateway is used with [GitHub Agentic Workflows](https://github.com/githubnext/gh-aw) via the `sandbox.mcp` configuration to provide MCP server access to AI agents running in sandboxed environments.

📖 **[Full Configuration Specification](https://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md)** - Complete reference for all configuration options and validation rules.

## Features

- **Configuration Modes**: Supports both TOML files and JSON stdin configuration
  - **Spec-Compliant Validation**: Fail-fast validation with detailed error messages
  - **Variable Expansion**: Environment variable substitution with `${VAR_NAME}` syntax
  - **Type Normalization**: Automatic conversion of legacy `"local"` type to `"stdio"`
- **Routing Modes**: 
  - **Routed**: Each backend server accessible at `/mcp/{serverID}`
  - **Unified**: Single endpoint `/mcp` that routes to configured servers
- **Docker Support**: Launch backend MCP servers as Docker containers
- **Stdio Transport**: JSON-RPC 2.0 over stdin/stdout for MCP communication
- **Container Detection**: Automatic detection of containerized environments with security warnings
- **Enhanced Debugging**: Detailed error context and troubleshooting suggestions for command failures

## Getting Started

For detailed setup instructions, building from source, and local development, see [CONTRIBUTING.md](CONTRIBUTING.md).

### Quick Start with Docker

1. **Pull the Docker image** (when available):
   ```bash
   docker pull ghcr.io/githubnext/gh-aw-mcpg:latest
   ```

2. **Run the container**:
   ```bash
   docker run --rm -v $(pwd)/.env:/app/.env \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -p 8000:8000 \
     ghcr.io/githubnext/gh-aw-mcpg:latest
   ```

MCPG will start in routed mode on `http://127.0.0.1:8000`, proxying MCP requests to your configured backend servers.

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

For the complete JSON configuration specification with all validation rules, see the **[MCP Gateway Configuration Reference](https://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md)**.

```json
{
  "mcpServers": {
    "github": {
      "type": "stdio",
      "container": "ghcr.io/github/github-mcp-server:latest",
      "entrypoint": "/custom/entrypoint.sh",
      "entrypointArgs": ["--verbose"],
      "mounts": [
        "/host/config:/app/config:ro",
        "/host/data:/app/data:rw"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "",
        "EXPANDED_VAR": "${MY_HOME}/config"
      }
    }
  },
  "gateway": {
    "port": 8080,
    "apiKey": "your-api-key",
    "domain": "example.com",
    "startupTimeout": 30,
    "toolTimeout": 60
  }
}
```

#### Server Configuration Fields

- **`type`** (optional): Server transport type
  - `"stdio"` - Standard input/output transport (default)
  - `"http"` - HTTP transport (not yet implemented)
  - `"local"` - Alias for `"stdio"` (backward compatibility)

- **`container`** (required for stdio): Docker container image (e.g., `"ghcr.io/github/github-mcp-server:latest"`)
  - Automatically wraps as `docker run --rm -i <container>`
  - **Note**: The `command` field is NOT supported per the specification

- **`entrypoint`** (optional): Custom entrypoint for the container
  - Overrides the default container entrypoint
  - Applied as `--entrypoint` flag to Docker

- **`entrypointArgs`** (optional): Arguments passed to container entrypoint
  - Array of strings passed after the container image

- **`mounts`** (optional): Volume mounts for the container
  - Array of strings in format `"source:dest:mode"`
  - `source` - Host path to mount (can use environment variables with `${VAR}` syntax)
  - `dest` - Container path where the volume is mounted
  - `mode` - Either `"ro"` (read-only) or `"rw"` (read-write)
  - Example: `["/host/config:/app/config:ro", "/host/data:/app/data:rw"]`

- **`env`** (optional): Environment variables
  - Set to `""` (empty string) for passthrough from host environment
  - Set to `"value"` for explicit value
  - Use `"${VAR_NAME}"` for environment variable expansion (fails if undefined)

- **`url`** (required for http): HTTP endpoint URL for `type: "http"` servers

**Validation Rules:**

- **Stdio servers** must specify `container` (required)
- **HTTP servers** must specify `url` (required)
- Empty/"local" type automatically normalized to "stdio"
- Variable expansion with `${VAR_NAME}` fails fast on undefined variables
- All validation errors include JSONPath and helpful suggestions
- **The `command` field is not supported** - stdio servers must use `container`
- **Mount specifications** must follow `"source:dest:mode"` format
  - `mode` must be either `"ro"` or `"rw"`
  - Both source and destination paths are required (cannot be empty)

See **[Configuration Specification](https://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md)** for complete validation rules.

#### Gateway Configuration Fields (Reserved)

- **`port`** (optional): Gateway HTTP port (default: from `--listen` flag)
  - Valid range: 1-65535
- **`apiKey`** (optional): API key for authentication
- **`domain`** (optional): Domain name for the gateway
- **`startupTimeout`** (optional): Seconds to wait for backend startup (default: 30)
  - Must be positive integer
- **`toolTimeout`** (optional): Seconds to wait for tool execution (default: 60)
  - Must be positive integer

**Note**: Gateway configuration fields are validated and parsed but not yet fully implemented.

**Environment Variable Features**:
- **Passthrough**: Set value to empty string (`""`) to pass through from host
- **Expansion**: Use `${VAR_NAME}` syntax for dynamic substitution (fails if undefined)

## Usage

```
MCPG is a proxy server for Model Context Protocol (MCP) servers.
It provides routing, aggregation, and management of multiple MCP backend servers.

Usage:
  awmg [flags]

Flags:
  -c, --config string   Path to config file (default "config.toml")
      --config-stdin    Read MCP server configuration from stdin (JSON format). When enabled, overrides --config
      --env string      Path to .env file to load environment variables
  -h, --help            help for awmg
  -l, --listen string   HTTP server listen address (default "127.0.0.1:3000")
      --routed          Run in routed mode (each backend at /mcp/<server>)
      --unified         Run in unified mode (all backends at /mcp)
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

## Security Features

### Enhanced Error Debugging

Command failures now include extensive debugging information:

- Full command, arguments, and environment variables
- Context-specific troubleshooting suggestions:
  - Docker daemon connectivity checks
  - Container image availability
  - Network connectivity issues
  - MCP protocol compatibility checks

## Architecture

This Go port focuses on core MCP proxy functionality with optional security features:

### Core Features (Enabled)

- ✅ TOML and JSON stdin configuration with spec-compliant validation
- ✅ Environment variable expansion (`${VAR_NAME}`) with fail-fast behavior
- ✅ Stdio transport for backend servers (containerized execution only)
- ✅ Docker container launching
- ✅ Routed and unified modes
- ✅ Basic request/response proxying
- ✅ Enhanced error debugging and troubleshooting

### DIFC Integration (Not Yet Enabled)

MCPG includes a complete implementation of **Decentralized Information Flow Control (DIFC)** for information security, but it is **not yet enabled by default**. The DIFC system provides:

- **Label-based Security**: Track information flow with secrecy and integrity labels
- **Reference Monitor**: Centralized policy enforcement for all MCP operations
- **Guard Framework**: Domain-specific resource labeling (e.g., GitHub repos, files)
- **Agent Tracking**: Per-agent taint tracking across requests
- **Fine-grained Control**: Collection filtering for partial access to resources

#### DIFC Components (Implemented)

```
internal/difc/
├── labels.go        # Secrecy/integrity labels with flow semantics
├── resource.go      # Resource labeling (coarse & fine-grained)
├── evaluator.go     # DIFC policy evaluation & enforcement
├── agent.go         # Per-agent label tracking (taint tracking)
└── capabilities.go  # Global tag registry

internal/guard/
├── guard.go         # Guard interface definition
├── noop.go          # NoopGuard (default, allows all operations)
├── registry.go      # Guard registration & lookup
└── context.go       # Agent ID extraction utilities
```

#### How DIFC Works (When Enabled)

1. **Resource Labeling**: Guards label resources based on domain knowledge (e.g., "repo:owner/name", "visibility:private")
2. **Agent Tracking**: Each agent has secrecy/integrity labels that accumulate through reads (taint tracking)
3. **Policy Enforcement**: Reference Monitor checks if operations violate label flow semantics:
   - **Read**: Resource secrecy must flow to agent secrecy (resource ⊆ agent)
   - **Write**: Agent integrity must flow to resource integrity (agent ⊆ resource)
4. **Fine-grained Filtering**: Collections (e.g., search results) automatically filtered to allowed items

#### Enabling DIFC (Future)

To enable DIFC enforcement, you'll need to:

1. **Implement domain-specific guards** (e.g., GitHub, filesystem)
2. **Configure agent labels** in `config.toml`
3. **Register guards** in server initialization

See [`docs/DIFC_INTEGRATION_PROPOSAL.md`](docs/DIFC_INTEGRATION_PROPOSAL.md) for full design details.

**Current Status**: All DIFC infrastructure is implemented and tested, but only the `NoopGuard` is active (which returns empty labels, effectively disabling enforcement). Custom guards for specific backends (GitHub, filesystem, etc.) are not yet implemented.

## Contributing

For development setup, build instructions, testing guidelines, and project architecture details, see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT License
