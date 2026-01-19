# Expected Responses Documentation

This directory contains documentation about expected responses from the Serena MCP server for various test cases.

## MCP Initialize Response

Expected structure:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "serverInfo": {
      "name": "serena-mcp-server",
      "version": "..."
    },
    "capabilities": {
      "tools": {}
    }
  }
}
```

## Tools List Response

Expected structure:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "serena-go",
        "description": "...",
        "inputSchema": {...}
      },
      {
        "name": "serena-java",
        "description": "...",
        "inputSchema": {...}
      },
      {
        "name": "serena-javascript",
        "description": "...",
        "inputSchema": {...}
      },
      {
        "name": "serena-python",
        "description": "...",
        "inputSchema": {...}
      }
    ]
  }
}
```

## Symbol Analysis Response

Expected to contain references to:
- **Go**: `Calculator`, `NewCalculator`, `Add`, `Multiply`, `Greet`
- **Java**: `Calculator`, `add`, `multiply`, `greet`
- **JavaScript**: `Calculator`, `add`, `multiply`, `greet`
- **Python**: `Calculator`, `add`, `multiply`, `greet`

Example structure:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Symbol information including Calculator, Add, Multiply..."
      }
    ]
  }
}
```

## Diagnostics Response

Expected structure:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Diagnostics results (may be empty for valid code)"
      }
    ]
  }
}
```

## Error Response

For invalid requests:
```json
{
  "jsonrpc": "2.0",
  "id": 99,
  "error": {
    "code": -32601,
    "message": "Method not found"
  }
}
```

## Notes

- Actual response formats may vary depending on Serena MCP server version
- The test script checks for presence of expected symbols/keywords rather than exact matches
- Response content fields may vary in structure
- Tool names should be consistent with what Serena exposes
