package server

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

// authMiddleware implements API key authentication per MCP spec 2025-03-26 Authorization section
// Spec requirement: Access tokens MUST use the Authorization header field in format "Bearer <token>"
// For HTTP-based transports, implementations SHOULD conform to OAuth 2.1
//
// HTTP Status Codes per MCP Spec:
// - 400 Bad Request: Malformed authorization request (wrong format, token in query string)
// - 401 Unauthorized: Authorization required or token invalid/expired
// - 403 Forbidden: Valid token but insufficient permissions/scopes (not yet implemented)
//
// Note: HTTP 403 is currently not used because this gateway uses simple API key validation
// without scope-based permissions. When OAuth scopes are added in the future, 403 should be
// returned when a valid token lacks the required scopes for the requested operation.
func authMiddleware(apiKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// MCP Spec: Access tokens MUST NOT be included in URI query strings
		if r.URL.Query().Get("token") != "" || r.URL.Query().Get("access_token") != "" || r.URL.Query().Get("apiKey") != "" {
			logger.LogError("auth", "Authentication failed: token in query string, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
			logRuntimeError("authentication_failed", "token_in_query_string", r, nil)
			http.Error(w, "Bad Request: tokens must not be included in query string", http.StatusBadRequest)
			return
		}

		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			// MCP Spec: Missing token returns 401
			logger.LogError("auth", "Authentication failed: missing Authorization header, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
			logRuntimeError("authentication_failed", "missing_auth_header", r, nil)
			http.Error(w, "Unauthorized: missing Authorization header", http.StatusUnauthorized)
			return
		}

		// MCP Spec: Authorization header MUST be in format "Bearer <token>"
		if !strings.HasPrefix(authHeader, "Bearer ") {
			logger.LogError("auth", "Authentication failed: malformed Authorization header (missing 'Bearer ' prefix), remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
			logRuntimeError("authentication_failed", "malformed_auth_header", r, nil)
			http.Error(w, "Bad Request: Authorization header must be 'Bearer <token>'", http.StatusBadRequest)
			return
		}

		// Extract token after "Bearer " prefix
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		
		if token == "" {
			// Empty token after Bearer prefix
			logger.LogError("auth", "Authentication failed: empty token after Bearer prefix, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
			logRuntimeError("authentication_failed", "empty_token", r, nil)
			http.Error(w, "Bad Request: Bearer token cannot be empty", http.StatusBadRequest)
			return
		}

		// MCP Spec: Invalid or expired tokens MUST receive HTTP 401
		if token != apiKey {
			logger.LogError("auth", "Authentication failed: invalid API key, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
			logRuntimeError("authentication_failed", "invalid_token", r, nil)
			http.Error(w, "Unauthorized: invalid API key", http.StatusUnauthorized)
			return
		}

		logger.LogInfo("auth", "Authentication successful, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
		// Token is valid, proceed to handler
		next(w, r)
	}
}

// logRuntimeError logs runtime errors to stdout per spec section 9.2
func logRuntimeError(errorType, detail string, r *http.Request, serverName *string) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = "unknown"
	}

	server := "gateway"
	if serverName != nil {
		server = *serverName
	}

	// Spec 9.2: Log to stdout with timestamp, server name, request ID, error details
	log.Printf("[ERROR] timestamp=%s server=%s request_id=%s error_type=%s detail=%s path=%s method=%s",
		timestamp, server, requestID, errorType, detail, r.URL.Path, r.Method)
}
