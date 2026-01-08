package server

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// authMiddleware implements API key authentication per spec section 7.1
func authMiddleware(apiKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			// Spec 7.1: Missing token returns 401
			logRuntimeError("authentication_failed", "missing_auth_header", r, nil)
			http.Error(w, "Unauthorized: missing Authorization header", http.StatusUnauthorized)
			return
		}

		// Spec 7.1: Malformed header returns 400
		if !strings.HasPrefix(authHeader, "Bearer ") && authHeader != apiKey {
			logRuntimeError("authentication_failed", "malformed_auth_header", r, nil)
			http.Error(w, "Bad Request: Authorization header must be 'Bearer <token>' or plain API key", http.StatusBadRequest)
			return
		}

		// Extract token
		token := authHeader
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// Spec 7.1: Invalid token returns 401
		if token != apiKey {
			logRuntimeError("authentication_failed", "invalid_token", r, nil)
			http.Error(w, "Unauthorized: invalid API key", http.StatusUnauthorized)
			return
		}

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
