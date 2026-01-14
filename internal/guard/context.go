package guard

import (
	"context"
	"strings"
)

// ContextKey is used for storing values in context
type ContextKey string

const (
	// AgentIDContextKey stores the agent ID in the request context
	AgentIDContextKey ContextKey = "difc-agent-id"

	// RequestStateContextKey stores guard-specific request state
	RequestStateContextKey ContextKey = "difc-request-state"
)

// GetAgentIDFromContext extracts the agent ID from the context
// Returns "default" if not found
func GetAgentIDFromContext(ctx context.Context) string {
	if agentID, ok := ctx.Value(AgentIDContextKey).(string); ok && agentID != "" {
		return agentID
	}
	return "default"
}

// SetAgentIDInContext sets the agent ID in the context
func SetAgentIDInContext(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, AgentIDContextKey, agentID)
}

// ExtractAgentIDFromAuthHeader extracts agent ID from Authorization header
//
// Note: For MCP spec 7.1 compliant parsing, see internal/auth.ParseAuthHeader()
// which provides centralized authentication header parsing.
//
// This function supports formats:
//   - "Bearer <token>" - uses token as agent ID
//   - "Agent <agent-id>" - uses agent-id directly
//   - Any other format - uses the entire value as agent ID
func ExtractAgentIDFromAuthHeader(authHeader string) string {
	if authHeader == "" {
		return "default"
	}

	// Handle "Bearer <token>" format
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		// Use the token as the agent ID
		// In production, you might want to validate/decode the token
		return token
	}

	// Handle "Agent <agent-id>" format
	if strings.HasPrefix(authHeader, "Agent ") {
		return strings.TrimPrefix(authHeader, "Agent ")
	}

	// Use the entire header value as agent ID
	return authHeader
}

// GetRequestStateFromContext retrieves guard request state from context
func GetRequestStateFromContext(ctx context.Context) RequestState {
	if state, ok := ctx.Value(RequestStateContextKey).(RequestState); ok {
		return state
	}
	return nil
}

// SetRequestStateInContext stores guard request state in context
func SetRequestStateInContext(ctx context.Context, state RequestState) context.Context {
	return context.WithValue(ctx, RequestStateContextKey, state)
}
