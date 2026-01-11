package launcher

import (
	"context"
	"os"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

func TestHTTPConnection(t *testing.T) {
	// Create test config with HTTP server
	jsonConfig := `{
		"mcpServers": {
			"safeinputs": {
				"type": "http",
				"url": "http://host.docker.internal:3000/",
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

	if httpServer.URL != "http://host.docker.internal:3000/" {
		t.Errorf("Expected URL 'http://host.docker.internal:3000/', got '%s'", httpServer.URL)
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

	if conn.GetHTTPURL() != "http://host.docker.internal:3000/" {
		t.Errorf("Expected URL 'http://host.docker.internal:3000/', got '%s'", conn.GetHTTPURL())
	}

	if conn.GetHTTPHeaders()["Authorization"] != "test-auth-secret" {
		t.Errorf("Expected Authorization header 'test-auth-secret', got '%s'", conn.GetHTTPHeaders()["Authorization"])
	}
}

func TestHTTPConnectionWithVariableExpansion(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_AUTH_TOKEN", "secret-token-value")
	defer os.Unsetenv("TEST_AUTH_TOKEN")

	// Create test config with variable expansion
	jsonConfig := `{
		"mcpServers": {
			"safeinputs": {
				"type": "http",
				"url": "http://host.docker.internal:3000/",
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
