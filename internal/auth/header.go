// Package auth provides authentication header parsing and middleware
// for the MCP Gateway server.
//
// This package implements MCP specification 7.1 for authentication,
// which requires Authorization headers to contain the API key directly
// without any scheme prefix (e.g., NOT "Bearer <key>").
//
// Example usage:
//
//	apiKey, agentID, err := auth.ParseAuthHeader(r.Header.Get("Authorization"))
//	if err != nil {
//		// Handle error
//	}
package auth

import (
	"errors"
	"strings"
)

var (
	// ErrMissingAuthHeader is returned when the Authorization header is missing
	ErrMissingAuthHeader = errors.New("missing Authorization header")
	// ErrInvalidAuthHeader is returned when the Authorization header format is invalid
	ErrInvalidAuthHeader = errors.New("invalid Authorization header format")
)

// ParseAuthHeader parses the Authorization header and extracts the API key and agent ID.
// Per MCP spec 7.1, the Authorization header should contain the API key directly
// without any Bearer prefix or other scheme.
//
// For backward compatibility, this function also supports:
//   - "Bearer <token>" format (uses token as both API key and agent ID)
//   - "Agent <agent-id>" format (extracts agent ID)
//
// Returns:
//   - apiKey: The extracted API key
//   - agentID: The extracted agent/session identifier
//   - error: ErrMissingAuthHeader if header is empty, nil otherwise
func ParseAuthHeader(authHeader string) (apiKey string, agentID string, error error) {
	if authHeader == "" {
		return "", "", ErrMissingAuthHeader
	}

	// Handle "Bearer <token>" format (backward compatibility)
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return token, token, nil
	}

	// Handle "Agent <agent-id>" format
	if strings.HasPrefix(authHeader, "Agent ") {
		agentIDValue := strings.TrimPrefix(authHeader, "Agent ")
		return agentIDValue, agentIDValue, nil
	}

	// Per MCP spec 7.1: Authorization header contains API key directly
	// Use the entire header value as both API key and agent/session ID
	return authHeader, authHeader, nil
}

// ValidateAPIKey checks if the provided API key matches the expected key.
// Returns true if they match, false otherwise.
func ValidateAPIKey(provided, expected string) bool {
	if expected == "" {
		// No API key configured, authentication is disabled
		return true
	}
	return provided == expected
}
