package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
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
	Timestamp string                 `json:"timestamp"`
	Direction string                 `json:"direction"` // "IN" or "OUT"
	Type      string                 `json:"type"`      // "REQUEST" or "RESPONSE"
	ServerID  string                 `json:"server_id"`
	Method    string                 `json:"method,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Payload   map[string]interface{} `json:"payload"` // Full sanitized payload
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

	if jl.logFile != nil {
		// Sync any remaining buffered data before closing
		if err := jl.logFile.Sync(); err != nil {
			// Log sync errors but continue with close
			return err
		}
		return jl.logFile.Close()
	}
	return nil
}

// sanitizePayload sanitizes a JSON payload by applying regex patterns to the entire JSON string
// It takes raw bytes, sanitizes using regex patterns, then parses back to a map
func sanitizePayload(payloadBytes []byte) map[string]interface{} {
	// Apply regex sanitization to the entire JSON string
	sanitized := sanitizeSecrets(string(payloadBytes))

	// Parse the sanitized JSON back into a map
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(sanitized), &payload); err != nil {
		// If we can't parse the payload, return an error object
		return map[string]interface{}{
			"_error": "failed to parse JSON",
			"_raw":   sanitized,
		}
	}

	return payload
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
