package logger

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTruncateAndSanitize(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		wantLen   int // Expected length (may be less due to sanitization)
		wantRedacted bool
	}{
		{
			name:      "short message without secrets",
			input:     "Hello, world!",
			maxLength: 50,
			wantLen:   13,
			wantRedacted: false,
		},
		{
			name:      "long message gets truncated",
			input:     `{"method":"test","data":"` + strings.Repeat("x", 200) + `"}`,
			maxLength: 100,
			wantLen:   103, // 100 + "..."
			wantRedacted: false,
		},
		{
			name:      "message with token gets sanitized",
			input:     "Authorization: ghp_1234567890123456789012345678901234567890",
			maxLength: 150,
			wantLen:   -1, // Variable due to redaction
			wantRedacted: true,
		},
		{
			name:      "message with password gets sanitized",
			input:     "password=supersecretpassword123",
			maxLength: 150,
			wantLen:   -1, // Variable due to redaction
			wantRedacted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateAndSanitize(tt.input, tt.maxLength)
			
			if tt.wantRedacted {
				if !strings.Contains(result, "[REDACTED]") {
					t.Errorf("Expected result to contain [REDACTED], got: %s", result)
				}
			} else {
				if tt.wantLen > 0 && len(result) != tt.wantLen {
					t.Errorf("Expected length %d, got %d: %s", tt.wantLen, len(result), result)
				}
			}
			
			// Ensure result is not longer than maxLength + 3 (for "...")
			if !tt.wantRedacted && len(result) > tt.maxLength+3 {
				t.Errorf("Result too long: %d > %d", len(result), tt.maxLength+3)
			}
		})
	}
}

func TestExtractEssentialFields(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		wantKeys []string
	}{
		{
			name:     "JSON-RPC request",
			payload:  `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
			wantKeys: []string{"jsonrpc", "id", "method", "params_keys"},
		},
		{
			name:     "JSON-RPC response with result",
			payload:  `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`,
			wantKeys: []string{"jsonrpc", "id"},
		},
		{
			name:     "JSON-RPC response with error",
			payload:  `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid request"}}`,
			wantKeys: []string{"jsonrpc", "id", "error"},
		},
		{
			name:     "invalid JSON",
			payload:  `{invalid json}`,
			wantKeys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEssentialFields([]byte(tt.payload))
			
			if tt.wantKeys == nil {
				if result != nil {
					t.Errorf("Expected nil result for invalid JSON, got: %v", result)
				}
				return
			}
			
			if result == nil {
				t.Fatalf("Expected result map, got nil")
			}
			
			for _, key := range tt.wantKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("Expected key %s not found in result: %v", key, result)
				}
			}
		})
	}
}

func TestFormatRPCMessage(t *testing.T) {
	tests := []struct {
		name string
		info *RPCMessageInfo
		want []string // Strings that should be present in output
	}{
		{
			name: "outbound request",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionOutbound,
				MessageType: RPCMessageRequest,
				ServerID:    "github",
				Method:      "tools/list",
				PayloadSize: 50,
				Payload:     `{"jsonrpc":"2.0","method":"tools/list"}`,
			},
			want: []string{"[OUT]", "[REQUEST]", "server=github", "method=tools/list", "size=50", "payload="},
		},
		{
			name: "inbound response with error",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionInbound,
				MessageType: RPCMessageResponse,
				ServerID:    "github",
				PayloadSize: 100,
				Payload:     `{"jsonrpc":"2.0","error":{"code":-32600}}`,
				Error:       "Invalid request",
			},
			want: []string{"[IN]", "[RESPONSE]", "server=github", "size=100", "error=Invalid request"},
		},
		{
			name: "client request",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionInbound,
				MessageType: RPCMessageRequest,
				ServerID:    "client",
				Method:      "tools/call",
				PayloadSize: 200,
				Payload:     `{"method":"tools/call","params":{}}`,
			},
			want: []string{"[IN]", "[REQUEST]", "server=client", "method=tools/call", "size=200"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRPCMessage(tt.info)
			
			for _, expected := range tt.want {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}

func TestFormatRPCMessageMarkdown(t *testing.T) {
	tests := []struct {
		name string
		info *RPCMessageInfo
		want []string // Strings that should be present in output
	}{
		{
			name: "outbound request",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionOutbound,
				MessageType: RPCMessageRequest,
				ServerID:    "github",
				Method:      "tools/list",
				PayloadSize: 50,
				Payload:     `{"jsonrpc":"2.0"}`,
			},
			want: []string{"Server **github**", "→", "`tools/list`", "(50 bytes)", "```"},
		},
		{
			name: "inbound response",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionInbound,
				MessageType: RPCMessageResponse,
				ServerID:    "github",
				PayloadSize: 100,
				Payload:     `{"result":{}}`,
			},
			want: []string{"Server **github**", "←", "RESPONSE", "(100 bytes)", "```"},
		},
		{
			name: "response with error",
			info: &RPCMessageInfo{
				Direction:   RPCDirectionInbound,
				MessageType: RPCMessageResponse,
				ServerID:    "github",
				PayloadSize: 100,
				Error:       "Connection timeout",
			},
			want: []string{"Server **github**", "Error: Connection timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRPCMessageMarkdown(tt.info)
			
			for _, expected := range tt.want {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}

func TestLogRPCRequest(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	
	// Initialize both loggers
	if err := InitFileLogger(logDir, "test.log"); err != nil {
		t.Fatalf("InitFileLogger failed: %v", err)
	}
	defer CloseGlobalLogger()
	
	if err := InitMarkdownLogger(logDir, "test.md"); err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}
	defer CloseMarkdownLogger()
	
	// Log an RPC request
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	LogRPCRequest(RPCDirectionOutbound, "github", "tools/list", payload)
	
	// Close loggers to flush
	CloseGlobalLogger()
	CloseMarkdownLogger()
	
	// Check text log
	textLog := filepath.Join(logDir, "test.log")
	textContent, err := os.ReadFile(textLog)
	if err != nil {
		t.Fatalf("Failed to read text log: %v", err)
	}
	
	textStr := string(textContent)
	expectedInText := []string{"[OUT]", "[REQUEST]", "server=github", "method=tools/list", "size="}
	for _, expected := range expectedInText {
		if !strings.Contains(textStr, expected) {
			t.Errorf("Text log does not contain %q", expected)
		}
	}
	
	// Check markdown log
	mdLog := filepath.Join(logDir, "test.md")
	mdContent, err := os.ReadFile(mdLog)
	if err != nil {
		t.Fatalf("Failed to read markdown log: %v", err)
	}
	
	mdStr := string(mdContent)
	expectedInMd := []string{"Server **github**", "→", "`tools/list`", "bytes"}
	for _, expected := range expectedInMd {
		if !strings.Contains(mdStr, expected) {
			t.Errorf("Markdown log does not contain %q", expected)
		}
	}
}

func TestLogRPCResponse(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	
	// Initialize both loggers
	if err := InitFileLogger(logDir, "test.log"); err != nil {
		t.Fatalf("InitFileLogger failed: %v", err)
	}
	defer CloseGlobalLogger()
	
	if err := InitMarkdownLogger(logDir, "test.md"); err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}
	defer CloseMarkdownLogger()
	
	// Log an RPC response with error
	payload := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid request"}}`)
	err := errors.New("backend connection failed")
	LogRPCResponse(RPCDirectionInbound, "github", payload, err)
	
	// Close loggers to flush
	CloseGlobalLogger()
	CloseMarkdownLogger()
	
	// Check text log
	textLog := filepath.Join(logDir, "test.log")
	textContent, err := os.ReadFile(textLog)
	if err != nil {
		t.Fatalf("Failed to read text log: %v", err)
	}
	
	textStr := string(textContent)
	expectedInText := []string{"[IN]", "[RESPONSE]", "server=github", "error=backend connection failed"}
	for _, expected := range expectedInText {
		if !strings.Contains(textStr, expected) {
			t.Errorf("Text log does not contain %q", expected)
		}
	}
	
	// Check markdown log
	mdLog := filepath.Join(logDir, "test.md")
	mdContent, err := os.ReadFile(mdLog)
	if err != nil {
		t.Fatalf("Failed to read markdown log: %v", err)
	}
	
	mdStr := string(mdContent)
	expectedInMd := []string{"Server **github**", "←", "Error: backend connection failed"}
	for _, expected := range expectedInMd {
		if !strings.Contains(mdStr, expected) {
			t.Errorf("Markdown log does not contain %q", expected)
		}
	}
}

func TestLogRPCRequestWithSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	
	// Initialize both loggers
	if err := InitFileLogger(logDir, "test.log"); err != nil {
		t.Fatalf("InitFileLogger failed: %v", err)
	}
	defer CloseGlobalLogger()
	
	if err := InitMarkdownLogger(logDir, "test.md"); err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}
	defer CloseMarkdownLogger()
	
	// Log an RPC request with a secret
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"authenticate","params":{"token":"ghp_1234567890123456789012345678901234567890"}}`)
	LogRPCRequest(RPCDirectionInbound, "client", "authenticate", payload)
	
	// Close loggers to flush
	CloseGlobalLogger()
	CloseMarkdownLogger()
	
	// Check text log - should NOT contain the actual token
	textLog := filepath.Join(logDir, "test.log")
	textContent, err := os.ReadFile(textLog)
	if err != nil {
		t.Fatalf("Failed to read text log: %v", err)
	}
	
	textStr := string(textContent)
	if strings.Contains(textStr, "ghp_1234567890123456789012345678901234567890") {
		t.Errorf("Text log contains secret that should be redacted")
	}
	if !strings.Contains(textStr, "[REDACTED]") {
		t.Errorf("Text log does not contain [REDACTED] marker")
	}
	
	// Check markdown log - should NOT contain the actual token
	mdLog := filepath.Join(logDir, "test.md")
	mdContent, err := os.ReadFile(mdLog)
	if err != nil {
		t.Fatalf("Failed to read markdown log: %v", err)
	}
	
	mdStr := string(mdContent)
	if strings.Contains(mdStr, "ghp_1234567890123456789012345678901234567890") {
		t.Errorf("Markdown log contains secret that should be redacted")
	}
	if !strings.Contains(mdStr, "[REDACTED]") {
		t.Errorf("Markdown log does not contain [REDACTED] marker")
	}
}

func TestLogRPCRequestPayloadTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	
	// Initialize both loggers
	if err := InitFileLogger(logDir, "test.log"); err != nil {
		t.Fatalf("InitFileLogger failed: %v", err)
	}
	defer CloseGlobalLogger()
	
	if err := InitMarkdownLogger(logDir, "test.md"); err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}
	defer CloseMarkdownLogger()
	
	// Create a large payload (> 120 characters)
	largeData := strings.Repeat("x", 200)
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"test","params":{"data":"` + largeData + `"}}`)
	LogRPCRequest(RPCDirectionOutbound, "backend", "test", payload)
	
	// Close loggers to flush
	CloseGlobalLogger()
	CloseMarkdownLogger()
	
	// Check text log - payload should be truncated
	textLog := filepath.Join(logDir, "test.log")
	textContent, err := os.ReadFile(textLog)
	if err != nil {
		t.Fatalf("Failed to read text log: %v", err)
	}
	
	textStr := string(textContent)
	if !strings.Contains(textStr, "...") {
		t.Errorf("Text log does not show truncation marker")
	}
	
	// The logged payload should not contain the full 200 x's
	// (it should be truncated to 120 chars + "...")
	xCount := strings.Count(textStr, strings.Repeat("x", 150))
	if xCount > 0 {
		t.Errorf("Text log contains more data than expected after truncation")
	}
}
