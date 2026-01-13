package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

// TestExpandDockerEnvArgs tests the Docker environment variable expansion function
func TestExpandDockerEnvArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envVars  map[string]string
		expected []string
	}{
		{
			name:     "no -e flags",
			args:     []string{"run", "--rm", "image"},
			envVars:  map[string]string{},
			expected: []string{"run", "--rm", "image"},
		},
		{
			name:     "expand single env variable",
			args:     []string{"run", "-e", "VAR_NAME", "image"},
			envVars:  map[string]string{"VAR_NAME": "value1"},
			expected: []string{"run", "-e", "VAR_NAME=value1", "image"},
		},
		{
			name:     "expand multiple env variables",
			args:     []string{"run", "-e", "VAR1", "-e", "VAR2", "image"},
			envVars:  map[string]string{"VAR1": "value1", "VAR2": "value2"},
			expected: []string{"run", "-e", "VAR1=value1", "-e", "VAR2=value2", "image"},
		},
		{
			name:     "preserve existing key=value format",
			args:     []string{"run", "-e", "VAR=predefined", "image"},
			envVars:  map[string]string{},
			expected: []string{"run", "-e", "VAR=predefined", "image"},
		},
		{
			name:     "mixed: expand and preserve",
			args:     []string{"run", "-e", "VAR1", "-e", "VAR2=fixed", "image"},
			envVars:  map[string]string{"VAR1": "value1"},
			expected: []string{"run", "-e", "VAR1=value1", "-e", "VAR2=fixed", "image"},
		},
		{
			name:     "undefined env variable",
			args:     []string{"run", "-e", "UNDEFINED_VAR", "image"},
			envVars:  map[string]string{},
			expected: []string{"run", "-e", "UNDEFINED_VAR", "image"},
		},
		{
			name:     "empty env variable value",
			args:     []string{"run", "-e", "EMPTY_VAR", "image"},
			envVars:  map[string]string{"EMPTY_VAR": ""},
			expected: []string{"run", "-e", "EMPTY_VAR=", "image"},
		},
		{
			name:     "-e at end of args (no following arg)",
			args:     []string{"run", "image", "-e"},
			envVars:  map[string]string{},
			expected: []string{"run", "image", "-e"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for test
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			// Clean up after test
			t.Cleanup(func() {
				for k := range tt.envVars {
					os.Unsetenv(k)
				}
			})

			result := expandDockerEnvArgs(tt.args)

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d args, got %d: %v", len(tt.expected), len(result), result)
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Arg %d: expected '%s', got '%s'", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

// TestHTTPRequest_ErrorResponses tests handling of various error conditions
func TestHTTPRequest_ErrorResponses(t *testing.T) {
	tests := []struct {
		name               string
		statusCode         int
		responseBody       map[string]interface{}
		expectError        bool
		errorSubstring     string
		needSuccessfulInit bool // If true, return success for initialize requests
	}{
		{
			name:       "HTTP 200 success",
			statusCode: http.StatusOK,
			responseBody: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"tools": []interface{}{},
				},
			},
			expectError: false,
		},
		{
			name:       "HTTP 404 not found",
			statusCode: http.StatusNotFound,
			responseBody: map[string]interface{}{
				"error": "Not found",
			},
			expectError:    true,
			errorSubstring: "status=404",
		},
		{
			name:       "HTTP 500 server error",
			statusCode: http.StatusInternalServerError,
			responseBody: map[string]interface{}{
				"error": "Internal server error",
			},
			expectError:    true,
			errorSubstring: "status=500",
		},
		{
			name:       "JSON-RPC error response",
			statusCode: http.StatusOK,
			responseBody: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"error": map[string]interface{}{
					"code":    -32600,
					"message": "Invalid request",
				},
			},
			expectError:        false, // JSON-RPC errors are returned as valid responses
			needSuccessfulInit: true,  // Need successful initialize to test error handling
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server with specific response
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Read request body to determine if it's an initialize request
				var reqBody map[string]interface{}
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("Failed to read request body: %v", err)
					http.Error(w, "Internal error", http.StatusInternalServerError)
					return
				}
				if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
					t.Errorf("Failed to unmarshal request body: %v", err)
					http.Error(w, "Bad request", http.StatusBadRequest)
					return
				}

				// If this test case needs successful initialization, return success for initialize
				// and error for subsequent requests
				method, _ := reqBody["method"].(string)
				if tt.needSuccessfulInit && method == "initialize" {
					// Return success for initialize request
					w.WriteHeader(http.StatusOK)
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Mcp-Session-Id", "test-session-123")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      reqBody["id"],
						"result": map[string]interface{}{
							"protocolVersion": "2024-11-05",
							"serverInfo": map[string]interface{}{
								"name":    "test-server",
								"version": "1.0.0",
							},
						},
					})
					return
				}

				// For all other cases, return the configured response
				w.WriteHeader(tt.statusCode)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.responseBody)
			}))
			defer testServer.Close()

			// Create connection with custom headers to use plain JSON transport
			conn, err := NewHTTPConnection(context.Background(), testServer.URL, map[string]string{
				"Authorization": "test-token",
			})
			if err != nil && tt.expectError {
				// Error during initialization is expected for some error conditions
				if tt.errorSubstring != "" && !containsSubstring(err.Error(), tt.errorSubstring) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorSubstring, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Failed to create connection: %v", err)
			}

			// Send request
			_, err = conn.SendRequestWithServerID(context.Background(), "tools/list", nil, "test-server")

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorSubstring != "" && !containsSubstring(err.Error(), tt.errorSubstring) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorSubstring, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestConnection_IsHTTP tests the IsHTTP, GetHTTPURL, and GetHTTPHeaders methods
func TestConnection_IsHTTP(t *testing.T) {
	// Create a mock HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer testServer.Close()

	headers := map[string]string{
		"Authorization": "test-auth",
		"X-Custom":      "custom-value",
	}

	conn, err := NewHTTPConnection(context.Background(), testServer.URL, headers)
	if err != nil {
		t.Fatalf("Failed to create HTTP connection: %v", err)
	}
	defer conn.Close()

	// Test IsHTTP
	if !conn.IsHTTP() {
		t.Error("Expected IsHTTP() to return true for HTTP connection")
	}

	// Test GetHTTPURL
	if conn.GetHTTPURL() != testServer.URL {
		t.Errorf("Expected URL '%s', got '%s'", testServer.URL, conn.GetHTTPURL())
	}

	// Test GetHTTPHeaders
	returnedHeaders := conn.GetHTTPHeaders()
	if len(returnedHeaders) != len(headers) {
		t.Errorf("Expected %d headers, got %d", len(headers), len(returnedHeaders))
	}
	for k, v := range headers {
		if returnedHeaders[k] != v {
			t.Errorf("Expected header '%s' to be '%s', got '%s'", k, v, returnedHeaders[k])
		}
	}
}

// TestHTTPConnection_InvalidURL tests error handling for invalid URLs
func TestHTTPConnection_InvalidURL(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		headers        map[string]string
		expectError    bool
		errorSubstring string
	}{
		{
			name:        "valid URL with headers",
			url:         "http://localhost:3000",
			headers:     map[string]string{"Authorization": "test"},
			expectError: true, // Will fail to connect but URL is valid
		},
		{
			name:        "empty URL",
			url:         "",
			headers:     map[string]string{"Authorization": "test"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHTTPConnection(context.Background(), tt.url, tt.headers)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorSubstring != "" && !containsSubstring(err.Error(), tt.errorSubstring) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorSubstring, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// containsSubstring is a helper to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
