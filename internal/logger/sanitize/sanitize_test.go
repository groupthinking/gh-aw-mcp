package sanitize

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "GitHub PAT",
			input:    "token=ghp_1234567890123456789012345678901234567890",
			expected: "[REDACTED]",
		},
		{
			name:     "GitHub fine-grained PAT",
			input:    "token=github_pat_1234567890123456789012_12345678901234567890123456789012345678901234567890123456789",
			expected: "[REDACTED]",
		},
		{
			name:     "API key with equals",
			input:    "API_KEY=sk_test_abcdefghijklmnopqrstuvwxyz123456",
			expected: "[REDACTED]",
		},
		{
			name:     "Password with colon",
			input:    "password: supersecretpassword123",
			expected: "[REDACTED]",
		},
		{
			name:     "Normal log message",
			input:    "Normal log message without secrets",
			expected: "Normal log message without secrets",
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer abcdefghijklmnopqrstuvwxyz",
			expected: "[REDACTED]",
		},
		{
			name:     "Long hex string",
			input:    "api_key=abcdef1234567890abcdef1234567890abcdef12",
			expected: "[REDACTED]",
		},
		{
			name:     "JWT token",
			input:    "jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature",
			expected: "[REDACTED]",
		},
		{
			name:     "OAuth client secret",
			input:    "client_secret=oauth_secret_12345678",
			expected: "[REDACTED]",
		},
		{
			name:     "Multiple secrets in one message",
			input:    "token=secret123 and password=pass12345678",
			expected: "[REDACTED] and [REDACTED]",
		},
		{
			name:     "JSON with secret field",
			input:    `{"token":"ghp_1234567890123456789012345678901234567890"}`,
			expected: `{"token"=[REDACTED]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)
			if !strings.Contains(result, "[REDACTED]") && tt.expected != tt.input {
				t.Errorf("SanitizeString() should contain [REDACTED], got: %s", result)
			}
			// Verify the original secret is not in the result
			if tt.expected != tt.input {
				// Extract the secret portion
				if strings.Contains(tt.input, "ghp_") && strings.Contains(result, "ghp_") {
					t.Errorf("GitHub PAT not sanitized: %s", result)
				}
				if strings.Contains(tt.input, "secret123") && strings.Contains(result, "secret123") {
					t.Errorf("Secret not sanitized: %s", result)
				}
				if strings.Contains(tt.input, "pass12345678") && strings.Contains(result, "pass12345678") {
					t.Errorf("Password not sanitized: %s", result)
				}
			}
		})
	}
}

func TestSanitizeJSON(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectRedacted bool
		checkField     string
	}{
		{
			name:           "token in payload",
			input:          `{"token":"ghp_1234567890123456789012345678901234567890"}`,
			expectRedacted: true,
			checkField:     "token",
		},
		{
			name:           "nested token in params",
			input:          `{"params":{"auth":"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.sig"}}`,
			expectRedacted: true,
			checkField:     "params.auth",
		},
		{
			name:           "password field",
			input:          `{"password":"supersecret123"}`,
			expectRedacted: true,
			checkField:     "password",
		},
		{
			name:           "clean payload",
			input:          `{"method":"tools/list","id":1}`,
			expectRedacted: false,
			checkField:     "method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeJSON([]byte(tt.input))

			if result == nil {
				t.Fatalf("SanitizeJSON returned nil")
			}

			// The result is already a sanitized string
			sanitizedStr := string(result)

			if tt.expectRedacted {
				// Should contain [REDACTED]
				if !strings.Contains(sanitizedStr, "[REDACTED]") {
					t.Errorf("Expected sanitized payload to contain [REDACTED], got: %s", sanitizedStr)
				}

				// Should NOT contain the original secret patterns
				if strings.Contains(sanitizedStr, "ghp_") {
					t.Errorf("Sanitized payload still contains GitHub token")
				}
				if strings.Contains(sanitizedStr, "Bearer eyJ") {
					t.Errorf("Sanitized payload still contains Bearer token")
				}
				if strings.Contains(sanitizedStr, "supersecret") {
					t.Errorf("Sanitized payload still contains password")
				}
			} else {
				// Should not contain [REDACTED] for clean payloads
				if strings.Contains(sanitizedStr, "[REDACTED]") {
					t.Errorf("Clean payload should not be redacted, got: %s", sanitizedStr)
				}
			}
		})
	}
}

func TestSanitizeJSONWithNestedStructures(t *testing.T) {
	input := `{
		"params": {
			"credentials": {
				"apiKey": "test_fake_api_key_1234567890abcdefghij",
				"token": "ghp_1234567890123456789012345678901234567890"
			},
			"data": {
				"items": [
					{"name": "item1", "secret": "password123"},
					{"name": "item2", "value": "safe"}
				]
			}
		}
	}`

	result := SanitizeJSON([]byte(input))

	// The result is already a sanitized string
	sanitizedStr := string(result)

	// Should redact all secrets at all levels
	if !strings.Contains(sanitizedStr, "[REDACTED]") {
		t.Errorf("Expected [REDACTED] in sanitized output")
	}

	// Should NOT contain original secrets
	if strings.Contains(sanitizedStr, "test_fake_api_key") {
		t.Errorf("API key not sanitized")
	}
	if strings.Contains(sanitizedStr, "ghp_") {
		t.Errorf("GitHub token not sanitized")
	}
	if strings.Contains(sanitizedStr, "password123") {
		t.Errorf("Password not sanitized")
	}

	// Should preserve non-secret values
	if !strings.Contains(sanitizedStr, "item1") {
		t.Errorf("Non-secret value 'item1' was lost")
	}
	if !strings.Contains(sanitizedStr, "safe") {
		t.Errorf("Non-secret value 'safe' was lost")
	}
}

func TestSanitizeJSONCompactsMultiline(t *testing.T) {
	// Test that multi-line JSON is compacted to a single line
	multilineJSON := `{
		"jsonrpc": "2.0",
		"method": "test",
		"params": {
			"nested": {
				"value": "test"
			}
		}
	}`

	result := SanitizeJSON([]byte(multilineJSON))

	// The result should not contain newlines
	resultStr := string(result)
	if strings.Contains(resultStr, "\n") {
		t.Errorf("Result contains newlines, should be single-line JSON: %s", resultStr)
	}

	// Should still be valid JSON
	var tmp interface{}
	if err := json.Unmarshal(result, &tmp); err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}

	// Should contain the expected values
	if !strings.Contains(resultStr, "jsonrpc") || !strings.Contains(resultStr, "test") {
		t.Errorf("Result missing expected content: %s", resultStr)
	}
}

func TestSanitizeJSONWithInvalidJSON(t *testing.T) {
	invalidJSON := `{invalid json}`

	result := SanitizeJSON([]byte(invalidJSON))

	// Should return a valid JSON object with error marker
	var payloadObj map[string]interface{}
	if err := json.Unmarshal(result, &payloadObj); err != nil {
		t.Fatalf("Failed to parse result as JSON: %v", err)
	}

	if payloadObj["_error"] != "invalid JSON" {
		t.Errorf("Expected _error field in result, got: %v", payloadObj)
	}

	if !strings.Contains(payloadObj["_raw"].(string), "invalid") {
		t.Errorf("Expected _raw field to contain original invalid JSON, got: %v", payloadObj["_raw"])
	}
}

func TestAllSecretPatterns(t *testing.T) {
	// Test all 10 secret patterns
	testCases := []struct {
		pattern string
		example string
	}{
		{
			pattern: "token/key/secret/password/auth",
			example: "token=secretvalue12345678",
		},
		{
			pattern: "GitHub PAT",
			example: "ghp_1234567890123456789012345678901234567890",
		},
		{
			pattern: "GitHub fine-grained PAT",
			example: "github_pat_1234567890123456789012_12345678901234567890123456789012345678901234567890123456789",
		},
		{
			pattern: "Bearer token",
			example: "Bearer abcdefghijklmnopqrstuvwxyz1234567890",
		},
		{
			pattern: "Authorization header",
			example: "authorization: Basic dXNlcjpwYXNz",
		},
		{
			pattern: "Long hex string",
			example: "abcdef1234567890abcdef1234567890abcdef12",
		},
		{
			pattern: "API key",
			example: "apikey=test_key_12345678",
		},
		{
			pattern: "OAuth secret",
			example: "client_secret=oauth_secret_12345678",
		},
		{
			pattern: "JWT token",
			example: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature",
		},
		{
			pattern: "JSON secret field",
			example: `"password":"mysecretpassword"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.pattern, func(t *testing.T) {
			result := SanitizeString(tc.example)
			if !strings.Contains(result, "[REDACTED]") {
				t.Errorf("Pattern %s not detected in: %s, got: %s", tc.pattern, tc.example, result)
			}
		})
	}
}
