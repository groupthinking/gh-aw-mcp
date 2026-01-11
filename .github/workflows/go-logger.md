---
name: Go Logger Enhancement
description: Analyzes and enhances Go logging practices across the codebase for improved debugging and observability
on:
  schedule: daily
  workflow_dispatch:

permissions:
  contents: read
  issues: read
  pull-requests: read

engine: copilot

safe-outputs:
  create-pull-request:
    title-prefix: "[log] "
    labels: [enhancement, automation]
    draft: true

steps:
  - name: Set up Go
    uses: actions/setup-go@v6
    with:
      go-version-file: go.mod
      cache: true

tools:
  github:
    toolsets: [default]
  edit:
  bash:
    - "find internal -name '*.go' -type f ! -name '*_test.go'"
    - "grep -r 'var log = logger.New' internal --include='*.go'"
    - "grep -n 'func ' internal/*.go"
    - "head -n * internal/**/*.go"
    - "wc -l internal/**/*.go"
    - "go build -o awmg"
    - "go test ./..."
    - "go vet ./..."
  cache-memory:

timeout-minutes: 15
---

# Go Logger Enhancement

You are an AI agent that improves Go code by adding debug logging statements to help with troubleshooting and development.

## Efficiency First: Check Cache

Before analyzing files:

1. Check `/tmp/gh-aw/cache-memory/go-logger/` for previous logging sessions
2. Read `processed-files.json` to see which files were already enhanced
3. Read `last-run.json` for the last commit SHA processed
4. If current commit SHA matches and no new .go files exist, exit early with success
5. Update cache after processing:
   - Save list of processed files to `processed-files.json`
   - Save current commit SHA to `last-run.json`
   - Save summary of changes made

This prevents re-analyzing already-processed files and reduces token usage significantly.

## Mission

Add meaningful debug logging calls to Go files in the `internal/` directory following the project's logging guidelines from AGENTS.md.

## Important Constraints

1. **Process exactly 1 file per pull request** - Focus deeply on a single file for thorough, reviewable changes
2. **Skip test files** - Never modify files ending in `_test.go`
3. **No side effects** - Logger arguments must NOT compute anything or cause side effects
4. **Follow logger naming convention** - Use `pkg:filename` pattern (e.g., `server:routed`)
5. **Reuse existing loggers** - If a file already has a logger declaration, preserve it and add new logging calls

## Logger Guidelines from AGENTS.md

This codebase uses **two complementary logging systems**. Choose the right one based on your use case:

### Two Logging Systems

**1. Debug Logger (`logger.New()`)** - For development and troubleshooting
- Controlled by the `DEBUG` environment variable
- Zero overhead when disabled (logs not computed)
- Outputs to stderr with colors and timing
- Use for: detailed operation flow, performance insights, internal state tracking

**2. File Logger (`logger.LogInfo/Warn/Error/Debug()`)** - For operational events
- Always written to `mcp-gateway.log` file
- Structured format with timestamps and categories
- Use for: startup/shutdown events, client interactions, backend operations, authentication events, errors

### Debug Logger Declaration

If a file doesn't have a debug logger, add this at the top of the file (after imports):

```go
import "github.com/githubnext/gh-aw-mcpg/internal/logger"

var log = logger.New("pkg:filename")
```

Replace `pkg:filename` with the actual package and filename:
- For `internal/server/routed.go` → `"server:routed"`
- For `internal/cmd/root.go` → `"cmd:root"`
- For `internal/launcher/launcher.go` → `"launcher:launcher"`
- For `internal/mcp/connection.go` → `"mcp:connection"`

**Category Naming Convention:** Use pattern `pkg:filename` (e.g., `server:routed`, `launcher:launcher`)

### Debug Logger Usage Patterns

**Good debug logging examples:**

```go
// Log function entry with parameters (no side effects)
func ProcessRequest(path string, count int) error {
    log.Printf("Processing request: path=%s, count=%d", path, count)
    // ... function body ...
}

// Log important state changes
log.Printf("Started %d MCP servers successfully", len(servers))

// Log before expensive operations (check if enabled first)
if log.Enabled() {
    log.Printf("Starting server with config: %+v", config)
}

// Log control flow decisions
log.Print("Cache hit, skipping initialization")
log.Printf("No matching server found, using default: %s", defaultServer)
```

### File Logger Usage Patterns

**Good operational logging examples:**

```go
// Log startup/initialization events
logger.LogInfo("startup", "MCP Gateway initialized with %d servers", serverCount)

// Log client interactions
logger.LogInfo("client", "Received request from session=%s, method=%s", sessionID, method)

// Log backend operations
logger.LogInfo("backend", "Creating new MCP backend connection, command=%s, args=%v", command, args)
logger.LogInfo("backend", "Starting MCP backend server, command=%s, args=%v", command, expandedArgs)

// Log authentication events
logger.LogWarn("auth", "Authentication failed for session=%s", sessionID)

// Log errors
logger.LogError("backend", "Failed to start MCP server: %v", err)

// Log debug details (operational debug, not development debug)
logger.LogDebug("client", "Processing request details: %+v", requestDetails)
```

**File Logger Categories:**
- `startup` - Gateway initialization and configuration
- `shutdown` - Graceful shutdown events
- `client` - MCP client interactions and requests
- `backend` - Backend MCP server operations
- `auth` - Authentication events (success and failures)

### What NOT to do

**Avoid these anti-patterns:**

```go
// WRONG - causes side effects
log.Printf("Servers: %s", expensiveOperation())  // Don't call functions in log args

// WRONG - not meaningful
log.Print("Here")  // Too vague

// WRONG - duplicates user-facing messages
fmt.Fprintln(os.Stderr, "Starting server...")
log.Print("Starting server...")  // Redundant with user message above

// WRONG - using debug logger for operational events
log.Printf("Server started successfully")  // Should use logger.LogInfo("startup", ...)
```

### When to Add Logging

**Add debug logging (`log.Printf`) for:**
1. **Function entry** - Especially for public functions with parameters
2. **Important control flow** - Branches, loops, conditional logic
3. **State changes** - Before/after modifying important state
4. **Performance-sensitive sections** - Before/after expensive operations
5. **Debugging context** - Information that would help troubleshoot issues

**Add file logging (`logger.LogInfo/Warn/Error`) for:**
1. **Startup/shutdown** - Initialization and teardown events
2. **Client requests** - API interactions, session management
3. **Backend operations** - MCP server lifecycle, connections
4. **Authentication** - Auth success/failure events
5. **Errors** - Operational failures and warnings

**Do NOT add logging for:**
1. **Simple getters/setters** - Too verbose
2. **Already logged operations** - Don't duplicate existing logs
3. **User-facing messages** - Debug logs are separate from console output
4. **Test files** - Skip all `*_test.go` files

## Task Steps

### 1. Find Candidate Go Files

Use bash to identify Go files that could benefit from additional logging:

```bash
# Find all non-test Go files in internal/
find internal -name '*.go' -type f ! -name '*_test.go'

# Check which files already have loggers
grep -r 'var log = logger.New' internal --include='*.go'
```

### 2. Select File for Enhancement

From the list of Go files:
1. Prioritize files without debug loggers or with minimal logging
2. Focus on files with complex logic (server, launcher, config, mcp)
3. Avoid trivial files with just simple functions
4. **Select exactly 1 file** for this PR - focus deeply on a single file for quality
5. **Check if the file already has a debug logger** - if it does, reuse it rather than creating a new one
6. Look for opportunities to add both debug logging (for development) and file logging (for operations)

### 3. Analyze the Selected File

For the selected file:
1. Read the file content to understand its structure and purpose
2. **Check if the file already has a debug logger** (search for `var log = logger.New`)
3. If a debug logger exists, note its namespace and reuse it
4. **Identify which logging system to use:**
   - Debug logger (`log.Printf`) for function flow, state changes, control flow decisions
   - File logger (`logger.LogInfo/Warn/Error`) for startup events, client interactions, backend operations, auth events
5. Identify functions that would benefit from logging
6. Plan where to add logging calls (both debug and operational where appropriate)

### 4. Add Logger and Logging Calls

For the selected file:

1. **Add debug logger declaration if missing:**
   - Add import: `"github.com/githubnext/gh-aw-mcpg/internal/logger"`
   - Add logger variable using correct naming: `var log = logger.New("pkg:filename")`
   
2. **Reuse existing logger if present:**
   - If the file already has a debug logger declaration (e.g., `var log = logger.New("server:routed")`), keep it as-is
   - Do NOT create a duplicate logger
   - Use the existing logger variable name for all new debug logging calls

3. **Add meaningful debug logging calls (`log.Printf`):**
   - Add logging at function entry for important functions
   - Add logging before/after state changes
   - Add logging for control flow decisions
   - Ensure log arguments don't have side effects
   - Use `log.Enabled()` check for expensive debug info

4. **Add operational file logging calls where appropriate (`logger.LogInfo/Warn/Error`):**
   - Add for startup/initialization events (category: `"startup"`)
   - Add for client interactions (category: `"client"`)
   - Add for backend operations (category: `"backend"`)
   - Add for authentication events (category: `"auth"`)
   - Add for errors and warnings (use `logger.LogError` and `logger.LogWarn`)

5. **Keep it focused:**
   - 3-7 logging calls for this single file is appropriate (mix of debug and operational)
   - Don't over-log - focus on the most useful information
   - Ensure messages are meaningful and helpful for debugging or operations
   - Choose the right logging system for each situation

### 5. Validate Changes

After adding logging to the selected file, **validate your changes** before creating a PR:

1. **Build the project to ensure no compilation errors:**
   ```bash
   go build -o awmg
   ```
   This will compile the Go code and catch any syntax errors or import issues.

2. **Run tests to ensure no regressions:**
   ```bash
   go test ./...
   ```
   This validates that your changes don't break existing functionality.

3. **Run vet to check for common mistakes:**
   ```bash
   go vet ./...
   ```
   This catches potential issues in your code.

4. **Test debug logging with the binary:**
   ```bash
   # Test with all debug logs enabled
   DEBUG=* ./awmg --config config.toml
   
   # Test with specific namespace enabled (if you added debug logs)
   DEBUG=server:* ./awmg --config config.toml
   ```
   This validates that:
   - The binary was built successfully
   - Debug logging from your changes appears when enabled
   - Debug logging is silent when disabled (zero overhead)
   - The application runs correctly

5. **Verify file logging (if you added operational logs):**
   ```bash
   # Check the log file after running the server
   tail -f /tmp/gh-aw/mcp-logs/mcp-gateway.log
   ```
   This validates that operational logs are written to the file correctly.

### 6. Create Pull Request

After validating your changes:

1. The safe-outputs create-pull-request will automatically create a draft PR
2. Ensure your changes follow the guidelines above
3. The PR title will automatically have the "[log] " prefix
4. Since only one file is modified, the PR will be focused and easy to review

## Example Transformation

**Before:**
```go
package server

import (
    "fmt"
    "net/http"
)

func StartServer(port int) error {
    addr := fmt.Sprintf(":%d", port)
    return http.ListenAndServe(addr, nil)
}
```

**After (with debug logging):**
```go
package server

import (
    "fmt"
    "net/http"
    
    "github.com/githubnext/gh-aw-mcpg/internal/logger"
)

var log = logger.New("server:server")

func StartServer(port int) error {
    log.Printf("Starting server on port %d", port)
    
    addr := fmt.Sprintf(":%d", port)
    
    log.Printf("Listening on address: %s", addr)
    return http.ListenAndServe(addr, nil)
}
```

**After (with operational file logging):**
```go
package server

import (
    "fmt"
    "net/http"
    
    "github.com/githubnext/gh-aw-mcpg/internal/logger"
)

var log = logger.New("server:server")

func StartServer(port int) error {
    logger.LogInfo("startup", "Starting HTTP server on port %d", port)
    log.Printf("Starting server on port %d", port)
    
    addr := fmt.Sprintf(":%d", port)
    
    log.Printf("Listening on address: %s", addr)
    err := http.ListenAndServe(addr, nil)
    
    if err != nil {
        logger.LogError("startup", "Server failed to start: %v", err)
    }
    
    return err
}
```

**Choosing the right logging:**
- Use **debug logging** (`log.Printf`) for detailed operation flow during development
- Use **file logging** (`logger.LogInfo/Error`) for important operational events
- Often you'll want both: file logging for the key event, debug logging for details

## Quality Checklist

Before creating the PR, verify:

- [ ] Exactly 1 file modified (focused, single-file PR)
- [ ] No test files modified (`*_test.go`)
- [ ] File has debug logger declaration if debug logging was added (added if missing, or reused if present)
- [ ] Debug logger naming follows the `pkg:filename` convention
- [ ] Used appropriate logging system for each situation:
  - [ ] Debug logging (`log.Printf`) for detailed operation flow
  - [ ] File logging (`logger.LogInfo/Warn/Error`) for operational events with proper categories
- [ ] Logger arguments don't compute anything or cause side effects
- [ ] Logging messages are meaningful and helpful
- [ ] No duplicate logging with existing logs
- [ ] Import statements are properly formatted
- [ ] Changes validated with `go build -o awmg` (no compilation errors)
- [ ] Tests pass with `go test ./...`
- [ ] Code checked with `go vet ./...`
- [ ] Debug logging tested with `DEBUG=* ./awmg --config config.toml` (if debug logs added)
- [ ] File logging verified in `/tmp/gh-aw/mcp-logs/mcp-gateway.log` (if operational logs added)

## Important Notes

- You have access to the edit tool to modify files
- You have access to bash commands to explore the codebase
- The safe-outputs create-pull-request will automatically create a draft PR
- Focus on quality over quantity - 1 well-logged file with 3-7 meaningful logging calls is the goal
- **Two logging systems available:**
  - **Debug logger** (`logger.New()` + `log.Printf()`) - For development insights (controlled by DEBUG env var)
  - **File logger** (`logger.LogInfo/Warn/Error/Debug()`) - For operational events (written to mcp-gateway.log)
- Choose the appropriate logging system based on the type of information being logged
- Remember: debug logs are for developers, operational logs are for production monitoring
- **Always check for existing logger declarations** before adding a new one
- Reuse existing logger infrastructure when present

Good luck enhancing the codebase with better logging!
