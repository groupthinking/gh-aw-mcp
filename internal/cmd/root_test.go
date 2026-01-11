package cmd

import (
	"os"
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
