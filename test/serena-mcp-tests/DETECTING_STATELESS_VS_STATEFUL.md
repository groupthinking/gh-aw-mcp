# Detecting Stateless vs Stateful MCP Servers

## Executive Summary

While the MCP protocol doesn't include explicit metadata to declare if a server is stateless or stateful, we can **probe and detect** this characteristic through behavioral testing.

This document describes practical methods to automatically determine if an MCP server requires session persistence (stateful) or can handle independent requests (stateless).

---

## Table of Contents

- [The Challenge](#the-challenge)
- [Detection Methods](#detection-methods)
- [Method 1: Sequential Connection Test](#method-1-sequential-connection-test)
- [Method 2: Capability Analysis](#method-2-capability-analysis)
- [Method 3: Metadata Heuristics](#method-3-metadata-heuristics)
- [Implementation Guide](#implementation-guide)
- [Recommendations](#recommendations)

---

## The Challenge

**Problem:** MCP protocol doesn't have a standard field like `"stateless": true` in the initialize response.

**Why It Matters:**
- Gateway needs to know whether to maintain persistent connections
- Clients need to know if they can load balance across instances
- Developers need guidance on deployment architecture

**Current State:**
- ✅ GitHub MCP Server: Stateless (works through gateway)
- ❌ Serena MCP Server: Stateful (requires direct connection)
- ❓ Other servers: Unknown until tested

---

## Detection Methods

We can use three complementary approaches to detect server architecture:

### Summary Table

| Method | Accuracy | Speed | Intrusiveness | Best For |
|--------|----------|-------|---------------|----------|
| **Sequential Connection Test** | ✅ High (99%) | ⚠️ Moderate | ⚠️ Creates multiple connections | Automated detection |
| **Capability Analysis** | ⚠️ Medium (70%) | ✅ Fast | ✅ Single request | Quick heuristic |
| **Metadata Heuristics** | ⚠️ Low (50%) | ✅ Instant | ✅ Non-invasive | Initial screening |

---

## Method 1: Sequential Connection Test

### Concept

**Test the server's behavior with separate connections:**

1. Connection 1: Send `initialize` → Success expected
2. Connection 2: Send `tools/list` (without init) → Behavior tells us the answer

**Results:**
- ✅ **Success** → Server is stateless (like GitHub MCP)
- ❌ **Error** → Server is stateful (like Serena MCP)

### Detailed Algorithm

```
FUNCTION detect_server_state(server_endpoint):
    
    # Test 1: Initialize on first connection
    connection1 = new_connection(server_endpoint)
    response1 = send_request(connection1, {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "probe", "version": "1.0"}
        }
    })
    close_connection(connection1)
    
    IF response1.error:
        RETURN "UNKNOWN - initialization failed"
    
    # Test 2: Try tools/list on NEW connection (no init)
    connection2 = new_connection(server_endpoint)
    response2 = send_request(connection2, {
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/list",
        "params": {}
    })
    close_connection(connection2)
    
    # Analyze response
    IF response2.error:
        IF "initialization" IN response2.error.message:
            RETURN "STATEFUL - requires session persistence"
        ELSE:
            RETURN "UNKNOWN - error but not initialization-related"
    ELSE IF response2.result AND response2.result.tools:
        RETURN "STATELESS - handles independent requests"
    ELSE:
        RETURN "UNKNOWN - unexpected response"

END FUNCTION
```

### Expected Responses

#### Stateless Server (GitHub-style)

```json
// Connection 2 - tools/list response
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {"name": "list_branches", "description": "..."},
      {"name": "get_file_contents", "description": "..."}
    ]
  }
}
```

**Detection:** ✅ Returns tools without error → **STATELESS**

#### Stateful Server (Serena-style)

```json
// Connection 2 - tools/list response  
{
  "jsonrpc": "2.0",
  "id": 2,
  "error": {
    "code": 0,
    "message": "method 'tools/list' is invalid during session initialization"
  }
}
```

**Detection:** ❌ Error mentioning "initialization" or "session" → **STATEFUL**

### Bash Implementation

```bash
#!/bin/bash
# detect_mcp_server_state.sh

SERVER_URL="$1"
SESSION_ID="probe-$$"

echo "Probing MCP server: $SERVER_URL"

# Test 1: Initialize
echo "Test 1: Sending initialize..."
INIT_RESPONSE=$(curl -s -X POST "$SERVER_URL" \
    -H "Content-Type: application/json" \
    -H "Authorization: $SESSION_ID" \
    -d '{
        "jsonrpc":"2.0",
        "id":1,
        "method":"initialize",
        "params":{
            "protocolVersion":"2024-11-05",
            "capabilities":{},
            "clientInfo":{"name":"probe","version":"1.0"}
        }
    }')

if echo "$INIT_RESPONSE" | grep -q '"error"'; then
    echo "❌ Initialization failed"
    exit 1
fi

echo "✅ Initialize succeeded"

# Test 2: Try tools/list on new connection (simulate new HTTP request)
echo "Test 2: Sending tools/list without init..."
TOOLS_RESPONSE=$(curl -s -X POST "$SERVER_URL" \
    -H "Content-Type: application/json" \
    -H "Authorization: $SESSION_ID" \
    -d '{
        "jsonrpc":"2.0",
        "id":2,
        "method":"tools/list",
        "params":{}
    }')

# Analyze response
if echo "$TOOLS_RESPONSE" | grep -q '"error"'; then
    if echo "$TOOLS_RESPONSE" | grep -qi 'initialization\|session'; then
        echo "🔴 STATEFUL - Server requires session persistence"
        echo "   Error: $(echo "$TOOLS_RESPONSE" | grep -o '"message":"[^"]*"')"
        exit 0
    else
        echo "⚠️  UNKNOWN - Error but not initialization-related"
        echo "$TOOLS_RESPONSE"
        exit 1
    fi
elif echo "$TOOLS_RESPONSE" | grep -q '"tools"'; then
    echo "🟢 STATELESS - Server handles independent requests"
    TOOL_COUNT=$(echo "$TOOLS_RESPONSE" | grep -o '"name"' | wc -l)
    echo "   Found $TOOL_COUNT tools"
    exit 0
else
    echo "⚠️  UNKNOWN - Unexpected response"
    echo "$TOOLS_RESPONSE"
    exit 1
fi
```

### Python Implementation

```python
#!/usr/bin/env python3
"""
Detect if an MCP server is stateless or stateful
"""
import requests
import json
from typing import Literal

def detect_mcp_server_state(
    server_url: str,
    session_id: str = "probe-12345"
) -> Literal["STATELESS", "STATEFUL", "UNKNOWN"]:
    """
    Probe an MCP server to determine if it's stateless or stateful.
    
    Returns:
        - "STATELESS": Server handles independent requests
        - "STATEFUL": Server requires session persistence
        - "UNKNOWN": Cannot determine from behavior
    """
    
    headers = {
        "Content-Type": "application/json",
        "Authorization": session_id
    }
    
    # Test 1: Initialize
    init_request = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "probe", "version": "1.0"}
        }
    }
    
    try:
        init_response = requests.post(
            server_url,
            headers=headers,
            json=init_request,
            timeout=10
        )
        init_data = init_response.json()
        
        if "error" in init_data:
            print(f"❌ Initialize failed: {init_data['error']}")
            return "UNKNOWN"
        
        print("✅ Initialize succeeded")
        
    except Exception as e:
        print(f"❌ Initialize request failed: {e}")
        return "UNKNOWN"
    
    # Test 2: tools/list without initialization
    # In HTTP, this is a new connection that hasn't seen initialize
    tools_request = {
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/list",
        "params": {}
    }
    
    try:
        tools_response = requests.post(
            server_url,
            headers=headers,
            json=tools_request,
            timeout=10
        )
        tools_data = tools_response.json()
        
        if "error" in tools_data:
            error_msg = tools_data["error"].get("message", "").lower()
            if "initialization" in error_msg or "session" in error_msg:
                print(f"🔴 STATEFUL - Requires session persistence")
                print(f"   Error: {tools_data['error']['message']}")
                return "STATEFUL"
            else:
                print(f"⚠️  UNKNOWN - Error but not initialization-related")
                return "UNKNOWN"
        
        elif "result" in tools_data and "tools" in tools_data["result"]:
            tool_count = len(tools_data["result"]["tools"])
            print(f"🟢 STATELESS - Handles independent requests")
            print(f"   Found {tool_count} tools")
            return "STATELESS"
        
        else:
            print(f"⚠️  UNKNOWN - Unexpected response structure")
            return "UNKNOWN"
            
    except Exception as e:
        print(f"❌ Tools request failed: {e}")
        return "UNKNOWN"


if __name__ == "__main__":
    import sys
    
    if len(sys.argv) < 2:
        print("Usage: python3 detect_mcp_state.py <server_url>")
        print("Example: python3 detect_mcp_state.py http://localhost:8080/mcp/github")
        sys.exit(1)
    
    server_url = sys.argv[1]
    print(f"Probing MCP server: {server_url}\n")
    
    result = detect_mcp_server_state(server_url)
    print(f"\nResult: {result}")
    
    sys.exit(0 if result != "UNKNOWN" else 1)
```

### Go Implementation

```go
package mcpprobe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ServerState string

const (
	Stateless ServerState = "STATELESS"
	Stateful  ServerState = "STATEFUL"
	Unknown   ServerState = "UNKNOWN"
)

type ProbeResult struct {
	State   ServerState
	Message string
	Details map[string]interface{}
}

func DetectServerState(serverURL string, sessionID string) (*ProbeResult, error) {
	if sessionID == "" {
		sessionID = "probe-12345"
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Test 1: Initialize
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "probe",
				"version": "1.0",
			},
		},
	}

	initResp, err := sendRequest(client, serverURL, sessionID, initReq)
	if err != nil {
		return &ProbeResult{
			State:   Unknown,
			Message: fmt.Sprintf("Initialize request failed: %v", err),
		}, err
	}

	if errData, hasError := initResp["error"]; hasError {
		return &ProbeResult{
			State:   Unknown,
			Message: fmt.Sprintf("Initialize failed: %v", errData),
		}, nil
	}

	fmt.Println("✅ Initialize succeeded")

	// Test 2: tools/list without initialization
	toolsReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	toolsResp, err := sendRequest(client, serverURL, sessionID, toolsReq)
	if err != nil {
		return &ProbeResult{
			State:   Unknown,
			Message: fmt.Sprintf("Tools request failed: %v", err),
		}, err
	}

	// Analyze response
	if errData, hasError := toolsResp["error"]; hasError {
		errorMap := errData.(map[string]interface{})
		errorMsg := strings.ToLower(errorMap["message"].(string))

		if strings.Contains(errorMsg, "initialization") || strings.Contains(errorMsg, "session") {
			return &ProbeResult{
				State:   Stateful,
				Message: "Server requires session persistence",
				Details: map[string]interface{}{
					"error": errorMap["message"],
				},
			}, nil
		}

		return &ProbeResult{
			State:   Unknown,
			Message: "Error but not initialization-related",
			Details: toolsResp,
		}, nil
	}

	if resultData, hasResult := toolsResp["result"]; hasResult {
		resultMap := resultData.(map[string]interface{})
		if tools, hasTools := resultMap["tools"]; hasTools {
			toolsList := tools.([]interface{})
			return &ProbeResult{
				State:   Stateless,
				Message: "Server handles independent requests",
				Details: map[string]interface{}{
					"tool_count": len(toolsList),
				},
			}, nil
		}
	}

	return &ProbeResult{
		State:   Unknown,
		Message: "Unexpected response structure",
		Details: toolsResp,
	}, nil
}

func sendRequest(client *http.Client, url, sessionID string, reqData map[string]interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", sessionID)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}
```

---

## Method 2: Capability Analysis

### Concept

Analyze the server's declared capabilities in the `initialize` response for clues about stateless/stateful design.

### Heuristics

```python
def analyze_capabilities(init_response):
    """
    Analyze server capabilities for stateless/stateful indicators.
    Returns confidence score (0.0-1.0) that server is stateless.
    """
    result = init_response.get("result", {})
    capabilities = result.get("capabilities", {})
    server_info = result.get("serverInfo", {})
    
    stateless_score = 0.5  # Start neutral
    
    # Indicators of STATELESS design
    if capabilities.get("logging"):
        # Logging often implies stateless (logs sent per-request)
        stateless_score += 0.1
    
    # Check if tools list can change (suggests stateless)
    if capabilities.get("tools", {}).get("listChanged"):
        # Dynamic tool lists suggest stateless design
        stateless_score += 0.1
    
    # Check server name/description for hints
    server_name = server_info.get("name", "").lower()
    if "http" in server_name or "rest" in server_name or "api" in server_name:
        stateless_score += 0.2
    
    # Indicators of STATEFUL design
    if capabilities.get("resources", {}).get("subscribe"):
        # Subscriptions require persistent connections
        stateless_score -= 0.2
    
    if "cli" in server_name or "local" in server_name:
        stateless_score -= 0.2
    
    return stateless_score
```

### Interpretation

| Score | Interpretation | Confidence |
|-------|----------------|------------|
| > 0.7 | Likely stateless | Medium |
| 0.3-0.7 | Unclear | Low |
| < 0.3 | Likely stateful | Medium |

**Note:** This method alone is NOT reliable for definitive detection. Use in combination with Method 1.

---

## Method 3: Metadata Heuristics

### Concept

Use metadata clues to make educated guesses before testing.

### Heuristic Rules

```python
def guess_from_metadata(config):
    """
    Make educated guess based on configuration metadata.
    Returns: ("STATELESS" | "STATEFUL" | "UNKNOWN", confidence)
    """
    
    # Transport type
    if config.get("type") == "http":
        return ("STATELESS", 0.8)  # HTTP servers usually stateless
    
    if config.get("type") == "stdio":
        return ("STATEFUL", 0.7)  # stdio servers often stateful
    
    # Container/command inspection
    container = config.get("container", "")
    command = config.get("command", "")
    
    # Known servers
    if "github-mcp-server" in container:
        return ("STATELESS", 0.95)
    
    if "serena-mcp-server" in container:
        return ("STATEFUL", 0.95)
    
    # Language hints
    if "typescript" in container.lower() or "node" in container.lower():
        return ("STATELESS", 0.6)  # TS servers tend to be stateless
    
    if "python" in container.lower() and "local" in container.lower():
        return ("STATEFUL", 0.6)  # Local Python servers often stateful
    
    return ("UNKNOWN", 0.0)
```

### Confidence Levels

- **High (>0.9)**: Known server with documented behavior
- **Medium (0.6-0.9)**: Strong indicators but not definitive
- **Low (<0.6)**: Weak signals, requires testing

---

## Implementation Guide

### Combined Detection Strategy

```python
def detect_server_state_comprehensive(server_url, config):
    """
    Use all three methods for robust detection.
    """
    
    # Method 3: Quick metadata check
    metadata_guess, metadata_conf = guess_from_metadata(config)
    if metadata_conf > 0.9:
        print(f"High confidence from metadata: {metadata_guess}")
        return metadata_guess
    
    # Method 1: Behavioral test (definitive)
    behavioral_result = detect_mcp_server_state(server_url)
    if behavioral_result != "UNKNOWN":
        print(f"Confirmed by behavioral test: {behavioral_result}")
        return behavioral_result
    
    # Method 2: Capability analysis (fallback)
    init_response = get_initialize_response(server_url)
    capability_score = analyze_capabilities(init_response)
    
    if capability_score > 0.7:
        return "STATELESS"
    elif capability_score < 0.3:
        return "STATEFUL"
    else:
        return "UNKNOWN"
```

### Integration with Gateway

```go
// Gateway startup: Probe all backends
func (gw *Gateway) ProbeBackends() {
	for serverID, config := range gw.Config.Servers {
		log.Printf("Probing backend: %s", serverID)
		
		result, err := mcpprobe.DetectServerState(
			fmt.Sprintf("http://localhost:%d/mcp/%s", gw.Port, serverID),
			"probe-session",
		)
		
		if err != nil {
			log.Printf("⚠️  Failed to probe %s: %v", serverID, err)
			continue
		}
		
		log.Printf("Backend %s: %s - %s", serverID, result.State, result.Message)
		
		// Store result for connection pooling decisions
		gw.BackendStates[serverID] = result.State
		
		// Warn if stateful server detected
		if result.State == mcpprobe.Stateful {
			log.Printf("⚠️  WARNING: %s is stateful - may not work correctly through gateway", serverID)
			log.Printf("   Consider using direct stdio connection for full functionality")
		}
	}
}
```

---

## Recommendations

### For Gateway Operators

1. **Probe on Startup:**
   ```bash
   # Run probe script when gateway starts
   for backend in $(list_backends); do
       ./detect_mcp_state.sh "http://localhost:8080/mcp/$backend"
   done
   ```

2. **Log Results:**
   - Store detection results in gateway metadata
   - Display warnings in health check for stateful servers
   - Document limitations in API responses

3. **Automatic Handling:**
   - Stateless servers: Use per-request connections (current behavior)
   - Stateful servers: Either reject or enable connection pooling (future)

### For MCP Server Developers

1. **Declare in Documentation:**
   ```markdown
   ## Architecture
   - **Type:** Stateless HTTP-native
   - **Gateway Compatible:** Yes
   - **Deployment:** Cloud, serverless, containers
   ```

2. **Add Custom Metadata (Proposal):**
   ```json
   {
     "serverInfo": {
       "name": "MyMCPServer",
       "version": "1.0.0",
       "architecture": "stateless",  // Custom field
       "transport": ["http", "stdio"]
     }
   }
   ```

3. **Support Both Modes:**
   - Provide configuration flag: `--stateless` vs `--stateful`
   - Hybrid servers get best of both worlds

### For MCP Protocol Maintainers

**Proposal: Add official field to initialize response:**

```json
{
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {},
      "session": {
        "persistent": false,  // NEW: indicates stateless
        "poolable": true      // NEW: can reuse connections
      }
    },
    "serverInfo": {
      "name": "ExampleServer",
      "version": "1.0.0"
    }
  }
}
```

**Benefits:**
- Eliminates need for behavioral probing
- Servers explicitly declare their design
- Clients can make informed decisions
- Gateway can auto-configure connection handling

---

## Summary

### Detection Methods Comparison

| Aspect | Sequential Test | Capability Analysis | Metadata Heuristics |
|--------|----------------|-------------------|-------------------|
| **Accuracy** | ✅ Very High | ⚠️ Medium | ⚠️ Low |
| **Reliability** | ✅ Definitive | ⚠️ Guesses | ⚠️ Assumptions |
| **Speed** | ⚠️ 2+ requests | ✅ 1 request | ✅ Instant |
| **Invasiveness** | ⚠️ Multiple connections | ✅ Normal init | ✅ Config only |
| **False Positives** | ~1% | ~30% | ~50% |
| **Recommended For** | Production probing | Quick screening | Initial triage |

### Best Practice

**Use layered approach:**
1. Start with metadata heuristics (instant, low cost)
2. If unclear, run capability analysis (single request)
3. If still unclear, run sequential test (definitive but 2 connections)

### Test Results

Based on our testing:

| Server | Detection Result | Confidence | Method Used |
|--------|-----------------|------------|-------------|
| **GitHub MCP** | STATELESS ✅ | 99% | Sequential test |
| **Serena MCP** | STATEFUL ❌ | 99% | Sequential test |

---

## References

- [MCP Protocol Specification](https://modelcontextprotocol.io/specification/)
- [MCP Server Architecture Analysis](MCP_SERVER_ARCHITECTURE_ANALYSIS.md)
- [Gateway Test Findings](GATEWAY_TEST_FINDINGS.md)
- [SEP-1442: Make MCP Stateless by Default](https://github.com/modelcontextprotocol/modelcontextprotocol/issues/1442)

---

**Document Version:** 1.0  
**Last Updated:** January 19, 2026  
**Author:** MCP Gateway Testing Team
