package sanitize

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		shouldRedact   bool
		mustNotContain string
	}{
		{
			name:           "GitHub PAT",
			input:          "token=ghp_1234567890123456789012345678901234567890",
			shouldRedact:   true,
			mustNotContain: "ghp_1234567890123456789012345678901234567890",
		},
		{
			name:           "GitHub fine-grained PAT",
			input:          "token=github_pat_1234567890123456789012_1234567890123456789012345678901234567890123456789012345678901234",
			shouldRedact:   true,
			mustNotContain: "github_pat_",
		},
		{
			name:           "Bearer token",
			input:          "Authorization: Bearer abcdefghijklmnopqrstuvwxyz",
			shouldRedact:   true,
			mustNotContain: "Bearer abcdefghijklmnopqrstuvwxyz",
		},
		{
			name:           "API key with equals",
			input:          "API_KEY=sk_test_abcdefghijklmnopqrstuvwxyz123456",
			shouldRedact:   true,
			mustNotContain: "sk_test_abcdefghijklmnopqrstuvwxyz123456",
		},
		{
			name:           "Password with colon",
			input:          "password: supersecretpassword123",
			shouldRedact:   true,
			mustNotContain: "supersecretpassword123",
		},
		{
			name:           "JWT token",
			input:          "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			shouldRedact:   true,
			mustNotContain: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:           "Long hex string",
			input:          "key=abcdef1234567890abcdef1234567890abcdef12",
			shouldRedact:   true,
			mustNotContain: "abcdef1234567890abcdef1234567890abcdef12",
		},
		{
			name:           "OAuth client secret",
			input:          "client_secret=cs_test_1234567890abcdefghij",
			shouldRedact:   true,
			mustNotContain: "cs_test_1234567890abcdefghij",
		},
		{
			name:           "JSON token field",
			input:          `{"token":"ghp_1234567890123456789012345678901234567890"}`,
			shouldRedact:   true,
			mustNotContain: "ghp_1234567890123456789012345678901234567890",
		},
		{
			name:           "JSON password field",
			input:          `{"password":"mysecretpassword"}`,
			shouldRedact:   true,
			mustNotContain: "mysecretpassword",
		},
		{
			name:         "Normal message without secrets",
			input:        "Normal log message without secrets",
			shouldRedact: false,
		},
		{
			name:         "Message with short password-like word",
			input:        "password for this feature is supported",
			shouldRedact: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)

			if tt.shouldRedact {
				// Should contain [REDACTED]
				if !strings.Contains(result, "[REDACTED]") {
					t.Errorf("Expected sanitized string to contain [REDACTED], got: %s", result)
				}

				// Should NOT contain the secret
				if tt.mustNotContain != "" && strings.Contains(result, tt.mustNotContain) {
					t.Errorf("Sanitized string still contains secret: %s", tt.mustNotContain)
				}
			} else {
				// Should not be modified
				if result != tt.input {
					t.Errorf("Clean message was modified. Input: %s, Output: %s", tt.input, result)
				}
			}
		})
	}
}

func TestSanitizeStringPreservesPrefix(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedStart string
	}{
		{
			name:          "Equals separator",
			input:         "token=ghp_1234567890123456789012345678901234567890",
			expectedStart: "token=",
		},
		{
			name:          "Colon separator",
			input:         "API_KEY: sk_test_abcdefghijklmnopqrstuvwxyz123456",
			expectedStart: "API_KEY",
		},
		{
			name:          "Password with colon and space",
			input:         "password: supersecret",
			expectedStart: "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)

			if !strings.Contains(result, tt.expectedStart) {
				t.Errorf("Expected result to contain prefix '%s', got: %s", tt.expectedStart, result)
			}

			if !strings.Contains(result, "[REDACTED]") {
				t.Errorf("Expected result to contain [REDACTED], got: %s", result)
			}
		})
	}
}

func TestSanitizeJSON(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectRedacted bool
		mustNotContain string
	}{
		{
			name:           "JSON with token field",
			input:          `{"token":"ghp_1234567890123456789012345678901234567890"}`,
			expectRedacted: true,
			mustNotContain: "ghp_",
		},
		{
			name:           "Nested JSON with auth",
			input:          `{"params":{"auth":"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.sig"}}`,
			expectRedacted: true,
			mustNotContain: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:           "JSON with password field",
			input:          `{"password":"supersecret123"}`,
			expectRedacted: true,
			mustNotContain: "supersecret123",
		},
		{
			name:           "Clean JSON payload",
			input:          `{"method":"tools/list","id":1}`,
			expectRedacted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeJSON([]byte(tt.input))

			if result == nil {
				t.Fatalf("SanitizeJSON returned nil")
			}

			resultStr := string(result)

			if tt.expectRedacted {
				// Should contain [REDACTED]
				if !strings.Contains(resultStr, "[REDACTED]") {
					t.Errorf("Expected sanitized JSON to contain [REDACTED], got: %s", resultStr)
				}

				// Should NOT contain the original secret
				if tt.mustNotContain != "" && strings.Contains(resultStr, tt.mustNotContain) {
					t.Errorf("Sanitized JSON still contains secret: %s", tt.mustNotContain)
				}
			} else {
				// Should not contain [REDACTED] for clean payloads
				if strings.Contains(resultStr, "[REDACTED]") {
					t.Errorf("Clean payload should not be redacted, got: %s", resultStr)
				}
			}

			// Result should be valid JSON
			var tmp interface{}
			if err := json.Unmarshal(result, &tmp); err != nil {
				t.Errorf("Result is not valid JSON: %v", err)
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
	resultStr := string(result)

	// Should redact all secrets at all levels
	if !strings.Contains(resultStr, "[REDACTED]") {
		t.Errorf("Expected [REDACTED] in sanitized output")
	}

	// Should NOT contain original secrets
	secrets := []string{
		"test_fake_api_key_1234567890abcdefghij",
		"ghp_1234567890123456789012345678901234567890",
		"password123",
	}
	for _, secret := range secrets {
		if strings.Contains(resultStr, secret) {
			t.Errorf("Secret not sanitized: %s", secret)
		}
	}

	// Should preserve non-secret values
	if !strings.Contains(resultStr, "item1") {
		t.Errorf("Non-secret value 'item1' was lost")
	}
	if !strings.Contains(resultStr, "safe") {
		t.Errorf("Non-secret value 'safe' was lost")
	}

	// Result should be valid JSON
	var tmp interface{}
	if err := json.Unmarshal(result, &tmp); err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}
}

func TestSanitizeJSONCompactsMultiline(t *testing.T) {
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
	resultStr := string(result)

	// Should not contain newlines
	if strings.Contains(resultStr, "\n") {
		t.Errorf("Result contains newlines, should be single-line JSON: %s", resultStr)
	}

	// Should still be valid JSON
	var tmp interface{}
	if err := json.Unmarshal(result, &tmp); err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}

	// Should contain expected values
	if !strings.Contains(resultStr, "jsonrpc") || !strings.Contains(resultStr, "test") {
		t.Errorf("Result missing expected content: %s", resultStr)
	}
}

func TestSanitizeJSONWithInvalidJSON(t *testing.T) {
	invalidJSON := `{invalid json}`

	result := SanitizeJSON([]byte(invalidJSON))

	// Should still return valid JSON (wrapped)
	var payloadObj map[string]interface{}
	if err := json.Unmarshal(result, &payloadObj); err != nil {
		t.Fatalf("Failed to parse result as JSON: %v", err)
	}

	// Should have error marker
	if payloadObj["_error"] != "invalid JSON" {
		t.Errorf("Expected _error field in result, got: %v", payloadObj)
	}

	// Should preserve original content in _raw field
	if !strings.Contains(fmt.Sprintf("%v", payloadObj["_raw"]), "invalid") {
		t.Errorf("Expected _raw field to contain original content, got: %v", payloadObj["_raw"])
	}
}

func TestSanitizeStringMultipleSecretsInSameString(t *testing.T) {
	input := "token=ghp_123456789012345678901234567890123456 password=mysecret apikey=sk_test_1234567890"

	result := SanitizeString(input)

	// Should redact all secrets
	secretCount := strings.Count(result, "[REDACTED]")
	if secretCount < 3 {
		t.Errorf("Expected at least 3 [REDACTED] markers, got %d in: %s", secretCount, result)
	}

	// Should not contain any of the secrets
	secrets := []string{"ghp_", "mysecret", "sk_test_"}
	for _, secret := range secrets {
		if strings.Contains(result, secret) {
			t.Errorf("Secret not sanitized: %s", secret)
		}
	}
}

func TestSecretPatternsCount(t *testing.T) {
	// Verify we have all 10 patterns as documented
	expectedPatternCount := 10
	actualCount := len(SecretPatterns)

	if actualCount != expectedPatternCount {
		t.Errorf("Expected %d secret patterns, got %d", expectedPatternCount, actualCount)
	}
}
