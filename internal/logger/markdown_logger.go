package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/githubnext/gh-aw-mcpg/internal/logger/sanitize"
)

// MarkdownLogger manages logging to a markdown file for GitHub workflow previews
type MarkdownLogger struct {
	logFile     *os.File
	mu          sync.Mutex
	logDir      string
	fileName    string
	useFallback bool
	initialized bool
}

var (
	globalMarkdownLogger *MarkdownLogger
	globalMarkdownMu     sync.RWMutex
)

// InitMarkdownLogger initializes the global markdown logger
func InitMarkdownLogger(logDir, fileName string) error {
	globalMarkdownMu.Lock()
	defer globalMarkdownMu.Unlock()

	if globalMarkdownLogger != nil {
		// Close existing logger
		globalMarkdownLogger.Close()
	}

	ml := &MarkdownLogger{
		logDir:   logDir,
		fileName: fileName,
	}

	// Try to create the log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		ml.useFallback = true
		globalMarkdownLogger = ml
		return nil
	}

	// Try to open the log file
	logPath := filepath.Join(logDir, fileName)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		ml.useFallback = true
		globalMarkdownLogger = ml
		return nil
	}

	ml.logFile = file
	ml.initialized = false // Will be initialized on first write

	globalMarkdownLogger = ml
	return nil
}

// initializeFile writes the HTML details header on first write
func (ml *MarkdownLogger) initializeFile() error {
	if ml.initialized {
		return nil
	}

	if ml.logFile != nil {
		header := "<details>\n<summary>MCP Gateway</summary>\n\n"
		if _, err := ml.logFile.WriteString(header); err != nil {
			return err
		}
		ml.initialized = true
	}
	return nil
}

// Close closes the log file and writes the closing details tag
func (ml *MarkdownLogger) Close() error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	if ml.logFile != nil {
		// Write closing details tag before closing
		footer := "\n</details>\n"
		if _, err := ml.logFile.WriteString(footer); err != nil {
			// Even if footer write fails, try to close the file properly
			return closeLogFile(ml.logFile, &ml.mu, "markdown")
		}

		// Footer written successfully, now close
		return closeLogFile(ml.logFile, &ml.mu, "markdown")
	}
	return nil
}

// sanitizeSecrets replaces potential secrets with [REDACTED]
// This function is deprecated and will be removed in a future version.
// Use sanitize.SanitizeString() directly instead.
func sanitizeSecrets(message string) string {
	return sanitize.SanitizeString(message)
}

// getEmojiForLevel returns the appropriate emoji for the log level
func getEmojiForLevel(level LogLevel) string {
	switch level {
	case LogLevelInfo:
		return "✓"
	case LogLevelWarn:
		return "⚠️"
	case LogLevelError:
		return "✗"
	case LogLevelDebug:
		return "🔍"
	default:
		return "•"
	}
}

// Log writes a log message in markdown format with emoji bullet points
func (ml *MarkdownLogger) Log(level LogLevel, category, format string, args ...interface{}) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	if ml.useFallback {
		return
	}

	// Initialize file with header on first write
	if err := ml.initializeFile(); err != nil {
		return
	}

	message := fmt.Sprintf(format, args...)

	// Sanitize potential secrets
	message = sanitizeSecrets(message)

	emoji := getEmojiForLevel(level)

	// Format as markdown bullet point with emoji
	// Use code blocks for multi-line content or technical details
	var logLine string

	// Check if message is already pre-formatted (RPC messages with markdown formatting)
	// RPC messages start with ** and contain → or ← arrows
	isPreformatted := strings.HasPrefix(message, "**") && (strings.Contains(message, "→") || strings.Contains(message, "←"))

	if isPreformatted {
		// Pre-formatted content (like RPC messages) - just add bullet and emoji
		logLine = fmt.Sprintf("- %s %s %s\n", emoji, category, message)
	} else if strings.Contains(message, "\n") || strings.Contains(message, "command=") || strings.Contains(message, "args=") {
		// Multi-line or technical content - use code block
		logLine = fmt.Sprintf("- %s **%s**\n  ```\n  %s\n  ```\n", emoji, category, message)
	} else {
		// Simple single-line message
		logLine = fmt.Sprintf("- %s **%s** %s\n", emoji, category, message)
	}

	if ml.logFile != nil {
		if _, err := ml.logFile.WriteString(logLine); err != nil {
			return
		}
		// Flush immediately
		_ = ml.logFile.Sync() // Ignore sync errors
	}
}

// Global logging functions that also write to markdown logger

// LogInfoMd logs to both regular and markdown loggers
func LogInfoMd(category, format string, args ...interface{}) {
	// Log to regular logger
	LogInfo(category, format, args...)

	// Log to markdown logger
	globalMarkdownMu.RLock()
	defer globalMarkdownMu.RUnlock()

	if globalMarkdownLogger != nil {
		globalMarkdownLogger.Log(LogLevelInfo, category, format, args...)
	}
}

// LogWarnMd logs to both regular and markdown loggers
func LogWarnMd(category, format string, args ...interface{}) {
	// Log to regular logger
	LogWarn(category, format, args...)

	// Log to markdown logger
	globalMarkdownMu.RLock()
	defer globalMarkdownMu.RUnlock()

	if globalMarkdownLogger != nil {
		globalMarkdownLogger.Log(LogLevelWarn, category, format, args...)
	}
}

// LogErrorMd logs to both regular and markdown loggers
func LogErrorMd(category, format string, args ...interface{}) {
	// Log to regular logger
	LogError(category, format, args...)

	// Log to markdown logger
	globalMarkdownMu.RLock()
	defer globalMarkdownMu.RUnlock()

	if globalMarkdownLogger != nil {
		globalMarkdownLogger.Log(LogLevelError, category, format, args...)
	}
}

// LogDebugMd logs to both regular and markdown loggers
func LogDebugMd(category, format string, args ...interface{}) {
	// Log to regular logger
	LogDebug(category, format, args...)

	// Log to markdown logger
	globalMarkdownMu.RLock()
	defer globalMarkdownMu.RUnlock()

	if globalMarkdownLogger != nil {
		globalMarkdownLogger.Log(LogLevelDebug, category, format, args...)
	}
}

// CloseMarkdownLogger closes the global markdown logger
func CloseMarkdownLogger() error {
	globalMarkdownMu.Lock()
	defer globalMarkdownMu.Unlock()

	if globalMarkdownLogger != nil {
		err := globalMarkdownLogger.Close()
		globalMarkdownLogger = nil
		return err
	}
	return nil
}
