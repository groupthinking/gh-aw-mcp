package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

// TestToolsListIncludesInputSchema verifies that tools/list responses include
// inputSchema for all tools, which is required for clients to understand
// the parameter structure.
func TestToolsListIncludesInputSchema(t *testing.T) {
	// Create a mock backend that returns a tool with inputSchema
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		var request map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &request); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		method, _ := request["method"].(string)
		requestID := request["id"]

		if method == "initialize" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "backend-session-123")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      requestID,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "test-backend",
						"version": "1.0.0",
					},
				},
			})
			return
		}

		if method == "tools/list" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      requestID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "test_tool",
							"description": "A test tool",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"body": map[string]interface{}{
										"type":        "string",
										"description": "The body parameter",
									},
								},
								"required": []string{"body"},
							},
						},
					},
				},
			})
			return
		}

		http.Error(w, "Unknown method", http.StatusBadRequest)
	}))
	defer mockBackend.Close()

	// Create gateway configuration with the mock backend
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"testserver": {
				Type: "http",
				URL:  mockBackend.URL,
				Headers: map[string]string{
					"Authorization": "test-auth",
				},
			},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	// Create the unified server's HTTP handler
	httpServer := CreateHTTPServerForMCP("127.0.0.1:0", us, "")
	testServer := httptest.NewServer(httpServer.Handler)
	defer testServer.Close()

	client := &http.Client{}

	// First, initialize the session
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{},
		},
	}

	initBody, _ := json.Marshal(initReq)
	initReqHTTP, _ := http.NewRequest("POST", testServer.URL+"/mcp", bytes.NewReader(initBody))
	initReqHTTP.Header.Set("Content-Type", "application/json")
	initReqHTTP.Header.Set("Accept", "application/json, text/event-stream")
	initReqHTTP.Header.Set("Authorization", "test-session-123")

	initResp, err := client.Do(initReqHTTP)
	require.NoError(t, err, "Initialize request failed")
	io.Copy(io.Discard, initResp.Body)
	initResp.Body.Close()

	// Now make tools/list request through the gateway
	toolsListReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	toolsListBody, _ := json.Marshal(toolsListReq)
	req, _ := http.NewRequest("POST", testServer.URL+"/mcp", bytes.NewReader(toolsListBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "test-session-123")

	resp, err := client.Do(req)
	require.NoError(t, err, "Tools/list request failed")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Tools/list response: %s", string(body))

	// Parse the response - it could be SSE format or JSON
	var result map[string]interface{}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "text/event-stream" {
		// Parse SSE format
		lines := bytes.Split(body, []byte("\n"))
		for _, line := range lines {
			if bytes.HasPrefix(line, []byte("data: ")) {
				jsonData := bytes.TrimPrefix(line, []byte("data: "))
				if err := json.Unmarshal(jsonData, &result); err == nil {
					break
				}
			}
		}
	} else {
		// Parse regular JSON
		err = json.Unmarshal(body, &result)
		require.NoError(t, err, "Failed to parse tools/list response")
	}

	// Verify the response structure
	require.Contains(t, result, "result", "Response should contain result")
	resultData := result["result"].(map[string]interface{})
	require.Contains(t, resultData, "tools", "Result should contain tools array")

	tools := resultData["tools"].([]interface{})
	require.NotEmpty(t, tools, "Tools array should not be empty")

	// Check the first tool
	tool := tools[0].(map[string]interface{})
	t.Logf("Tool: %+v", tool)

	assert.Contains(t, tool, "name", "Tool should have name")
	assert.Contains(t, tool, "description", "Tool should have description")

	// CRITICAL CHECK: Tool MUST have inputSchema for clients to understand parameters
	assert.Contains(t, tool, "inputSchema", "Tool MUST have inputSchema - this is the bug being fixed")

	if schema, ok := tool["inputSchema"]; ok {
		schemaMap := schema.(map[string]interface{})
		assert.Equal(t, "object", schemaMap["type"], "InputSchema should have type: object")
		assert.Contains(t, schemaMap, "properties", "InputSchema should have properties")

		properties := schemaMap["properties"].(map[string]interface{})
		assert.Contains(t, properties, "body", "InputSchema should define the 'body' parameter")
	} else {
		t.Error("ISSUE CONFIRMED: inputSchema is missing from tools/list response")
		t.Error("This causes clients to not understand the tool's parameter structure")
	}
}
