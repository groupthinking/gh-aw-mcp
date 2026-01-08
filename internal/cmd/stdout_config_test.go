package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

func TestWriteGatewayConfigToStdout(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.Config
		listenAddr string
		mode       string
		wantHost   string
		wantPort   string
	}{
		{
			name: "routed mode with single server",
			cfg: &config.Config{
				Servers: map[string]*config.ServerConfig{
					"github": {
						Command: "docker",
						Args:    []string{"run", "--rm", "-i", "ghcr.io/github/github-mcp-server:latest"},
					},
				},
			},
			listenAddr: "127.0.0.1:8080",
			mode:       "routed",
			wantHost:   "127.0.0.1",
			wantPort:   "8080",
		},
		{
			name: "unified mode with multiple servers",
			cfg: &config.Config{
				Servers: map[string]*config.ServerConfig{
					"github": {
						Command: "docker",
					},
					"fetch": {
						Command: "docker",
					},
				},
			},
			listenAddr: "0.0.0.0:3000",
			mode:       "unified",
			wantHost:   "0.0.0.0",
			wantPort:   "3000",
		},
		{
			name: "default port when address has no port",
			cfg: &config.Config{
				Servers: map[string]*config.ServerConfig{
					"test": {
						Command: "echo",
					},
				},
			},
			listenAddr: "localhost",
			mode:       "routed",
			wantHost:   "127.0.0.1",
			wantPort:   "3000",
		},
		{
			name: "IPv6 address with port",
			cfg: &config.Config{
				Servers: map[string]*config.ServerConfig{
					"test": {
						Command: "echo",
					},
				},
			},
			listenAddr: "[::1]:8080",
			mode:       "routed",
			wantHost:   "::1",
			wantPort:   "8080",
		},
		{
			name: "IPv6 address with full notation",
			cfg: &config.Config{
				Servers: map[string]*config.ServerConfig{
					"github": {
						Command: "docker",
					},
				},
			},
			listenAddr: "[2001:db8::1]:3000",
			mode:       "unified",
			wantHost:   "2001:db8::1",
			wantPort:   "3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer to capture output
			var buf bytes.Buffer

			// Write configuration to buffer
			err := writeGatewayConfig(tt.cfg, tt.listenAddr, tt.mode, &buf)

			if err != nil {
				t.Fatalf("writeGatewayConfig() error = %v", err)
			}
			output := buf.String()

			// Parse JSON output
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
			}

			// Verify structure
			mcpServers, ok := result["mcpServers"].(map[string]interface{})
			if !ok {
				t.Fatal("Output missing 'mcpServers' field or wrong type")
			}

			// Verify all servers are present
			if len(mcpServers) != len(tt.cfg.Servers) {
				t.Errorf("Expected %d servers, got %d", len(tt.cfg.Servers), len(mcpServers))
			}

			// Verify each server configuration
			for serverName := range tt.cfg.Servers {
				serverConfig, ok := mcpServers[serverName].(map[string]interface{})
				if !ok {
					t.Errorf("Server '%s' missing or wrong type", serverName)
					continue
				}

				// Verify type is "http"
				if serverType, ok := serverConfig["type"].(string); !ok || serverType != "http" {
					t.Errorf("Server '%s' type = %v, want 'http'", serverName, serverConfig["type"])
				}

				// Verify URL format
				url, ok := serverConfig["url"].(string)
				if !ok {
					t.Errorf("Server '%s' missing url or wrong type", serverName)
					continue
				}

				// Check URL contains expected components
				expectedPrefix := "http://" + tt.wantHost + ":" + tt.wantPort + "/mcp"
				if len(url) < len(expectedPrefix) || url[:len(expectedPrefix)] != expectedPrefix {
					t.Errorf("Server '%s' url = %v, want prefix %v", serverName, url, expectedPrefix)
				}

				// In routed mode, URL should include server name
				if tt.mode == "routed" {
					expectedURL := expectedPrefix + "/" + serverName
					if url != expectedURL {
						t.Errorf("Server '%s' url = %v, want %v", serverName, url, expectedURL)
					}
				} else {
					// In unified mode, URL should be just /mcp
					if url != expectedPrefix {
						t.Errorf("Server '%s' url = %v, want %v", serverName, url, expectedPrefix)
					}
				}
			}
		})
	}
}

func TestWriteGatewayConfigToStdout_EmptyConfig(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	// Create buffer to capture output
	var buf bytes.Buffer

	err := writeGatewayConfig(cfg, "127.0.0.1:8080", "routed", &buf)

	if err != nil {
		t.Fatalf("writeGatewayConfig() error = %v", err)
	}

	// Parse output
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	mcpServers := result["mcpServers"].(map[string]interface{})
	if len(mcpServers) != 0 {
		t.Errorf("Expected empty mcpServers, got %d servers", len(mcpServers))
	}
}

func TestWriteGatewayConfigToStdout_JSONFormat(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test": {
				Command: "echo",
			},
		},
	}

	// Create buffer to capture output
	var buf bytes.Buffer

	err := writeGatewayConfig(cfg, "localhost:3000", "routed", &buf)

	if err != nil {
		t.Fatalf("writeGatewayConfig() error = %v", err)
	}

	output := buf.String()

	// Verify it's valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Verify output is pretty-printed (contains newlines)
	if !bytes.Contains(buf.Bytes(), []byte("\n")) {
		t.Error("Output should be pretty-printed with indentation")
	}
}
