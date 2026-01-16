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

// TestInvalidSchemaFromBackend tests what happens when a backend returns
// a tool with an invalid schema (e.g., "object" type without "properties")
func TestInvalidSchemaFromBackend(t *testing.T) {
	var initReceived bool

	// Create a mock backend that returns a tool with an invalid schema
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
			// Return a tool with INVALID schema: type is "object" but no "properties"
			// This is similar to what the GitHub MCP server might be returning
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
								// INVALID: object type without "properties" field
								// This violates JSON Schema spec
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

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

	// Check what's stored internally
	us.toolsMu.RLock()
	toolInfo, exists := us.tools["github___get_commit"]
	us.toolsMu.RUnlock()

	require.True(t, exists, "Expected tool to be registered")

	// The gateway stores the invalid schema as-is
	assert.NotNil(t, toolInfo.InputSchema, "Expected InputSchema to be stored")
	t.Logf("Stored InputSchema (invalid): %+v", toolInfo.InputSchema)

	// Check what it contains
	if toolInfo.InputSchema != nil {
		schemaType, hasType := toolInfo.InputSchema["type"]
		assert.True(t, hasType, "Expected type field")
		assert.Equal(t, "object", schemaType, "Expected type to be object")

		_, hasProperties := toolInfo.InputSchema["properties"]
		assert.False(t, hasProperties, "Confirmed: Invalid schema has NO properties field")

		t.Logf("✓ Gateway correctly stores invalid schema as-is")
		t.Logf("⚠️  The invalid schema is from the backend, NOT created by the gateway")
	}
}

// TestEmptySchemaFromBackend tests what happens when backend returns empty schema
func TestEmptySchemaFromBackend(t *testing.T) {
	var initReceived bool

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
			// Return a tool with completely empty schema
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "get_commit",
							"description": "Get commit details",
							"inputSchema": map[string]interface{}{
								// Completely empty - no type, no properties, nothing
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {
				Type:    "http",
				URL:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err)
	defer us.Close()

	require.True(t, initReceived)

	us.toolsMu.RLock()
	toolInfo, exists := us.tools["github___get_commit"]
	us.toolsMu.RUnlock()

	require.True(t, exists)

	// Gateway stores empty schema as-is
	assert.NotNil(t, toolInfo.InputSchema, "Expected InputSchema to be stored")
	assert.Empty(t, toolInfo.InputSchema, "Expected InputSchema to be empty map")

	t.Logf("✓ Gateway stores empty schema from backend as-is")
	t.Logf("⚠️  Empty schema {} is technically valid JSON but invalid for tools")
}

// TestMissingSchemaFromBackend tests when backend doesn't include inputSchema at all
func TestMissingSchemaFromBackend(t *testing.T) {
	var initReceived bool

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
			// Return a tool WITHOUT inputSchema field at all
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "get_commit",
							"description": "Get commit details",
							// No inputSchema field
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {
				Type:    "http",
				URL:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err)
	defer us.Close()

	require.True(t, initReceived)

	us.toolsMu.RLock()
	toolInfo, exists := us.tools["github___get_commit"]
	us.toolsMu.RUnlock()

	require.True(t, exists)

	// When backend doesn't send inputSchema, it will be nil or empty
	t.Logf("InputSchema when not provided by backend: %+v", toolInfo.InputSchema)
	
	if toolInfo.InputSchema == nil {
		t.Logf("✓ InputSchema is nil when not provided by backend")
	} else if len(toolInfo.InputSchema) == 0 {
		t.Logf("✓ InputSchema is empty map when not provided by backend")
	}
}
