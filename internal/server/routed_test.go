package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

// TestCloseEndpoint_Success tests the successful shutdown flow
func TestCloseEndpoint_Success(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
			"fetch":  {Command: "docker", Args: []string{}},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Enable test mode to prevent os.Exit()
	us.SetTestMode(true)

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// Create test request
	req := httptest.NewRequest(http.MethodPost, "/close", nil)
	w := httptest.NewRecorder()

	// Send request
	httpServer.Handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "Close endpoint should return 200 OK")

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&response), "Failed to decode response")

	// Check response fields
	assert.Equal(t, "closed", response["status"], "Expected status 'closed'")
	assert.Equal(t, "Gateway shutdown initiated", response["message"], "Expected shutdown message")

	// Should report 2 servers terminated
	serversTerminated, ok := response["serversTerminated"].(float64)
	require.True(t, ok, "serversTerminated should be a number")
	assert.InDelta(t, 2.0, serversTerminated, 0.01, "Expected 2 servers terminated")

	// Verify server is marked as shutdown
	assert.True(t, us.IsShutdown(), "Expected server to be marked as shutdown")
}

// TestCloseEndpoint_Idempotency tests that subsequent calls return 410 Gone
func TestCloseEndpoint_Idempotency(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Enable test mode to prevent os.Exit()
	us.SetTestMode(true)

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// First call
	req1 := httptest.NewRequest(http.MethodPost, "/close", nil)
	w1 := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("First call: expected status 200, got %d", w1.Code)
	}

	// Second call (should be idempotent)
	req2 := httptest.NewRequest(http.MethodPost, "/close", nil)
	w2 := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w2, req2)

	// Should return 410 Gone
	if w2.Code != http.StatusGone {
		t.Errorf("Second call: expected status 410 (Gone), got %d", w2.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w2.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if errMsg, ok := response["error"].(string); !ok || errMsg != "Gateway has already been closed" {
		t.Errorf("Expected error message 'Gateway has already been closed', got %v", response["error"])
	}
}

// TestCloseEndpoint_MethodNotAllowed tests that non-POST requests are rejected
func TestCloseEndpoint_MethodNotAllowed(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// Try GET request
	req := httptest.NewRequest(http.MethodGet, "/close", nil)
	w := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 (Method Not Allowed), got %d", w.Code)
	}
}

// TestCloseEndpoint_RequiresAuth tests that authentication is enforced when configured
func TestCloseEndpoint_RequiresAuth(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Enable test mode to prevent os.Exit()
	us.SetTestMode(true)

	apiKey := "test-secret-key"

	// Create routed mode server with API key
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, apiKey)

	// Request without auth header
	req := httptest.NewRequest(http.MethodPost, "/close", nil)
	w := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 (Unauthorized), got %d", w.Code)
	}

	// Request with correct auth header
	req2 := httptest.NewRequest(http.MethodPost, "/close", nil)
	req2.Header.Set("Authorization", apiKey)
	w2 := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 with correct auth, got %d", w2.Code)
	}
}

func TestCreateFilteredServer_ToolFiltering(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Add test tools - Handler is not tested directly, just use nil
	us.toolsMu.Lock()
	us.tools["github___issue_read"] = &ToolInfo{
		Name:        "github___issue_read",
		Description: "Read an issue",
		BackendID:   "github",
		Handler:     nil,
	}
	us.tools["github___repo_list"] = &ToolInfo{
		Name:        "github___repo_list",
		Description: "List repos",
		BackendID:   "github",
		Handler:     nil,
	}
	us.tools["fetch___get"] = &ToolInfo{
		Name:        "fetch___get",
		Description: "Fetch URL",
		BackendID:   "fetch",
		Handler:     nil,
	}
	us.toolsMu.Unlock()

	// Create filtered server for github backend
	filteredServer := createFilteredServer(us, "github")

	// We can't easily inspect the filtered server's tools without SDK internals,
	// but we can verify GetToolsForBackend returns correct filtered list
	tools := us.GetToolsForBackend("github")
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools for github backend, got %d", len(tools))
	}

	// Verify tool names have prefix stripped
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	if !toolNames["issue_read"] {
		t.Error("Expected tool 'issue_read' not found")
	}
	if !toolNames["repo_list"] {
		t.Error("Expected tool 'repo_list' not found")
	}
	if toolNames["get"] {
		t.Error("Tool 'get' from fetch backend should not be in github filtered server")
	}

	_ = filteredServer // Use variable to avoid unused error
}

func TestGetToolHandler(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Create a mock handler with correct signature
	mockHandler := func(ctx context.Context, req *sdk.CallToolRequest, state interface{}) (*sdk.CallToolResult, interface{}, error) {
		return &sdk.CallToolResult{IsError: false}, state, nil
	}

	// Add test tool with handler
	us.toolsMu.Lock()
	us.tools["github___test_tool"] = &ToolInfo{
		Name:        "github___test_tool",
		Description: "Test tool",
		BackendID:   "github",
		Handler:     mockHandler,
	}
	us.toolsMu.Unlock()

	// Test retrieval with non-prefixed name (routed mode format)
	handler := us.GetToolHandler("github", "test_tool")
	require.NotNil(t, handler, "GetToolHandler() returned nil for non-prefixed tool name")

	// Test non-existent tool
	handler = us.GetToolHandler("github", "nonexistent_tool")
	if handler != nil {
		t.Error("GetToolHandler() should return nil for non-existent tool")
	}

	// Test wrong backend (test_tool belongs to github, not fetch)
	handler = us.GetToolHandler("fetch", "test_tool")
	if handler != nil {
		t.Error("GetToolHandler() should return nil when backend doesn't match")
	}
}

func TestCreateHTTPServerForRoutedMode_ServerIDs(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
			"fetch":  {Command: "docker", Args: []string{}},
		},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:8000", us, "")
	require.NotNil(t, httpServer, "CreateHTTPServerForRoutedMode() returned nil")

	// Verify server IDs are correctly set up
	serverIDs := us.GetServerIDs()
	if len(serverIDs) != 2 {
		t.Errorf("Expected 2 server IDs, got %d", len(serverIDs))
	}

	expectedIDs := map[string]bool{"github": true, "fetch": true}
	for _, id := range serverIDs {
		if !expectedIDs[id] {
			t.Errorf("Unexpected server ID: %s", id)
		}
	}
}

func TestRoutedMode_SysToolsBackend_DIFCDisabled(t *testing.T) {
	// When DIFC is disabled (default), sys tools should NOT be registered
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
		},
		EnableDIFC: false, // Explicitly disable DIFC (this is the default)
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Verify sys tools are NOT registered when DIFC is disabled
	sysTools := us.GetToolsForBackend("sys")
	if len(sysTools) != 0 {
		t.Errorf("Expected no sys tools when DIFC is disabled, got %d", len(sysTools))
	}
}

func TestRoutedMode_SysToolsBackend_DIFCEnabled(t *testing.T) {
	// When DIFC is enabled, sys tools SHOULD be registered
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
		},
		EnableDIFC: true, // Enable DIFC
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Verify sys tools exist when DIFC is enabled
	sysTools := us.GetToolsForBackend("sys")
	if len(sysTools) == 0 {
		t.Error("Expected sys tools to be registered when DIFC is enabled, got none")
	}

	// Check for expected sys tools
	toolNames := make(map[string]bool)
	for _, tool := range sysTools {
		toolNames[tool.Name] = true
	}

	expectedSysTools := []string{"init", "list_servers"}
	for _, expectedTool := range expectedSysTools {
		if !toolNames[expectedTool] {
			t.Errorf("Expected sys tool '%s' not found", expectedTool)
		}
	}

	// Verify sys tools have correct backend ID
	for _, tool := range sysTools {
		if tool.BackendID != "sys" {
			t.Errorf("Expected BackendID 'sys', got '%s'", tool.BackendID)
		}
	}
}

func TestRoutedMode_SysRouteNotExposed_DIFCDisabled(t *testing.T) {
	// When DIFC is disabled (default), /mcp/sys route should NOT be registered
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
		},
		EnableDIFC: false, // Explicitly disable DIFC (this is the default)
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// Try to access /mcp/sys route - should get 404
	req := httptest.NewRequest(http.MethodGet, "/mcp/sys", nil)
	req.Header.Set("Authorization", "test-session")
	w := httptest.NewRecorder()

	httpServer.Handler.ServeHTTP(w, req)

	// Should return 404 because the route is not registered
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for /mcp/sys when DIFC is disabled, got %d", w.Code)
	}
}

func TestRoutedMode_SysRouteExposed_DIFCEnabled(t *testing.T) {
	// When DIFC is enabled, /mcp/sys route SHOULD be registered
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"github": {Command: "docker", Args: []string{}},
		},
		EnableDIFC: true, // Enable DIFC
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "NewUnified() failed")
	defer us.Close()

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:0", us, "")

	// Try to access /mcp/sys route - should NOT get 404
	req := httptest.NewRequest(http.MethodGet, "/mcp/sys", nil)
	req.Header.Set("Authorization", "test-session")
	w := httptest.NewRecorder()

	httpServer.Handler.ServeHTTP(w, req)

	// Should NOT return 404 because the route should be registered
	if w.Code == http.StatusNotFound {
		t.Errorf("Expected /mcp/sys route to be registered when DIFC is enabled, but got 404")
	}
}
