package server

import (
	"context"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

func TestCreateFilteredServer_ToolFiltering(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	if err != nil {
		t.Fatalf("NewUnified() failed: %v", err)
	}
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
	if err != nil {
		t.Fatalf("NewUnified() failed: %v", err)
	}
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
	if handler == nil {
		t.Fatal("GetToolHandler() returned nil for non-prefixed tool name")
	}

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
	if err != nil {
		t.Fatalf("NewUnified() failed: %v", err)
	}
	defer us.Close()

	// Create routed mode server
	httpServer := CreateHTTPServerForRoutedMode("127.0.0.1:8000", us, "")
	if httpServer == nil {
		t.Fatal("CreateHTTPServerForRoutedMode() returned nil")
	}

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
	if err != nil {
		t.Fatalf("NewUnified() failed: %v", err)
	}
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
	if err != nil {
		t.Fatalf("NewUnified() failed: %v", err)
	}
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
