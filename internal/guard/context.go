package guard

import (
	"context"

	"github.com/githubnext/gh-aw-mcpg/internal/auth"
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

// ExtractAgentIDFromAuthHeader extracts agent ID from Authorization header.
//
// Deprecated: Use auth.ExtractAgentID() instead for centralized authentication parsing.
// This function is maintained for backward compatibility but delegates to the auth package.
//
// For MCP spec 7.1 compliant parsing with full error handling, use auth.ParseAuthHeader().
func ExtractAgentIDFromAuthHeader(authHeader string) string {
	return auth.ExtractAgentID(authHeader)
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
