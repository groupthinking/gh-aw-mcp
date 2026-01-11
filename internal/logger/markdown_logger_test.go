package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitMarkdownLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "test.md"

	err := InitMarkdownLogger(logDir, fileName)
	if err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}
	defer CloseMarkdownLogger()

	// Check that the log directory was created
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("Log directory was not created: %s", logDir)
	}

	// Check that the log file was created
	logPath := filepath.Join(logDir, fileName)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %s", logPath)
	}
}

func TestMarkdownLoggerFormatting(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "format-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	if err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}

	// Write messages at different levels
	LogInfoMd("test", "This is an info message")
	LogWarnMd("test", "This is a warning message")
	LogErrorMd("test", "This is an error message")
	LogDebugMd("test", "This is a debug message")

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Check for HTML details wrapper
	if !strings.Contains(logContent, "<details>") {
		t.Errorf("Log file does not contain opening <details> tag")
	}
	if !strings.Contains(logContent, "<summary>MCP Gateway</summary>") {
		t.Errorf("Log file does not contain summary tag")
	}
	if !strings.Contains(logContent, "</details>") {
		t.Errorf("Log file does not contain closing </details> tag")
	}

	// Check for emoji bullet points
	expectedEmojis := []struct {
		emoji   string
		message string
	}{
		{"✓", "This is an info message"},
		{"⚠️", "This is a warning message"},
		{"✗", "This is an error message"},
		{"🔍", "This is a debug message"},
	}

	for _, expected := range expectedEmojis {
		if !strings.Contains(logContent, expected.emoji) {
			t.Errorf("Log file does not contain emoji: %s", expected.emoji)
		}
		if !strings.Contains(logContent, expected.message) {
			t.Errorf("Log file does not contain message: %s", expected.message)
		}
	}

	// Check for markdown bullet points
	if !strings.Contains(logContent, "- ✓") {
		t.Errorf("Log file does not contain markdown bullet points")
	}
}

func TestMarkdownLoggerSecretSanitization(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "secret-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	if err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}

	// Test various secret patterns
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"token=ghp_1234567890123456789012345678901234567890",
			"[REDACTED]",
		},
		{
			"API_KEY=sk_test_abcdefghijklmnopqrstuvwxyz123456",
			"[REDACTED]",
		},
		{
			"password: supersecretpassword123",
			"[REDACTED]",
		},
		{
			"Normal log message without secrets",
			"Normal log message without secrets",
		},
		{
			"Authorization: Bearer abcdefghijklmnopqrstuvwxyz",
			"[REDACTED]",
		},
	}

	for _, tc := range testCases {
		LogInfoMd("test", "%s", tc.input)
	}

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify secrets are redacted
	secretStrings := []string{
		"ghp_1234567890123456789012345678901234567890",
		"sk_test_abcdefghijklmnopqrstuvwxyz123456",
		"supersecretpassword123",
		"Bearer abcdefghijklmnopqrstuvwxyz",
	}

	for _, secret := range secretStrings {
		if strings.Contains(logContent, secret) {
			t.Errorf("Log file contains secret that should be redacted: %s", secret)
		}
	}

	// Verify redaction marker is present
	if !strings.Contains(logContent, "[REDACTED]") {
		t.Errorf("Log file does not contain [REDACTED] marker")
	}

	// Verify normal message is not redacted
	if !strings.Contains(logContent, "Normal log message without secrets") {
		t.Errorf("Log file does not contain non-secret message")
	}
}

func TestMarkdownLoggerCategories(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "category-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	if err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}

	// Log messages with different categories
	categories := []string{"startup", "client", "backend", "shutdown"}
	for _, category := range categories {
		LogInfoMd(category, "Message for category %s", category)
	}

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify all categories are present
	for _, category := range categories {
		if !strings.Contains(logContent, category) {
			t.Errorf("Log file does not contain category: %s", category)
		}
	}
}

func TestMarkdownLoggerConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "concurrent-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	if err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}
	defer CloseMarkdownLogger()

	// Write from multiple goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				LogInfoMd("concurrent", "Message from goroutine %d, iteration %d", id, j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Count the number of log lines (each starts with "- ✓")
	lines := strings.Count(logContent, "- ✓")
	// Should have 100 lines (10 goroutines * 10 messages each)
	expectedLines := 100
	if lines != expectedLines {
		t.Errorf("Expected %d log lines, got %d", expectedLines, lines)
	}
}

func TestMarkdownLoggerCodeBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "codeblock-test.md"

	err := InitMarkdownLogger(logDir, fileName)
	if err != nil {
		t.Fatalf("InitMarkdownLogger failed: %v", err)
	}

	// Log messages with technical content that should use code blocks
	LogInfoMd("test", "command=/usr/bin/docker args=[run --rm -i container]")
	LogInfoMd("test", "Multi-line\ncontent\nhere")
	LogInfoMd("test", "Simple message")

	CloseMarkdownLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Check for code blocks for technical content
	codeBlockCount := strings.Count(logContent, "```")
	if codeBlockCount < 2 {
		t.Errorf("Expected at least 2 code block markers (opening and closing), got %d", codeBlockCount)
	}
}

func TestMarkdownLoggerFallback(t *testing.T) {
	// Use a non-writable directory
	logDir := "/root/nonexistent/directory"
	fileName := "test.md"

	// Initialize the logger - should not fail, but use fallback
	err := InitMarkdownLogger(logDir, fileName)
	if err != nil {
		t.Fatalf("InitMarkdownLogger should not fail on fallback: %v", err)
	}
	defer CloseMarkdownLogger()

	globalMarkdownMu.RLock()
	useFallback := globalMarkdownLogger.useFallback
	globalMarkdownMu.RUnlock()

	if !useFallback {
		t.Logf("Logger initialized without fallback (may have permissions)")
	}

	// Should not panic when logging
	LogInfoMd("test", "This should not crash")
}
