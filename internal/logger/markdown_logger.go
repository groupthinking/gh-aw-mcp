package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
	// Patterns for detecting potential secrets (simple heuristics)
	secretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(token|key|secret|password|auth)[=:]\s*[^\s]{8,}`),
		regexp.MustCompile(`ghp_[a-zA-Z0-9]{36,}`),                                  // GitHub PATs
		regexp.MustCompile(`github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}`),            // GitHub fine-grained PATs
		regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-._~+/]+=*`),                    // Bearer tokens
		regexp.MustCompile(`(?i)authorization:\s*[a-zA-Z0-9\-._~+/]+=*`),            // Auth headers
		regexp.MustCompile(`[a-f0-9]{32,}`),                                         // Long hex strings (API keys)
		regexp.MustCompile(`(?i)(apikey|api_key|access_key)[=:]\s*[^\s]{8,}`),       // API keys
		regexp.MustCompile(`(?i)(client_secret|client_id)[=:]\s*[^\s]{8,}`),         // OAuth secrets
		regexp.MustCompile(`[a-zA-Z0-9_-]{20,}\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`), // JWT tokens
		// JSON-specific patterns for field:value pairs
		regexp.MustCompile(`(?i)"(token|password|passwd|pwd|apikey|api_key|api-key|secret|client_secret|api_secret|authorization|auth|key|private_key|public_key|credentials|credential|access_token|refresh_token|bearer_token)"\s*:\s*"[^"]{1,}"`),
	}
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
		// Write closing details tag
		footer := "\n</details>\n"
		if _, err := ml.logFile.WriteString(footer); err != nil {
			// Sync any remaining buffered data before closing
			_ = ml.logFile.Sync() // Continue with close even if sync fails
			return ml.logFile.Close()
		}

		// Sync and close
		_ = ml.logFile.Sync() // Continue with close even if sync fails
		return ml.logFile.Close()
	}
	return nil
}

// sanitizeSecrets replaces potential secrets with [REDACTED]
func sanitizeSecrets(message string) string {
	result := message
	for _, pattern := range secretPatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// Keep the prefix (key name) but redact the value
			if strings.Contains(match, "=") || strings.Contains(match, ":") {
				parts := regexp.MustCompile(`[=:]\s*`).Split(match, 2)
				if len(parts) == 2 {
					return parts[0] + "=[REDACTED]"
				}
			}
			// For tokens without key=value format, redact entirely
			return "[REDACTED]"
		})
	}
	return result
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
	if strings.Contains(message, "\n") || strings.Contains(message, "command=") || strings.Contains(message, "args=") {
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
