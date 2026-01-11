package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// captureStderr captures stderr output during test execution
func captureStderr(f func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		debugEnv  string
		namespace string
		enabled   bool
	}{
		{
			name:      "empty DEBUG disables all loggers",
			debugEnv:  "",
			namespace: "test:logger",
			enabled:   false,
		},
		{
			name:      "wildcard enables all loggers",
			debugEnv:  "*",
			namespace: "test:logger",
			enabled:   true,
		},
		{
			name:      "exact match enables logger",
			debugEnv:  "test:logger",
			namespace: "test:logger",
			enabled:   true,
		},
		{
			name:      "exact match different namespace disabled",
			debugEnv:  "test:logger",
			namespace: "other:logger",
			enabled:   false,
		},
		{
			name:      "namespace wildcard enables matching loggers",
			debugEnv:  "test:*",
			namespace: "test:logger",
			enabled:   true,
		},
		{
			name:      "namespace wildcard matches deeply nested",
			debugEnv:  "test:*",
			namespace: "test:sub:logger",
			enabled:   true,
		},
		{
			name:      "namespace wildcard does not match different prefix",
			debugEnv:  "test:*",
			namespace: "other:logger",
			enabled:   false,
		},
		{
			name:      "multiple patterns with comma",
			debugEnv:  "test:*,other:*",
			namespace: "test:logger",
			enabled:   true,
		},
		{
			name:      "multiple patterns second matches",
			debugEnv:  "test:*,other:*",
			namespace: "other:logger",
			enabled:   true,
		},
		{
			name:      "exclusion pattern disables specific logger",
			debugEnv:  "test:*,-test:skip",
			namespace: "test:skip",
			enabled:   false,
		},
		{
			name:      "exclusion does not affect other loggers",
			debugEnv:  "test:*,-test:skip",
			namespace: "test:logger",
			enabled:   true,
		},
		{
			name:      "exclusion with wildcard",
			debugEnv:  "*,-test:*",
			namespace: "test:logger",
			enabled:   false,
		},
		{
			name:      "exclusion with wildcard allows others",
			debugEnv:  "*,-test:*",
			namespace: "other:logger",
			enabled:   true,
		},
		{
			name:      "suffix wildcard",
			debugEnv:  "*:logger",
			namespace: "test:logger",
			enabled:   true,
		},
		{
			name:      "suffix wildcard no match",
			debugEnv:  "*:logger",
			namespace: "test:other",
			enabled:   false,
		},
		{
			name:      "middle wildcard",
			debugEnv:  "test:*:end",
			namespace: "test:middle:end",
			enabled:   true,
		},
		{
			name:      "middle wildcard no match prefix",
			debugEnv:  "test:*:end",
			namespace: "other:middle:end",
			enabled:   false,
		},
		{
			name:      "middle wildcard no match suffix",
			debugEnv:  "test:*:end",
			namespace: "test:middle:other",
			enabled:   false,
		},
		{
			name:      "spaces in patterns are trimmed",
			debugEnv:  "test:* , other:*",
			namespace: "other:logger",
			enabled:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use t.Setenv to set environment variable for this test
			t.Setenv("DEBUG", tt.debugEnv)

			logger := New(tt.namespace)
			if logger.Enabled() != tt.enabled {
				t.Errorf("New(%q) with DEBUG=%q: enabled = %v, want %v",
					tt.namespace, tt.debugEnv, logger.Enabled(), tt.enabled)
			}
		})
	}
}

func TestLogger_Printf(t *testing.T) {
	tests := []struct {
		name      string
		debugEnv  string
		namespace string
		format    string
		args      []any
		wantLog   bool
	}{
		{
			name:      "enabled logger prints",
			debugEnv:  "*",
			namespace: "test:logger",
			format:    "hello %s",
			args:      []any{"world"},
			wantLog:   true,
		},
		{
			name:      "disabled logger does not print",
			debugEnv:  "",
			namespace: "test:logger",
			format:    "hello %s",
			args:      []any{"world"},
			wantLog:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use t.Setenv to set environment variable for this test
			t.Setenv("DEBUG", tt.debugEnv)

			logger := New(tt.namespace)

			output := captureStderr(func() {
				logger.Printf(tt.format, tt.args...)
			})

			if tt.wantLog {
				if output == "" {
					t.Errorf("Printf() should have logged but got empty output")
				}
				if !strings.Contains(output, tt.namespace) {
					t.Errorf("Printf() output should contain namespace %q, got %q", tt.namespace, output)
				}
				expectedMessage := "hello world"
				if !strings.Contains(output, expectedMessage) {
					t.Errorf("Printf() output should contain %q, got %q", expectedMessage, output)
				}
			} else {
				if output != "" {
					t.Errorf("Printf() should not have logged but got %q", output)
				}
			}
		})
	}
}

func TestLogger_Print(t *testing.T) {
	// Use t.Setenv to set environment variable for this test
	t.Setenv("DEBUG", "*")

	logger := New("test:print")

	output := captureStderr(func() {
		logger.Print("hello", " ", "world")
	})

	if !strings.Contains(output, "test:print") {
		t.Errorf("Print() output should contain namespace, got %q", output)
	}
	if !strings.Contains(output, "hello world") {
		t.Errorf("Print() output should contain message, got %q", output)
	}
	// Check that time diff is included
	if !strings.Contains(output, "+") {
		t.Errorf("Print() output should contain time diff, got %q", output)
	}
}

func TestLogger_TimeDiff(t *testing.T) {
	// Use t.Setenv to set environment variable for this test
	t.Setenv("DEBUG", "*")

	logger := New("test:timediff")

	// First log
	output1 := captureStderr(func() {
		logger.Printf("first message")
	})

	// Small delay
	time.Sleep(10 * time.Millisecond)

	// Second log
	output2 := captureStderr(func() {
		logger.Printf("second message")
	})

	// Both should have time diff
	if !strings.Contains(output1, "+") {
		t.Errorf("First log should contain time diff, got %q", output1)
	}
	if !strings.Contains(output2, "+") {
		t.Errorf("Second log should contain time diff, got %q", output2)
	}

	// Second log should show at least 10ms diff
	if !strings.Contains(output2, "ms") && !strings.Contains(output2, "µs") {
		t.Errorf("Second log should show millisecond or microsecond time diff, got %q", output2)
	}
}

func TestColorSelection(t *testing.T) {
	// Test that selectColor returns consistent colors for the same namespace
	color1 := selectColor("test:namespace")
	color2 := selectColor("test:namespace")
	if color1 != color2 {
		t.Errorf("selectColor should return same color for same namespace")
	}

	// Test that different namespaces can get different colors
	// (not guaranteed but likely with our hash function)
	color3 := selectColor("other:namespace")
	// Just verify it's a valid color from palette or empty
	found := color3 == ""
	for _, c := range colorPalette {
		if color3 == c {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("selectColor returned invalid color: %q", color3)
	}
}

func TestColorDisabling(t *testing.T) {
	// Save original values
	origDebugColors := debugColors
	origIsTTY := isTTY
	defer func() {
		debugColors = origDebugColors
		isTTY = origIsTTY
	}()

	// Test with colors disabled via DEBUG_COLORS
	debugColors = false
	isTTY = true
	color := selectColor("test:namespace")
	if color != "" {
		t.Errorf("selectColor should return empty when debugColors=false, got %q", color)
	}

	// Test with TTY disabled
	debugColors = true
	isTTY = false
	color = selectColor("test:namespace")
	if color != "" {
		t.Errorf("selectColor should return empty when isTTY=false, got %q", color)
	}

	// Test with both enabled
	debugColors = true
	isTTY = true
	color = selectColor("test:namespace")
	if color == "" {
		t.Error("selectColor should return color when both enabled")
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		pattern   string
		want      bool
	}{
		{"exact match", "test:logger", "test:logger", true},
		{"no match", "test:logger", "other:logger", false},
		{"wildcard all", "test:logger", "*", true},
		{"prefix wildcard", "test:logger", "test:*", true},
		{"prefix wildcard no match", "test:logger", "other:*", false},
		{"suffix wildcard", "test:logger", "*:logger", true},
		{"suffix wildcard no match", "test:logger", "*:other", false},
		{"middle wildcard", "test:middle:logger", "test:*:logger", true},
		{"middle wildcard no match prefix", "other:middle:logger", "test:*:logger", false},
		{"middle wildcard no match suffix", "test:middle:other", "test:*:logger", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.namespace, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.namespace, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestComputeEnabled(t *testing.T) {
	tests := []struct {
		name      string
		debugEnv  string
		namespace string
		want      bool
	}{
		{"single pattern match", "test:*", "test:logger", true},
		{"single pattern no match", "test:*", "other:logger", false},
		{"multiple patterns first match", "test:*,other:*", "test:logger", true},
		{"multiple patterns second match", "test:*,other:*", "other:logger", true},
		{"multiple patterns no match", "test:*,other:*", "third:logger", false},
		{"exclusion disables", "test:*,-test:skip", "test:skip", false},
		{"exclusion allows others", "test:*,-test:skip", "test:logger", true},
		{"exclusion wildcard", "*,-test:*", "test:logger", false},
		{"exclusion wildcard allows", "*,-test:*", "other:logger", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use t.Setenv to set DEBUG for this test
			t.Setenv("DEBUG", tt.debugEnv)
			got := computeEnabled(tt.namespace)
			if got != tt.want {
				t.Errorf("computeEnabled(%q) with DEBUG=%q = %v, want %v",
					tt.namespace, tt.debugEnv, got, tt.want)
			}
		})
	}
}

func TestDebugLoggerWritesToFile(t *testing.T) {
	// Create a temporary directory for the file logger
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "debug-test.log"

	// Initialize the file logger
	err := InitFileLogger(logDir, fileName)
	if err != nil {
		t.Fatalf("InitFileLogger failed: %v", err)
	}
	defer CloseGlobalLogger()

	// Use t.Setenv to enable all debug loggers
	t.Setenv("DEBUG", "*")

	// Create a debug logger
	log := New("test:debug")

	// Capture stderr to verify stderr output
	stderrOutput := captureStderr(func() {
		log.Printf("Test message %d", 42)
		log.Print("Another test message")
	})

	// Verify stderr output contains the messages
	if !strings.Contains(stderrOutput, "Test message 42") {
		t.Errorf("Stderr should contain debug message, got: %s", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "Another test message") {
		t.Errorf("Stderr should contain debug message, got: %s", stderrOutput)
	}

	// Close the file logger to flush all data
	CloseGlobalLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify the file logger contains the same messages (text-only, no colors)
	if !strings.Contains(logContent, "Test message 42") {
		t.Errorf("Log file should contain debug message, got: %s", logContent)
	}
	if !strings.Contains(logContent, "Another test message") {
		t.Errorf("Log file should contain debug message, got: %s", logContent)
	}

	// Verify the file logger has DEBUG level
	if !strings.Contains(logContent, "[DEBUG]") {
		t.Errorf("Log file should contain [DEBUG] level, got: %s", logContent)
	}

	// Verify the file logger has the namespace as category
	if !strings.Contains(logContent, "[test:debug]") {
		t.Errorf("Log file should contain [test:debug] category, got: %s", logContent)
	}

	// Verify no color codes in file output
	if strings.Contains(logContent, "\033[") {
		t.Errorf("Log file should not contain ANSI color codes, got: %s", logContent)
	}
}

func TestDebugLoggerDisabledNoFileWrite(t *testing.T) {
	// Create a temporary directory for the file logger
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "debug-disabled-test.log"

	// Initialize the file logger
	err := InitFileLogger(logDir, fileName)
	if err != nil {
		t.Fatalf("InitFileLogger failed: %v", err)
	}
	defer CloseGlobalLogger()

	// Use t.Setenv to disable all debug loggers
	t.Setenv("DEBUG", "")

	// Create a debug logger (should be disabled)
	log := New("test:disabled")

	// Verify logger is disabled
	if log.Enabled() {
		t.Fatal("Logger should be disabled when DEBUG is empty")
	}

	// Try to log (should not write anywhere)
	log.Printf("This should not appear")

	// Close the file logger to flush all data
	CloseGlobalLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify the message is NOT in the file (logger was disabled)
	if strings.Contains(logContent, "This should not appear") {
		t.Errorf("Disabled logger should not write to file, got: %s", logContent)
	}
}
