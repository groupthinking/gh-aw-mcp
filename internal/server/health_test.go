package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

// TestHealthEndpoint_RoutedMode tests that the health endpoint returns proper JSON response
func TestHealthEndpoint_RoutedMode(t *testing.T) {
	// Create a minimal config
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}
	
	// Create unified server
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create unified server: %v", err)
	}
	defer us.Close()
	
	// Create HTTP server
	httpServer := CreateHTTPServerForRoutedMode(":0", us, "")
	
	// Create test request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	// Execute request
	httpServer.Handler.ServeHTTP(w, req)
	
	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
	
	// Check response body
	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	
	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response["status"])
	}
	
	if response["protocolVersion"] != MCPProtocolVersion {
		t.Errorf("Expected protocolVersion '%s', got '%s'", MCPProtocolVersion, response["protocolVersion"])
	}
	
	if response["version"] == "" {
		t.Error("Expected version to be non-empty")
	}
}

// TestHealthEndpoint_UnifiedMode tests that the health endpoint returns proper JSON response in unified mode
func TestHealthEndpoint_UnifiedMode(t *testing.T) {
	// Create a minimal config
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}
	
	// Create unified server
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create unified server: %v", err)
	}
	defer us.Close()
	
	// Create HTTP server
	httpServer := CreateHTTPServerForMCP(":0", us, "")
	
	// Create test request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	// Execute request
	httpServer.Handler.ServeHTTP(w, req)
	
	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
	
	// Check response body
	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	
	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response["status"])
	}
	
	if response["protocolVersion"] != MCPProtocolVersion {
		t.Errorf("Expected protocolVersion '%s', got '%s'", MCPProtocolVersion, response["protocolVersion"])
	}
	
	if response["version"] == "" {
		t.Error("Expected version to be non-empty")
	}
}

// TestHealthEndpoint_NoAuth tests that health endpoint doesn't require authentication
func TestHealthEndpoint_NoAuth(t *testing.T) {
	// Create a minimal config
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}
	
	// Create unified server
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create unified server: %v", err)
	}
	defer us.Close()
	
	// Create HTTP server WITH API key (health should still work without auth)
	httpServer := CreateHTTPServerForRoutedMode(":0", us, "test-api-key")
	
	// Create test request WITHOUT Authorization header
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	// Execute request
	httpServer.Handler.ServeHTTP(w, req)
	
	// Health endpoint should work without auth
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (health should not require auth), got %d", w.Code)
	}
	
	// Verify JSON response
	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	
	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response["status"])
	}
	
	if response["protocolVersion"] != MCPProtocolVersion {
		t.Errorf("Expected protocolVersion '%s', got '%s'", MCPProtocolVersion, response["protocolVersion"])
	}
	
	if response["version"] == "" {
		t.Error("Expected version to be non-empty")
	}
}
