package server

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

// authMiddleware implements API key authentication per spec section 7.1
// Per spec: Authorization header MUST contain the API key directly (NOT Bearer scheme)
//
// For header parsing logic, see internal/auth package which provides:
//   - ParseAuthHeader() for extracting API keys and agent IDs
//   - ValidateAPIKey() for key validation
func authMiddleware(apiKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			// Spec 7.1: Missing token returns 401
			logger.LogErrorMd("auth", "Authentication failed: missing Authorization header, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
			logRuntimeError("authentication_failed", "missing_auth_header", r, nil)
			http.Error(w, "Unauthorized: missing Authorization header", http.StatusUnauthorized)
			return
		}

		// Spec 7.1: Authorization header must contain API key directly (not Bearer scheme)
		if authHeader != apiKey {
			logger.LogErrorMd("auth", "Authentication failed: invalid API key, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
			logRuntimeError("authentication_failed", "invalid_api_key", r, nil)
			http.Error(w, "Unauthorized: invalid API key", http.StatusUnauthorized)
			return
		}

		logger.LogInfo("auth", "Authentication successful, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
		// Token is valid, proceed to handler
		next(w, r)
	}
}

// extractSessionFromAuth extracts session ID from Authorization header
// Per spec 7.1: When API key is configured, Authorization contains plain API key
// When API key is not configured, supports Bearer token for backward compatibility
func extractSessionFromAuth(authHeader string) string {
	if strings.HasPrefix(authHeader, "Bearer ") {
		// Bearer token format (for backward compatibility when no API key)
		sessionID := strings.TrimPrefix(authHeader, "Bearer ")
		return strings.TrimSpace(sessionID)
	} else if authHeader != "" {
		// Plain format (per spec 7.1 - API key is session ID)
		return authHeader
	}
	return ""
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
