package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitFileLogger(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "test.log"

	// Initialize the logger
	err := InitFileLogger(logDir, fileName)
	require.NoError(t, err, "InitFileLogger failed")
	defer CloseGlobalLogger()

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

func TestFileLoggerFallback(t *testing.T) {
	// Use a non-writable directory (e.g., root on Unix)
	// This will trigger fallback to stdout
	logDir := "/root/nonexistent/directory"
	fileName := "test.log"

	// Initialize the logger - should not fail, but use fallback
	err := InitFileLogger(logDir, fileName)
	require.NoError(t, err, "InitFileLogger should not fail on fallback")
	defer CloseGlobalLogger()

	globalLoggerMu.RLock()
	useFallback := globalFileLogger.useFallback
	globalLoggerMu.RUnlock()

	if !useFallback {
		// Note: This might not fail on systems where we have root access
		// In that case, we just verify the logger was initialized
		t.Logf("Logger initialized without fallback (may have permissions)")
	}
}

func TestFileLoggerLogging(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "test.log"

	err := InitFileLogger(logDir, fileName)
	require.NoError(t, err, "InitFileLogger failed")
	defer CloseGlobalLogger()

	// Write some log messages
	LogInfo("test", "This is an info message")
	LogWarn("test", "This is a warning message with value: %d", 42)
	LogError("test", "This is an error message")
	LogDebug("test", "This is a debug message")

	// Close the logger to ensure all data is flushed
	CloseGlobalLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	logContent := string(content)

	// Verify log messages are present
	expectedMessages := []struct {
		level   string
		message string
	}{
		{"INFO", "This is an info message"},
		{"WARN", "This is a warning message with value: 42"},
		{"ERROR", "This is an error message"},
		{"DEBUG", "This is a debug message"},
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(logContent, expected.level) {
			t.Errorf("Log file does not contain level: %s", expected.level)
		}
		if !strings.Contains(logContent, expected.message) {
			t.Errorf("Log file does not contain message: %s", expected.message)
		}
		assert.True(t, strings.Contains(logContent, "[test]"), "Log file does not contain category: [test]")
	}

	// Verify timestamp format (RFC3339)
	lines := strings.Split(strings.TrimSpace(logContent), "\n")
	if len(lines) < 4 {
		t.Errorf("Expected at least 4 log lines, got %d", len(lines))
	}

	for _, line := range lines {
		if line == "" {
			continue
		}
		// Each line should start with a timestamp in brackets
		if !strings.HasPrefix(line, "[") {
			t.Errorf("Log line does not start with timestamp: %s", line)
		}
	}
}

func TestFileLoggerAppend(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "append-test.log"

	// First logger session
	err := InitFileLogger(logDir, fileName)
	require.NoError(t, err, "InitFileLogger failed")
	LogInfo("test", "First message")
	CloseGlobalLogger()

	// Second logger session - should append
	err = InitFileLogger(logDir, fileName)
	require.NoError(t, err, "InitFileLogger failed")
	LogInfo("test", "Second message")
	CloseGlobalLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	logContent := string(content)

	// Verify both messages are present
	assert.True(t, strings.Contains(logContent, "First message"), "Log file does not contain first message")
	assert.True(t, strings.Contains(logContent, "Second message"), "Log file does not contain second message")
}

func TestFileLoggerConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "concurrent-test.log"

	err := InitFileLogger(logDir, fileName)
	require.NoError(t, err, "InitFileLogger failed")
	defer CloseGlobalLogger()

	// Write from multiple goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				LogInfo("concurrent", "Message from goroutine %d, iteration %d", id, j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	CloseGlobalLogger()

	// Read the log file
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	// Count the number of log lines
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	// Should have 100 lines (10 goroutines * 10 messages each)
	expectedLines := 100
	if len(lines) != expectedLines {
		t.Errorf("Expected %d log lines, got %d", expectedLines, len(lines))
	}
}

func TestFileLoggerReadableByOtherProcesses(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "readable-test.log"
	logPath := filepath.Join(logDir, fileName)

	// Initialize the logger
	err := InitFileLogger(logDir, fileName)
	require.NoError(t, err, "InitFileLogger failed")

	// Write some data
	LogInfo("test", "Testing file readability")

	// Verify another process can open and read the file without any locks
	// This simulates another process trying to read the log file
	readFile, err := os.Open(logPath)
	require.NoError(t, err, "Failed to open log file for reading")
	defer readFile.Close()

	// Read the content to verify it's readable
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file content")

	assert.True(t, strings.Contains(string(content), "Testing file readability"), "Log file does not contain expected content")

	// Close the logger
	CloseGlobalLogger()

	// Verify file is still readable after close
	content, err = os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file after close")

	assert.True(t, strings.Contains(string(content), "Testing file readability"), "Log file does not contain expected content after close")
}

func TestFileLoggerFlushes(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "flush-test.log"
	logPath := filepath.Join(logDir, fileName)

	// Initialize the logger
	err := InitFileLogger(logDir, fileName)
	require.NoError(t, err, "InitFileLogger failed")
	defer CloseGlobalLogger()

	// Write a message
	LogInfo("test", "Immediate flush test")

	// Immediately read the file without closing the logger
	// This tests that Sync() is being called after each write
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read log file")

	assert.True(t, strings.Contains(string(content), "Immediate flush test"), "Log file does not contain message immediately after write - data not flushed")
}
