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

	// Check response body - use map[string]interface{} to handle mixed types
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Check status field - should be "healthy" or "unhealthy"
	status, ok := response["status"].(string)
	if !ok {
		t.Error("Expected 'status' field to be a string")
	}
	if status != "healthy" && status != "unhealthy" {
		t.Errorf("Expected status 'healthy' or 'unhealthy', got '%s'", status)
	}

	// Check specVersion field (required by spec 8.1.1)
	specVersion, ok := response["specVersion"].(string)
	if !ok {
		t.Error("Expected 'specVersion' field to be a string")
	}
	if specVersion != MCPGatewaySpecVersion {
		t.Errorf("Expected specVersion '%s', got '%s'", MCPGatewaySpecVersion, specVersion)
	}

	// Check gatewayVersion field (required by spec 8.1.1)
	gatewayVersion, ok := response["gatewayVersion"].(string)
	if !ok {
		t.Error("Expected 'gatewayVersion' field to be a string")
	}
	if gatewayVersion == "" {
		t.Error("Expected gatewayVersion to be non-empty")
	}

	// Check servers field (required by spec 8.1.1)
	servers, ok := response["servers"].(map[string]interface{})
	if !ok {
		t.Error("Expected 'servers' field to be an object")
	}
	// With empty config, servers map should be empty
	if len(servers) != 0 {
		t.Errorf("Expected empty servers map with no configured servers, got %d servers", len(servers))
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

	// Check response body - use map[string]interface{} to handle mixed types
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Check status field - should be "healthy" or "unhealthy"
	status, ok := response["status"].(string)
	if !ok {
		t.Error("Expected 'status' field to be a string")
	}
	if status != "healthy" && status != "unhealthy" {
		t.Errorf("Expected status 'healthy' or 'unhealthy', got '%s'", status)
	}

	// Check specVersion field (required by spec 8.1.1)
	specVersion, ok := response["specVersion"].(string)
	if !ok {
		t.Error("Expected 'specVersion' field to be a string")
	}
	if specVersion != MCPGatewaySpecVersion {
		t.Errorf("Expected specVersion '%s', got '%s'", MCPGatewaySpecVersion, specVersion)
	}

	// Check gatewayVersion field (required by spec 8.1.1)
	gatewayVersion, ok := response["gatewayVersion"].(string)
	if !ok {
		t.Error("Expected 'gatewayVersion' field to be a string")
	}
	if gatewayVersion == "" {
		t.Error("Expected gatewayVersion to be non-empty")
	}

	// Check servers field (required by spec 8.1.1)
	servers, ok := response["servers"].(map[string]interface{})
	if !ok {
		t.Error("Expected 'servers' field to be an object")
	}
	// With empty config, servers map should be empty
	if len(servers) != 0 {
		t.Errorf("Expected empty servers map with no configured servers, got %d servers", len(servers))
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
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Check status field - should be "healthy" or "unhealthy"
	status, ok := response["status"].(string)
	if !ok {
		t.Error("Expected 'status' field to be a string")
	}
	if status != "healthy" && status != "unhealthy" {
		t.Errorf("Expected status 'healthy' or 'unhealthy', got '%s'", status)
	}

	// Check specVersion field (required by spec 8.1.1)
	specVersion, ok := response["specVersion"].(string)
	if !ok {
		t.Error("Expected 'specVersion' field to be a string")
	}
	if specVersion != MCPGatewaySpecVersion {
		t.Errorf("Expected specVersion '%s', got '%s'", MCPGatewaySpecVersion, specVersion)
	}

	// Check gatewayVersion field (required by spec 8.1.1)
	gatewayVersion, ok := response["gatewayVersion"].(string)
	if !ok {
		t.Error("Expected 'gatewayVersion' field to be a string")
	}
	if gatewayVersion == "" {
		t.Error("Expected gatewayVersion to be non-empty")
	}
}
