# Safe Inputs Specification for MCP Gateway

## Overview

This document specifies how to integrate the **Safe Inputs** functionality from GitHub Agentic Workflows into the MCP Gateway as a built-in feature. Safe Inputs allows users to define custom MCP tools inline using JavaScript, shell scripts, Python, or Go code, with controlled secret access and automatic runtime generation.

## Background

### What is Safe Inputs?

Safe Inputs is a feature in GitHub Agentic Workflows that allows workflow authors to define custom MCP tools directly in their workflow configuration using inline code (JavaScript, shell, Python, or Go). These tools are:

1. **Generated at runtime** from inline configuration
2. **Mounted as MCP servers** with automatic stdio transport
3. **Isolated** through process-level execution
4. **Secured** with controlled environment variable and secret access
5. **Validated** with JSON Schema input validation

### Current Implementation (gh-aw)

The current implementation in the `gh-aw` repository consists of:

- **`safe_inputs_mcp_server.cjs`**: Main server that reads tool configuration from JSON
- **`mcp_server_core.cjs`**: Core MCP protocol implementation with JSON-RPC 2.0
- **Handler modules**: Separate handlers for different execution types:
  - `mcp_handler_javascript.cjs`: Execute .cjs files in separate Node.js processes
  - `mcp_handler_shell.cjs`: Execute .sh files with GitHub Actions conventions
  - `mcp_handler_python.cjs`: Execute .py files with JSON stdin
  - `mcp_handler_go.cjs`: Execute .go files with `go run`
- **Configuration modules**:
  - `safe_inputs_config_loader.cjs`: Load and validate tool configurations
  - `safe_inputs_tool_factory.cjs`: Factory for creating tool configs
  - `safe_inputs_bootstrap.cjs`: Bootstrap and cleanup logic
  - `safe_inputs_validation.cjs`: Input validation helpers

### Key Features

1. **Multiple Language Support**: JavaScript, Shell, Python, and Go
2. **Input Validation**: JSON Schema-based validation with required fields
3. **Secret Management**: Controlled environment variable access via `env:` configuration
4. **Timeout Control**: Configurable execution timeouts (default: 60 seconds)
5. **Output Handling**: Large outputs (>500 chars) saved to files automatically
6. **Handler Isolation**: Each handler type runs in a separate process
7. **Path Security**: Relative paths validated within base directory to prevent traversal

## Proposed Integration into MCP Gateway

### Configuration Schema

The MCP Gateway configuration should support a new top-level `safeInputs` section that allows defining inline MCP tools. This can be specified in both JSON and TOML formats.

#### JSON Configuration Format

```json
{
  "mcpServers": {
    "github": {
      "type": "stdio",
      "container": "ghcr.io/github/github-mcp-server:latest"
    }
  },
  "safeInputs": {
    "serverName": "custom-tools",
    "version": "1.0.0",
    "logDir": "/var/log/mcp-gateway",
    "tools": [
      {
        "name": "greet_user",
        "description": "Greet a user by name",
        "inputSchema": {
          "type": "object",
          "properties": {
            "name": {
              "type": "string",
              "description": "The name to greet"
            }
          },
          "required": ["name"]
        },
        "handler": "greet.cjs",
        "timeout": 30
      },
      {
        "name": "list_files",
        "description": "List files in a directory",
        "inputSchema": {
          "type": "object",
          "properties": {
            "path": {
              "type": "string",
              "description": "Directory path to list"
            }
          },
          "required": ["path"]
        },
        "handler": "list_files.sh",
        "timeout": 60,
        "env": {
          "WORKSPACE_DIR": "/workspace",
          "LOG_LEVEL": "debug"
        }
      },
      {
        "name": "analyze_data",
        "description": "Analyze data using Python",
        "inputSchema": {
          "type": "object",
          "properties": {
            "data": {
              "type": "string",
              "description": "Comma-separated numbers"
            }
          },
          "required": ["data"]
        },
        "handler": "analyze.py",
        "timeout": 120
      },
      {
        "name": "calculate",
        "description": "Perform calculations with Go",
        "inputSchema": {
          "type": "object",
          "properties": {
            "a": {
              "type": "number",
              "description": "First number"
            },
            "b": {
              "type": "number",
              "description": "Second number"
            }
          },
          "required": ["a", "b"]
        },
        "handler": "calculate.go",
        "timeout": 45
      }
    ],
    "handlersPath": "/opt/mcp-gateway/handlers"
  }
}
```

#### TOML Configuration Format

```toml
[servers.github]
command = "docker"
args = ["run", "--rm", "-i", "ghcr.io/github/github-mcp-server:latest"]

[safeInputs]
serverName = "custom-tools"
version = "1.0.0"
logDir = "/var/log/mcp-gateway"
handlersPath = "/opt/mcp-gateway/handlers"

[[safeInputs.tools]]
name = "greet_user"
description = "Greet a user by name"
handler = "greet.cjs"
timeout = 30

[safeInputs.tools.inputSchema]
type = "object"
required = ["name"]

[safeInputs.tools.inputSchema.properties.name]
type = "string"
description = "The name to greet"

[[safeInputs.tools]]
name = "list_files"
description = "List files in a directory"
handler = "list_files.sh"
timeout = 60

[safeInputs.tools.inputSchema]
type = "object"
required = ["path"]

[safeInputs.tools.inputSchema.properties.path]
type = "string"
description = "Directory path to list"

[safeInputs.tools.env]
WORKSPACE_DIR = "/workspace"
LOG_LEVEL = "debug"
```

### Configuration Fields

#### Top-Level `safeInputs` Object

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `serverName` | string | No | `"safeinputs"` | Name of the generated MCP server |
| `version` | string | No | `"1.0.0"` | Version of the generated MCP server |
| `logDir` | string | No | (inherit from gateway) | Directory for safe inputs server logs |
| `handlersPath` | string | **Yes** | - | Base directory containing handler files (security boundary) |
| `tools` | array | **Yes** | - | Array of tool definitions |

#### Tool Definition Object

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | **Yes** | - | Unique tool name (will be normalized: dashes→underscores, lowercase) |
| `description` | string | **Yes** | - | Human-readable description shown to the agent |
| `inputSchema` | object | **Yes** | - | JSON Schema defining tool inputs |
| `handler` | string | **Yes** | - | Path to handler file relative to `handlersPath` |
| `timeout` | number | No | `60` | Maximum execution time in seconds |
| `env` | object | No | `{}` | Environment variables to pass to the handler |

#### Handler Types (by file extension)

- **`.cjs`, `.js`, `.mjs`**: JavaScript/Node.js handlers
  - Executed in separate Node.js process
  - Inputs passed as JSON via stdin
  - Expected to output JSON to stdout
  
- **`.sh`**: Shell script handlers
  - Executed with bash
  - Inputs passed as environment variables (e.g., `name` → `INPUT_NAME`)
  - Outputs read from `$GITHUB_OUTPUT` file (key=value format)
  - Returns `{ stdout, stderr, outputs }`

- **`.py`**: Python script handlers
  - Executed with `python3`
  - Inputs passed as JSON via stdin (accessible as `inputs` dict)
  - Expected to output JSON to stdout

- **`.go`**: Go code handlers
  - Executed with `go run`
  - Inputs passed as JSON via stdin (parsed into `inputs map[string]any`)
  - Expected to output JSON to stdout

### Security Model

#### Path Security

1. **Base Path Validation**: All handler paths must be relative to `handlersPath`
2. **Directory Traversal Prevention**: Resolved paths validated to be within `handlersPath`
3. **Absolute Path Handling**: Absolute paths should be rejected or logged with warnings
4. **File Existence Check**: Handler files must exist before server starts

#### Environment Variable Security

1. **Explicit Declaration**: Only environment variables declared in tool's `env` config are passed
2. **Inheritance**: Handler inherits gateway's environment unless explicitly overridden
3. **Secret Masking**: Sensitive values should be masked in logs
4. **Variable Expansion**: Support `${VAR_NAME}` expansion from gateway's environment

#### Process Isolation

1. **Separate Processes**: Each handler execution runs in isolated process
2. **Timeout Enforcement**: Handlers terminated after timeout expires
3. **Resource Limits**: Consider adding CPU/memory limits per handler
4. **Output Buffering**: 10MB max buffer for stdout/stderr

### Implementation Approach

#### Phase 1: Configuration Loading

1. **Extend Config Parser**: Add `safeInputs` section to config validation
2. **Validation Rules**:
   - Require `handlersPath` to be absolute path
   - Validate `handlersPath` exists and is readable
   - Validate each tool has required fields
   - Validate handler file paths don't escape `handlersPath`
   - Validate handler files exist

3. **Error Handling**:
   - Fail-fast on missing `handlersPath`
   - Fail-fast on missing handler files
   - Provide detailed error messages with file paths

#### Phase 2: MCP Server Generation

1. **Safe Inputs Server Creation**:
   - Generate internal MCP server from configuration
   - Register server in gateway's server map
   - Use stdio transport (same as other backends)

2. **Tool Registration**:
   - Create tool definitions from config
   - Load handlers dynamically by file extension
   - Register handlers with MCP server core

3. **Handler Loading**:
   - Implement or port handler modules from gh-aw:
     - JavaScript handler (Node.js subprocess)
     - Shell handler (bash with GitHub Actions conventions)
     - Python handler (python3 subprocess)
     - Go handler (go run subprocess)

#### Phase 3: Runtime Execution

1. **Request Routing**:
   - Route requests to safe inputs server (e.g., `/mcp/safeinputs`)
   - Standard MCP protocol handling (initialize, tools/list, tools/call)

2. **Handler Execution**:
   - Validate inputs against JSON Schema
   - Prepare environment variables
   - Execute handler with timeout
   - Capture stdout/stderr
   - Parse and return results in MCP format

3. **Error Handling**:
   - Timeout errors
   - Handler execution errors
   - Input validation errors
   - Output parsing errors

#### Phase 4: Advanced Features

1. **Large Output Handling**:
   - Detect outputs >500 characters
   - Save to temporary file
   - Return file path in response
   - Cleanup temporary files

2. **Logging and Debugging**:
   - Log handler execution start/stop
   - Log execution time
   - Log handler output (truncated)
   - Support debug mode for full output

3. **Metrics and Monitoring**:
   - Track handler execution counts
   - Track handler execution times
   - Track handler failures
   - Track timeout occurrences

### Integration Points

#### 1. Configuration Module (`internal/config/`)

**Files to modify**:
- `config.go`: Add `SafeInputs` struct
- `validation.go`: Add validation for safe inputs section

**New types**:
```go
type SafeInputsConfig struct {
    ServerName   string                 `json:"serverName" toml:"serverName"`
    Version      string                 `json:"version" toml:"version"`
    LogDir       string                 `json:"logDir" toml:"logDir"`
    HandlersPath string                 `json:"handlersPath" toml:"handlersPath"`
    Tools        []SafeInputsToolConfig `json:"tools" toml:"tools"`
}

type SafeInputsToolConfig struct {
    Name        string                 `json:"name" toml:"name"`
    Description string                 `json:"description" toml:"description"`
    InputSchema map[string]interface{} `json:"inputSchema" toml:"inputSchema"`
    Handler     string                 `json:"handler" toml:"handler"`
    Timeout     int                    `json:"timeout" toml:"timeout"`
    Env         map[string]string      `json:"env" toml:"env"`
}
```

#### 2. Launcher Module (`internal/launcher/`)

**New package**: `internal/launcher/safeinputs/`

**Files to create**:
- `server.go`: Safe inputs MCP server implementation
- `handlers.go`: Handler loading and execution logic
- `javascript.go`: JavaScript handler implementation
- `shell.go`: Shell handler implementation
- `python.go`: Python handler implementation
- `go.go`: Go handler implementation
- `validation.go`: Input validation against JSON Schema

**Key interfaces**:
```go
type Handler interface {
    Execute(ctx context.Context, args map[string]interface{}) (*HandlerResult, error)
}

type HandlerResult struct {
    Content []ContentBlock `json:"content"`
    IsError bool           `json:"isError"`
}

type ContentBlock struct {
    Type string `json:"type"`
    Text string `json:"text"`
}
```

#### 3. Server Module (`internal/server/`)

**Files to modify**:
- `routed.go`: Register safe inputs server route
- `unified.go`: Include safe inputs tools in unified endpoint

**Integration**:
- Safe inputs server appears as a regular backend MCP server
- Accessible at `/mcp/{serverName}` in routed mode
- Tools merged into `/mcp` in unified mode
- Same authentication and authorization as other servers

#### 4. MCP Module (`internal/mcp/`)

**Potential new files**:
- `safeinputs.go`: Safe inputs-specific MCP protocol helpers
- `schema.go`: JSON Schema validation helpers

### Error Scenarios and Handling

| Scenario | Detection | Response | HTTP Status |
|----------|-----------|----------|-------------|
| Missing `handlersPath` | Config validation | Fatal error, refuse to start | N/A |
| Invalid `handlersPath` | Config validation | Fatal error with path details | N/A |
| Handler file not found | Config validation | Fatal error with file path | N/A |
| Handler path traversal | Config validation | Fatal error with security warning | N/A |
| Handler timeout | Runtime | MCP error response | 200 (JSON-RPC error) |
| Input validation failure | Runtime | MCP error response with details | 200 (JSON-RPC error) |
| Handler execution error | Runtime | MCP error response with stderr | 200 (JSON-RPC error) |
| Output parse error | Runtime | Return raw output as text | 200 |

### Logging Strategy

#### Startup Logs
```
[startup] Loading safe inputs configuration
[startup] Safe inputs server: custom-tools v1.0.0
[startup] Handlers path: /opt/mcp-gateway/handlers
[startup] Registered tool: greet_user (greet.cjs)
[startup] Registered tool: list_files (list_files.sh)
[startup] Safe inputs server ready with 2 tools
```

#### Runtime Logs
```
[safeinputs:server] Received tools/call: greet_user
[safeinputs:handler] Executing greet.cjs with timeout 30s
[safeinputs:handler] Handler completed in 142ms
[safeinputs:handler] Output: 50 characters
```

#### Error Logs
```
[safeinputs:validation] Input validation failed for greet_user: missing required field 'name'
[safeinputs:handler] Handler timeout after 30s: greet.cjs
[safeinputs:handler] Handler execution error: exit code 1
[safeinputs:handler] stderr: Python script error: invalid syntax
```

### Testing Strategy

#### Unit Tests

1. **Configuration Tests**:
   - Parse valid safe inputs config (JSON, TOML)
   - Reject invalid configs (missing fields, wrong types)
   - Validate path security (traversal attempts)
   - Validate environment variable expansion

2. **Handler Tests**:
   - JavaScript handler execution
   - Shell handler with INPUT_ env vars
   - Python handler with JSON stdin
   - Go handler with JSON stdin
   - Timeout enforcement
   - Error handling

3. **Validation Tests**:
   - JSON Schema validation (valid inputs)
   - JSON Schema validation (missing required)
   - JSON Schema validation (wrong types)

#### Integration Tests

1. **End-to-End Flow**:
   - Load config with safe inputs
   - Start gateway
   - Call safe inputs tool via HTTP
   - Verify tool execution
   - Verify response format

2. **Multi-Handler Tests**:
   - Register multiple tools
   - Call each handler type
   - Verify isolation between handlers

3. **Security Tests**:
   - Attempt path traversal in handler path
   - Verify environment isolation
   - Verify timeout enforcement

### Example Handler Files

#### JavaScript Handler (`greet.cjs`)
```javascript
// Read JSON input from stdin
const chunks = [];
process.stdin.on('data', chunk => chunks.push(chunk));
process.stdin.on('end', () => {
  const input = JSON.parse(Buffer.concat(chunks).toString());
  const name = input.name || 'World';
  const result = { message: `Hello, ${name}!` };
  console.log(JSON.stringify(result));
});
```

#### Shell Handler (`list_files.sh`)
```bash
#!/bin/bash
# Inputs available as INPUT_* environment variables
# Output written to $GITHUB_OUTPUT file

ls -la "$INPUT_PATH" 2>&1 | head -20 > /tmp/files.txt

echo "count=$(ls "$INPUT_PATH" | wc -l)" >> "$GITHUB_OUTPUT"
echo "path=$INPUT_PATH" >> "$GITHUB_OUTPUT"
```

#### Python Handler (`analyze.py`)
```python
#!/usr/bin/env python3
import sys
import json

# Read inputs from stdin
inputs = json.load(sys.stdin)
data_str = inputs.get('data', '')

# Process data
numbers = [float(x.strip()) for x in data_str.split(',') if x.strip()]

# Output result as JSON
result = {
    "count": len(numbers),
    "sum": sum(numbers),
    "average": sum(numbers) / len(numbers) if numbers else 0
}

print(json.dumps(result))
```

#### Go Handler (`calculate.go`)
```go
// Standard imports are automatically included by handler wrapper

// Read inputs from stdin
var inputs map[string]any
json.NewDecoder(os.Stdin).Decode(&inputs)

a := inputs["a"].(float64)
b := inputs["b"].(float64)

result := map[string]any{
    "sum":     a + b,
    "product": a * b,
}

json.NewEncoder(os.Stdout).Encode(result)
```

### Differences from gh-aw Implementation

1. **Configuration Source**: 
   - gh-aw: Inline in workflow YAML frontmatter, converted to JSON
   - Gateway: Direct JSON/TOML configuration file

2. **Handler Storage**:
   - gh-aw: Handlers written to temporary directory at runtime
   - Gateway: Handlers exist as files in configured directory

3. **Script Types**:
   - gh-aw: Supports `script:`, `run:`, `py:`, `go:` with inline code
   - Gateway: Requires handler files (`.cjs`, `.sh`, `.py`, `.go`)

4. **Server Lifecycle**:
   - gh-aw: Server process lifetime tied to workflow execution
   - Gateway: Server persistent, handlers executed on-demand

5. **Authentication**:
   - gh-aw: Uses workflow-level authentication
   - Gateway: Uses gateway API key authentication

6. **File Cleanup**:
   - gh-aw: Deletes config file after loading (security)
   - Gateway: Config file persistent, handlers persistent

### Migration Path for gh-aw Users

For users migrating from gh-aw inline safe inputs to gateway-based safe inputs:

1. **Extract Inline Scripts**: Convert `script:`, `run:`, `py:`, `go:` blocks to separate files
2. **Create Handlers Directory**: Create directory for handler files
3. **Update Configuration**: Convert frontmatter to gateway config format
4. **Maintain Handler Files**: Commit handlers to repository or mount as volume

**Example migration**:

gh-aw workflow:
```yaml
safe-inputs:
  greet-user:
    description: "Greet a user"
    script: |
      return { message: `Hello, ${name}!` };
```

Gateway equivalent:
1. Create `handlers/greet_user.cjs`:
```javascript
const chunks = [];
process.stdin.on('data', chunk => chunks.push(chunk));
process.stdin.on('end', () => {
  const input = JSON.parse(Buffer.concat(chunks).toString());
  const result = { message: `Hello, ${input.name}!` };
  console.log(JSON.stringify(result));
});
```

2. Add to gateway config:
```json
{
  "safeInputs": {
    "handlersPath": "/workspace/handlers",
    "tools": [{
      "name": "greet_user",
      "description": "Greet a user",
      "handler": "greet_user.cjs",
      "inputSchema": {
        "type": "object",
        "properties": {
          "name": { "type": "string" }
        },
        "required": ["name"]
      }
    }]
  }
}
```

### Future Enhancements

1. **Inline Script Support**: Allow inline code in config (like gh-aw)
2. **Handler Hot Reload**: Detect handler file changes and reload
3. **Handler Caching**: Cache compiled/validated handlers
4. **Resource Limits**: Add CPU/memory limits per handler
5. **Handler Marketplace**: Share and discover community handlers
6. **Output Streaming**: Stream large outputs instead of buffering
7. **Async Handlers**: Support long-running async operations
8. **Handler Dependencies**: Auto-install npm/pip packages
9. **Handler Debugging**: Remote debugging support for handlers
10. **Handler Metrics**: Detailed performance metrics per handler

## Summary

This specification proposes integrating Safe Inputs functionality into the MCP Gateway by:

1. Adding a `safeInputs` configuration section (JSON/TOML)
2. Supporting multiple handler types (JavaScript, Shell, Python, Go)
3. Implementing secure handler execution with path validation
4. Providing runtime tool generation as internal MCP server
5. Maintaining compatibility with MCP protocol standards

The implementation leverages existing gateway infrastructure (launcher, server routing, MCP protocol) while adding specialized handler execution and validation logic. The result is a powerful extension that allows custom tools without requiring separate MCP server deployments.
