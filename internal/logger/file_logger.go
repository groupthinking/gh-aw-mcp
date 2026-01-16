package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
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
}

var (
	globalFileLogger *FileLogger
	globalLoggerMu   sync.RWMutex
)

// InitFileLogger initializes the global file logger
// If the log directory doesn't exist and can't be created, falls back to stdout
func InitFileLogger(logDir, fileName string) error {
	fl := &FileLogger{
		logDir:   logDir,
		fileName: fileName,
	}

	// Try to initialize the log file
	file, err := initLogFile(logDir, fileName, os.O_APPEND)
	if err != nil {
		// File initialization failed - fallback to stdout
		log.Printf("WARNING: Failed to initialize log file: %v", err)
		log.Printf("WARNING: Falling back to stdout for logging")
		fl.useFallback = true
		fl.logger = log.New(os.Stdout, "", 0) // We'll add our own timestamp
		initGlobalFileLogger(fl)
		return nil
	}

	fl.logFile = file
	fl.logger = log.New(file, "", 0)

	log.Printf("Logging to file: %s", filepath.Join(logDir, fileName))

	initGlobalFileLogger(fl)
	return nil
}

// Close closes the log file
func (fl *FileLogger) Close() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	return closeLogFile(fl.logFile, &fl.mu, "file")
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

	// Flush the log to disk immediately to ensure it's readable by other processes
	if fl.logFile != nil {
		if err := fl.logFile.Sync(); err != nil {
			// Log sync errors to stderr to avoid infinite recursion
			log.Printf("WARNING: Failed to sync log file: %v", err)
		}
	}
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
	return closeGlobalFileLogger()
}
