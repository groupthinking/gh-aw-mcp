package sys

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSysServer(t *testing.T) {
	tests := []struct {
		name      string
		serverIDs []string
		wantCount int
	}{
		{
			name:      "empty server list",
			serverIDs: []string{},
			wantCount: 0,
		},
		{
			name:      "single server",
			serverIDs: []string{"github"},
			wantCount: 1,
		},
		{
			name:      "multiple servers",
			serverIDs: []string{"github", "slack", "jira"},
			wantCount: 3,
		},
		{
			name:      "nil server list",
			serverIDs: nil,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewSysServer(tt.serverIDs)

			require.NotNil(t, server, "NewSysServer should never return nil")
			assert.Equal(t, tt.wantCount, len(server.serverIDs), "Server count mismatch")

			if tt.serverIDs != nil && len(tt.serverIDs) > 0 {
				assert.Equal(t, tt.serverIDs, server.serverIDs, "Server IDs should match")
			}
		})
	}
}

func TestHandleRequest_ToolsList(t *testing.T) {
	server := NewSysServer([]string{"github", "slack"})

	result, err := server.HandleRequest("tools/list", nil)

	require.NoError(t, err, "tools/list should not return error")
	require.NotNil(t, result, "Result should not be nil")

	// Verify response structure
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok, "Result should be a map")

	tools, ok := resultMap["tools"].([]map[string]interface{})
	require.True(t, ok, "tools field should be an array of maps")
	assert.Equal(t, 2, len(tools), "Should have 2 tools")

	// Verify sys_init tool
	sysInitTool := tools[0]
	assert.Equal(t, "sys_init", sysInitTool["name"], "First tool should be sys_init")
	assert.Contains(t, sysInitTool["description"], "Initialize", "Description should mention Initialize")
	assert.NotNil(t, sysInitTool["inputSchema"], "Should have inputSchema")

	// Verify sys_list_servers tool
	listServersTool := tools[1]
	assert.Equal(t, "sys_list_servers", listServersTool["name"], "Second tool should be sys_list_servers")
	assert.Contains(t, listServersTool["description"], "List all", "Description should mention List all")
	assert.NotNil(t, listServersTool["inputSchema"], "Should have inputSchema")
}

func TestHandleRequest_ToolsCall_SysInit(t *testing.T) {
	serverIDs := []string{"github", "slack", "jira"}
	server := NewSysServer(serverIDs)

	params := json.RawMessage(`{
		"name": "sys_init",
		"arguments": {}
	}`)

	result, err := server.HandleRequest("tools/call", params)

	require.NoError(t, err, "sys_init should not return error")
	require.NotNil(t, result, "Result should not be nil")

	// Verify response structure
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok, "Result should be a map")

	content, ok := resultMap["content"].([]map[string]interface{})
	require.True(t, ok, "content field should be an array of maps")
	require.Equal(t, 1, len(content), "Should have 1 content item")

	contentItem := content[0]
	assert.Equal(t, "text", contentItem["type"], "Content type should be text")

	text, ok := contentItem["text"].(string)
	require.True(t, ok, "text field should be a string")
	assert.Contains(t, text, "MCPG initialized", "Text should mention initialization")
	assert.Contains(t, text, "github", "Text should mention github server")
	assert.Contains(t, text, "slack", "Text should mention slack server")
	assert.Contains(t, text, "jira", "Text should mention jira server")
}

func TestHandleRequest_ToolsCall_ListServers(t *testing.T) {
	tests := []struct {
		name      string
		serverIDs []string
		wantCount int
	}{
		{
			name:      "empty servers",
			serverIDs: []string{},
			wantCount: 0,
		},
		{
			name:      "single server",
			serverIDs: []string{"github"},
			wantCount: 1,
		},
		{
			name:      "multiple servers",
			serverIDs: []string{"github", "slack", "jira"},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewSysServer(tt.serverIDs)

			params := json.RawMessage(`{
				"name": "sys_list_servers",
				"arguments": {}
			}`)

			result, err := server.HandleRequest("tools/call", params)

			require.NoError(t, err, "sys_list_servers should not return error")
			require.NotNil(t, result, "Result should not be nil")

			// Verify response structure
			resultMap, ok := result.(map[string]interface{})
			require.True(t, ok, "Result should be a map")

			content, ok := resultMap["content"].([]map[string]interface{})
			require.True(t, ok, "content field should be an array of maps")
			require.Equal(t, 1, len(content), "Should have 1 content item")

			contentItem := content[0]
			assert.Equal(t, "text", contentItem["type"], "Content type should be text")

			text, ok := contentItem["text"].(string)
			require.True(t, ok, "text field should be a string")
			assert.Contains(t, text, "Configured MCP Servers", "Text should mention configured servers")

			// Verify each server ID is listed
			for i, id := range tt.serverIDs {
				expectedLine := (i + 1)
				assert.Contains(t, text, id, "Text should contain server ID: %s", id)
				// Verify numbering format: "1. github"
				assert.Contains(t, text, id, "Text should contain numbered server: %d. %s", expectedLine, id)
			}
		})
	}
}

func TestHandleRequest_ToolsCall_InvalidJSON(t *testing.T) {
	server := NewSysServer([]string{"github"})

	tests := []struct {
		name   string
		params json.RawMessage
	}{
		{
			name:   "invalid JSON",
			params: json.RawMessage(`{invalid json`),
		},
		{
			name:   "missing name field",
			params: json.RawMessage(`{"arguments": {}}`),
		},
		{
			name:   "null params",
			params: json.RawMessage(`null`),
		},
		{
			name:   "empty object",
			params: json.RawMessage(`{}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.HandleRequest("tools/call", tt.params)

			assert.Error(t, err, "Should return error for invalid params")
			assert.Nil(t, result, "Result should be nil on error")
			assert.Contains(t, err.Error(), "invalid params", "Error should mention invalid params")
		})
	}
}

func TestHandleRequest_ToolsCall_UnknownTool(t *testing.T) {
	server := NewSysServer([]string{"github"})

	tests := []struct {
		name     string
		toolName string
	}{
		{
			name:     "unknown tool",
			toolName: "unknown_tool",
		},
		{
			name:     "empty tool name",
			toolName: "",
		},
		{
			name:     "misspelled tool",
			toolName: "sys_initialize",
		},
		{
			name:     "case sensitive tool name",
			toolName: "SYS_INIT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := json.RawMessage(`{
				"name": "` + tt.toolName + `",
				"arguments": {}
			}`)

			result, err := server.HandleRequest("tools/call", params)

			assert.Error(t, err, "Should return error for unknown tool")
			assert.Nil(t, result, "Result should be nil on error")
			assert.Contains(t, err.Error(), "unknown tool", "Error should mention unknown tool")
		})
	}
}

func TestHandleRequest_UnsupportedMethod(t *testing.T) {
	server := NewSysServer([]string{"github"})

	tests := []struct {
		name   string
		method string
	}{
		{
			name:   "resources/list",
			method: "resources/list",
		},
		{
			name:   "prompts/list",
			method: "prompts/list",
		},
		{
			name:   "empty method",
			method: "",
		},
		{
			name:   "invalid method",
			method: "invalid/method",
		},
		{
			name:   "case sensitive method",
			method: "Tools/List",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.HandleRequest(tt.method, nil)

			assert.Error(t, err, "Should return error for unsupported method")
			assert.Nil(t, result, "Result should be nil on error")
			assert.Contains(t, err.Error(), "unsupported method", "Error should mention unsupported method")
			assert.Contains(t, err.Error(), tt.method, "Error should include the method name")
		})
	}
}

func TestHandleRequest_ToolsCall_WithArguments(t *testing.T) {
	server := NewSysServer([]string{"github"})

	// Test that arguments are accepted even if not used
	params := json.RawMessage(`{
		"name": "sys_init",
		"arguments": {
			"unused": "value",
			"another": 123
		}
	}`)

	result, err := server.HandleRequest("tools/call", params)

	require.NoError(t, err, "Should not error with extra arguments")
	require.NotNil(t, result, "Result should not be nil")
}

func TestListTools_ResponseStructure(t *testing.T) {
	server := NewSysServer([]string{"github"})

	result, err := server.listTools()

	require.NoError(t, err, "listTools should not error")
	require.NotNil(t, result, "Result should not be nil")

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok, "Result should be a map")

	tools, ok := resultMap["tools"].([]map[string]interface{})
	require.True(t, ok, "tools field should be an array")
	require.Equal(t, 2, len(tools), "Should have exactly 2 tools")

	// Verify each tool has required fields
	for i, tool := range tools {
		assert.NotEmpty(t, tool["name"], "Tool %d should have name", i)
		assert.NotEmpty(t, tool["description"], "Tool %d should have description", i)
		assert.NotNil(t, tool["inputSchema"], "Tool %d should have inputSchema", i)

		// Verify inputSchema structure
		schema, ok := tool["inputSchema"].(map[string]interface{})
		require.True(t, ok, "Tool %d inputSchema should be a map", i)
		assert.Equal(t, "object", schema["type"], "Tool %d inputSchema type should be object", i)
		assert.NotNil(t, schema["properties"], "Tool %d inputSchema should have properties", i)
	}
}

func TestCallTool_AllTools(t *testing.T) {
	serverIDs := []string{"github", "slack"}
	server := NewSysServer(serverIDs)

	tests := []struct {
		name         string
		toolName     string
		args         map[string]interface{}
		expectError  bool
		validateFunc func(t *testing.T, result interface{})
	}{
		{
			name:        "sys_init with empty args",
			toolName:    "sys_init",
			args:        map[string]interface{}{},
			expectError: false,
			validateFunc: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				content := resultMap["content"].([]map[string]interface{})
				text := content[0]["text"].(string)
				assert.Contains(t, text, "MCPG initialized")
				assert.Contains(t, text, "github")
				assert.Contains(t, text, "slack")
			},
		},
		{
			name:        "sys_init with ignored args",
			toolName:    "sys_init",
			args:        map[string]interface{}{"ignored": "value"},
			expectError: false,
			validateFunc: func(t *testing.T, result interface{}) {
				assert.NotNil(t, result)
			},
		},
		{
			name:        "sys_list_servers with empty args",
			toolName:    "sys_list_servers",
			args:        map[string]interface{}{},
			expectError: false,
			validateFunc: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				content := resultMap["content"].([]map[string]interface{})
				text := content[0]["text"].(string)
				assert.Contains(t, text, "Configured MCP Servers")
				assert.Contains(t, text, "1. github")
				assert.Contains(t, text, "2. slack")
			},
		},
		{
			name:        "sys_list_servers with nil args",
			toolName:    "sys_list_servers",
			args:        nil,
			expectError: false,
			validateFunc: func(t *testing.T, result interface{}) {
				assert.NotNil(t, result)
			},
		},
		{
			name:        "unknown tool",
			toolName:    "nonexistent",
			args:        map[string]interface{}{},
			expectError: true,
			validateFunc: func(t *testing.T, result interface{}) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.callTool(tt.toolName, tt.args)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unknown tool")
			} else {
				require.NoError(t, err)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, result)
			}
		})
	}
}

func TestSysInit_ServerListFormatting(t *testing.T) {
	tests := []struct {
		name      string
		serverIDs []string
	}{
		{
			name:      "empty servers",
			serverIDs: []string{},
		},
		{
			name:      "single server",
			serverIDs: []string{"github"},
		},
		{
			name:      "many servers",
			serverIDs: []string{"github", "slack", "jira", "notion", "linear"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewSysServer(tt.serverIDs)
			result, err := server.sysInit()

			require.NoError(t, err)
			require.NotNil(t, result)

			resultMap := result.(map[string]interface{})
			content := resultMap["content"].([]map[string]interface{})
			text := content[0]["text"].(string)

			assert.Contains(t, text, "MCPG initialized")
			assert.Contains(t, text, "Available servers")
		})
	}
}

func TestListServers_Formatting(t *testing.T) {
	tests := []struct {
		name      string
		serverIDs []string
		expected  []string
	}{
		{
			name:      "empty list",
			serverIDs: []string{},
			expected:  []string{"Configured MCP Servers:"},
		},
		{
			name:      "single server",
			serverIDs: []string{"github"},
			expected:  []string{"Configured MCP Servers:", "1. github"},
		},
		{
			name:      "multiple servers",
			serverIDs: []string{"github", "slack", "jira"},
			expected:  []string{"Configured MCP Servers:", "1. github", "2. slack", "3. jira"},
		},
		{
			name:      "servers with special characters",
			serverIDs: []string{"server-1", "server_2", "server.3"},
			expected:  []string{"1. server-1", "2. server_2", "3. server.3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewSysServer(tt.serverIDs)
			result, err := server.listServers()

			require.NoError(t, err)
			require.NotNil(t, result)

			resultMap := result.(map[string]interface{})
			content := resultMap["content"].([]map[string]interface{})
			require.Equal(t, 1, len(content))

			contentItem := content[0]
			assert.Equal(t, "text", contentItem["type"])

			text := contentItem["text"].(string)

			// Verify all expected strings are present
			for _, expectedStr := range tt.expected {
				assert.Contains(t, text, expectedStr, "Output should contain: %s", expectedStr)
			}
		})
	}
}

func TestHandleRequest_NilParams(t *testing.T) {
	server := NewSysServer([]string{"github"})

	// tools/list with nil params should work
	result, err := server.HandleRequest("tools/list", nil)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// tools/call with nil params should fail
	result, err = server.HandleRequest("tools/call", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestHandleRequest_EmptyParams(t *testing.T) {
	server := NewSysServer([]string{"github"})

	// tools/call with empty JSON object should fail (missing name field)
	params := json.RawMessage(`{}`)
	result, err := server.HandleRequest("tools/call", params)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSysServer_MultipleSequentialCalls(t *testing.T) {
	server := NewSysServer([]string{"github", "slack"})

	// Call tools/list multiple times
	for i := 0; i < 3; i++ {
		result, err := server.HandleRequest("tools/list", nil)
		require.NoError(t, err, "Call %d should not error", i)
		assert.NotNil(t, result, "Call %d should return result", i)
	}

	// Call sys_init multiple times
	params := json.RawMessage(`{"name": "sys_init", "arguments": {}}`)
	for i := 0; i < 3; i++ {
		result, err := server.HandleRequest("tools/call", params)
		require.NoError(t, err, "Call %d should not error", i)
		assert.NotNil(t, result, "Call %d should return result", i)
	}

	// Call sys_list_servers multiple times
	params = json.RawMessage(`{"name": "sys_list_servers", "arguments": {}}`)
	for i := 0; i < 3; i++ {
		result, err := server.HandleRequest("tools/call", params)
		require.NoError(t, err, "Call %d should not error", i)
		assert.NotNil(t, result, "Call %d should return result", i)
	}
}
