package config

import (
	"os"
	"strings"
	"testing"
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
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, result)
				}
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
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				// Check error message contains server name
				if !strings.Contains(err.Error(), tt.serverName) {
					t.Errorf("Error should mention server name %q", tt.serverName)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
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
			name: "valid with command",
			server: &StdinServerConfig{
				Type:    "stdio",
				Command: "node",
				Args:    []string{"server.js"},
			},
			shouldErr: false,
		},
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
			name: "missing command and container",
			server: &StdinServerConfig{
				Type: "stdio",
			},
			shouldErr: true,
			errorMsg:  "either 'command' or 'container' is required",
		},
		{
			name: "both command and container",
			server: &StdinServerConfig{
				Type:      "stdio",
				Command:   "node",
				Container: "test:latest",
			},
			shouldErr: true,
			errorMsg:  "mutually exclusive",
		},
		{
			name: "entrypointArgs without container",
			server: &StdinServerConfig{
				Type:           "stdio",
				Command:        "node",
				EntrypointArgs: []string{"--verbose"},
			},
			shouldErr: true,
			errorMsg:  "'entrypointArgs' is only valid when 'container' is specified",
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
			name: "empty type defaults to stdio",
			server: &StdinServerConfig{
				Command: "node",
			},
			shouldErr: false,
		},
		{
			name: "local type normalizes to stdio",
			server: &StdinServerConfig{
				Type:    "local",
				Command: "node",
			},
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStdioServer("test-server", tt.server)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
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
			errorMsg:  "startupTimeout must be non-negative",
		},
		{
			name: "negative toolTimeout",
			gateway: &StdinGatewayConfig{
				ToolTimeout: intPtr(-1),
			},
			shouldErr: true,
			errorMsg:  "toolTimeout must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGatewayConfig(tt.gateway)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
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
				"command": "node",
				"args": ["server.js"],
				"env": {
					"TOKEN": "${GITHUB_TOKEN}",
					"LITERAL": "static-value"
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

	server := cfg.Servers["github"]
	if server.Env["TOKEN"] != "ghp_expanded" {
		t.Errorf("Expected TOKEN to be expanded to 'ghp_expanded', got %q", server.Env["TOKEN"])
	}
	if server.Env["LITERAL"] != "static-value" {
		t.Errorf("Expected LITERAL to remain 'static-value', got %q", server.Env["LITERAL"])
	}
}

func TestLoadFromStdin_UndefinedVariable(t *testing.T) {
	jsonConfig := `{
		"mcpServers": {
			"github": {
				"type": "stdio",
				"command": "node",
				"env": {
					"TOKEN": "${UNDEFINED_GITHUB_TOKEN}"
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

	_, err := LoadFromStdin()
	os.Stdin = oldStdin

	if err == nil {
		t.Fatal("Expected error for undefined variable")
	}

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
			name: "missing command and container",
			config: `{
				"mcpServers": {
					"test": {
						"type": "stdio"
					}
				}
			}`,
			shouldErr: true,
			errorMsg:  "either 'command' or 'container' is required",
		},
		{
			name: "mutually exclusive command and container",
			config: `{
				"mcpServers": {
					"test": {
						"type": "stdio",
						"command": "node",
						"container": "test:latest"
					}
				}
			}`,
			shouldErr: true,
			errorMsg:  "mutually exclusive",
		},
		{
			name: "invalid gateway port",
			config: `{
				"mcpServers": {
					"test": {
						"type": "stdio",
						"command": "node"
					}
				},
				"gateway": {
					"port": 99999
				}
			}`,
			shouldErr: true,
			errorMsg:  "port must be between",
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
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// Helper function
func intPtr(i int) *int {
	return &i
}
