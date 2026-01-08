package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestLoadFromStdin_ValidJSON(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"test": {
				"type": "stdio",
				"container": "test/container:latest",
				"entrypointArgs": ["arg1", "arg2"],
				"env": {
					"TEST_VAR": "value",
					"PASSTHROUGH_VAR": ""
				}
			}
		}
	}`

	// Mock stdin
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadFromStdin() returned nil config")
	}

	if len(cfg.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(cfg.Servers))
	}

	server, ok := cfg.Servers["test"]
	if !ok {
		t.Fatal("Server 'test' not found in config")
	}

	if server.Command != "docker" {
		t.Errorf("Expected command 'docker', got '%s'", server.Command)
	}

	// Check that standard Docker env vars are included
	hasNoColor := false
	hasTerm := false
	hasPythonUnbuffered := false
	hasTestVar := false
	hasPassthrough := false

	for i := 0; i < len(server.Args); i++ {
		arg := server.Args[i]
		if arg == "-e" && i+1 < len(server.Args) {
			nextArg := server.Args[i+1]
			switch nextArg {
			case "NO_COLOR=1":
				hasNoColor = true
			case "TERM=dumb":
				hasTerm = true
			case "PYTHONUNBUFFERED=1":
				hasPythonUnbuffered = true
			case "TEST_VAR=value":
				hasTestVar = true
			case "PASSTHROUGH_VAR":
				hasPassthrough = true
			}
		}
	}

	if !hasNoColor {
		t.Error("Standard env var NO_COLOR=1 not found")
	}
	if !hasTerm {
		t.Error("Standard env var TERM=dumb not found")
	}
	if !hasPythonUnbuffered {
		t.Error("Standard env var PYTHONUNBUFFERED=1 not found")
	}
	if !hasTestVar {
		t.Error("Custom env var TEST_VAR=value not found")
	}
	if !hasPassthrough {
		t.Error("Passthrough env var PASSTHROUGH_VAR not found")
	}

	// Check that container name is in args
	if !contains(server.Args, "test/container:latest") {
		t.Error("Container name not found in args")
	}

	// Check that entrypoint args are included
	if !contains(server.Args, "arg1") || !contains(server.Args, "arg2") {
		t.Error("Entrypoint args not found")
	}
}

func TestLoadFromStdin_WithGateway(t *testing.T) {
	port := 8080
	jsonConfig := `{
		"mcpServers": {
			"test": {
				"type": "stdio",
				"container": "test/container:latest"
			}
		},
		"gateway": {
			"port": 8080,
			"apiKey": "test-key"
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	_, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	// Gateway should be parsed but not affect server config
	var stdinCfg StdinConfig
	json.Unmarshal([]byte(jsonConfig), &stdinCfg)

	if stdinCfg.Gateway == nil {
		t.Error("Gateway not parsed")
	}
	if stdinCfg.Gateway.Port == nil || *stdinCfg.Gateway.Port != port {
		t.Error("Gateway port not correct")
	}
	if stdinCfg.Gateway.APIKey != "test-key" {
		t.Error("Gateway API key not correct")
	}
}

func TestLoadFromStdin_UnsupportedType(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"unsupported": {
				"type": "remote",
				"container": "test/container:latest"
			},
			"supported": {
				"type": "stdio",
				"container": "test/server:latest"
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	// Should fail validation for unsupported type
	if err == nil {
		t.Fatal("Expected error for unsupported type 'remote'")
	}

	// Error should mention validation issue
	if !strings.Contains(err.Error(), "remote") && !strings.Contains(err.Error(), "required") {
		t.Errorf("Expected validation error, got: %v", err)
	}

	// Config should be nil on validation error
	if cfg != nil {
		t.Error("Config should be nil when validation fails")
	}
}

func TestLoadFromStdin_DirectCommand(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"direct": {
				"type": "stdio",
				"command": "node",
				"args": ["index.js"],
				"env": {
					"NODE_ENV": "production"
				}
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	// Command field is no longer supported - should cause validation error
	if err == nil {
		t.Fatal("Expected error for deprecated 'command' field, got nil")
	}

	if !strings.Contains(err.Error(), "command") && !strings.Contains(err.Error(), "container") {
		t.Errorf("Expected validation error about command/container field, got: %v", err)
	}

	// Config should be nil on validation error
	if cfg != nil {
		t.Error("Config should be nil when validation fails")
	}
}

func TestLoadFromStdin_InvalidJSON(t *testing.T) {
	jsonConfig := `{invalid json}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	_, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "parse JSON") {
		t.Errorf("Expected 'parse JSON' error, got: %v", err)
	}
}

func TestLoadFromStdin_StdioType(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"stdio-server": {
				"type": "stdio",
				"container": "test/server:latest",
				"entrypointArgs": ["server.js"],
				"env": {
					"NODE_ENV": "test"
				}
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	if len(cfg.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(cfg.Servers))
	}

	server, ok := cfg.Servers["stdio-server"]
	if !ok {
		t.Fatal("Server 'stdio-server' not found")
	}

	if server.Command != "docker" {
		t.Errorf("Expected command 'docker', got '%s'", server.Command)
	}

	if !contains(server.Args, "test/server:latest") {
		t.Error("Container not found in args")
	}

	if !contains(server.Args, "server.js") {
		t.Error("Entrypoint args not preserved for stdio type")
	}

	// Check env vars
	hasNodeEnv := false
	for i := 0; i < len(server.Args); i++ {
		if server.Args[i] == "-e" && i+1 < len(server.Args) {
			if server.Args[i+1] == "NODE_ENV=test" {
				hasNodeEnv = true
			}
		}
	}

	if !hasNodeEnv {
		t.Error("Env var NODE_ENV=test not found")
	}
}

func TestLoadFromStdin_HttpType(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"http-server": {
				"type": "http",
				"url": "https://example.com/mcp"
			},
			"stdio-server": {
				"type": "stdio",
				"container": "test/server:latest",
				"entrypointArgs": ["server.js"]
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	// HTTP type should be skipped (not yet implemented)
	if len(cfg.Servers) != 1 {
		t.Errorf("Expected 1 server (http skipped), got %d", len(cfg.Servers))
	}

	if _, ok := cfg.Servers["http-server"]; ok {
		t.Error("HTTP server should not be loaded (not yet implemented)")
	}

	if _, ok := cfg.Servers["stdio-server"]; !ok {
		t.Error("stdio server should be loaded")
	}
}

func TestLoadFromStdin_LocalTypeBackwardCompatibility(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"legacy": {
				"type": "local",
				"container": "test/server:latest",
				"entrypointArgs": ["server.js"]
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	// "local" type should work as alias for "stdio"
	if len(cfg.Servers) != 1 {
		t.Errorf("Expected 1 server (local treated as stdio), got %d", len(cfg.Servers))
	}

	server, ok := cfg.Servers["legacy"]
	if !ok {
		t.Fatal("Server 'legacy' with type 'local' not loaded")
	}

	if server.Command != "docker" {
		t.Errorf("Expected command 'docker', got '%s'", server.Command)
	}

	if !contains(server.Args, "test/server:latest") {
		t.Error("Container not found in args")
	}
}

func TestLoadFromStdin_GatewayWithAllFields(t *testing.T) {
	port := 8080
	startupTimeout := 30
	toolTimeout := 60
	jsonConfig := `{
		"mcpServers": {
			"test": {
				"type": "stdio",
				"container": "test/server:latest",
				"entrypointArgs": ["server.js"]
			}
		},
		"gateway": {
			"port": 8080,
			"apiKey": "test-key-123",
			"domain": "example.com",
			"startupTimeout": 30,
			"toolTimeout": 60
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	_, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	// Parse gateway config to verify all fields
	var stdinCfg StdinConfig
	json.Unmarshal([]byte(jsonConfig), &stdinCfg)

	if stdinCfg.Gateway == nil {
		t.Fatal("Gateway not parsed")
	}

	if stdinCfg.Gateway.Port == nil || *stdinCfg.Gateway.Port != port {
		t.Errorf("Expected gateway port %d, got %v", port, stdinCfg.Gateway.Port)
	}

	if stdinCfg.Gateway.APIKey != "test-key-123" {
		t.Errorf("Expected gateway API key 'test-key-123', got '%s'", stdinCfg.Gateway.APIKey)
	}

	if stdinCfg.Gateway.Domain != "example.com" {
		t.Errorf("Expected gateway domain 'example.com', got '%s'", stdinCfg.Gateway.Domain)
	}

	if stdinCfg.Gateway.StartupTimeout == nil || *stdinCfg.Gateway.StartupTimeout != startupTimeout {
		t.Errorf("Expected gateway startupTimeout %d, got %v", startupTimeout, stdinCfg.Gateway.StartupTimeout)
	}

	if stdinCfg.Gateway.ToolTimeout == nil || *stdinCfg.Gateway.ToolTimeout != toolTimeout {
		t.Errorf("Expected gateway toolTimeout %d, got %v", toolTimeout, stdinCfg.Gateway.ToolTimeout)
	}
}

func TestLoadFromStdin_ServerWithURL(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"http-server": {
				"type": "http",
				"url": "https://example.com/mcp"
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	_, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	// Parse to verify URL field
	var stdinCfg StdinConfig
	json.Unmarshal([]byte(jsonConfig), &stdinCfg)

	server, ok := stdinCfg.MCPServers["http-server"]
	if !ok {
		t.Fatal("Server 'http-server' not parsed")
	}

	if server.URL != "https://example.com/mcp" {
		t.Errorf("Expected URL 'https://example.com/mcp', got '%s'", server.URL)
	}
}

func TestLoadFromStdin_MixedServerTypes(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"stdio-container-1": {
				"type": "stdio",
				"container": "test/server:latest"
			},
			"stdio-container-2": {
				"type": "stdio",
				"container": "test/another:v1"
			},
			"local-container": {
				"type": "local",
				"container": "test/legacy:latest"
			},
			"http-server": {
				"type": "http",
				"url": "https://example.com/mcp"
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	// Should load: stdio-container-1, stdio-container-2, local-container (3 total)
	// Should skip: http-server (not implemented)
	if len(cfg.Servers) != 3 {
		t.Errorf("Expected 3 servers, got %d", len(cfg.Servers))
	}

	if _, ok := cfg.Servers["stdio-container-1"]; !ok {
		t.Error("stdio-container-1 server not loaded")
	}

	if _, ok := cfg.Servers["stdio-container-2"]; !ok {
		t.Error("stdio-container-2 server not loaded")
	}

	if _, ok := cfg.Servers["local-container"]; !ok {
		t.Error("local-container server not loaded")
	}

	if _, ok := cfg.Servers["http-server"]; ok {
		t.Error("http-server should be skipped")
	}
}

func TestLoadFromStdin_ContainerWithStdioType(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"docker-stdio": {
				"type": "stdio",
				"container": "test/container:latest",
				"entrypointArgs": ["--verbose"],
				"env": {
					"DEBUG": "true",
					"TOKEN": ""
				}
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	server, ok := cfg.Servers["docker-stdio"]
	if !ok {
		t.Fatal("Server 'docker-stdio' not found")
	}

	// Should be converted to docker command
	if server.Command != "docker" {
		t.Errorf("Expected command 'docker', got '%s'", server.Command)
	}

	// Check container name is in args
	if !contains(server.Args, "test/container:latest") {
		t.Error("Container name not found in args")
	}

	// Check entrypoint args
	if !contains(server.Args, "--verbose") {
		t.Error("Entrypoint args not found")
	}

	// Check env vars (both explicit and passthrough)
	hasDebug := false
	hasToken := false
	for i := 0; i < len(server.Args); i++ {
		if server.Args[i] == "-e" && i+1 < len(server.Args) {
			switch server.Args[i+1] {
			case "DEBUG=true":
				hasDebug = true
			case "TOKEN":
				hasToken = true
			}
		}
	}

	if !hasDebug {
		t.Error("Explicit env var DEBUG=true not found")
	}
	if !hasToken {
		t.Error("Passthrough env var TOKEN not found")
	}
}

// Helper function to check if slice contains item
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestLoadFromStdin_WithEntrypoint(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"custom": {
				"type": "stdio",
				"container": "test/container:latest",
				"entrypoint": "/custom/entrypoint.sh",
				"entrypointArgs": ["--verbose"]
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	server, ok := cfg.Servers["custom"]
	if !ok {
		t.Fatal("Server 'custom' not found")
	}

	// Check that --entrypoint flag is present
	hasEntrypoint := false
	for i := 0; i < len(server.Args); i++ {
		if server.Args[i] == "--entrypoint" && i+1 < len(server.Args) {
			if server.Args[i+1] == "/custom/entrypoint.sh" {
				hasEntrypoint = true
			}
		}
	}

	if !hasEntrypoint {
		t.Error("Entrypoint flag not found in Docker args")
	}

	// Check that entrypoint args are present
	if !contains(server.Args, "--verbose") {
		t.Error("Entrypoint args not found")
	}
}

func TestLoadFromStdin_WithMounts(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"mounted": {
				"type": "stdio",
				"container": "test/container:latest",
				"mounts": [
					"/host/path:/container/path:ro",
					"/host/data:/app/data:rw"
				]
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	server, ok := cfg.Servers["mounted"]
	if !ok {
		t.Fatal("Server 'mounted' not found")
	}

	// Check that volume mount flags are present
	mountCount := 0
	for i := 0; i < len(server.Args); i++ {
		if server.Args[i] == "-v" && i+1 < len(server.Args) {
			nextArg := server.Args[i+1]
			if nextArg == "/host/path:/container/path:ro" || nextArg == "/host/data:/app/data:rw" {
				mountCount++
			}
		}
	}

	if mountCount != 2 {
		t.Errorf("Expected 2 volume mounts, found %d", mountCount)
	}
}

func TestLoadFromStdin_WithAllNewFields(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"comprehensive": {
				"type": "stdio",
				"container": "test/container:latest",
				"entrypoint": "/bin/bash",
				"entrypointArgs": ["-c", "echo test"],
				"mounts": ["/tmp:/data:rw"],
				"env": {
					"DEBUG": "true"
				}
			}
		}
	}`

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	server, ok := cfg.Servers["comprehensive"]
	if !ok {
		t.Fatal("Server 'comprehensive' not found")
	}

	// Verify command is docker
	if server.Command != "docker" {
		t.Errorf("Expected command 'docker', got '%s'", server.Command)
	}

	// Check entrypoint
	hasEntrypoint := false
	for i := 0; i < len(server.Args)-1; i++ {
		if server.Args[i] == "--entrypoint" && server.Args[i+1] == "/bin/bash" {
			hasEntrypoint = true
			break
		}
	}
	if !hasEntrypoint {
		t.Error("Entrypoint not found in args")
	}

	// Check mounts
	hasMount := false
	for i := 0; i < len(server.Args)-1; i++ {
		if server.Args[i] == "-v" && server.Args[i+1] == "/tmp:/data:rw" {
			hasMount = true
			break
		}
	}
	if !hasMount {
		t.Error("Mount not found in args")
	}

	// Check env var
	hasDebug := false
	for i := 0; i < len(server.Args)-1; i++ {
		if server.Args[i] == "-e" && server.Args[i+1] == "DEBUG=true" {
			hasDebug = true
			break
		}
	}
	if !hasDebug {
		t.Error("Environment variable DEBUG=true not found")
	}

	// Check entrypoint args
	if !contains(server.Args, "-c") || !contains(server.Args, "echo test") {
		t.Error("Entrypoint args not found")
	}

	// Verify container name is present
	if !contains(server.Args, "test/container:latest") {
		t.Error("Container name not found")
	}
}

func TestLoadFromStdin_InvalidMountFormat(t *testing.T) {
	tests := []struct {
		name     string
		mounts   string
		errorMsg string
	}{
		{
			name:     "missing mode",
			mounts:   `["/host:/container"]`,
			errorMsg: "invalid mount format",
		},
		{
			name:     "invalid mode",
			mounts:   `["/host:/container:invalid"]`,
			errorMsg: "invalid mount mode",
		},
		{
			name:     "empty source",
			mounts:   `[":/container:ro"]`,
			errorMsg: "mount source cannot be empty",
		},
		{
			name:     "empty destination",
			mounts:   `["/host::ro"]`,
			errorMsg: "mount destination cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonConfig := fmt.Sprintf(`{
				"mcpServers": {
					"test": {
						"type": "stdio",
						"container": "test:latest",
						"mounts": %s
					}
				}
			}`, tt.mounts)

			r, w, _ := os.Pipe()
			oldStdin := os.Stdin
			os.Stdin = r
			go func() {
				w.Write([]byte(jsonConfig))
				w.Close()
			}()

			_, err := LoadFromStdin()
			os.Stdin = oldStdin

			if err == nil {
				t.Error("Expected error but got none")
			} else if !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
			}
		})
	}
}
