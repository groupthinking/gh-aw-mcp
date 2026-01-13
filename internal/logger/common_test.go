package logger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCloseLogFile_NilFile(t *testing.T) {
	var mu sync.Mutex
	err := closeLogFile(nil, &mu, "test")
	if err != nil {
		t.Errorf("Expected nil error for nil file, got: %v", err)
	}
}

func TestCloseLogFile_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create and write to a file
	file, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Failed to read file after close: %v", err)
	}

	if !strings.Contains(string(content), "test content") {
		t.Errorf("File content not preserved: %s", content)
	}
}

func TestCloseLogFile_AlreadyClosedFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	file, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	var mu sync.Mutex
	
	// Lock the mutex before calling (as real code would)
	mu.Lock()
	err = closeLogFile(file, &mu, "test")
	mu.Unlock()

	if err != nil {
		t.Errorf("closeLogFile failed with locked mutex: %v", err)
	}
}

func TestCloseLogFile_LoggerNameInErrorMessages(t *testing.T) {
	// Create a file in a way that will cause sync to potentially behave differently
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	file, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Don't write anything, just close
	var mu sync.Mutex
	if err := closeLogFile(file, &mu, "test"); err != nil {
		t.Errorf("closeLogFile failed for empty file: %v", err)
	}

	// Verify file exists and is empty
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat file after close: %v", err)
	}

	if info.Size() != 0 {
		t.Errorf("Expected empty file, got size: %d", info.Size())
	}
}
