package cmd

import (
	"os"
	"strings"
	"testing"
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
			defer func() {
				if originalValue != "" {
					os.Setenv("MCP_GATEWAY_LOG_DIR", originalValue)
				} else {
					os.Unsetenv("MCP_GATEWAY_LOG_DIR")
				}
			}()

			// Set test environment variable
			if tt.envValue != "" {
				os.Setenv("MCP_GATEWAY_LOG_DIR", tt.envValue)
			} else {
				os.Unsetenv("MCP_GATEWAY_LOG_DIR")
			}

			// Test getDefaultLogDir
			got := getDefaultLogDir()
			if got != tt.want {
				t.Errorf("getDefaultLogDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConfigFile(t *testing.T) {
	// Verify that the default config file is empty (no default config loading)
	if defaultConfigFile != "" {
		t.Errorf("defaultConfigFile should be empty string, got %q", defaultConfigFile)
	}
}

func TestRunRequiresConfigSource(t *testing.T) {
	// Save original values
	origConfigFile := configFile
	origConfigStdin := configStdin
	defer func() {
		configFile = origConfigFile
		configStdin = origConfigStdin
	}()

	// Test case 1: No config source provided
	configFile = ""
	configStdin = false
	err := run(nil, nil)
	if err == nil {
		t.Error("Expected error when neither --config nor --config-stdin is provided")
	}
	if !strings.Contains(err.Error(), "configuration source required") {
		t.Errorf("Expected 'configuration source required' error, got: %v", err)
	}

	// Test case 2: Config file provided (would fail later due to missing file, but should pass validation)
	configFile = "test.toml"
	configStdin = false
	err = run(nil, nil)
	// Should not be the "configuration source required" error
	if err != nil && strings.Contains(err.Error(), "configuration source required") {
		t.Errorf("Should not require config source when --config is provided, got: %v", err)
	}

	// Test case 3: Config stdin provided (would fail later due to stdin not being readable, but should pass validation)
	configFile = ""
	configStdin = true
	err = run(nil, nil)
	// Should not be the "configuration source required" error
	if err != nil && strings.Contains(err.Error(), "configuration source required") {
		t.Errorf("Should not require config source when --config-stdin is provided, got: %v", err)
	}
}
