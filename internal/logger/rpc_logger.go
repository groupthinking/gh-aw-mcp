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
	// MaxPayloadPreviewLength is the maximum number of characters to include in log preview
	MaxPayloadPreviewLength = 120
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
	// Format: [DIRECTION] [TYPE] server=<id> method=<method> size=<bytes> payload=<preview>
	parts := []string{
		fmt.Sprintf("[%s]", info.Direction),
		fmt.Sprintf("[%s]", info.MessageType),
	}

	if info.ServerID != "" {
		parts = append(parts, fmt.Sprintf("server=%s", info.ServerID))
	}

	if info.Method != "" {
		parts = append(parts, fmt.Sprintf("method=%s", info.Method))
	}

	parts = append(parts, fmt.Sprintf("size=%d", info.PayloadSize))

	if info.Error != "" {
		parts = append(parts, fmt.Sprintf("error=%s", info.Error))
	}

	if info.Payload != "" {
		parts = append(parts, fmt.Sprintf("payload=%s", info.Payload))
	}

	return strings.Join(parts, " ")
}

// formatRPCMessageMarkdown formats an RPC message for markdown logging
func formatRPCMessageMarkdown(info *RPCMessageInfo) string {
	// Format: Server **<id>** <direction> <type> `<method>` (<size> bytes)
	var parts []string

	// Start with server attribution
	if info.ServerID != "" {
		parts = append(parts, fmt.Sprintf("Server **%s**", info.ServerID))
	}

	// Add direction and type
	directionSymbol := "→"
	if info.Direction == RPCDirectionInbound {
		directionSymbol = "←"
	}
	parts = append(parts, directionSymbol)

	// Add method in code formatting
	if info.Method != "" {
		parts = append(parts, fmt.Sprintf("`%s`", info.Method))
	} else {
		parts = append(parts, string(info.MessageType))
	}

	// Add size
	parts = append(parts, fmt.Sprintf("(%d bytes)", info.PayloadSize))

	message := strings.Join(parts, " ")

	// Add payload preview if available
	if info.Payload != "" {
		message += fmt.Sprintf("\n  ```\n  %s\n  ```", info.Payload)
	}

	// Add error if present
	if info.Error != "" {
		message += fmt.Sprintf("\n  Error: %s", info.Error)
	}

	return message
}

// LogRPCRequest logs an RPC request message to both text and markdown logs
func LogRPCRequest(direction RPCMessageDirection, serverID, method string, payload []byte) {
	info := &RPCMessageInfo{
		Direction:   direction,
		MessageType: RPCMessageRequest,
		ServerID:    serverID,
		Method:      method,
		PayloadSize: len(payload),
		Payload:     truncateAndSanitize(string(payload), MaxPayloadPreviewLength),
	}

	// Log to text file
	LogDebug("rpc", "%s", formatRPCMessage(info))

	// Log to markdown file
	globalMarkdownMu.RLock()
	defer globalMarkdownMu.RUnlock()

	if globalMarkdownLogger != nil {
		globalMarkdownLogger.Log(LogLevelDebug, "rpc", "%s", formatRPCMessageMarkdown(info))
	}
}

// LogRPCResponse logs an RPC response message to both text and markdown logs
func LogRPCResponse(direction RPCMessageDirection, serverID string, payload []byte, err error) {
	info := &RPCMessageInfo{
		Direction:   direction,
		MessageType: RPCMessageResponse,
		ServerID:    serverID,
		PayloadSize: len(payload),
		Payload:     truncateAndSanitize(string(payload), MaxPayloadPreviewLength),
	}

	if err != nil {
		info.Error = err.Error()
	}

	// Log to text file
	LogDebug("rpc", "%s", formatRPCMessage(info))

	// Log to markdown file
	globalMarkdownMu.RLock()
	defer globalMarkdownMu.RUnlock()

	if globalMarkdownLogger != nil {
		globalMarkdownLogger.Log(LogLevelDebug, "rpc", "%s", formatRPCMessageMarkdown(info))
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
