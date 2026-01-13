package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/logger/sanitize"
)

// JSONLLogger manages logging RPC messages to a JSONL file (one JSON object per line)
type JSONLLogger struct {
	logFile  *os.File
	mu       sync.Mutex
	logDir   string
	fileName string
	encoder  *json.Encoder
}

var (
	globalJSONLLogger *JSONLLogger
	globalJSONLMu     sync.RWMutex
)

// JSONLRPCMessage represents a single RPC message log entry in JSONL format
type JSONLRPCMessage struct {
	Timestamp string          `json:"timestamp"`
	Direction string          `json:"direction"` // "IN" or "OUT"
	Type      string          `json:"type"`      // "REQUEST" or "RESPONSE"
	ServerID  string          `json:"server_id"`
	Method    string          `json:"method,omitempty"`
	Error     string          `json:"error,omitempty"`
	Payload   json.RawMessage `json:"payload"` // Full sanitized payload as raw JSON
}

// InitJSONLLogger initializes the global JSONL logger
func InitJSONLLogger(logDir, fileName string) error {
	globalJSONLMu.Lock()
	defer globalJSONLMu.Unlock()

	if globalJSONLLogger != nil {
		// Close existing logger
		globalJSONLLogger.Close()
	}

	jl := &JSONLLogger{
		logDir:   logDir,
		fileName: fileName,
	}

	// Try to create the log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// If we can't create the directory, just return without setting up the logger
		// This allows the gateway to continue running even if JSONL logging fails
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Try to open the log file
	logPath := filepath.Join(logDir, fileName)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	jl.logFile = file
	jl.encoder = json.NewEncoder(file)

	globalJSONLLogger = jl
	return nil
}

// Close closes the JSONL log file
func (jl *JSONLLogger) Close() error {
	jl.mu.Lock()
	defer jl.mu.Unlock()

	return closeLogFile(jl.logFile, &jl.mu, "JSONL")
}

// sanitizePayload sanitizes a payload by applying regex patterns to the entire string
// It takes raw bytes, applies regex sanitization in one pass, and returns sanitized bytes
// This function is deprecated and will be removed in a future version.
// Use sanitize.SanitizeJSON() directly instead.
func sanitizePayload(payloadBytes []byte) json.RawMessage {
	return sanitize.SanitizeJSON(payloadBytes)
}

// LogMessage logs an RPC message to the JSONL file
func (jl *JSONLLogger) LogMessage(entry *JSONLRPCMessage) error {
	jl.mu.Lock()
	defer jl.mu.Unlock()

	if jl.logFile == nil {
		return fmt.Errorf("JSONL logger not initialized")
	}

	// Encode and write the JSON object followed by a newline
	if err := jl.encoder.Encode(entry); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	// Flush to disk immediately
	if err := jl.logFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync log file: %w", err)
	}

	return nil
}

// CloseJSONLLogger closes the global JSONL logger
func CloseJSONLLogger() error {
	globalJSONLMu.Lock()
	defer globalJSONLMu.Unlock()

	if globalJSONLLogger != nil {
		err := globalJSONLLogger.Close()
		globalJSONLLogger = nil
		return err
	}
	return nil
}

// LogRPCMessageJSONL logs an RPC message to the global JSONL logger
func LogRPCMessageJSONL(direction RPCMessageDirection, messageType RPCMessageType, serverID, method string, payloadBytes []byte, err error) {
	globalJSONLMu.RLock()
	defer globalJSONLMu.RUnlock()

	if globalJSONLLogger == nil {
		return
	}

	entry := &JSONLRPCMessage{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Direction: string(direction),
		Type:      string(messageType),
		ServerID:  serverID,
		Method:    method,
		Payload:   sanitizePayload(payloadBytes),
	}

	if err != nil {
		entry.Error = err.Error()
	}

	// Best effort logging - don't fail if JSONL logging fails
	_ = globalJSONLLogger.LogMessage(entry)
}
