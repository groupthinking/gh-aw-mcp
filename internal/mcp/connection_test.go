package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHTTPRequest_SessionIDHeader tests that the Mcp-Session-Id header is added to HTTP requests
func TestHTTPRequest_SessionIDHeader(t *testing.T) {
	// Create a test HTTP server that captures headers
	var receivedSessionID string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the Mcp-Session-Id header
		receivedSessionID = r.Header.Get("Mcp-Session-Id")

		// Return a mock JSON-RPC response
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": []map[string]interface{}{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	// Create an HTTP connection
	conn, err := NewHTTPConnection(context.Background(), testServer.URL, map[string]string{
		"Authorization": "test-auth-token",
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP connection: %v", err)
	}

	// Create a context with session ID
	sessionID := "test-session-123"
	ctx := context.WithValue(context.Background(), SessionIDContextKey, sessionID)

	// Send a request with the context containing the session ID
	_, err = conn.SendRequestWithServerID(ctx, "tools/list", nil, "test-server")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Verify the Mcp-Session-Id header was received
	if receivedSessionID != sessionID {
		t.Errorf("Expected Mcp-Session-Id header '%s', got '%s'", sessionID, receivedSessionID)
	}
}

// TestHTTPRequest_NoSessionID tests that requests work without session ID
func TestHTTPRequest_NoSessionID(t *testing.T) {
	// Create a test HTTP server
	var receivedSessionID string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSessionID = r.Header.Get("Mcp-Session-Id")

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": []map[string]interface{}{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	// Create an HTTP connection
	conn, err := NewHTTPConnection(context.Background(), testServer.URL, map[string]string{
		"Authorization": "test-auth-token",
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP connection: %v", err)
	}

	// Send a request without session ID in context
	ctx := context.Background()
	_, err = conn.SendRequestWithServerID(ctx, "tools/list", nil, "test-server")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Verify no Mcp-Session-Id header was sent (empty string is acceptable)
	if receivedSessionID != "" {
		t.Logf("Received Mcp-Session-Id header: '%s' (expected empty)", receivedSessionID)
	}
}

// TestHTTPRequest_ConfiguredHeaders tests that configured headers are still sent
func TestHTTPRequest_ConfiguredHeaders(t *testing.T) {
	// Create a test HTTP server that captures headers
	var receivedAuth, receivedSessionID string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedSessionID = r.Header.Get("Mcp-Session-Id")

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": []map[string]interface{}{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	// Create an HTTP connection with configured headers
	authToken := "configured-auth-token"
	conn, err := NewHTTPConnection(context.Background(), testServer.URL, map[string]string{
		"Authorization": authToken,
	})
	if err != nil {
		t.Fatalf("Failed to create HTTP connection: %v", err)
	}

	// Create a context with session ID
	sessionID := "session-with-auth"
	ctx := context.WithValue(context.Background(), SessionIDContextKey, sessionID)

	// Send a request
	_, err = conn.SendRequestWithServerID(ctx, "tools/list", nil, "test-server")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Verify both headers were received
	if receivedAuth != authToken {
		t.Errorf("Expected Authorization header '%s', got '%s'", authToken, receivedAuth)
	}
	if receivedSessionID != sessionID {
		t.Errorf("Expected Mcp-Session-Id header '%s', got '%s'", sessionID, receivedSessionID)
	}
}
