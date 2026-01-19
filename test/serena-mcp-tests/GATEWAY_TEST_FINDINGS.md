# Serena Gateway Test Results - Behavioral Differences

## Summary

This document describes the behavioral differences discovered when testing Serena MCP Server through the MCP Gateway versus direct stdio connection.

## Test Setup

- **Direct Connection Tests** (`test_serena.sh`): Connect directly to Serena container via stdio
- **Gateway Tests** (`test_serena_via_gateway.sh`): Connect to Serena through MCP Gateway via HTTP

## Key Findings

### 1. Session Initialization Differences

**Direct Stdio Connection:**
- Sends multiple JSON-RPC messages in a single stdin stream
- Example:
  ```json
  {"jsonrpc":"2.0","id":1,"method":"initialize",...}
  {"jsonrpc":"2.0","method":"notifications/initialized"}
  {"jsonrpc":"2.0","id":2,"method":"tools/list",...}
  ```
- All messages are processed in sequence on the same connection
- Serena maintains session state throughout the connection

**HTTP Gateway Connection:**
- Each HTTP request is independent and stateless
- Initialize, notification, and tool calls are sent as separate HTTP POST requests
- The gateway creates a new filtered connection for each request
- Serena treats each HTTP request as a new session attempt

### 2. Error Manifestation

When sending `tools/list` or `tools/call` via separate HTTP requests after initialization:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "error": {
    "code": 0,
    "message": "method \"tools/list\" is invalid during session initialization"
  }
}
```

This error comes from Serena itself, not the gateway. Serena expects the initialization handshake to be completed in the same connection/stream before accepting tool calls.

### 3. Test Results

**Passing Tests (7/23):**
1. Docker availability
2. Curl availability
3. Gateway container image availability
4. Serena container image availability
5. Gateway startup with Serena backend
6. MCP initialize (succeeds on each request)
7. Invalid tool error handling

**Failing Tests (15/23):**
- All `tools/list` and `tools/call` requests fail with "invalid during session initialization"
- This includes: Go/Java/JS/Python symbol analysis, file operations, memory operations

## Root Cause

The issue stems from a fundamental difference in connection models:

1. **Stdio MCP Servers** (like Serena) are designed for persistent, streaming connections where:
   - The client sends an initialize request
   - The server responds
   - The client sends an initialized notification
   - From that point forward, the same connection can make tool calls
   - Session state is maintained throughout

2. **HTTP-Based MCP Connections** are stateless:
   - Each HTTP request is independent
   - The gateway tries to maintain session state using the Authorization header
   - However, Serena itself doesn't support this stateless model
   - Serena requires initialization to be part of the same connection stream

## Implications

This behavioral difference means:

1. **Stdio-based MCP servers** (like Serena) work perfectly with direct stdio connections
2. **HTTP proxying** of stdio-based servers through the gateway has limitations when the backend server expects streaming/stateful connections
3. **HTTP-native MCP servers** would work fine through the gateway since they're designed for stateless HTTP

## Recommendations

For users wanting to use Serena through the MCP Gateway:

1. **Current Limitation**: Full Serena functionality is not available through HTTP-based gateway connections
2. **Workaround**: Use direct stdio connections to Serena when full functionality is needed
3. **Future Enhancement**: The gateway could be enhanced to maintain persistent stdio connections to backends and map multiple HTTP requests to the same backend session

## Test Suite Value

Despite the failures, this test suite provides significant value:

1. ✅ **Validates gateway startup** with Serena backend
2. ✅ **Demonstrates MCP initialize** works through the gateway  
3. ✅ **Identifies behavioral differences** between stdio and HTTP transport
4. ✅ **Documents limitations** for future improvements
5. ✅ **Provides regression testing** for when/if the gateway adds session persistence

## Conclusion

The test suite successfully identifies that stdio-based MCP servers like Serena require connection-level session state that is not currently supported when proxying through the HTTP-based gateway. This is expected behavior given the current gateway architecture and is valuable information for users and developers.
