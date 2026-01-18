package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	"github.com/itchyny/gojq"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var logMiddleware = logger.New("middleware:jqschema")

// jqSchemaFilter is the jq filter that transforms JSON to schema
// This is the same logic as in gh-aw shared/jqschema.md
const jqSchemaFilter = `
def walk(f):
  . as $in |
  if type == "object" then
    reduce keys[] as $k ({}; . + {($k): ($in[$k] | walk(f))})
  elif type == "array" then
    if length == 0 then [] else [.[0] | walk(f)] end
  else
    type
  end;
walk(.)
`

// generateRandomID generates a random ID for payload storage
func generateRandomID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("fallback-%d", os.Getpid())
	}
	return hex.EncodeToString(bytes)
}

// applyJqSchema applies the jq schema transformation to JSON data
func applyJqSchema(jsonData interface{}) (string, error) {
	// Parse the jq query
	query, err := gojq.Parse(jqSchemaFilter)
	if err != nil {
		return "", fmt.Errorf("failed to parse jq schema filter: %w", err)
	}

	// Run the query
	iter := query.Run(jsonData)
	v, ok := iter.Next()
	if !ok {
		return "", fmt.Errorf("jq schema filter returned no results")
	}

	// Check for errors
	if err, ok := v.(error); ok {
		return "", fmt.Errorf("jq schema filter error: %w", err)
	}

	// Convert result to JSON
	schemaJSON, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema result: %w", err)
	}

	return string(schemaJSON), nil
}

// savePayload saves the payload to disk and returns the file path
func savePayload(queryID string, payload []byte) (string, error) {
	// Create directory structure: /tmp/gh-awmg/tools-calls/{RND}
	dir := filepath.Join("/tmp", "gh-awmg", "tools-calls", queryID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create payload directory: %w", err)
	}

	// Save payload to file
	filePath := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(filePath, payload, 0644); err != nil {
		return "", fmt.Errorf("failed to write payload file: %w", err)
	}

	return filePath, nil
}

// WrapToolHandler wraps a tool handler with jqschema middleware
// This middleware:
// 1. Generates a random ID for the query
// 2. Saves the response payload to /tmp/gh-awmg/tools-calls/{RND}/payload.json
// 3. Returns first 500 chars of payload + jq inferred schema
func WrapToolHandler(
	handler func(context.Context, *sdk.CallToolRequest, interface{}) (*sdk.CallToolResult, interface{}, error),
	toolName string,
) func(context.Context, *sdk.CallToolRequest, interface{}) (*sdk.CallToolResult, interface{}, error) {
	return func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
		// Generate random query ID
		queryID := generateRandomID()
		logMiddleware.Printf("Processing tool call: tool=%s, queryID=%s", toolName, queryID)

		// Call the original handler
		result, data, err := handler(ctx, req, args)
		if err != nil {
			logMiddleware.Printf("Tool call failed: tool=%s, queryID=%s, error=%v", toolName, queryID, err)
			return result, data, err
		}

		// Only process successful results with data
		if result == nil || result.IsError || data == nil {
			return result, data, err
		}

		// Marshal the response data to JSON
		payloadJSON, marshalErr := json.Marshal(data)
		if marshalErr != nil {
			logMiddleware.Printf("Failed to marshal response: tool=%s, queryID=%s, error=%v", toolName, queryID, marshalErr)
			return result, data, err
		}

		// Save the payload
		filePath, saveErr := savePayload(queryID, payloadJSON)
		if saveErr != nil {
			logMiddleware.Printf("Failed to save payload: tool=%s, queryID=%s, error=%v", toolName, queryID, saveErr)
			// Continue even if save fails - don't break the tool call
		} else {
			logMiddleware.Printf("Saved payload: tool=%s, queryID=%s, path=%s, size=%d bytes",
				toolName, queryID, filePath, len(payloadJSON))
		}

		// Apply jq schema transformation
		var schemaJSON string
		if schemaErr := func() error {
			// Unmarshal to interface{} for jq processing
			var jsonData interface{}
			if err := json.Unmarshal(payloadJSON, &jsonData); err != nil {
				return fmt.Errorf("failed to unmarshal for schema: %w", err)
			}

			schema, err := applyJqSchema(jsonData)
			if err != nil {
				return err
			}
			schemaJSON = schema
			return nil
		}(); schemaErr != nil {
			logMiddleware.Printf("Failed to apply jq schema: tool=%s, queryID=%s, error=%v", toolName, queryID, schemaErr)
			// Continue with original response if schema extraction fails
			return result, data, err
		}

		// Build the transformed response: first 500 chars + schema
		payloadStr := string(payloadJSON)
		var preview string
		if len(payloadStr) > 500 {
			preview = payloadStr[:500] + "..."
		} else {
			preview = payloadStr
		}

		// Create rewritten response
		rewrittenResponse := map[string]interface{}{
			"queryID":      queryID,
			"payloadPath":  filePath,
			"preview":      preview,
			"schema":       schemaJSON,
			"originalSize": len(payloadJSON),
			"truncated":    len(payloadStr) > 500,
		}

		logMiddleware.Printf("Rewritten response: tool=%s, queryID=%s, originalSize=%d, truncated=%v",
			toolName, queryID, len(payloadJSON), len(payloadStr) > 500)

		// Parse the schema JSON string back to an object for cleaner display
		var schemaObj interface{}
		if err := json.Unmarshal([]byte(schemaJSON), &schemaObj); err == nil {
			rewrittenResponse["schema"] = schemaObj
		}

		return result, rewrittenResponse, nil
	}
}

// ShouldApplyMiddleware determines if the middleware should be applied to a tool
// Currently applies to all tools, but can be configured to filter specific tools
func ShouldApplyMiddleware(toolName string) bool {
	// Apply to all tools except sys tools
	return !strings.HasPrefix(toolName, "sys___")
}
