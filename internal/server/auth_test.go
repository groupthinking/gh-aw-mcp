package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAuthMiddleware_MissingHeader tests that missing Authorization header returns 401
func TestAuthMiddleware_MissingHeader(t *testing.T) {
	apiKey := "test-secret-key"

	handler := authMiddleware(apiKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No Authorization header set
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "Unauthorized: missing Authorization header\n" {
		t.Errorf("Unexpected error message: %s", body)
	}
}

// TestAuthMiddleware_ValidBearerToken tests that valid Bearer token is accepted
func TestAuthMiddleware_ValidBearerToken(t *testing.T) {
	apiKey := "test-secret-key"

	handler := authMiddleware(apiKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "success" {
		t.Errorf("Handler was not called")
	}
}

// TestAuthMiddleware_InvalidBearerToken tests that invalid Bearer token returns 401
func TestAuthMiddleware_InvalidBearerToken(t *testing.T) {
	apiKey := "test-secret-key"

	handler := authMiddleware(apiKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "Unauthorized: invalid API key\n" {
		t.Errorf("Unexpected error message: %s", body)
	}
}

// TestAuthMiddleware_PlainAPIKeyNotAccepted tests that plain API key (without Bearer) returns 400
// Per MCP spec 2025-03-26: "Access tokens MUST use the Authorization header field in format 'Bearer <token>'"
func TestAuthMiddleware_PlainAPIKeyNotAccepted(t *testing.T) {
	apiKey := "test-secret-key"

	handler := authMiddleware(apiKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", apiKey) // Plain key without "Bearer " prefix
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "Bad Request: Authorization header must be 'Bearer <token>'\n" {
		t.Errorf("Unexpected error message: %s", body)
	}
}

// TestAuthMiddleware_EmptyBearerToken tests that empty token after Bearer prefix returns 400
func TestAuthMiddleware_EmptyBearerToken(t *testing.T) {
	apiKey := "test-secret-key"

	handler := authMiddleware(apiKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer ") // Empty token
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "Bad Request: Bearer token cannot be empty\n" {
		t.Errorf("Unexpected error message: %s", body)
	}
}

// TestAuthMiddleware_BearerTokenWithWhitespace tests that Bearer token with whitespace is trimmed
func TestAuthMiddleware_BearerTokenWithWhitespace(t *testing.T) {
	apiKey := "test-secret-key"

	handler := authMiddleware(apiKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer  "+apiKey+"  ") // Extra whitespace
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (whitespace should be trimmed), got %d", w.Code)
	}
}

// TestAuthMiddleware_TokenInQueryString tests that tokens in query string are rejected
// Per MCP spec 2025-03-26: "Access tokens MUST NOT be included in URI query strings"
func TestAuthMiddleware_TokenInQueryString(t *testing.T) {
	apiKey := "test-secret-key"

	handler := authMiddleware(apiKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	tests := []struct {
		name string
		path string
	}{
		{"token parameter", "/test?token=secret123"},
		{"access_token parameter", "/test?access_token=secret123"},
		{"apiKey parameter", "/test?apiKey=secret123"},
		{"mixed with other params", "/test?foo=bar&token=secret123&baz=qux"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Authorization", "Bearer "+apiKey)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}

			body := w.Body.String()
			if body != "Bad Request: tokens must not be included in query string\n" {
				t.Errorf("Unexpected error message: %s", body)
			}
		})
	}
}

// TestAuthMiddleware_CaseSensitiveBearer tests that "bearer" (lowercase) is not accepted
func TestAuthMiddleware_CaseSensitiveBearer(t *testing.T) {
	apiKey := "test-secret-key"

	handler := authMiddleware(apiKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "bearer "+apiKey) // lowercase "bearer"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d (Bearer must be capitalized)", w.Code)
	}
}

// TestAuthMiddleware_BasicAuthNotAccepted tests that Basic auth is not accepted
func TestAuthMiddleware_BasicAuthNotAccepted(t *testing.T) {
	apiKey := "test-secret-key"

	handler := authMiddleware(apiKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic dGVzdDp0ZXN0") // Base64 encoded "test:test"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestAuthMiddleware_MultipleAuthHeaders tests behavior with multiple Authorization headers
func TestAuthMiddleware_MultipleAuthHeaders(t *testing.T) {
	apiKey := "test-secret-key"

	handler := authMiddleware(apiKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Add multiple Authorization headers (only first is used by http.Header.Get)
	req.Header.Add("Authorization", "Bearer "+apiKey)
	req.Header.Add("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should use the first header value
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (first header should be used), got %d", w.Code)
	}
}
