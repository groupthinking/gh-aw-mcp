package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"encoding/json"
	"github.com/githubnext/gh-aw-mcpg/internal/config"
	"io"
	"net/http"
	"net/http/httptest"
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

	// Check that tools registered in the UnifiedServer have InputSchema
	us.toolsMu.RLock()
	tools := us.tools
	us.toolsMu.RUnlock()

	require.NotEmpty(t, tools, "Should have registered tools")

	// Find our test tool
	var testTool *ToolInfo
	for name, tool := range tools {
		if tool.BackendID == "testserver" {
			testTool = tool
			t.Logf("Found tool: %s", name)
			break
		}
	}

	require.NotNil(t, testTool, "Should have found test tool")

	// Verify the tool has InputSchema
	assert.NotNil(t, testTool.InputSchema, "Tool MUST have InputSchema")
	assert.NotEmpty(t, testTool.InputSchema, "InputSchema should not be empty")

	// Verify the schema structure
	assert.Equal(t, "object", testTool.InputSchema["type"], "InputSchema should have type: object")
	assert.Contains(t, testTool.InputSchema, "properties", "InputSchema should have properties")

	propertiesValue := testTool.InputSchema["properties"]
	require.NotNil(t, propertiesValue, "properties value should not be nil")
	properties, ok := propertiesValue.(map[string]interface{})
	require.True(t, ok, "properties should be a map[string]interface{}")
	assert.Contains(t, properties, "body", "InputSchema should define the 'body' parameter")

	t.Logf("✓ Tool has proper InputSchema: %+v", testTool.InputSchema)
}
