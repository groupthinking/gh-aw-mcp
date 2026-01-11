package logger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitJSONLLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	// Test successful initialization
	err := InitJSONLLogger(logDir, "test.jsonl")
	if err != nil {
		t.Fatalf("InitJSONLLogger failed: %v", err)
	}
	defer CloseJSONLLogger()

	// Verify log file was created
	logPath := filepath.Join(logDir, "test.jsonl")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %s", logPath)
	}
}

func TestJSONLLoggerClose(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	if err := InitJSONLLogger(logDir, "test.jsonl"); err != nil {
		t.Fatalf("InitJSONLLogger failed: %v", err)
	}

	// Test closing
	if err := CloseJSONLLogger(); err != nil {
		t.Errorf("CloseJSONLLogger failed: %v", err)
	}

	// Test closing again (should not error)
	if err := CloseJSONLLogger(); err != nil {
		t.Errorf("CloseJSONLLogger failed on second call: %v", err)
	}
}

func TestLogRPCMessageJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	if err := InitJSONLLogger(logDir, "test.jsonl"); err != nil {
		t.Fatalf("InitJSONLLogger failed: %v", err)
	}
	defer CloseJSONLLogger()

	// Log a request
	requestPayload := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	LogRPCMessageJSONL(RPCDirectionOutbound, RPCMessageRequest, "github", "tools/list", requestPayload, nil)

	// Log a response
	responsePayload := []byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`)
	LogRPCMessageJSONL(RPCDirectionInbound, RPCMessageResponse, "github", "", responsePayload, nil)

	// Close to flush
	CloseJSONLLogger()

	// Read and verify the log file
	logPath := filepath.Join(logDir, "test.jsonl")
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		var entry JSONLRPCMessage
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Failed to parse JSONL line %d: %v, line: %s", lineCount, err, line)
			continue
		}

		// Verify common fields
		if entry.Timestamp == "" {
			t.Errorf("Line %d: missing timestamp", lineCount)
		}
		if entry.Direction == "" {
			t.Errorf("Line %d: missing direction", lineCount)
		}
		if entry.Type == "" {
			t.Errorf("Line %d: missing type", lineCount)
		}
		if entry.ServerID == "" {
			t.Errorf("Line %d: missing server_id", lineCount)
		}
		if entry.Payload == nil {
			t.Errorf("Line %d: missing payload", lineCount)
		}

		// Verify line-specific fields
		if lineCount == 1 {
			// First line should be a REQUEST
			if entry.Type != "REQUEST" {
				t.Errorf("Line 1: expected type REQUEST, got %s", entry.Type)
			}
			if entry.Method != "tools/list" {
				t.Errorf("Line 1: expected method tools/list, got %s", entry.Method)
			}
			if entry.Direction != "OUT" {
				t.Errorf("Line 1: expected direction OUT, got %s", entry.Direction)
			}
		} else if lineCount == 2 {
			// Second line should be a RESPONSE
			if entry.Type != "RESPONSE" {
				t.Errorf("Line 2: expected type RESPONSE, got %s", entry.Type)
			}
			if entry.Direction != "IN" {
				t.Errorf("Line 2: expected direction IN, got %s", entry.Direction)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}

	if lineCount != 2 {
		t.Errorf("Expected 2 log entries, got %d", lineCount)
	}
}

func TestSanitizePayload(t *testing.T) {
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
			result := sanitizePayload([]byte(tt.input))

			if result == nil {
				t.Fatalf("sanitizePayload returned nil")
			}

			// Convert back to JSON to check if it contains secrets
			sanitized, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("Failed to marshal sanitized payload: %v", err)
			}

			sanitizedStr := string(sanitized)

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

func TestSanitizePayloadWithNestedStructures(t *testing.T) {
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

	result := sanitizePayload([]byte(input))
	sanitized, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal sanitized payload: %v", err)
	}

	sanitizedStr := string(sanitized)

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

func TestLogRPCMessageJSONLWithError(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	if err := InitJSONLLogger(logDir, "test.jsonl"); err != nil {
		t.Fatalf("InitJSONLLogger failed: %v", err)
	}
	defer CloseJSONLLogger()

	// Log a response with error
	responsePayload := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid request"}}`)
	testErr := fmt.Errorf("backend connection failed")
	LogRPCMessageJSONL(RPCDirectionInbound, RPCMessageResponse, "github", "", responsePayload, testErr)

	// Close to flush
	CloseJSONLLogger()

	// Read and verify
	logPath := filepath.Join(logDir, "test.jsonl")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var entry JSONLRPCMessage
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("Failed to parse JSONL: %v", err)
	}

	if entry.Error != "backend connection failed" {
		t.Errorf("Expected error 'backend connection failed', got: %s", entry.Error)
	}
}

func TestLogRPCMessageJSONLWithInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	if err := InitJSONLLogger(logDir, "test.jsonl"); err != nil {
		t.Fatalf("InitJSONLLogger failed: %v", err)
	}
	defer CloseJSONLLogger()

	// Log invalid JSON
	invalidPayload := []byte(`{invalid json}`)
	LogRPCMessageJSONL(RPCDirectionOutbound, RPCMessageRequest, "github", "test", invalidPayload, nil)

	// Close to flush
	CloseJSONLLogger()

	// Read and verify
	logPath := filepath.Join(logDir, "test.jsonl")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var entry JSONLRPCMessage
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("Failed to parse JSONL: %v", err)
	}

	// Should have an error field in payload indicating parse failure
	if entry.Payload["_error"] != "failed to parse JSON" {
		t.Errorf("Expected parse error in payload, got: %v", entry.Payload)
	}
}

func TestJSONLLoggerNotInitialized(t *testing.T) {
	// Ensure no global logger is set
	CloseJSONLLogger()

	// Should not panic when logging without initialization
	requestPayload := []byte(`{"jsonrpc":"2.0","id":1,"method":"test"}`)
	LogRPCMessageJSONL(RPCDirectionOutbound, RPCMessageRequest, "github", "test", requestPayload, nil)
	// Test passes if no panic occurs
}

func TestMultipleMessagesInJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	if err := InitJSONLLogger(logDir, "test.jsonl"); err != nil {
		t.Fatalf("InitJSONLLogger failed: %v", err)
	}
	defer CloseJSONLLogger()

	// Log multiple messages
	messages := []struct {
		direction RPCMessageDirection
		msgType   RPCMessageType
		serverID  string
		method    string
		payload   string
	}{
		{RPCDirectionOutbound, RPCMessageRequest, "github", "tools/list", `{"jsonrpc":"2.0","method":"tools/list"}`},
		{RPCDirectionInbound, RPCMessageResponse, "github", "", `{"jsonrpc":"2.0","result":{}}`},
		{RPCDirectionOutbound, RPCMessageRequest, "backend", "tools/call", `{"jsonrpc":"2.0","method":"tools/call"}`},
		{RPCDirectionInbound, RPCMessageResponse, "backend", "", `{"jsonrpc":"2.0","result":{}}`},
	}

	for _, msg := range messages {
		LogRPCMessageJSONL(msg.direction, msg.msgType, msg.serverID, msg.method, []byte(msg.payload), nil)
	}

	// Close to flush
	CloseJSONLLogger()

	// Read and verify all lines
	logPath := filepath.Join(logDir, "test.jsonl")
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		var entry JSONLRPCMessage
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Failed to parse JSONL line %d: %v", lineCount, err)
			continue
		}

		// Each line should be valid JSONL
		if entry.Timestamp == "" || entry.ServerID == "" {
			t.Errorf("Line %d: incomplete entry", lineCount)
		}
	}

	if lineCount != len(messages) {
		t.Errorf("Expected %d log entries, got %d", len(messages), lineCount)
	}
}
