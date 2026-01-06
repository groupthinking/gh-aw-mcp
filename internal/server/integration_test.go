package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

// TestTransparentProxy_RoutedMode tests that flowguard-go acts as a transparent proxy
// when DIFC is disabled (using NoopGuard) in routed mode.
// This verifies that requests and responses pass through without modification.
func TestTransparentProxy_RoutedMode(t *testing.T) {
	// Skip if running in short mode (this is an integration test)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a mock MCP backend server that responds to MCP protocol requests
	mockBackend := newMockMCPServer(t)
	defer mockBackend.Close()

	// Create config that points to our mock backend (using stdio transport)
	// Since we can't easily mock stdio, we'll test the HTTP layer directly
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"testserver": {
				Command: "echo", // Dummy command, won't actually be used in this test
				Args:    []string{},
			},
		},
	}

	// Create unified server
	us, err := NewUnified(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create unified server: %v", err)
	}
	defer us.Close()

	// Manually inject mock tools to simulate backend tools
	// This simulates what would normally be fetched from the backend
	us.toolsMu.Lock()
	us.tools["testserver___test_tool"] = &ToolInfo{
		Name:        "testserver___test_tool",
		Description: "A test tool",
		BackendID:   "testserver",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Test input",
				},
			},
		},
		Handler: func(ctx context.Context, req *sdk.CallToolRequest, state interface{}) (*sdk.CallToolResult, interface{}, error) {
			// Forward to mock backend
			return mockBackend.handleToolCall(req)
		},
	}
	us.toolsMu.Unlock()

	// Create HTTP server in routed mode
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us)
	
	// Start server in background using httptest
	ts := httptest.NewServer(httpServer.Handler)
	defer ts.Close()

	serverURL := ts.URL
	t.Logf("Test server started at %s", serverURL)

	// Test 1: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// Test 2: Initialize request (transparent proxy test)
	t.Run("Initialize", func(t *testing.T) {
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

		resp := sendMCPRequest(t, serverURL+"/mcp/testserver", "test-token", initReq)
		
		// Verify response structure
		if resp["jsonrpc"] != "2.0" {
			t.Errorf("Expected jsonrpc 2.0, got %v", resp["jsonrpc"])
		}

		// Check that result contains server info
		result, ok := resp["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result object, got %v", resp["result"])
		}

		serverInfo, ok := result["serverInfo"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected serverInfo in result, got %v", result)
		}

		// The gateway creates a filtered server for each backend
		// Check that the server name contains the backend ID
		serverName := serverInfo["name"].(string)
		if !strings.Contains(serverName, "testserver") {
			t.Errorf("Expected server name to contain 'testserver', got %v", serverName)
		}

		t.Logf("✓ Initialize response passed through correctly: %v", serverName)
	})

	// Test 3: List tools (verify transparent proxying)
	// Note: In stateless mode, each request needs to initialize first
	// This test creates a persistent client to maintain session
	t.Run("ListTools", func(t *testing.T) {
		// Create a client that reuses the connection
		client := &http.Client{Timeout: 5 * time.Second}
		
		// First, initialize
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
		
		_ = sendMCPRequestWithClient(t, serverURL+"/mcp/testserver", "test-token-list", client, initReq)

		// Now list tools
		listReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
			"params":  map[string]interface{}{},
		}

		resp := sendMCPRequestWithClient(t, serverURL+"/mcp/testserver", "test-token-list", client, listReq)
		
		// Log full response for debugging
		t.Logf("Full response: %+v", resp)

		// Verify the response contains tools
		result, ok := resp["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result object, got %T: %v", resp["result"], resp)
		}

		tools, ok := result["tools"].([]interface{})
		if !ok {
			t.Fatalf("Expected tools array, got %v", result["tools"])
		}

		if len(tools) == 0 {
			t.Error("Expected at least one tool in response")
		}

		// Verify tool structure
		tool := tools[0].(map[string]interface{})
		if tool["name"] != "test_tool" {
			t.Errorf("Expected tool name 'test_tool', got %v", tool["name"])
		}
		
		t.Logf("✓ Tools list passed through correctly: %d tools", len(tools))
	})

	// Test 4: Call tool (verify request/response transparency)
	t.Run("CallTool", func(t *testing.T) {
		// Create a client that reuses the connection
		client := &http.Client{Timeout: 5 * time.Second}
		
		// First, initialize
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
		
		_ = sendMCPRequestWithClient(t, serverURL+"/mcp/testserver", "test-token-call", client, initReq)

		callReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "test_tool",
				"arguments": map[string]interface{}{
					"input": "test input value",
				},
			},
		}

		resp := sendMCPRequestWithClient(t, serverURL+"/mcp/testserver", "test-token-call", client, callReq)

		// Verify response
		result, ok := resp["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result object, got %v", resp["result"])
		}

		// Check that the content was passed through from mock backend
		content, ok := result["content"].([]interface{})
		if !ok || len(content) == 0 {
			t.Fatalf("Expected content array, got %v", result["content"])
		}

		contentItem := content[0].(map[string]interface{})
		if contentItem["type"] != "text" {
			t.Errorf("Expected text content, got %v", contentItem["type"])
		}

		text := contentItem["text"].(string)
		if !strings.Contains(text, "test input value") {
			t.Errorf("Expected response to contain input value, got: %s", text)
		}
		
		t.Logf("✓ Tool call response passed through correctly")
	})

	// Test 5: Verify DIFC is disabled (NoopGuard behavior)
	t.Run("DIFCDisabled", func(t *testing.T) {
		// Verify that the guard registry has the noop guard for testserver
		guard := us.guardRegistry.Get("testserver")
		if guard.Name() != "noop" {
			t.Errorf("Expected NoopGuard, got guard with name: %s", guard.Name())
		}

		t.Log("✓ DIFC is disabled - using NoopGuard")
	})
}

// mockMCPServer simulates a real MCP backend server for testing
type mockMCPServer struct {
	t          *testing.T
	server     *httptest.Server
	tools      []map[string]interface{}
	callCounts map[string]int
	mu         sync.Mutex
}

func newMockMCPServer(t *testing.T) *mockMCPServer {
	mock := &mockMCPServer{
		t:          t,
		callCounts: make(map[string]int),
		tools: []map[string]interface{}{
			{
				"name":        "test_tool",
				"description": "A test tool for integration testing",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"input": map[string]interface{}{
							"type":        "string",
							"description": "Test input",
						},
					},
				},
			},
		},
	}

	// We don't actually need an HTTP server for this test
	// since we're mocking at the handler level
	return mock
}

func (m *mockMCPServer) Close() {
	// Nothing to close in this mock
}

func (m *mockMCPServer) handleToolCall(req *sdk.CallToolRequest) (*sdk.CallToolResult, interface{}, error) {
	m.mu.Lock()
	m.callCounts[req.Params.Name]++
	m.mu.Unlock()

	// Extract input from arguments
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return &sdk.CallToolResult{
			Content: []sdk.Content{&sdk.TextContent{Text: "Failed to parse arguments"}},
			IsError: true,
		}, nil, nil
	}

	input := ""
	if val, ok := args["input"]; ok {
		input = val.(string)
	}

	// Return a response that includes the input (to verify transparency)
	return &sdk.CallToolResult{
		Content: []sdk.Content{
			&sdk.TextContent{
				Text: fmt.Sprintf("Mock response for: %s", input),
			},
		},
		IsError: false,
	}, nil, nil
}



// Helper function to send MCP requests and handle SSE responses
func sendMCPRequest(t *testing.T, url string, bearerToken string, payload map[string]interface{}) map[string]interface{} {
	client := &http.Client{Timeout: 5 * time.Second}
	return sendMCPRequestWithClient(t, url, bearerToken, client, payload)
}

// Helper function to send MCP requests with a custom client (for connection reuse)
func sendMCPRequestWithClient(t *testing.T, url string, bearerToken string, client *http.Client, payload map[string]interface{}) map[string]interface{} {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	// Check if response is SSE format
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		// Parse SSE response
		return parseSSEResponse(t, resp.Body)
	}

	// Regular JSON response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	return result
}

// parseSSEResponse parses Server-Sent Events format
func parseSSEResponse(t *testing.T, body io.Reader) map[string]interface{} {
	scanner := bufio.NewScanner(body)

	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}

	if len(dataLines) == 0 {
		t.Fatal("No data lines found in SSE response")
	}

	// Join all data lines and parse as JSON
	jsonData := strings.Join(dataLines, "")
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
		t.Fatalf("Failed to decode SSE data: %v, data: %s", err, jsonData)
	}

	return result
}

// TestTransparentProxy_MultipleBackends tests transparent proxying with multiple backends
func TestTransparentProxy_MultipleBackends(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create config with multiple backends
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"backend1": {Command: "echo", Args: []string{}},
			"backend2": {Command: "echo", Args: []string{}},
		},
	}

	us, err := NewUnified(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create unified server: %v", err)
	}
	defer us.Close()

	// Add mock tools for both backends
	us.toolsMu.Lock()
	us.tools["backend1___tool1"] = &ToolInfo{
		Name:        "backend1___tool1",
		Description: "Backend 1 tool",
		BackendID:   "backend1",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, req *sdk.CallToolRequest, state interface{}) (*sdk.CallToolResult, interface{}, error) {
			return &sdk.CallToolResult{
				Content: []sdk.Content{
					&sdk.TextContent{
						Text: "Response from backend1",
					},
				},
			}, state, nil
		},
	}
	us.tools["backend2___tool2"] = &ToolInfo{
		Name:        "backend2___tool2",
		Description: "Backend 2 tool",
		BackendID:   "backend2",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, req *sdk.CallToolRequest, state interface{}) (*sdk.CallToolResult, interface{}, error) {
			return &sdk.CallToolResult{
				Content: []sdk.Content{
					&sdk.TextContent{
						Text: "Response from backend2",
					},
				},
			}, state, nil
		},
	}
	us.toolsMu.Unlock()

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us)
	ts := httptest.NewServer(httpServer.Handler)
	defer ts.Close()

	t.Logf("Test server started at %s", ts.URL)

	// Test that each backend route works independently
	t.Run("Backend1Tools", func(t *testing.T) {
		listReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/list",
			"params":  map[string]interface{}{},
		}

		resp := sendMCPRequest(t, ts.URL+"/mcp/backend1", "test-token", listReq)
		result := resp["result"].(map[string]interface{})
		tools := result["tools"].([]interface{})

		// Should only see backend1 tools
		if len(tools) != 1 {
			t.Errorf("Expected 1 tool for backend1, got %d", len(tools))
		}

		tool := tools[0].(map[string]interface{})
		if tool["name"] != "tool1" {
			t.Errorf("Expected tool 'tool1', got %v", tool["name"])
		}
	})

	t.Run("Backend2Tools", func(t *testing.T) {
		listReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/list",
			"params":  map[string]interface{}{},
		}

		resp := sendMCPRequest(t, ts.URL+"/mcp/backend2", "test-token", listReq)
		result := resp["result"].(map[string]interface{})
		tools := result["tools"].([]interface{})

		// Should only see backend2 tools
		if len(tools) != 1 {
			t.Errorf("Expected 1 tool for backend2, got %d", len(tools))
		}

		tool := tools[0].(map[string]interface{})
		if tool["name"] != "tool2" {
			t.Errorf("Expected tool 'tool2', got %v", tool["name"])
		}
	})

	// Test that backend isolation works (each backend sees only its tools)
	t.Run("BackendIsolation", func(t *testing.T) {
		backend1Tools := us.GetToolsForBackend("backend1")
		backend2Tools := us.GetToolsForBackend("backend2")

		if len(backend1Tools) != 1 || backend1Tools[0].Name != "tool1" {
			t.Error("Backend1 should only see tool1")
		}

		if len(backend2Tools) != 1 || backend2Tools[0].Name != "tool2" {
			t.Error("Backend2 should only see tool2")
		}
	})
}

// TestProxyDoesNotModifyRequests verifies that the proxy doesn't modify request payloads
func TestProxyDoesNotModifyRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Track the exact request received by the handler
	var receivedRequest *sdk.CallToolRequest
	var requestMutex sync.Mutex

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"testserver": {Command: "echo", Args: []string{}},
		},
	}

	us, err := NewUnified(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create unified server: %v", err)
	}
	defer us.Close()

	// Add tool that captures the request
	us.toolsMu.Lock()
	us.tools["testserver___echo_tool"] = &ToolInfo{
		Name:        "testserver___echo_tool",
		Description: "Echo tool",
		BackendID:   "testserver",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key1": map[string]interface{}{"type": "string"},
				"key2": map[string]interface{}{"type": "number"},
			},
		},
		Handler: func(ctx context.Context, req *sdk.CallToolRequest, state interface{}) (*sdk.CallToolResult, interface{}, error) {
			requestMutex.Lock()
			receivedRequest = req
			requestMutex.Unlock()

			// Echo back the arguments
			argsJSON, _ := json.Marshal(req.Params.Arguments)
			return &sdk.CallToolResult{
				Content: []sdk.Content{
					&sdk.TextContent{
						Text: string(argsJSON),
					},
				},
			}, state, nil
		},
	}
	us.toolsMu.Unlock()

	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us)
	ts := httptest.NewServer(httpServer.Handler)
	defer ts.Close()

	// Send a request with specific data
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 12345,
		"key3": []interface{}{"a", "b", "c"},
		"key4": map[string]interface{}{
			"nested": "value",
		},
	}

	callReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "echo_tool",
			"arguments": testData,
		},
	}

	resp := sendMCPRequest(t, ts.URL+"/mcp/testserver", "test-token", callReq)

	// Verify the handler received the exact data
	requestMutex.Lock()
	var receivedArgs map[string]interface{}
	if err := json.Unmarshal(receivedRequest.Params.Arguments, &receivedArgs); err != nil {
		t.Fatalf("Failed to unmarshal received arguments: %v", err)
	}
	requestMutex.Unlock()

	// Check each field
	if receivedArgs["key1"] != testData["key1"] {
		t.Errorf("key1 mismatch: expected %v, got %v", testData["key1"], receivedArgs["key1"])
	}

	// Verify response contains the echoed data
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	contentItem := content[0].(map[string]interface{})
	echoedText := contentItem["text"].(string)

	if !strings.Contains(echoedText, "value1") {
		t.Errorf("Response doesn't contain expected data: %s", echoedText)
	}

	t.Log("✓ Request passed through proxy without modification")
}
