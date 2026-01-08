package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLoadFromStdin_ValidJSON(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"test": {
				"type": "local",
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
				"type": "local",
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
				"command": "node"
			},
			"supported": {
				"type": "local",
				"command": "node"
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
				"type": "local",
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

	if err != nil {
		t.Fatalf("LoadFromStdin() failed: %v", err)
	}

	server, ok := cfg.Servers["direct"]
	if !ok {
		t.Fatal("Server 'direct' not found")
	}

	if server.Command != "node" {
		t.Errorf("Expected command 'node', got '%s'", server.Command)
	}

	if !contains(server.Args, "index.js") {
		t.Error("Args not preserved for direct command")
	}

	if server.Env["NODE_ENV"] != "production" {
		t.Error("Env vars not preserved for direct command")
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
				"command": "node",
				"args": ["server.js"],
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

	if server.Command != "node" {
		t.Errorf("Expected command 'node', got '%s'", server.Command)
	}

	if !contains(server.Args, "server.js") {
		t.Error("Args not preserved for stdio type")
	}

	if server.Env["NODE_ENV"] != "test" {
		t.Error("Env vars not preserved for stdio type")
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
				"command": "node",
				"args": ["server.js"]
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
				"command": "node",
				"args": ["server.js"]
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

	if server.Command != "node" {
		t.Errorf("Expected command 'node', got '%s'", server.Command)
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
				"command": "node",
				"args": ["server.js"]
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
			"stdio-direct": {
				"type": "stdio",
				"command": "node",
				"args": ["server.js"]
			},
			"stdio-container": {
				"type": "stdio",
				"container": "test/container:latest"
			},
			"local-legacy": {
				"type": "local",
				"command": "python",
				"args": ["server.py"]
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

	// Should load: stdio-direct, stdio-container, local-legacy (3 total)
	// Should skip: http-server (not implemented)
	if len(cfg.Servers) != 3 {
		t.Errorf("Expected 3 servers, got %d", len(cfg.Servers))
	}

	if _, ok := cfg.Servers["stdio-direct"]; !ok {
		t.Error("stdio-direct server not loaded")
	}

	if _, ok := cfg.Servers["stdio-container"]; !ok {
		t.Error("stdio-container server not loaded")
	}

	if _, ok := cfg.Servers["local-legacy"]; !ok {
		t.Error("local-legacy server not loaded")
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
