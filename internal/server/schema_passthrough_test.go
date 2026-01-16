package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

// TestUnifiedModeInputSchemaPassthrough tests whether InputSchema is passed through
// from backend servers to clients in unified mode
func TestUnifiedModeInputSchemaPassthrough(t *testing.T) {
	var initReceived bool
	
	// Create a mock HTTP MCP server that returns tools with InputSchema
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			ID     interface{}     `json:"id"`
			Params json.RawMessage `json:"params,omitempty"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		
		if req.Method == "initialize" {
			initReceived = true
			// Return valid initialize response
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{},
					},
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if req.Method == "tools/list" {
			// Return tools with detailed InputSchema
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "get_commit",
							"description": "Get commit details",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"owner": map[string]interface{}{
										"type":        "string",
										"description": "Repository owner",
									},
									"repo": map[string]interface{}{
										"type":        "string",
										"description": "Repository name",
									},
									"sha": map[string]interface{}{
										"type":        "string",
										"description": "Commit SHA",
									},
								},
								"required": []string{"owner", "repo", "sha"},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		
		// Default response for other methods
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create config with HTTP backend
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {
				Type:    "http",
				URL:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	// Create unified server
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()
	
	require.True(t, initReceived, "Expected initialize to be called on backend")

	// Now check what tools are registered
	// The gateway stores tools in us.tools with InputSchema
	us.toolsMu.RLock()
	toolInfo, exists := us.tools["github___get_commit"]
	us.toolsMu.RUnlock()

	require.True(t, exists, "Expected tool 'github___get_commit' to be registered")
	
	// Check if InputSchema is stored internally
	assert.NotNil(t, toolInfo.InputSchema, "Expected InputSchema to be stored internally")
	if toolInfo.InputSchema != nil {
		t.Logf("✓ InputSchema is stored internally: %+v", toolInfo.InputSchema)
		
		// Check that it has the expected structure
		schemaType, hasType := toolInfo.InputSchema["type"]
		assert.True(t, hasType, "Expected InputSchema to have 'type' field")
		assert.Equal(t, "object", schemaType, "Expected InputSchema type to be 'object'")
		
		properties, hasProperties := toolInfo.InputSchema["properties"]
		assert.True(t, hasProperties, "Expected InputSchema to have 'properties' field")
		assert.NotNil(t, properties, "Expected properties to be non-nil")
	}

	// The issue: When the gateway registers tools with sdk.AddTool, it omits InputSchema
	// This is documented in unified.go:258-260:
	// "Note: InputSchema is intentionally omitted to avoid validation errors
	//  when backend MCP servers use different JSON Schema versions (e.g., draft-07)
	//  than what the SDK supports (draft-2020-12)"
	//
	// However, this means clients calling tools/list via the SDK won't receive InputSchema
	// This test confirms that InputSchema is stored internally but may not be passed to clients
	
	t.Logf("✓ Test confirms: InputSchema IS stored internally in gateway")
	t.Logf("⚠️  However, it may not be passed through to SDK clients")
	t.Logf("   See unified.go:261-264 where tools are registered without InputSchema")
}

// TestRoutedModeSchemaNote documents that routed mode forwards requests directly
// to backends, so it should pass through InputSchema without modification
func TestRoutedModeSchemaNote(t *testing.T) {
	t.Log("ℹ️  Routed mode forwards tools/list requests directly to backend servers")
	t.Log("   This means InputSchema from backends should pass through unmodified")
	t.Log("   See server.go:182-183 where tools/list is forwarded via SendRequestWithServerID")
	t.Log("   This behavior is correct and should be preserved")
}
