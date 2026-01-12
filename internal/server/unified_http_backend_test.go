package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
	"github.com/githubnext/gh-aw-mcpg/internal/launcher"
	"github.com/githubnext/gh-aw-mcpg/internal/mcp"
)

// TestHTTPBackendInitialization tests that HTTP backends receive session ID during initialization
func TestHTTPBackendInitialization(t *testing.T) {
	// Track whether the session ID header was received
	var receivedSessionID string
	var requestMethod string

	// Create a mock HTTP MCP server that requires Mcp-Session-Id header
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSessionID = r.Header.Get("Mcp-Session-Id")

		// Parse the JSON-RPC request to get the method
		var req struct {
			Method string `json:"method"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		requestMethod = req.Method

		// Simulate a backend that requires Mcp-Session-Id header
		if receivedSessionID == "" {
			w.WriteHeader(http.StatusBadRequest)
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -32600,
					"message": "Invalid Request: Missing Mcp-Session-Id header",
				},
				"id": 1,
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Return a successful tools/list response
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "test_tool",
						"description": "A test tool",
						"inputSchema": map[string]interface{}{
							"type": "object",
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create config with HTTP backend
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"http-backend": {
				Type:    "http",
				URL:     mockServer.URL,
				Headers: map[string]string{},
			},
		},
	}

	// Create unified server - this should call tools/list during initialization
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create unified server: %v", err)
	}
	defer us.Close()

	// Verify that the session ID was sent
	if receivedSessionID == "" {
		t.Errorf("Expected Mcp-Session-Id header to be sent during initialization, but it was empty")
	}

	// Verify the session ID follows the gateway-init pattern
	expectedPrefix := "gateway-init-"
	if len(receivedSessionID) < len(expectedPrefix) || receivedSessionID[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("Expected session ID to start with '%s', got '%s'", expectedPrefix, receivedSessionID)
	}

	// Verify it was a tools/list request
	if requestMethod != "tools/list" {
		t.Errorf("Expected method 'tools/list', got '%s'", requestMethod)
	}

	t.Logf("Successfully initialized HTTP backend with session ID: %s", receivedSessionID)
}

// TestHTTPBackendInitializationWithSessionIDRequirement tests the exact error scenario from the problem statement
func TestHTTPBackendInitializationWithSessionIDRequirement(t *testing.T) {
	// Create a strict HTTP MCP server that fails without Mcp-Session-Id header
	strictServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.Header.Get("Mcp-Session-Id")

		if sessionID == "" {
			// Return the exact error from the problem statement
			w.WriteHeader(http.StatusBadRequest)
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -32600,
					"message": "Invalid Request: Missing Mcp-Session-Id header",
				},
				"id": 1,
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Success - return tools list
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "safe_tool",
						"description": "A safe tool",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer strictServer.Close()

	// Create config with strict HTTP backend (simulating "safeinputs")
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"safeinputs": {
				Type: "http",
				URL:  strictServer.URL,
			},
		},
	}

	// Create unified server - should succeed with our fix
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create unified server with strict HTTP backend: %v. This indicates the Mcp-Session-Id header is not being sent during initialization.", err)
	}
	defer us.Close()

	// Verify tools were registered
	tools := us.GetToolsForBackend("safeinputs")
	if len(tools) == 0 {
		t.Errorf("Expected tools to be registered from safeinputs backend, got none")
	}

	t.Logf("Successfully initialized strict HTTP backend 'safeinputs' with %d tools", len(tools))
}

// TestHTTPBackend_SessionIDPropagation tests that session ID is propagated through tool calls
func TestHTTPBackend_SessionIDPropagation(t *testing.T) {
	// Track session IDs received at different stages
	initSessionID := ""
	toolCallSessionID := ""

	// Create a mock HTTP MCP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.Header.Get("Mcp-Session-Id")

		var req struct {
			Method string      `json:"method"`
			Params interface{} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method == "tools/list" {
			initSessionID = sessionID
			// Return tools list
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "echo",
							"description": "Echo tool",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if req.Method == "tools/call" {
			toolCallSessionID = sessionID
			// Return tool result
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": "echo response",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer mockServer.Close()

	// Create config
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test-http": {
				Type: "http",
				URL:  mockServer.URL,
			},
		},
	}

	// Create unified server
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create unified server: %v", err)
	}
	defer us.Close()

	// Create a connection and call a tool with a specific session ID
	conn, err := launcher.GetOrLaunch(us.launcher, "test-http")
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	clientSessionID := "client-session-12345"
	ctxWithSession := context.WithValue(context.Background(), mcp.SessionIDContextKey, clientSessionID)

	_, err = conn.SendRequestWithServerID(ctxWithSession, "tools/call", map[string]interface{}{
		"name":      "echo",
		"arguments": map[string]interface{}{"message": "test"},
	}, "test-http")
	if err != nil {
		t.Fatalf("Failed to call tool: %v", err)
	}

	// Verify session IDs were received
	if initSessionID == "" {
		t.Errorf("No session ID received during initialization")
	} else {
		t.Logf("Init session ID: %s", initSessionID)
	}

	if toolCallSessionID == "" {
		t.Errorf("No session ID received during tool call")
	} else if toolCallSessionID != clientSessionID {
		t.Errorf("Expected tool call session ID to be '%s', got '%s'", clientSessionID, toolCallSessionID)
	}

	t.Logf("Session ID propagation test passed")
}
