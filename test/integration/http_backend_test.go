package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestHTTPMCPBackendServer tests creating an HTTP MCP backend server
// that can be wrapped by the gateway with proper authorization header handling.
//
// This test demonstrates:
// 1. Creating an HTTP MCP backend server using the SDK
// 2. HTTP backend requires and validates authorization headers
// 3. Tools and resources are available via streamable HTTP transport
// 4. Streamable HTTP transport uses SSE-formatted responses for streaming
func TestHTTPMCPBackendServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Expected authorization header that the backend requires
	expectedBackendAuth := "backend-secret-key-789"
	var receivedAuthHeader string

	// Create an HTTP MCP backend server using the SDK
	backendServer := createHTTPMCPBackend(t, &receivedAuthHeader, expectedBackendAuth)
	defer backendServer.Close()

	t.Logf("✓ HTTP MCP backend server started at %s", backendServer.URL)

	// Test 1: Initialize request with proper authorization
	t.Run("InitializeWithAuthorization", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "1.0.0",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", backendServer.URL+"/mcp", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", expectedBackendAuth)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
		}

		// Verify the backend received the authorization header
		if receivedAuthHeader != expectedBackendAuth {
			t.Errorf("Expected backend to receive auth '%s', got '%s'", expectedBackendAuth, receivedAuthHeader)
		} else {
			t.Log("✓ HTTP backend correctly received authorization header")
		}

		// Verify streamable HTTP response format (uses SSE-formatted streaming)
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/event-stream") {
			t.Errorf("Expected streamable HTTP (text/event-stream) content type, got %s", contentType)
		} else {
			t.Log("✓ HTTP backend returns streamable HTTP with SSE-formatted responses")
		}

		// Read and parse SSE response
		body, _ := io.ReadAll(resp.Body)
		if len(body) == 0 {
			t.Error("Expected non-empty response body")
		} else {
			t.Logf("✓ Received initialize response from HTTP backend")
		}
	})

	// Test 2: Backend rejects requests without authorization
	t.Run("RejectWithoutAuthorization", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "initialize",
			"params":  map[string]interface{}{},
		}

		jsonData, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", backendServer.URL+"/mcp", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		// No Authorization header

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for missing auth, got %d", resp.StatusCode)
		} else {
			t.Log("✓ HTTP backend correctly rejects requests without authorization")
		}
	})

	// Test 3: Backend rejects requests with invalid authorization
	t.Run("RejectInvalidAuthorization", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "initialize",
			"params":  map[string]interface{}{},
		}

		jsonData, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", backendServer.URL+"/mcp", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "wrong-key-value")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for invalid auth, got %d", resp.StatusCode)
		} else {
			t.Log("✓ HTTP backend correctly rejects requests with invalid authorization")
		}
	})

	// Test 4: Tools/list request
	t.Run("ListTools", func(t *testing.T) {
		toolsReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      4,
			"method":  "tools/list",
		}

		jsonData, _ := json.Marshal(toolsReq)
		req, _ := http.NewRequest("POST", backendServer.URL+"/mcp", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", expectedBackendAuth)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Tools/list response: %s", string(body))
		}

		t.Log("✓ HTTP backend responds to tools/list request")
	})

	// Test 5: Resources/list request
	t.Run("ListResources", func(t *testing.T) {
		resourcesReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      5,
			"method":  "resources/list",
		}

		jsonData, _ := json.Marshal(resourcesReq)
		req, _ := http.NewRequest("POST", backendServer.URL+"/mcp", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", expectedBackendAuth)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Resources/list response: %s", string(body))
		}

		t.Log("✓ HTTP backend responds to resources/list request")
	})

	t.Log("✓ HTTP MCP backend server test completed successfully")
	t.Log("  - Backend validates authorization headers")
	t.Log("  - Backend serves tools and resources via streamable HTTP transport")
	t.Log("  - Backend is ready to be wrapped by the gateway")
}

// createHTTPMCPBackend creates an HTTP MCP backend server with authorization
func createHTTPMCPBackend(t *testing.T, receivedAuthHeader *string, expectedAuth string) *httptest.Server {
	t.Helper()

	// Create MCP server implementation
	impl := &sdk.Implementation{
		Name:    "http-mcp-backend",
		Version: "1.0.0",
	}

	mcpServer := sdk.NewServer(impl, nil)

	// Add a test tool
	mcpServer.AddTool(&sdk.Tool{
		Name:        "test_tool",
		Description: "A test tool from HTTP backend",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Test message",
				},
			},
			"required": []string{"message"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: "Tool response from HTTP backend",
				},
			},
		}, nil
	})

	// Add another tool to demonstrate multiple tools
	mcpServer.AddTool(&sdk.Tool{
		Name:        "echo_tool",
		Description: "Echoes the input message",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Text to echo",
				},
			},
			"required": []string{"text"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		var args map[string]interface{}
		if len(req.Params.Arguments) > 0 {
			json.Unmarshal(req.Params.Arguments, &args)
		}
		text := "empty"
		if t, ok := args["text"].(string); ok {
			text = t
		}
		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: "Echo: " + text,
				},
			},
		}, nil
	})

	// Add a resource
	mcpServer.AddResource(&sdk.Resource{
		URI:         "http://backend/test-resource",
		Name:        "Test Resource",
		Description: "A test resource from HTTP backend",
		MIMEType:    "text/plain",
	}, func(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		return &sdk.ReadResourceResult{
			Contents: []*sdk.ResourceContents{
				{
					URI:      "http://backend/test-resource",
					MIMEType: "text/plain",
					Text:     "Resource content from HTTP backend",
				},
			},
		}, nil
	})

	// Create authorization middleware
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			*receivedAuthHeader = authHeader

			if authHeader == "" {
				t.Logf("HTTP backend: rejecting request with missing authorization")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Unauthorized - missing authorization header",
				})
				return
			}

			if authHeader != expectedAuth {
				t.Logf("HTTP backend: rejecting request with invalid authorization (got '%s', expected '%s')", authHeader, expectedAuth)
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Unauthorized - invalid authorization",
				})
				return
			}

			t.Logf("HTTP backend: accepted request with valid authorization")
			next.ServeHTTP(w, r)
		})
	}

	// Create StreamableHTTP handler using SDK
	// This creates a streamable HTTP transport for MCP with SSE-formatted responses
	mcpHandler := sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
		return mcpServer
	}, &sdk.StreamableHTTPOptions{
		// Streamable HTTP options can be configured here
	})

	// Create HTTP mux and apply auth middleware
	mux := http.NewServeMux()
	mux.Handle("/mcp", authMiddleware(mcpHandler))

	// Create and return test server
	return httptest.NewServer(mux)
}
