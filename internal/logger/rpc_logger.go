package logger

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RPCMessageType represents the direction of an RPC message
type RPCMessageType string

const (
	// RPCMessageRequest represents an outbound request or inbound client request
	RPCMessageRequest RPCMessageType = "REQUEST"
	// RPCMessageResponse represents an inbound response from backend or outbound response to client
	RPCMessageResponse RPCMessageType = "RESPONSE"
)

// RPCMessageDirection represents whether the message is inbound or outbound
type RPCMessageDirection string

const (
	// RPCDirectionInbound represents messages coming into the gateway
	RPCDirectionInbound RPCMessageDirection = "IN"
	// RPCDirectionOutbound represents messages going out from the gateway
	RPCDirectionOutbound RPCMessageDirection = "OUT"
)

const (
	// MaxPayloadPreviewLengthText is the maximum number of characters to include in text log preview (10KB)
	MaxPayloadPreviewLengthText = 10 * 1024 // 10KB
	// MaxPayloadPreviewLengthMarkdown is the maximum number of characters to include in markdown log preview
	MaxPayloadPreviewLengthMarkdown = 512
)

// RPCMessageInfo contains information about an RPC message for logging
type RPCMessageInfo struct {
	Direction   RPCMessageDirection // IN or OUT
	MessageType RPCMessageType      // REQUEST or RESPONSE
	ServerID    string              // Backend server ID or "client" for client messages
	Method      string              // RPC method name (for requests)
	PayloadSize int                 // Size of the payload in bytes
	Payload     string              // First N characters of payload (sanitized)
	Error       string              // Error message if any (for responses)
}

// truncateAndSanitize truncates the payload to max length and sanitizes secrets
func truncateAndSanitize(payload string, maxLength int) string {
	// First sanitize secrets
	sanitized := sanitizeSecrets(payload)

	// Then truncate if needed
	if len(sanitized) > maxLength {
		return sanitized[:maxLength] + "..."
	}
	return sanitized
}

// extractEssentialFields extracts key fields from the payload for logging
func extractEssentialFields(payload []byte) map[string]interface{} {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil
	}

	// Extract only essential fields
	essential := make(map[string]interface{})

	// Common JSON-RPC fields
	if method, ok := data["method"].(string); ok {
		essential["method"] = method
	}
	if id, ok := data["id"]; ok {
		essential["id"] = id
	}
	if jsonrpc, ok := data["jsonrpc"].(string); ok {
		essential["jsonrpc"] = jsonrpc
	}

	// For responses, include error info
	if errData, ok := data["error"]; ok {
		essential["error"] = errData
	}

	// For requests, include params summary (but not full params)
	if params, ok := data["params"]; ok {
		if paramsMap, ok := params.(map[string]interface{}); ok {
			// Include param count and keys, but not values
			essential["params_keys"] = getMapKeys(paramsMap)
		}
	}

	return essential
}

// getMapKeys returns the keys of a map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// formatRPCMessage formats an RPC message for logging
func formatRPCMessage(info *RPCMessageInfo) string {
	// Short format: server→method (or server←resp) size payload
	var dir string
	if info.Direction == RPCDirectionOutbound {
		dir = "→"
	} else {
		dir = "←"
	}

	var parts []string

	// Server and direction
	if info.ServerID != "" {
		if info.Method != "" {
			parts = append(parts, fmt.Sprintf("%s%s%s", info.ServerID, dir, info.Method))
		} else {
			parts = append(parts, fmt.Sprintf("%s%sresp", info.ServerID, dir))
		}
	}

	// Size
	parts = append(parts, fmt.Sprintf("%db", info.PayloadSize))

	// Error (if present)
	if info.Error != "" {
		parts = append(parts, fmt.Sprintf("err:%s", info.Error))
	}

	// Payload preview (if present)
	if info.Payload != "" {
		parts = append(parts, info.Payload)
	}

	return strings.Join(parts, " ")
}

// formatJSONWithoutFields formats JSON by removing specified fields and indenting with 2 spaces
// Returns the formatted string and a boolean indicating if the JSON was valid
func formatJSONWithoutFields(jsonStr string, fieldsToRemove []string) (string, bool) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// If not valid JSON, return as-is with false
		return jsonStr, false
	}

	// Remove specified fields
	for _, field := range fieldsToRemove {
		delete(data, field)
	}

	// Re-marshal with 2-space indentation
	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return jsonStr, false
	}

	// Compress JSON: remove newline after opening brace and before closing brace
	result := string(formatted)
	// Remove newline after opening brace
	result = strings.Replace(result, "{\n", "{ ", 1)
	// Remove newline before closing brace
	lastNewline := strings.LastIndex(result, "\n}")
	if lastNewline != -1 {
		result = result[:lastNewline] + " }"
	}

	return result, true
}

// formatRPCMessageMarkdown formats an RPC message for markdown logging
func formatRPCMessageMarkdown(info *RPCMessageInfo) string {
	// Concise format: **server**→method \n~~~ \n{formatted json} \n~~~
	var dir string
	if info.Direction == RPCDirectionOutbound {
		dir = "→"
	} else {
		dir = "←"
	}

	var message string

	// Server, direction, and method/type
	if info.ServerID != "" {
		if info.Method != "" {
			message = fmt.Sprintf("**%s**%s`%s`", info.ServerID, dir, info.Method)
		} else {
			message = fmt.Sprintf("**%s**%sresp", info.ServerID, dir)
		}
	}

	// Add formatted payload in code block
	if info.Payload != "" {
		// Remove jsonrpc and method fields, then format
		formatted, isValidJSON := formatJSONWithoutFields(info.Payload, []string{"jsonrpc", "method"})
		if isValidJSON {
			// Valid JSON: use code block for better readability (compact formatting)
			// Empty line before ~~~ per markdown convention
			message += fmt.Sprintf(" \n\n~~~\n%s\n~~~", formatted)
		} else {
			// Invalid JSON: use inline backticks to avoid malformed markdown
			message += fmt.Sprintf(" `%s`", formatted)
		}
	}

	// Error (if present)
	if info.Error != "" {
		message += fmt.Sprintf(" ⚠️`%s`", info.Error)
	}

	return message
}

// LogRPCRequest logs an RPC request message to both text and markdown logs
func LogRPCRequest(direction RPCMessageDirection, serverID, method string, payload []byte) {
	// Create info for text log (with larger payload preview)
	infoText := &RPCMessageInfo{
		Direction:   direction,
		MessageType: RPCMessageRequest,
		ServerID:    serverID,
		Method:      method,
		PayloadSize: len(payload),
		Payload:     truncateAndSanitize(string(payload), MaxPayloadPreviewLengthText),
	}

	// Log to text file
	LogDebug("rpc", "%s", formatRPCMessage(infoText))

	// Create info for markdown log (with shorter payload preview)
	infoMarkdown := &RPCMessageInfo{
		Direction:   direction,
		MessageType: RPCMessageRequest,
		ServerID:    serverID,
		Method:      method,
		PayloadSize: len(payload),
		Payload:     truncateAndSanitize(string(payload), MaxPayloadPreviewLengthMarkdown),
	}

	// Log to markdown file
	globalMarkdownMu.RLock()
	defer globalMarkdownMu.RUnlock()

	if globalMarkdownLogger != nil {
		globalMarkdownLogger.Log(LogLevelDebug, "rpc", "%s", formatRPCMessageMarkdown(infoMarkdown))
	}
}

// LogRPCResponse logs an RPC response message to both text and markdown logs
func LogRPCResponse(direction RPCMessageDirection, serverID string, payload []byte, err error) {
	// Create info for text log (with larger payload preview)
	infoText := &RPCMessageInfo{
		Direction:   direction,
		MessageType: RPCMessageResponse,
		ServerID:    serverID,
		PayloadSize: len(payload),
		Payload:     truncateAndSanitize(string(payload), MaxPayloadPreviewLengthText),
	}

	if err != nil {
		infoText.Error = err.Error()
	}

	// Log to text file
	LogDebug("rpc", "%s", formatRPCMessage(infoText))

	// Create info for markdown log (with shorter payload preview)
	infoMarkdown := &RPCMessageInfo{
		Direction:   direction,
		MessageType: RPCMessageResponse,
		ServerID:    serverID,
		PayloadSize: len(payload),
		Payload:     truncateAndSanitize(string(payload), MaxPayloadPreviewLengthMarkdown),
	}

	if err != nil {
		infoMarkdown.Error = err.Error()
	}

	// Log to markdown file
	globalMarkdownMu.RLock()
	defer globalMarkdownMu.RUnlock()

	if globalMarkdownLogger != nil {
		globalMarkdownLogger.Log(LogLevelDebug, "rpc", "%s", formatRPCMessageMarkdown(infoMarkdown))
	}
}

// LogRPCMessage logs a generic RPC message with custom info
func LogRPCMessage(info *RPCMessageInfo) {
	// Log to text file
	LogDebug("rpc", "%s", formatRPCMessage(info))

	// Log to markdown file
	globalMarkdownMu.RLock()
	defer globalMarkdownMu.RUnlock()

	if globalMarkdownLogger != nil {
		globalMarkdownLogger.Log(LogLevelDebug, "rpc", "%s", formatRPCMessageMarkdown(info))
	}
}
