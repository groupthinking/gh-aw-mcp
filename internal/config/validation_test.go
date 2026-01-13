package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandVariables(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		envVars   map[string]string
		expected  string
		shouldErr bool
	}{
		{
			name:     "simple variable",
			input:    "${TEST_VAR}",
			envVars:  map[string]string{"TEST_VAR": "value"},
			expected: "value",
		},
		{
			name:     "multiple variables",
			input:    "${VAR1}-${VAR2}",
			envVars:  map[string]string{"VAR1": "hello", "VAR2": "world"},
			expected: "hello-world",
		},
		{
			name:     "variable in middle",
			input:    "prefix-${VAR}-suffix",
			envVars:  map[string]string{"VAR": "middle"},
			expected: "prefix-middle-suffix",
		},
		{
			name:     "no variables",
			input:    "static-value",
			envVars:  map[string]string{},
			expected: "static-value",
		},
		{
			name:      "undefined variable",
			input:     "${UNDEFINED_VAR}",
			envVars:   map[string]string{},
			shouldErr: true,
		},
		{
			name:      "mixed defined and undefined",
			input:     "${DEFINED}-${UNDEFINED}",
			envVars:   map[string]string{"DEFINED": "value"},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			result, err := expandVariables(tt.input, "test.path")

			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err, "Unexpected error")
				assert.Equal(t, tt.expected, result, "%q, got %q")
			}
		})
	}
}

func TestExpandEnvVariables(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "ghp_test123")
	os.Setenv("API_KEY", "secret")
	defer os.Unsetenv("GITHUB_TOKEN")
	defer os.Unsetenv("API_KEY")

	tests := []struct {
		name       string
		input      map[string]string
		serverName string
		expected   map[string]string
		shouldErr  bool
	}{
		{
			name: "expand single variable",
			input: map[string]string{
				"TOKEN": "${GITHUB_TOKEN}",
			},
			serverName: "test",
			expected: map[string]string{
				"TOKEN": "ghp_test123",
			},
		},
		{
			name: "expand multiple variables",
			input: map[string]string{
				"TOKEN":   "${GITHUB_TOKEN}",
				"API_KEY": "${API_KEY}",
			},
			serverName: "test",
			expected: map[string]string{
				"TOKEN":   "ghp_test123",
				"API_KEY": "secret",
			},
		},
		{
			name: "mixed literal and variable",
			input: map[string]string{
				"LITERAL": "static",
				"DYNAMIC": "${GITHUB_TOKEN}",
			},
			serverName: "test",
			expected: map[string]string{
				"LITERAL": "static",
				"DYNAMIC": "ghp_test123",
			},
		},
		{
			name: "undefined variable",
			input: map[string]string{
				"TOKEN": "${UNDEFINED_VAR}",
			},
			serverName: "test",
			shouldErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandEnvVariables(tt.input, tt.serverName)

			if tt.shouldErr {
				assert.Error(t, err)
				// Check error message contains server name
				if !strings.Contains(err.Error(), tt.serverName) {
					t.Errorf("Error should mention server name %q", tt.serverName)
				}
			} else {
				assert.NoError(t, err, "Unexpected error")
				for k, v := range tt.expected {
					if result[k] != v {
						t.Errorf("For key %q: expected %q, got %q", k, v, result[k])
					}
				}
			}
		})
	}
}

func TestValidateStdioServer(t *testing.T) {
	tests := []struct {
		name      string
		server    *StdinServerConfig
		shouldErr bool
		errorMsg  string
	}{
		{
			name: "valid with container",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
			},
			shouldErr: false,
		},
		{
			name: "valid with entrypointArgs and container",
			server: &StdinServerConfig{
				Type:           "stdio",
				Container:      "test:latest",
				EntrypointArgs: []string{"--verbose"},
			},
			shouldErr: false,
		},
		{
			name: "valid with entrypoint and container",
			server: &StdinServerConfig{
				Type:       "stdio",
				Container:  "test:latest",
				Entrypoint: "/bin/bash",
			},
			shouldErr: false,
		},
		{
			name: "valid with mounts (ro)",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host/path:/container/path:ro"},
			},
			shouldErr: false,
		},
		{
			name: "valid with mounts (rw)",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host/data:/app/data:rw"},
			},
			shouldErr: false,
		},
		{
			name: "valid with multiple mounts",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts: []string{
					"/host/path1:/container/path1:ro",
					"/host/path2:/container/path2:rw",
				},
			},
			shouldErr: false,
		},
		{
			name: "valid with all new fields",
			server: &StdinServerConfig{
				Type:           "stdio",
				Container:      "test:latest",
				Entrypoint:     "/custom/entrypoint.sh",
				EntrypointArgs: []string{"--verbose", "--debug"},
				Mounts:         []string{"/host:/container:ro"},
			},
			shouldErr: false,
		},
		{
			name: "missing container",
			server: &StdinServerConfig{
				Type: "stdio",
			},
			shouldErr: true,
			errorMsg:  "'container' is required for stdio servers",
		},
		{
			name: "command field not supported",
			server: &StdinServerConfig{
				Type:      "stdio",
				Command:   "node",
				Container: "test:latest",
			},
			shouldErr: true,
			errorMsg:  "'command' field is not supported",
		},
		{
			name: "command without container",
			server: &StdinServerConfig{
				Type:    "stdio",
				Command: "node",
			},
			shouldErr: true,
			errorMsg:  "'container' is required for stdio servers",
		},
		{
			name: "http server without url",
			server: &StdinServerConfig{
				Type: "http",
			},
			shouldErr: true,
			errorMsg:  "'url' is required for HTTP servers",
		},
		{
			name: "http server with url",
			server: &StdinServerConfig{
				Type: "http",
				URL:  "https://example.com/mcp",
			},
			shouldErr: false,
		},
		{
			name: "empty type defaults to stdio with container",
			server: &StdinServerConfig{
				Container: "test:latest",
			},
			shouldErr: false,
		},
		{
			name: "local type normalizes to stdio with container",
			server: &StdinServerConfig{
				Type:      "local",
				Container: "test:latest",
			},
			shouldErr: false,
		},
		{
			name: "invalid mount format - missing mode",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host:/container"},
			},
			shouldErr: true,
			errorMsg:  "invalid mount format",
		},
		{
			name: "invalid mount format - too many parts",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host:/container:ro:extra"},
			},
			shouldErr: true,
			errorMsg:  "invalid mount format",
		},
		{
			name: "invalid mount mode",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host:/container:invalid"},
			},
			shouldErr: true,
			errorMsg:  "invalid mount mode",
		},
		{
			name: "mount with empty source",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{":/container:ro"},
			},
			shouldErr: true,
			errorMsg:  "mount source cannot be empty",
		},
		{
			name: "mount with empty destination",
			server: &StdinServerConfig{
				Type:      "stdio",
				Container: "test:latest",
				Mounts:    []string{"/host::ro"},
			},
			shouldErr: true,
			errorMsg:  "mount destination cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServerConfig("test-server", tt.server)

			if tt.shouldErr {
				assert.Error(t, err)
				if tt.errorMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func TestValidateGatewayConfig(t *testing.T) {
	tests := []struct {
		name      string
		gateway   *StdinGatewayConfig
		shouldErr bool
		errorMsg  string
	}{
		{
			name:      "nil gateway",
			gateway:   nil,
			shouldErr: false,
		},
		{
			name: "valid gateway",
			gateway: &StdinGatewayConfig{
				Port:           intPtr(8080),
				Domain:         "example.com",
				StartupTimeout: intPtr(30),
				ToolTimeout:    intPtr(60),
			},
			shouldErr: false,
		},
		{
			name: "port too low",
			gateway: &StdinGatewayConfig{
				Port: intPtr(0),
			},
			shouldErr: true,
			errorMsg:  "port must be between 1 and 65535",
		},
		{
			name: "port too high",
			gateway: &StdinGatewayConfig{
				Port: intPtr(70000),
			},
			shouldErr: true,
			errorMsg:  "port must be between 1 and 65535",
		},
		{
			name: "negative startupTimeout",
			gateway: &StdinGatewayConfig{
				StartupTimeout: intPtr(-1),
			},
			shouldErr: true,
			errorMsg:  "startupTimeout must be at least 1",
		},
		{
			name: "zero startupTimeout",
			gateway: &StdinGatewayConfig{
				StartupTimeout: intPtr(0),
			},
			shouldErr: true,
			errorMsg:  "startupTimeout must be at least 1",
		},
		{
			name: "negative toolTimeout",
			gateway: &StdinGatewayConfig{
				ToolTimeout: intPtr(-1),
			},
			shouldErr: true,
			errorMsg:  "toolTimeout must be at least 1",
		},
		{
			name: "zero toolTimeout",
			gateway: &StdinGatewayConfig{
				ToolTimeout: intPtr(0),
			},
			shouldErr: true,
			errorMsg:  "toolTimeout must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGatewayConfig(tt.gateway)

			if tt.shouldErr {
				assert.Error(t, err)
				if tt.errorMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func TestLoadFromStdin_WithVariableExpansion(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "ghp_expanded")
	defer os.Unsetenv("GITHUB_TOKEN")

	jsonConfig := `{
		"mcpServers": {
			"github": {
				"type": "stdio",
				"container": "ghcr.io/github/github-mcp-server:latest",
				"env": {
					"TOKEN": "${GITHUB_TOKEN}",
					"LITERAL": "static-value"
				}
			}
		},
		"gateway": {
			"port": 8080,
			"domain": "localhost",
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

	cfg, err := LoadFromStdin()
	os.Stdin = oldStdin

	require.NoError(t, err, "LoadFromStdin() failed")

	server := cfg.Servers["github"]
	// Check docker command is set up correctly
	if server.Command != "docker" {
		t.Errorf("Expected Command to be 'docker', got %q", server.Command)
	}
}

func TestLoadFromStdin_UndefinedVariable(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"github": {
				"type": "stdio",
				"container": "ghcr.io/github/github-mcp-server:latest",
				"env": {
					"TOKEN": "${UNDEFINED_GITHUB_TOKEN}"
				}
			}
		},
		"gateway": {
			"port": 8080,
			"domain": "localhost",
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

	require.Error(t, err)

	if !strings.Contains(err.Error(), "UNDEFINED_GITHUB_TOKEN") {
		t.Errorf("Error should mention the undefined variable, got: %v", err)
	}
	if !strings.Contains(err.Error(), "mcpServers.github.env") {
		t.Errorf("Error should include JSON path, got: %v", err)
	}
}

func TestLoadFromStdin_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		shouldErr bool
		errorMsg  string
	}{
		{
			name: "missing container",
			config: `{
				"mcpServers": {
					"test": {
						"type": "stdio"
					}
				},
				"gateway": {
					"port": 8080,
					"domain": "localhost",
					"apiKey": "test-key"
				}
			}`,
			shouldErr: true,
			errorMsg:  "validation error",
		},
		{
			name: "command field not supported",
			config: `{
				"mcpServers": {
					"test": {
						"type": "stdio",
						"command": "node",
						"container": "test:latest"
					}
				},
				"gateway": {
					"port": 8080,
					"domain": "localhost",
					"apiKey": "test-key"
				}
			}`,
			shouldErr: true,
			errorMsg:  "validation error",
		},
		{
			name: "invalid gateway port",
			config: `{
				"mcpServers": {
					"test": {
						"type": "stdio",
						"container": "test:latest"
					}
				},
				"gateway": {
					"port": 99999,
					"domain": "localhost",
					"apiKey": "test-key"
				}
			}`,
			shouldErr: true,
			errorMsg:  "validation error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, _ := os.Pipe()
			oldStdin := os.Stdin
			os.Stdin = r
			go func() {
				w.Write([]byte(tt.config))
				w.Close()
			}()

			_, err := LoadFromStdin()
			os.Stdin = oldStdin

			if tt.shouldErr {
				assert.Error(t, err)
				if err != nil && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

// Helper function
func intPtr(i int) *int {
	return &i
}
