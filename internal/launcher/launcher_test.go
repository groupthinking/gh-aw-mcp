package launcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

func TestHTTPConnection(t *testing.T) {
	// Create a mock HTTP server that handles initialize
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create test config with HTTP server
	jsonConfig := `{
		"mcpServers": {
			"safeinputs": {
				"type": "http",
				"url": "` + mockServer.URL + `",
				"headers": {
					"Authorization": "test-auth-secret"
				}
			}
		},
		"gateway": {
			"port": 3001,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`

	// Parse config via stdin
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := config.LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify HTTP server is loaded
	httpServer, ok := cfg.Servers["safeinputs"]
	if !ok {
		t.Fatal("HTTP server 'safeinputs' not found")
	}

	if httpServer.Type != "http" {
		t.Errorf("Expected type 'http', got '%s'", httpServer.Type)
	}

	if httpServer.URL != mockServer.URL {
		t.Errorf("Expected URL '%s', got '%s'", mockServer.URL, httpServer.URL)
	}

	if httpServer.Headers["Authorization"] != "test-auth-secret" {
		t.Errorf("Expected Authorization header 'test-auth-secret', got '%s'", httpServer.Headers["Authorization"])
	}

	// Test launcher
	ctx := context.Background()
	l := New(ctx, cfg)

	// Get connection
	conn, err := GetOrLaunch(l, "safeinputs")
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	if !conn.IsHTTP() {
		t.Error("Connection should be HTTP")
	}

	if conn.GetHTTPURL() != mockServer.URL {
		t.Errorf("Expected URL '%s', got '%s'", mockServer.URL, conn.GetHTTPURL())
	}

	if conn.GetHTTPHeaders()["Authorization"] != "test-auth-secret" {
		t.Errorf("Expected Authorization header 'test-auth-secret', got '%s'", conn.GetHTTPHeaders()["Authorization"])
	}
}

func TestHTTPConnectionWithVariableExpansion(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_AUTH_TOKEN", "secret-token-value")
	defer os.Unsetenv("TEST_AUTH_TOKEN")

	// Create a mock HTTP server that handles initialize
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create test config with variable expansion
	jsonConfig := `{
		"mcpServers": {
			"safeinputs": {
				"type": "http",
				"url": "` + mockServer.URL + `",
				"headers": {
					"Authorization": "${TEST_AUTH_TOKEN}"
				}
			}
		},
		"gateway": {
			"port": 3001,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`

	// Parse config via stdin
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := config.LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify HTTP server is loaded with expanded variable
	httpServer, ok := cfg.Servers["safeinputs"]
	if !ok {
		t.Fatal("HTTP server 'safeinputs' not found")
	}

	if httpServer.Headers["Authorization"] != "secret-token-value" {
		t.Errorf("Expected Authorization header 'secret-token-value' (expanded), got '%s'", httpServer.Headers["Authorization"])
	}

	// Test launcher
	ctx := context.Background()
	l := New(ctx, cfg)

	// Get connection
	conn, err := GetOrLaunch(l, "safeinputs")
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	if conn.GetHTTPHeaders()["Authorization"] != "secret-token-value" {
		t.Errorf("Expected Authorization header 'secret-token-value', got '%s'", conn.GetHTTPHeaders()["Authorization"])
	}
}

func TestMixedHTTPAndStdioServers(t *testing.T) {
	// Create test config with both HTTP and stdio servers
	jsonConfig := `{
		"mcpServers": {
			"http-server": {
				"type": "http",
				"url": "http://example.com/mcp"
			},
			"stdio-server": {
				"type": "stdio",
				"container": "test/server:latest"
			}
		},
		"gateway": {
			"port": 3001,
			"domain": "localhost",
			"apiKey": "test-key"
		}
	}`

	// Parse config via stdin
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		w.Write([]byte(jsonConfig))
		w.Close()
	}()

	cfg, err := config.LoadFromStdin()
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify both servers are loaded
	if len(cfg.Servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(cfg.Servers))
	}

	// Check HTTP server
	httpServer, ok := cfg.Servers["http-server"]
	if !ok {
		t.Error("HTTP server not found")
	} else if httpServer.Type != "http" {
		t.Errorf("Expected HTTP server type 'http', got '%s'", httpServer.Type)
	}

	// Check stdio server
	stdioServer, ok := cfg.Servers["stdio-server"]
	if !ok {
		t.Error("stdio server not found")
	} else if stdioServer.Type != "stdio" {
		t.Errorf("Expected stdio server type 'stdio', got '%s'", stdioServer.Type)
	}
}

func TestSanitizeEnvForLogging(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "nil env map",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty env map",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "single env var with long value",
			input: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "ghs_1234567890abcdefghijklmnop",
			},
			expected: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "ghs_...",
			},
		},
		{
			name: "multiple env vars with various lengths",
			input: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "ghs_1234567890abcdefghijklmnop",
				"API_KEY":                      "key_abc123xyz",
				"SHORT":                        "abc",
			},
			expected: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "ghs_...",
				"API_KEY":                      "key_...",
				"SHORT":                        "...",
			},
		},
		{
			name: "env var with exactly 4 characters",
			input: map[string]string{
				"TEST": "1234",
			},
			expected: map[string]string{
				"TEST": "...",
			},
		},
		{
			name: "env var with 5 characters",
			input: map[string]string{
				"TEST": "12345",
			},
			expected: map[string]string{
				"TEST": "1234...",
			},
		},
		{
			name: "env var with empty value",
			input: map[string]string{
				"EMPTY": "",
			},
			expected: map[string]string{
				"EMPTY": "...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeEnvForLogging(tt.input)

			// Check if both are nil
			if tt.expected == nil && result == nil {
				return
			}

			// Check if one is nil
			if (tt.expected == nil) != (result == nil) {
				t.Errorf("Expected nil=%v, got nil=%v", tt.expected == nil, result == nil)
				return
			}

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d entries, got %d", len(tt.expected), len(result))
				return
			}

			// Check each entry
			for key, expectedValue := range tt.expected {
				actualValue, ok := result[key]
				if !ok {
					t.Errorf("Missing key %s in result", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("For key %s: expected %s, got %s", key, expectedValue, actualValue)
				}
			}
		})
	}
}
