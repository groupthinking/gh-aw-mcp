package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultLogDir(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "no environment variable set",
			envValue: "",
			want:     defaultLogDir,
		},
		{
			name:     "environment variable set to custom path",
			envValue: "/custom/log/dir",
			want:     "/custom/log/dir",
		},
		{
			name:     "environment variable set to /var/log",
			envValue: "/var/log/mcp-gateway",
			want:     "/var/log/mcp-gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value and restore after test
			originalValue := os.Getenv("MCP_GATEWAY_LOG_DIR")
			t.Cleanup(func() {
				if originalValue != "" {
					os.Setenv("MCP_GATEWAY_LOG_DIR", originalValue)
				} else {
					os.Unsetenv("MCP_GATEWAY_LOG_DIR")
				}
			})

			// Set test environment variable
			if tt.envValue != "" {
				os.Setenv("MCP_GATEWAY_LOG_DIR", tt.envValue)
			} else {
				os.Unsetenv("MCP_GATEWAY_LOG_DIR")
			}

			// Test getDefaultLogDir
			got := getDefaultLogDir()
			assert.Equal(t, tt.want, got, "getDefaultLogDir() should return expected value")
		})
	}
}

func TestDefaultConfigFile(t *testing.T) {
	// Verify that the default config file is empty (no default config loading)
	assert.Empty(t, defaultConfigFile, "defaultConfigFile should be empty string")
}

func TestRunRequiresConfigSource(t *testing.T) {
	// Save original values
	origConfigFile := configFile
	origConfigStdin := configStdin
	t.Cleanup(func() {
		configFile = origConfigFile
		configStdin = origConfigStdin
	})

	t.Run("no config source provided", func(t *testing.T) {
		configFile = ""
		configStdin = false
		err := run(nil, nil)
		require.Error(t, err, "Expected error when neither --config nor --config-stdin is provided")
		assert.Contains(t, err.Error(), "configuration source required", "Error should mention configuration source required")
	})

	t.Run("config file provided", func(t *testing.T) {
		configFile = "test.toml"
		configStdin = false
		err := run(nil, nil)
		// Should not be the "configuration source required" error
		// (will fail later due to missing file, but should pass validation)
		if err != nil {
			assert.NotContains(t, err.Error(), "configuration source required",
				"Should not require config source when --config is provided")
		}
	})

	t.Run("config stdin provided", func(t *testing.T) {
		configFile = ""
		configStdin = true
		err := run(nil, nil)
		// Should not be the "configuration source required" error
		// (will fail later due to stdin not being readable, but should pass validation)
		if err != nil {
			assert.NotContains(t, err.Error(), "configuration source required",
				"Should not require config source when --config-stdin is provided")
		}
	})

	t.Run("both config file and stdin provided", func(t *testing.T) {
		configFile = "test.toml"
		configStdin = true
		err := run(nil, nil)
		// When both are provided, stdin takes precedence per flag description
		// Should not be the "configuration source required" error
		if err != nil {
			assert.NotContains(t, err.Error(), "configuration source required",
				"Should not require config source when both are provided")
		}
	})
}

func TestLoadEnvFile(t *testing.T) {
	t.Run("load valid env file", func(t *testing.T) {
		// Create temporary env file
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		content := `# Comment line
TEST_VAR1=value1
TEST_VAR2=value2
EMPTY_LINE=

# Another comment
TEST_VAR3=value with spaces
`
		err := os.WriteFile(envFile, []byte(content), 0644)
		require.NoError(t, err)

		// Save and restore environment variables
		origTestVar1, testVar1WasSet := os.LookupEnv("TEST_VAR1")
		origTestVar2, testVar2WasSet := os.LookupEnv("TEST_VAR2")
		origTestVar3, testVar3WasSet := os.LookupEnv("TEST_VAR3")
		origEmptyLine, emptyLineWasSet := os.LookupEnv("EMPTY_LINE")
		t.Cleanup(func() {
			if testVar1WasSet {
				require.NoError(t, os.Setenv("TEST_VAR1", origTestVar1))
			} else {
				require.NoError(t, os.Unsetenv("TEST_VAR1"))
			}
			if testVar2WasSet {
				require.NoError(t, os.Setenv("TEST_VAR2", origTestVar2))
			} else {
				require.NoError(t, os.Unsetenv("TEST_VAR2"))
			}
			if testVar3WasSet {
				require.NoError(t, os.Setenv("TEST_VAR3", origTestVar3))
			} else {
				require.NoError(t, os.Unsetenv("TEST_VAR3"))
			}
			if emptyLineWasSet {
				require.NoError(t, os.Setenv("EMPTY_LINE", origEmptyLine))
			} else {
				require.NoError(t, os.Unsetenv("EMPTY_LINE"))
			}
		})

		// Load env file
		err = loadEnvFile(envFile)
		require.NoError(t, err)

		// Verify variables are set
		assert.Equal(t, "value1", os.Getenv("TEST_VAR1"))
		assert.Equal(t, "value2", os.Getenv("TEST_VAR2"))
		assert.Equal(t, "value with spaces", os.Getenv("TEST_VAR3"))
		assert.Equal(t, "", os.Getenv("EMPTY_LINE"))
	})

	t.Run("nonexistent file", func(t *testing.T) {
		err := loadEnvFile("/nonexistent/path/.env")
		require.Error(t, err, "Should error on nonexistent file")
	})

	t.Run("env file with variable expansion", func(t *testing.T) {
		// Save original values and set up cleanup before modifying environment
		origBasePath, basePathWasSet := os.LookupEnv("BASE_PATH")
		origExpandedVar, expandedVarWasSet := os.LookupEnv("EXPANDED_VAR")
		t.Cleanup(func() {
			if basePathWasSet {
				_ = os.Setenv("BASE_PATH", origBasePath)
			} else {
				_ = os.Unsetenv("BASE_PATH")
			}
			if expandedVarWasSet {
				_ = os.Setenv("EXPANDED_VAR", origExpandedVar)
			} else {
				_ = os.Unsetenv("EXPANDED_VAR")
			}
		})

		// Set up a base variable for expansion
		os.Setenv("BASE_PATH", "/home/user")
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		content := `EXPANDED_VAR=$BASE_PATH/subdir`
		err := os.WriteFile(envFile, []byte(content), 0644)
		require.NoError(t, err)

		err = loadEnvFile(envFile)
		require.NoError(t, err)

		assert.Equal(t, "/home/user/subdir", os.Getenv("EXPANDED_VAR"))
	})

	t.Run("empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		err := os.WriteFile(envFile, []byte(""), 0644)
		require.NoError(t, err)

		err = loadEnvFile(envFile)
		require.NoError(t, err, "Empty file should not cause error")
	})
}

func TestWriteGatewayConfig(t *testing.T) {
	t.Run("unified mode with API key", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"test-server": {
					Type: "stdio",
				},
			},
			Gateway: &config.GatewayConfig{
				APIKey: "test-api-key",
			},
		}

		var buf bytes.Buffer
		err := writeGatewayConfig(cfg, "127.0.0.1:3000", "unified", &buf)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, `"mcpServers"`)
		assert.Contains(t, output, `"test-server"`)
		assert.Contains(t, output, `"type": "http"`)
		assert.Contains(t, output, `"url": "http://127.0.0.1:3000/mcp"`)
		assert.Contains(t, output, `"Authorization": "test-api-key"`)
	})

	t.Run("routed mode without API key", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"server1": {Type: "stdio"},
				"server2": {Type: "stdio"},
			},
		}

		var buf bytes.Buffer
		err := writeGatewayConfig(cfg, "localhost:8080", "routed", &buf)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, `"server1"`)
		assert.Contains(t, output, `"server2"`)
		assert.Contains(t, output, `"http://localhost:8080/mcp/server1"`)
		assert.Contains(t, output, `"http://localhost:8080/mcp/server2"`)
		assert.NotContains(t, output, `"Authorization"`)
	})

	t.Run("with tools field", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"test-server": {
					Type:  "stdio",
					Tools: []string{"tool1", "tool2"},
				},
			},
		}

		var buf bytes.Buffer
		err := writeGatewayConfig(cfg, "127.0.0.1:3000", "unified", &buf)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, `"tools"`)
		assert.Contains(t, output, `"tool1"`)
		assert.Contains(t, output, `"tool2"`)
	})

	t.Run("IPv6 address", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"test-server": {Type: "stdio"},
			},
		}

		var buf bytes.Buffer
		err := writeGatewayConfig(cfg, "[::1]:3000", "unified", &buf)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, `"url": "http://::1:3000/mcp"`)
	})

	t.Run("invalid listen address uses defaults", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"test-server": {Type: "stdio"},
			},
		}

		var buf bytes.Buffer
		err := writeGatewayConfig(cfg, "invalid-address", "unified", &buf)
		require.NoError(t, err)

		output := buf.String()
		// Should fall back to default host and port
		assert.Contains(t, output, DefaultListenIPv4)
		assert.Contains(t, output, DefaultListenPort)
	})
}

