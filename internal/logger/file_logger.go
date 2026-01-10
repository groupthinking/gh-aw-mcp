package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// FileLogger manages logging to a file with fallback to stdout
type FileLogger struct {
	logFile     *os.File
	logger      *log.Logger
	mu          sync.Mutex
	logDir      string
	fileName    string
	useFallback bool
	locked      bool // tracks if file is locked
}

var (
	globalFileLogger *FileLogger
	globalLoggerMu   sync.RWMutex
)

// InitFileLogger initializes the global file logger
// If the log directory doesn't exist and can't be created, falls back to stdout
func InitFileLogger(logDir, fileName string) error {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()

	if globalFileLogger != nil {
		// Close existing logger
		globalFileLogger.Close()
	}

	fl := &FileLogger{
		logDir:   logDir,
		fileName: fileName,
	}

	// Try to create the log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// Directory creation failed - fallback to stdout
		log.Printf("WARNING: Failed to create log directory %s: %v", logDir, err)
		log.Printf("WARNING: Falling back to stdout for logging")
		fl.useFallback = true
		fl.logger = log.New(os.Stdout, "", 0) // We'll add our own timestamp
		globalFileLogger = fl
		return nil
	}

	// Try to open the log file
	logPath := filepath.Join(logDir, fileName)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// File creation failed - fallback to stdout
		log.Printf("WARNING: Failed to open log file %s: %v", logPath, err)
		log.Printf("WARNING: Falling back to stdout for logging")
		fl.useFallback = true
		fl.logger = log.New(os.Stdout, "", 0)
		globalFileLogger = fl
		return nil
	}

	fl.logFile = file
	fl.logger = log.New(file, "", 0)
	
	// Apply a shared lock (LOCK_SH) to allow other processes to read the file
	// This is non-blocking (LOCK_NB) so we don't hang if another process has an exclusive lock
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		// If we can't get a shared lock, log a warning but continue
		// The file is still usable, just not with advisory locking guarantees
		log.Printf("WARNING: Failed to acquire shared lock on log file %s: %v", logPath, err)
		log.Printf("WARNING: Continuing without file lock - other processes may have limited access")
	} else {
		fl.locked = true
	}
	
	log.Printf("Logging to file: %s", logPath)

	globalFileLogger = fl
	return nil
}

// Close closes the log file and releases any locks
func (fl *FileLogger) Close() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.logFile != nil {
		// Release the file lock before closing
		if fl.locked {
			if err := syscall.Flock(int(fl.logFile.Fd()), syscall.LOCK_UN); err != nil {
				log.Printf("WARNING: Failed to release lock on log file: %v", err)
				// Continue to close the file even if unlock fails
			}
			fl.locked = false
		}
		return fl.logFile.Close()
	}
	return nil
}

// LogLevel represents the severity of a log message
type LogLevel string

const (
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelDebug LogLevel = "DEBUG"
)

// Log writes a log message with the specified level and category
func (fl *FileLogger) Log(level LogLevel, category, format string, args ...interface{}) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339)
	message := fmt.Sprintf(format, args...)

	logLine := fmt.Sprintf("[%s] [%s] [%s] %s", timestamp, level, category, message)
	fl.logger.Println(logLine)
}

// GetWriter returns the underlying io.Writer for the file logger
func (fl *FileLogger) GetWriter() io.Writer {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.logFile != nil {
		return fl.logFile
	}
	return os.Stdout
}

// Global logging functions that use the global file logger

// LogInfo logs an informational message
func LogInfo(category, format string, args ...interface{}) {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()

	if globalFileLogger != nil {
		globalFileLogger.Log(LogLevelInfo, category, format, args...)
	}
}

// LogWarn logs a warning message
func LogWarn(category, format string, args ...interface{}) {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()

	if globalFileLogger != nil {
		globalFileLogger.Log(LogLevelWarn, category, format, args...)
	}
}

// LogError logs an error message
func LogError(category, format string, args ...interface{}) {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()

	if globalFileLogger != nil {
		globalFileLogger.Log(LogLevelError, category, format, args...)
	}
}

// LogDebug logs a debug message
func LogDebug(category, format string, args ...interface{}) {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()

	if globalFileLogger != nil {
		globalFileLogger.Log(LogLevelDebug, category, format, args...)
	}
}

// CloseGlobalLogger closes the global file logger
func CloseGlobalLogger() error {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()

	if globalFileLogger != nil {
		err := globalFileLogger.Close()
		globalFileLogger = nil
		return err
	}
	return nil
}
