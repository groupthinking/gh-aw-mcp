package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateContainerID(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		shouldError bool
	}{
		{
			name:        "valid 64-char hex",
			containerID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			shouldError: false,
		},
		{
			name:        "valid 12-char hex (short form)",
			containerID: "abcdef123456",
			shouldError: false,
		},
		{
			name:        "valid with all hex digits",
			containerID: "0123456789abcdef",
			shouldError: false,
		},
		{
			name:        "empty string",
			containerID: "",
			shouldError: true,
		},
		{
			name:        "too short (11 chars)",
			containerID: "abcdef12345",
			shouldError: true,
		},
		{
			name:        "too long (65 chars)",
			containerID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678901",
			shouldError: true,
		},
		{
			name:        "invalid chars - uppercase",
			containerID: "ABCDEF123456",
			shouldError: true,
		},
		{
			name:        "invalid chars - special",
			containerID: "abc;def123456",
			shouldError: true,
		},
		{
			name:        "command injection attempt",
			containerID: "abcdef123456; rm -rf /",
			shouldError: true,
		},
		{
			name:        "path injection attempt",
			containerID: "../../../etc/passwd",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContainerID(tt.containerID)
			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func TestDetectContainerized(t *testing.T) {
	// This test verifies the function doesn't panic and returns consistent results
	isContainerized, containerID := detectContainerized()

	// In a test environment, we're typically not containerized
	// but we just verify the function works
	t.Logf("detectContainerized: isContainerized=%v, containerID=%s", isContainerized, containerID)

	// If we detect a container, the ID should have some content
	if isContainerized && containerID != "" {
		if len(containerID) < 12 {
			t.Errorf("Container ID should be at least 12 characters, got %d", len(containerID))
		}
	}
}

func TestCheckRequiredEnvVars(t *testing.T) {
	// Clear any existing env vars for the test
	for _, v := range RequiredEnvVars {
		os.Unsetenv(v)
	}
	defer func() {
		for _, v := range RequiredEnvVars {
			os.Unsetenv(v)
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected []string
	}{
		{
			name:     "all missing",
			envVars:  map[string]string{},
			expected: RequiredEnvVars,
		},
		{
			name: "all set",
			envVars: map[string]string{
				"MCP_GATEWAY_PORT":    "8080",
				"MCP_GATEWAY_DOMAIN":  "localhost",
				"MCP_GATEWAY_API_KEY": "test-key",
			},
			expected: nil,
		},
		{
			name: "partial set - missing port",
			envVars: map[string]string{
				"MCP_GATEWAY_DOMAIN":  "localhost",
				"MCP_GATEWAY_API_KEY": "test-key",
			},
			expected: []string{"MCP_GATEWAY_PORT"},
		},
		{
			name: "partial set - missing domain",
			envVars: map[string]string{
				"MCP_GATEWAY_PORT":    "8080",
				"MCP_GATEWAY_API_KEY": "test-key",
			},
			expected: []string{"MCP_GATEWAY_DOMAIN"},
		},
		{
			name: "partial set - missing api key",
			envVars: map[string]string{
				"MCP_GATEWAY_PORT":   "8080",
				"MCP_GATEWAY_DOMAIN": "localhost",
			},
			expected: []string{"MCP_GATEWAY_API_KEY"},
		},
		{
			name: "empty string values are missing",
			envVars: map[string]string{
				"MCP_GATEWAY_PORT":    "",
				"MCP_GATEWAY_DOMAIN":  "localhost",
				"MCP_GATEWAY_API_KEY": "test-key",
			},
			expected: []string{"MCP_GATEWAY_PORT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars first
			for _, v := range RequiredEnvVars {
				os.Unsetenv(v)
			}

			// Set up test environment
			for k, v := range tt.envVars {
				if v != "" {
					os.Setenv(k, v)
				}
			}

			missing := checkRequiredEnvVars()

			if len(missing) != len(tt.expected) {
				t.Errorf("Expected %d missing vars, got %d. Missing: %v", len(tt.expected), len(missing), missing)
				return
			}

			// Check each expected var is in the missing list
			for _, expectedVar := range tt.expected {
				found := false
				for _, missingVar := range missing {
					if missingVar == expectedVar {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected %s to be in missing list, but it wasn't. Missing: %v", expectedVar, missing)
				}
			}
		})
	}
}

func TestGetGatewayPortFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		setEnv      bool
		expected    int
		shouldError bool
	}{
		{
			name:        "valid port",
			envValue:    "8080",
			setEnv:      true,
			expected:    8080,
			shouldError: false,
		},
		{
			name:        "min port",
			envValue:    "1",
			setEnv:      true,
			expected:    1,
			shouldError: false,
		},
		{
			name:        "max port",
			envValue:    "65535",
			setEnv:      true,
			expected:    65535,
			shouldError: false,
		},
		{
			name:        "port zero - invalid",
			envValue:    "0",
			setEnv:      true,
			shouldError: true,
		},
		{
			name:        "port too high",
			envValue:    "65536",
			setEnv:      true,
			shouldError: true,
		},
		{
			name:        "negative port",
			envValue:    "-1",
			setEnv:      true,
			shouldError: true,
		},
		{
			name:        "non-numeric port",
			envValue:    "abc",
			setEnv:      true,
			shouldError: true,
		},
		{
			name:        "not set",
			setEnv:      false,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("MCP_GATEWAY_PORT")
			if tt.setEnv {
				os.Setenv("MCP_GATEWAY_PORT", tt.envValue)
			}
			defer os.Unsetenv("MCP_GATEWAY_PORT")

			port, err := GetGatewayPortFromEnv()

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				assert.NoError(t, err, "Unexpected error")
				assert.Equal(t, tt.expected, port, "port %d, got %d")
			}
		})
	}
}

func TestGetGatewayDomainFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		setEnv   bool
	}{
		{
			name:     "valid domain",
			envValue: "localhost",
			setEnv:   true,
		},
		{
			name:     "domain with subdomain",
			envValue: "mcp.example.com",
			setEnv:   true,
		},
		{
			name:   "not set",
			setEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("MCP_GATEWAY_DOMAIN")
			if tt.setEnv {
				os.Setenv("MCP_GATEWAY_DOMAIN", tt.envValue)
			}
			defer os.Unsetenv("MCP_GATEWAY_DOMAIN")

			domain := GetGatewayDomainFromEnv()

			if tt.setEnv && domain != tt.envValue {
				t.Errorf("Expected domain %s, got %s", tt.envValue, domain)
			}
			if !tt.setEnv && domain != "" {
				t.Errorf("Expected empty domain when not set, got %s", domain)
			}
		})
	}
}

func TestGetGatewayAPIKeyFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		setEnv   bool
	}{
		{
			name:     "valid key",
			envValue: "my-secret-key",
			setEnv:   true,
		},
		{
			name:     "complex key",
			envValue: "abc123!@#$%^&*()",
			setEnv:   true,
		},
		{
			name:   "not set",
			setEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("MCP_GATEWAY_API_KEY")
			if tt.setEnv {
				os.Setenv("MCP_GATEWAY_API_KEY", tt.envValue)
			}
			defer os.Unsetenv("MCP_GATEWAY_API_KEY")

			key := GetGatewayAPIKeyFromEnv()

			if tt.setEnv && key != tt.envValue {
				t.Errorf("Expected key %s, got %s", tt.envValue, key)
			}
			if !tt.setEnv && key != "" {
				t.Errorf("Expected empty key when not set, got %s", key)
			}
		})
	}
}

func TestEnvValidationResultIsValid(t *testing.T) {
	tests := []struct {
		name   string
		result *EnvValidationResult
		valid  bool
	}{
		{
			name:   "valid - no errors",
			result: &EnvValidationResult{},
			valid:  true,
		},
		{
			name: "valid - with warnings",
			result: &EnvValidationResult{
				ValidationWarnings: []string{"some warning"},
			},
			valid: true,
		},
		{
			name: "invalid - with errors",
			result: &EnvValidationResult{
				ValidationErrors: []string{"some error"},
			},
			valid: false,
		},
		{
			name: "invalid - multiple errors",
			result: &EnvValidationResult{
				ValidationErrors: []string{"error 1", "error 2"},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestEnvValidationResultError(t *testing.T) {
	tests := []struct {
		name     string
		result   *EnvValidationResult
		expected string
	}{
		{
			name:     "no errors",
			result:   &EnvValidationResult{},
			expected: "",
		},
		{
			name: "single error",
			result: &EnvValidationResult{
				ValidationErrors: []string{"Docker not accessible"},
			},
			expected: "Environment validation failed:\n  - Docker not accessible",
		},
		{
			name: "multiple errors",
			result: &EnvValidationResult{
				ValidationErrors: []string{"Error 1", "Error 2"},
			},
			expected: "Environment validation failed:\n  - Error 1\n  - Error 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValidateExecutionEnvironment(t *testing.T) {
	// This test verifies the function runs without panicking
	// The actual Docker check will fail in most test environments

	// Save original env vars
	origPort := os.Getenv("MCP_GATEWAY_PORT")
	origDomain := os.Getenv("MCP_GATEWAY_DOMAIN")
	origAPIKey := os.Getenv("MCP_GATEWAY_API_KEY")
	defer func() {
		if origPort != "" {
			os.Setenv("MCP_GATEWAY_PORT", origPort)
		}
		if origDomain != "" {
			os.Setenv("MCP_GATEWAY_DOMAIN", origDomain)
		}
		if origAPIKey != "" {
			os.Setenv("MCP_GATEWAY_API_KEY", origAPIKey)
		}
	}()

	t.Run("with all env vars set", func(t *testing.T) {
		os.Setenv("MCP_GATEWAY_PORT", "8080")
		os.Setenv("MCP_GATEWAY_DOMAIN", "localhost")
		os.Setenv("MCP_GATEWAY_API_KEY", "test-key")

		result := ValidateExecutionEnvironment()

		// Should not have missing env vars
		assert.False(t, len(result.MissingEnvVars) > 0, "Expected no missing env vars, got %v")
	})

	t.Run("with missing env vars", func(t *testing.T) {
		os.Unsetenv("MCP_GATEWAY_PORT")
		os.Unsetenv("MCP_GATEWAY_DOMAIN")
		os.Unsetenv("MCP_GATEWAY_API_KEY")

		result := ValidateExecutionEnvironment()

		// Should have missing env vars
		if len(result.MissingEnvVars) != 3 {
			t.Errorf("Expected 3 missing env vars, got %d: %v", len(result.MissingEnvVars), result.MissingEnvVars)
		}

		// Should have validation errors
		if len(result.ValidationErrors) == 0 {
			t.Error("Expected validation errors for missing env vars")
		}
	})
}

func TestRunDockerInspect(t *testing.T) {
	tests := []struct {
		name           string
		containerID    string
		formatTemplate string
		shouldError    bool
	}{
		{
			name:           "empty container ID",
			containerID:    "",
			formatTemplate: "{{.Config.OpenStdin}}",
			shouldError:    true,
		},
		{
			name:           "invalid container ID - too short",
			containerID:    "abc123",
			formatTemplate: "{{.Config.OpenStdin}}",
			shouldError:    true,
		},
		{
			name:           "invalid container ID - special chars",
			containerID:    "abc;def123456",
			formatTemplate: "{{.Config.OpenStdin}}",
			shouldError:    true,
		},
		{
			name:           "valid container ID format - command will fail without docker",
			containerID:    "abcdef123456",
			formatTemplate: "{{.Config.OpenStdin}}",
			shouldError:    true, // Will fail because container doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runDockerInspect(tt.containerID, tt.formatTemplate)

			if tt.shouldError {
				assert.Error(t, err, "Expected error but got none")
				assert.Empty(t, output, "Expected empty output on error")
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}
