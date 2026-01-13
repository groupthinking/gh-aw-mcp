package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloseLogFile_NilFile(t *testing.T) {
	var mu sync.Mutex
	err := closeLogFile(nil, &mu, "test")
	assert.NoError(t, err, "Expected nil error for nil file, got")
}

func TestCloseLogFile_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create and write to a file
	file, err := os.Create(logPath)
	require.NoError(t, err, "Failed to create test file")

	// Write some content
	if _, err := file.WriteString("test content\n"); err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}

	// Close using the helper
	var mu sync.Mutex
	if err := closeLogFile(file, &mu, "test"); err != nil {
		t.Errorf("closeLogFile failed: %v", err)
	}

	// Verify file was actually closed and flushed
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "Failed to read file after close")

	if !strings.Contains(string(content), "test content") {
		t.Errorf("File content not preserved: %s", content)
	}
}

func TestCloseLogFile_AlreadyClosedFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	file, err := os.Create(logPath)
	require.NoError(t, err, "Failed to create test file")

	// Close the file first
	if err := file.Close(); err != nil {
		t.Fatalf("Failed to close file initially: %v", err)
	}

	// Try to close again using helper - should return an error
	var mu sync.Mutex
	err = closeLogFile(file, &mu, "test")
	if err == nil {
		t.Error("Expected error when closing already-closed file, got nil")
	}
}

func TestCloseLogFile_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()

	// Test that multiple goroutines can't corrupt the close process
	// Each should have its own file
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			logPath := filepath.Join(tmpDir, "test"+string(rune('0'+id))+".log")
			file, err := os.Create(logPath)
			if err != nil {
				errors <- err
				return
			}

			if _, err := file.WriteString("content"); err != nil {
				errors <- err
				return
			}

			var mu sync.Mutex
			if err := closeLogFile(file, &mu, "test"); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent close error: %v", err)
	}
}

func TestCloseLogFile_PreservesMutexSemantics(t *testing.T) {
	// This test verifies that the helper doesn't interfere with mutex usage
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	file, err := os.Create(logPath)
	require.NoError(t, err, "Failed to create test file")

	var mu sync.Mutex

	// Lock the mutex before calling (as real code would)
	mu.Lock()
	err = closeLogFile(file, &mu, "test")
	mu.Unlock()

	assert.NoError(t, err, "closeLogFile failed with locked mutex")
}

func TestCloseLogFile_LoggerNameInErrorMessages(t *testing.T) {
	// Create a file in a way that will cause sync to potentially behave differently
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	file, err := os.Create(logPath)
	require.NoError(t, err, "Failed to create test file")

	// Close normally - this test mainly validates the function signature
	// In a real scenario, we'd capture log output to verify the logger name appears
	var mu sync.Mutex
	if err := closeLogFile(file, &mu, "MyCustomLogger"); err != nil {
		t.Errorf("closeLogFile failed: %v", err)
	}
}

func TestCloseLogFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "empty.log")

	file, err := os.Create(logPath)
	require.NoError(t, err, "Failed to create test file")

	// Don't write anything, just close
	var mu sync.Mutex
	if err := closeLogFile(file, &mu, "test"); err != nil {
		t.Errorf("closeLogFile failed for empty file: %v", err)
	}

	// Verify file exists and is empty
	info, err := os.Stat(logPath)
	require.NoError(t, err, "Failed to stat file after close")

	if info.Size() != 0 {
		t.Errorf("Expected empty file, got size: %d", info.Size())
	}
}

// Tests for initLogFile helper function

func TestInitLogFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "test.log"

	// Initialize log file with O_APPEND flag
	file, err := initLogFile(logDir, fileName, os.O_APPEND)
	if err != nil {
		t.Fatalf("initLogFile failed: %v", err)
	}
	defer file.Close()

	// Verify directory was created
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("Log directory was not created: %s", logDir)
	}

	// Verify file was created
	logPath := filepath.Join(logDir, fileName)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %s", logPath)
	}

	// Write some content to verify file is writable
	if _, err := file.WriteString("test content\n"); err != nil {
		t.Errorf("Failed to write to log file: %v", err)
	}
}

func TestInitLogFile_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "nested", "log", "directory")
	fileName := "test.log"

	// Directory doesn't exist yet
	if _, err := os.Stat(logDir); !os.IsNotExist(err) {
		t.Fatal("Directory should not exist yet")
	}

	file, err := initLogFile(logDir, fileName, os.O_APPEND)
	if err != nil {
		t.Fatalf("initLogFile failed: %v", err)
	}
	defer file.Close()

	// Verify nested directory was created
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("Nested log directory was not created: %s", logDir)
	}
}

func TestInitLogFile_AppendFlag(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "test.log"

	// Create file with initial content using O_TRUNC
	file1, err := initLogFile(logDir, fileName, os.O_TRUNC)
	if err != nil {
		t.Fatalf("First initLogFile failed: %v", err)
	}
	if _, err := file1.WriteString("initial content\n"); err != nil {
		t.Fatalf("Failed to write initial content: %v", err)
	}
	file1.Close()

	// Open file again with O_APPEND
	file2, err := initLogFile(logDir, fileName, os.O_APPEND)
	if err != nil {
		t.Fatalf("Second initLogFile failed: %v", err)
	}
	if _, err := file2.WriteString("appended content\n"); err != nil {
		t.Fatalf("Failed to write appended content: %v", err)
	}
	file2.Close()

	// Verify file contains both contents
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "initial content") {
		t.Errorf("File should contain initial content")
	}
	if !strings.Contains(contentStr, "appended content") {
		t.Errorf("File should contain appended content")
	}
}

func TestInitLogFile_TruncFlag(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := "test.log"

	// Create file with initial content
	file1, err := initLogFile(logDir, fileName, os.O_APPEND)
	if err != nil {
		t.Fatalf("First initLogFile failed: %v", err)
	}
	if _, err := file1.WriteString("initial content\n"); err != nil {
		t.Fatalf("Failed to write initial content: %v", err)
	}
	file1.Close()

	// Open file again with O_TRUNC (should truncate)
	file2, err := initLogFile(logDir, fileName, os.O_TRUNC)
	if err != nil {
		t.Fatalf("Second initLogFile failed: %v", err)
	}
	if _, err := file2.WriteString("new content\n"); err != nil {
		t.Fatalf("Failed to write new content: %v", err)
	}
	file2.Close()

	// Verify file only contains new content
	logPath := filepath.Join(logDir, fileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "initial content") {
		t.Errorf("File should not contain initial content (should be truncated)")
	}
	if !strings.Contains(contentStr, "new content") {
		t.Errorf("File should contain new content")
	}
}

func TestInitLogFile_InvalidDirectory(t *testing.T) {
	// Try to create a log file in a directory that can't be created
	// Use a path that includes a file as a directory component
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not-a-dir")

	// Create a regular file (not a directory)
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to create a log directory under this file (should fail)
	logDir := filepath.Join(filePath, "logs")
	fileName := "test.log"

	file, err := initLogFile(logDir, fileName, os.O_APPEND)
	if err == nil {
		file.Close()
		t.Fatal("initLogFile should fail when directory can't be created")
	}

	if !strings.Contains(err.Error(), "failed to create log directory") {
		t.Errorf("Expected 'failed to create log directory' error, got: %v", err)
	}
}

func TestInitLogFile_UnwritableDirectory(t *testing.T) {
	// Use a non-writable directory path
	// On most systems, /root or similar paths are not writable by regular users
	logDir := "/root/nonexistent/directory"
	fileName := "test.log"

	file, err := initLogFile(logDir, fileName, os.O_APPEND)
	if err == nil {
		file.Close()
		// If we succeeded, we might have unexpected permissions
		// This is OK - just skip the test
		t.Skip("Test requires non-writable directory, but directory was writable")
	}

	// Verify error message includes "failed to create log directory"
	if !strings.Contains(err.Error(), "failed to create log directory") {
		t.Errorf("Expected 'failed to create log directory' error, got: %v", err)
	}
}

func TestInitLogFile_EmptyFileName(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	fileName := ""

	file, err := initLogFile(logDir, fileName, os.O_APPEND)
	if err == nil {
		file.Close()
		t.Fatal("initLogFile should fail with empty fileName")
	}

	if !strings.Contains(err.Error(), "failed to open log file") {
		t.Errorf("Expected 'failed to open log file' error, got: %v", err)
	}
}

func TestInitLogFile_ConcurrentCreation(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Multiple goroutines trying to create files concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			fileName := fmt.Sprintf("test-%d.log", id)
			file, err := initLogFile(logDir, fileName, os.O_APPEND)
			if err != nil {
				errors <- err
				return
			}
			defer file.Close()

			// Write some content
			if _, err := fmt.Fprintf(file, "content from goroutine %d\n", id); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent file creation error: %v", err)
	}

	// Verify all files were created
	for i := 0; i < 10; i++ {
		fileName := fmt.Sprintf("test-%d.log", i)
		logPath := filepath.Join(logDir, fileName)
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Errorf("File not created: %s", logPath)
		}
	}
}
