# AGENTS.md

Quick reference for AI agents working with MCP Gateway (Go-based MCP proxy server).

## Quick Start

**Install**: `make install` (install toolchains and dependencies)  
**Build**: `make build` (builds `awmg` binary)  
**Test**: `make test`  
**Lint**: `make lint` (runs go vet and gofmt checks)  
**Coverage**: `make coverage` (tests with coverage report)  
**Format**: `make format` (auto-format code with gofmt)  
**Clean**: `make clean` (remove build artifacts)  
**Agent-Finished**: `make agent-finished` (run format, build, lint, test - ALWAYS run before completion)  
**Run**: `./awmg --config config.toml`
**Run with Custom Log Directory**: `./awmg --config config.toml --log-dir /path/to/logs`

## Project Structure

- `internal/cmd/` - CLI (Cobra)
- `internal/config/` - Config parsing (TOML/JSON) with validation
  - `validation.go` - Variable expansion and fail-fast validation
  - `validation_test.go` - 21 comprehensive validation tests
- `internal/server/` - HTTP server (routed/unified modes)
- `internal/mcp/` - MCP protocol types with enhanced error logging
- `internal/launcher/` - Backend process management
- `internal/difc/` - Security labels (not enabled)
- `internal/guard/` - Security guards (NoopGuard active)
- `internal/logger/` - Debug logging framework (micro logger)
- `internal/timeutil/` - Time formatting utilities

## Key Tech

- **Go 1.25.0** with `cobra`, `toml`, `go-sdk`
- **Protocol**: JSON-RPC 2.0 over stdio
- **Routing**: `/mcp/{serverID}` (routed) or `/mcp` (unified)
- **Docker**: Launches MCP servers as containers
- **Validation**: Spec-compliant with fail-fast error handling
- **Variable Expansion**: `${VAR_NAME}` syntax for environment variables

## Config Examples

**Configuration Spec**: See **[MCP Gateway Configuration Reference](https://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md)** for complete specification.

**TOML** (`config.toml`):
```toml
[servers.github]
command = "docker"
args = ["run", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "-i", "ghcr.io/github/github-mcp-server:latest"]
```

**JSON** (stdin):
```json
{
  "mcpServers": {
    "github": {
      "type": "stdio",
      "container": "ghcr.io/github/github-mcp-server:latest",
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "",
        "CONFIG_PATH": "${GITHUB_CONFIG_DIR}"
      }
    }
  }
}
```

**Supported Types**: `"stdio"`, `"http"` (not implemented), `"local"` (alias for stdio)

**Validation Features**:
- Environment variable expansion: `${VAR_NAME}` (fails if undefined)
- Required fields: `container` for stdio, `url` for http
- **Note**: The `command` field is not supported - stdio servers must use `container`
- Port range validation: 1-65535
- Timeout validation: positive integers only

## Go Conventions

- Internal packages in `internal/`
- Test files: `*_test.go` with table-driven tests
- Naming: camelCase (private), PascalCase (public)
- Always handle errors explicitly
- Godoc comments for exports
- Mock external dependencies (Docker, network)

## Common Tasks

**Add MCP Server**: Update config.toml with new server entry  
**Add Route**: Edit `internal/server/routed.go` or `unified.go`  
**Add Guard**: Implement in `internal/guard/` and register  
**Add Test**: Create `*_test.go` with Go testing package

## Agent Completion Checklist

**CRITICAL: Before returning to the user, ALWAYS run `make agent-finished`**

This command runs the complete verification pipeline:
1. **Format** - Auto-formats all Go code with gofmt
2. **Build** - Ensures the project compiles successfully
3. **Lint** - Runs go vet and gofmt checks
4. **Test** - Executes the full test suite

**Requirements:**
- **ALL failures must be fixed** before completion
- If `make agent-finished` fails at any stage, debug and fix the issue
- Re-run `make agent-finished` after fixes to verify success
- Only report completion to the user after seeing "✓ All agent-finished checks passed!"

**Example workflow:**
```bash
# Make your code changes
# ...

# Run verification before completion
make agent-finished

# If any step fails, fix the issues and run again
# Only complete the task after all checks pass
```

## Debug Logging

**ALWAYS use the logger package for debug logging:**

```go
import "github.com/githubnext/gh-aw-mcpg/internal/logger"

// Create a logger with namespace following pkg:filename convention
var log = logger.New("pkg:filename")

// Log debug messages (only shown when DEBUG environment variable matches)
log.Printf("Processing %d items", count)
log.Print("Simple debug message")

// Check if logging is enabled before expensive operations
if log.Enabled() {
    log.Printf("Expensive debug info: %+v", expensiveOperation())
}
```

**For operational/file logging, use the file logger:**

```go
import "github.com/githubnext/gh-aw-mcpg/internal/logger"

// Log operational events (written to mcp-gateway.log)
logger.LogInfo("category", "Operation completed successfully")
logger.LogWarn("category", "Potential issue detected: %s", issue)
logger.LogError("category", "Operation failed: %v", err)
logger.LogDebug("category", "Debug details: %+v", details)
```

**Logging Categories:**
- `startup` - Gateway initialization and configuration
- `shutdown` - Graceful shutdown events
- `client` - MCP client interactions and requests
- `backend` - Backend MCP server operations
- `auth` - Authentication events (success and failures)

**Category Naming Convention:**
- Follow the pattern: `pkg:filename` (e.g., `server:routed`, `launcher:docker`)
- Use colon (`:`) as separator between package and file/component name
- Be consistent with existing loggers in the codebase

**Debug Output Control:**
```bash
# Enable all debug logs
DEBUG=* ./awmg --config config.toml

# Enable specific package
DEBUG=server:* ./awmg --config config.toml

# Enable multiple packages
DEBUG=server:*,launcher:* ./awmg --config config.toml

# Exclude specific loggers
DEBUG=*,-launcher:test ./awmg --config config.toml

# Disable colors (auto-disabled when piping)
DEBUG_COLORS=0 DEBUG=* ./awmg --config config.toml
```

**Key Features:**
- **Zero overhead**: Logs only computed when DEBUG matches the logger's namespace
- **Time diff**: Shows elapsed time between log calls (e.g., `+50ms`, `+2.5s`)
- **Auto-colors**: Each namespace gets a consistent color in terminals
- **Pattern matching**: Supports wildcards (`*`) and exclusions (`-pattern`)

**When to Use:**
- Non-essential diagnostic information
- Performance insights and timing data
- Internal state tracking during development
- Detailed operation flow for debugging

**When NOT to Use:**
- Essential user-facing messages (use standard logging)
- Error messages (use proper error handling)
- Final output or results (use stdout)

## Environment Variables

- `GITHUB_PERSONAL_ACCESS_TOKEN` - GitHub auth
- `DOCKER_API_VERSION` - 1.43 (arm64) or 1.44 (amd64)
- `PORT`, `HOST`, `MODE` - Server config (via run.sh)
- `DEBUG` - Enable debug logging (e.g., `DEBUG=*`, `DEBUG=server:*,launcher:*`)
- `DEBUG_COLORS` - Control colored output (0 to disable, auto-disabled when piping)

**File Logging:**
- Operational logs are always written to `mcp-gateway.log` in the configured log directory
- Default log directory: `/tmp/gh-aw/sandbox/mcp` (configurable via `--log-dir` flag)
- Falls back to stdout if log directory cannot be created
- Logs include: startup, client interactions, backend operations, auth events, errors

## Error Debugging

**Enhanced Error Context**: Command failures include:
- Full command, args, and environment variables
- Context-specific troubleshooting suggestions:
  - Docker daemon connectivity checks
  - Container image availability
  - Network connectivity issues
  - MCP protocol compatibility checks

## Security Notes

- Auth: `Authorization: Bearer <token>` header
- Sessions: `Mcp-Session-Id` header
- DIFC: Implemented but disabled (NoopGuard active)
- Stdio servers: Containerized execution only (no direct command support)

## Resources

- [README.md](./README.md) - Full documentation
- [DIFC Proposal](./docs/DIFC_INTEGRATION_PROPOSAL.md) - Security design
- [MCP Protocol](https://github.com/modelcontextprotocol) - Specification
