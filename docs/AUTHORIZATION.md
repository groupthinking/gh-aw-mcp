# Authorization Architecture

This document describes how MCP Gateway handles authorization per the MCP specification (2025-03-26).

## MCP Specification Compliance

The gateway implements OAuth 2.1-compliant authorization for HTTP-based transports:

### HTTP Status Codes

Per MCP spec, the following HTTP status codes are used:

| Status Code | Description | Usage |
|-------------|-------------|-------|
| 400 Bad Request | Malformed authorization request | - Missing "Bearer " prefix<br>- Empty token<br>- Token in query string |
| 401 Unauthorized | Authorization required or token invalid | - Missing Authorization header<br>- Invalid/expired token |
| 403 Forbidden | Valid token but insufficient permissions | Not yet implemented (requires OAuth scopes) |

### Authorization Header Format

**Required Format:** `Authorization: Bearer <token>`

- **MUST** use "Bearer " prefix (case-sensitive)
- **MUST NOT** include tokens in URI query strings
- Plain API keys without "Bearer " prefix are **rejected** (returns 400)

## Two-Layer Authorization Architecture

The gateway uses a two-layer authorization approach:

### Layer 1: HTTP Authentication (authMiddleware)

**Purpose:** Validate that the request includes a valid API key

**Location:** `internal/server/auth.go` - `authMiddleware()`

**Behavior:**
- Applied to MCP endpoints when `--api-key` flag is set
- Validates `Authorization: Bearer <token>` header
- Compares token against configured API key
- Returns 401 if token is missing or invalid
- Returns 400 if header is malformed

**When Applied:**
- `/mcp` endpoint (unified mode)
- `/mcp/{serverID}` endpoints (routed mode)
- `/close` endpoint

**Not Applied:**
- `/health` endpoint (always public)
- `/.well-known/oauth-authorization-server` endpoint

### Layer 2: Session Identification (Transport Layer)

**Purpose:** Extract Bearer token to use as session ID for request routing

**Location:** 
- `internal/server/transport.go` - `CreateHTTPServerForMCP()`
- `internal/server/routed.go` - `CreateHTTPServerForRoutedMode()`

**Behavior:**
- Extracts Bearer token from `Authorization` header
- Uses token as session ID to group requests from same client
- Rejects connections without Bearer token (returns nil server)
- **Does NOT validate token value** (that's Layer 1's job)

**Session Management:**
- Same Bearer token = same session
- Session persists across multiple MCP requests
- DIFC labels accumulate within a session

## Authorization Flow

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Client Request                                           │
│    POST /mcp/github                                         │
│    Authorization: Bearer secret123                          │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. HTTP Auth Layer (if API key configured)                 │
│    - Check Bearer format (400 if wrong)                     │
│    - Validate token == apiKey (401 if wrong)                │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Transport Layer                                          │
│    - Extract Bearer token → session ID                      │
│    - Reject if no token (return nil)                        │
│    - Store session ID in context                            │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. MCP Protocol Handling                                    │
│    - Route to backend server                                │
│    - Execute tool calls                                     │
│    - Apply DIFC policies (if enabled)                       │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Without API Key (Development Mode)

```bash
./awmg --config config.toml
```

- **HTTP Auth Layer:** Disabled (all requests accepted)
- **Transport Layer:** Still requires Bearer token for session ID
- **Behavior:** Clients must send `Authorization: Bearer <any-value>` for session management

### With API Key (Production Mode)

```bash
./awmg --config config.toml --api-key secret123
```

Or via environment:
```bash
MCP_GATEWAY_API_KEY=secret123 ./awmg --config config.toml
```

- **HTTP Auth Layer:** Enabled (validates token == apiKey)
- **Transport Layer:** Uses validated Bearer token as session ID
- **Behavior:** Clients must send `Authorization: Bearer secret123` (exact match required)

## Bearer Token Usage Patterns

### Pattern 1: Development (No API Key)

```
Client A: Authorization: Bearer dev-session-1
Client B: Authorization: Bearer dev-session-2
```

- Both accepted (no validation)
- Separate sessions maintained

### Pattern 2: Production (With API Key)

```
Client A: Authorization: Bearer secret123  ✅ Accepted
Client B: Authorization: Bearer wrong-key  ❌ 401 Unauthorized
```

- Only matching API key accepted
- All clients share same session (same token)

### Pattern 3: Multi-Agent (Future)

When OAuth scopes are added:

```
Agent A: Authorization: Bearer token-with-read-scope   ✅ Read operations only
Agent B: Authorization: Bearer token-with-write-scope  ✅ Read + Write operations
Agent C: Authorization: Bearer token-no-scopes         ❌ 403 Forbidden
```

## Why Two Layers?

The separation provides flexibility:

1. **Development**: No API key needed, but still requires Bearer token for session management
2. **Production**: API key validation ensures only authorized clients can connect
3. **Future**: OAuth scopes can be added to auth layer without changing transport layer

## Testing

See `internal/server/auth_test.go` for comprehensive test coverage:

- ✅ Bearer token format validation
- ✅ Query string rejection
- ✅ Empty token rejection
- ✅ Invalid token rejection
- ✅ Case sensitivity
- ✅ Whitespace handling

## Limitations

### Current Limitations

1. **Simple API Key Only:** No OAuth scopes or fine-grained permissions
2. **Single API Key:** All clients share same token in production
3. **No Token Expiration:** Tokens don't expire (restart required to change)
4. **No HTTP 403:** All auth failures return 401 or 400 (no permission checks)

### Future Enhancements

1. **OAuth Scopes:** Add scope-based permissions (read, write, admin)
2. **Multiple Tokens:** Support different tokens for different clients
3. **Token Expiration:** JWT with expiry claims
4. **HTTP 403 Support:** Return 403 when valid token lacks required scopes
5. **Dynamic Client Registration:** Support RFC7591 for automatic client registration

## Security Considerations

### ✅ Compliant

- Bearer token required for all MCP operations
- Tokens not allowed in query strings (prevents log/history leaks)
- Strict format validation (prevents format confusion attacks)
- Case-sensitive Bearer prefix (prevents case-folding attacks)

### ⚠️ Recommendations

- **Always use HTTPS in production** (Bearer tokens in plaintext)
- **Use strong random tokens** (at least 32 bytes of entropy)
- **Rotate tokens regularly** (restart gateway with new token)
- **Restrict token exposure** (don't log full token values)

## References

- [MCP Specification 2025-03-26](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2025-03-26/basic/authorization.mdx)
- [OAuth 2.1 IETF Draft](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1-12)
- [RFC8414 - OAuth 2.0 Authorization Server Metadata](https://datatracker.ietf.org/doc/html/rfc8414)
- [RFC7591 - OAuth 2.0 Dynamic Client Registration](https://datatracker.ietf.org/doc/html/rfc7591)
