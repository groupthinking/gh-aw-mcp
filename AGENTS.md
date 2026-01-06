# Agent Documentation for MCP Gateway

This document provides information for AI agents working with the MCP Gateway project.

## Project Overview

**MCP Gateway** (formerly FlowGuard-Go) is a Go-based proxy server for Model Context Protocol (MCP) servers. It provides routing, aggregation, and management capabilities for multiple MCP backend servers.

### Key Features

- **Configuration Modes**: Supports both TOML files and JSON stdin configuration
- **Routing Modes**: 
  - **Routed**: Each backend server accessible at `/mcp/{serverID}`
  - **Unified**: Single endpoint `/mcp` that routes to configured servers
- **Docker Support**: Launch backend MCP servers as Docker containers
- **Stdio Transport**: JSON-RPC 2.0 over stdin/stdout for MCP communication
- **DIFC Integration**: Decentralized Information Flow Control for security (implemented but not yet enabled)

## Technology Stack

- **Language**: Go 1.25.0
- **Key Dependencies**:
  - `github.com/spf13/cobra` - CLI framework
  - `github.com/BurntSushi/toml` - TOML configuration parser
  - `github.com/modelcontextprotocol/go-sdk` - MCP protocol SDK
- **Container Runtime**: Docker
- **Protocol**: JSON-RPC 2.0, MCP (Model Context Protocol)

## Project Structure

```
/home/runner/work/gh-aw-mcpg/gh-aw-mcpg/
├── main.go              # Application entry point
├── go.mod               # Go module dependencies
├── Dockerfile           # Container image definition
├── config.toml          # TOML configuration example
├── config.json          # JSON configuration example
├── run.sh               # Launch script
├── internal/
│   ├── cmd/             # CLI commands (Cobra-based)
│   ├── config/          # Configuration loading and parsing
│   ├── launcher/        # Backend server process management
│   ├── mcp/             # MCP protocol types and connection handling
│   ├── server/          # HTTP server implementation
│   ├── difc/            # Decentralized Information Flow Control
│   ├── guard/           # Security guard framework
│   └── sys/             # System utilities
├── docs/                # Documentation
└── .github/
    └── workflows/       # GitHub Actions workflows
```

## Building and Testing

### Build Commands

```bash
# Build the binary
go build -o flowguard-go

# Build with Docker
docker build -t flowguard-go .
```

### Test Commands

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/server
go test ./internal/config
```

### Linting

```bash
# Run go vet
go vet ./...

# Run go fmt check
go fmt ./...
```

## Configuration

### TOML Configuration (`config.toml`)

```toml
[servers]

[servers.github]
command = "docker"
args = ["run", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "-i", "ghcr.io/github/github-mcp-server:latest"]

[servers.filesystem]
command = "node"
args = ["/path/to/filesystem-server.js"]
```

### JSON Stdin Configuration

```json
{
  "mcpServers": {
    "github": {
      "type": "local",
      "container": "ghcr.io/github/github-mcp-server:latest",
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": ""
      }
    }
  }
}
```

## Running the Application

### Local Development

```bash
# Run with TOML config
./flowguard-go --config config.toml

# Run with JSON stdin config
echo '{"mcpServers": {...}}' | ./flowguard-go --config-stdin

# Run with environment file
./flowguard-go --env .env --config config.toml --listen 127.0.0.1:8000 --routed
```

### With Docker

```bash
docker run --rm \
  -v $(pwd)/.env:/app/.env \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -p 8000:8000 \
  flowguard-go
```

## API Endpoints

### Routed Mode (default)
- `POST /mcp/{serverID}` - Send JSON-RPC request to specific server
- `GET /health` - Health check endpoint

### Unified Mode
- `POST /mcp` - Send JSON-RPC request (routed to first configured server)
- `GET /health` - Health check endpoint

### Supported MCP Methods
- `initialize` - Initialize MCP session
- `tools/list` - List available tools
- `tools/call` - Call a tool with parameters
- Any other MCP method (forwarded as-is)

## Code Conventions

### Go Style Guidelines

1. **Package Organization**: 
   - Internal packages live in `internal/`
   - Follow domain-driven design principles (cmd, config, server, etc.)

2. **Error Handling**:
   - Always check and handle errors explicitly
   - Use descriptive error messages
   - Wrap errors with context when appropriate

3. **Testing**:
   - Test files end with `_test.go`
   - Use table-driven tests where appropriate
   - Mock external dependencies (Docker, network calls)

4. **Naming Conventions**:
   - Use camelCase for unexported identifiers
   - Use PascalCase for exported identifiers
   - Interface names typically end with "-er" (e.g., `Launcher`, `Server`)

5. **Comments**:
   - Document all exported types, functions, and constants
   - Use godoc-style comments

## Security Considerations

### DIFC (Decentralized Information Flow Control)

The project includes a complete DIFC implementation for security:

- **Label-based Security**: Track information flow with secrecy and integrity labels
- **Reference Monitor**: Centralized policy enforcement for all MCP operations
- **Guard Framework**: Domain-specific resource labeling
- **Agent Tracking**: Per-agent taint tracking across requests

**Current Status**: All DIFC infrastructure is implemented in `internal/difc/` and `internal/guard/` but only the `NoopGuard` is active, effectively disabling enforcement.

### Authentication

- Uses `Authorization: Bearer <token>` header for agent identification
- Session management via `Mcp-Session-Id` header

## Common Tasks for Agents

### Adding a New MCP Server Configuration

1. Update `config.toml` or JSON configuration
2. Ensure the server container/command is available
3. Add necessary environment variables

### Adding New Server Routes

1. Modify `internal/server/routed.go` or `internal/server/unified.go`
2. Add handler functions
3. Update routing logic in `ServeHTTP` method

### Implementing a New Guard

1. Create new guard in `internal/guard/`
2. Implement the `Guard` interface
3. Register the guard in the registry
4. Update configuration to enable the guard

### Adding Tests

1. Create `*_test.go` file in the same package
2. Use Go's testing package
3. Follow existing test patterns (table-driven tests)
4. Mock external dependencies as needed

## Environment Variables

Key environment variables used by the application:

- `GITHUB_PERSONAL_ACCESS_TOKEN` - GitHub API authentication
- `DOCKER_API_VERSION` - Docker API version (1.43 for arm64, 1.44 for amd64)
- `CONFIG` - Path to config file (used by run.sh)
- `ENV_FILE` - Path to .env file (default: `.env`)
- `PORT` - Server port (default: `8000`)
- `HOST` - Server host (default: `127.0.0.1`)
- `MODE` - Server mode flag (default: `--routed`)

## Debugging Tips

1. **Enable verbose logging**: Check server logs for request/response details
2. **Test with curl**: Use curl commands to test endpoints directly (see README.md)
3. **Docker logs**: Check logs from backend MCP server containers
4. **Session tracking**: Use `Mcp-Session-Id` to track requests across calls

## Resources

- **Main Documentation**: [README.md](./README.md)
- **DIFC Proposal**: [docs/DIFC_INTEGRATION_PROPOSAL.md](./docs/DIFC_INTEGRATION_PROPOSAL.md)
- **MCP Protocol**: https://github.com/modelcontextprotocol
- **Go Documentation**: https://golang.org/doc/

## CI/CD Workflows

The project includes GitHub Actions workflows:

- **container.yml**: Builds and pushes multi-arch Docker images to GHCR
- **go-fan.md**: Daily Go module usage reviewer (Copilot step action)

## Notes for AI Agents

1. **Minimal Changes**: Make the smallest possible changes to achieve goals
2. **Test Before Committing**: Run `go test ./...` before committing changes
3. **Follow Go Conventions**: Use `gofmt` to format code
4. **Update Documentation**: Keep README.md and this file updated with changes
5. **Docker Dependencies**: Many tests may require Docker to be running
6. **Security First**: Be mindful of DIFC patterns when adding features
