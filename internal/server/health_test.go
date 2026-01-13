package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

// serverCreator defines a function type for creating HTTP servers
type serverCreator func(addr string, us *UnifiedServer, apiKey string) *http.Server

// TestHealthEndpoint tests the health endpoint across different server modes and scenarios
func TestHealthEndpoint(t *testing.T) {
	tests := []struct {
		name          string
		createServer  serverCreator
		apiKey        string
		expectServers int
		serverConfigs map[string]*config.ServerConfig
	}{
		{
			name:          "RoutedMode/EmptyConfig",
			createServer:  CreateHTTPServerForRoutedMode,
			apiKey:        "",
			expectServers: 0,
			serverConfigs: map[string]*config.ServerConfig{},
		},
		{
			name:          "UnifiedMode/EmptyConfig",
			createServer:  CreateHTTPServerForMCP,
			apiKey:        "",
			expectServers: 0,
			serverConfigs: map[string]*config.ServerConfig{},
		},
		{
			name:          "RoutedMode/WithApiKey",
			createServer:  CreateHTTPServerForRoutedMode,
			apiKey:        "test-api-key",
			expectServers: 0,
			serverConfigs: map[string]*config.ServerConfig{},
		},
		{
			name:          "UnifiedMode/WithApiKey",
			createServer:  CreateHTTPServerForMCP,
			apiKey:        "test-api-key",
			expectServers: 0,
			serverConfigs: map[string]*config.ServerConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config
			cfg := &config.Config{
				Servers: tt.serverConfigs,
			}

			// Create unified server
			ctx := context.Background()
			us, err := NewUnified(ctx, cfg)
			require.NoError(t, err, "Failed to create unified server")
			t.Cleanup(func() { us.Close() })

			// Create HTTP server using the provided creator function
			httpServer := tt.createServer(":0", us, tt.apiKey)

			// Create test request
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()

			// Execute request
			httpServer.Handler.ServeHTTP(w, req)

			// Verify response
			verifyHealthResponse(t, w, tt.expectServers)
		})
	}
}

// TestHealthEndpoint_NoAuthRequired tests that health endpoint works without authentication
func TestHealthEndpoint_NoAuthRequired(t *testing.T) {
	// Create config
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}

	// Create unified server
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	t.Cleanup(func() { us.Close() })

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

	// Verify basic response structure
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if _, ok := response["status"]; !ok {
		t.Error("Expected 'status' field in response")
	}
	if _, ok := response["specVersion"]; !ok {
		t.Error("Expected 'specVersion' field in response")
	}
	if _, ok := response["gatewayVersion"]; !ok {
		t.Error("Expected 'gatewayVersion' field in response")
	}
}

// TestHealthEndpoint_ResponseFields tests specific response field validation
func TestHealthEndpoint_ResponseFields(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		validate  func(t *testing.T, value interface{})
	}{
		{
			name:      "StatusField/ValidValues",
			fieldName: "status",
			validate: func(t *testing.T, value interface{}) {
				t.Helper()
				status, ok := value.(string)
				if !ok {
					t.Fatalf("Expected 'status' to be string, got %T", value)
				}
				if status != "healthy" && status != "unhealthy" {
					t.Errorf("Expected status 'healthy' or 'unhealthy', got '%s'", status)
				}
			},
		},
		{
			name:      "SpecVersionField/MustMatch",
			fieldName: "specVersion",
			validate: func(t *testing.T, value interface{}) {
				t.Helper()
				specVersion, ok := value.(string)
				if !ok {
					t.Fatalf("Expected 'specVersion' to be string, got %T", value)
				}
				assert.Equal(t, MCPGatewaySpecVersion, specVersion, "specVersion '%s', got '%s'")
			},
		},
		{
			name:      "GatewayVersionField/NotEmpty",
			fieldName: "gatewayVersion",
			validate: func(t *testing.T, value interface{}) {
				t.Helper()
				gatewayVersion, ok := value.(string)
				if !ok {
					t.Fatalf("Expected 'gatewayVersion' to be string, got %T", value)
				}
				if gatewayVersion == "" {
					t.Error("Expected gatewayVersion to be non-empty")
				}
			},
		},
		{
			name:      "ServersField/IsMap",
			fieldName: "servers",
			validate: func(t *testing.T, value interface{}) {
				t.Helper()
				servers, ok := value.(map[string]interface{})
				if !ok {
					t.Fatalf("Expected 'servers' to be map[string]interface{}, got %T", value)
				}
				// With empty config, servers map should be empty
				if len(servers) != 0 {
					t.Errorf("Expected empty servers map with no configured servers, got %d servers", len(servers))
				}
			},
		},
	}

	// Create test server once for all field validation tests
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{},
	}
	ctx := context.Background()
	us, err := NewUnified(ctx, cfg)
	require.NoError(t, err, "Failed to create unified server")
	defer us.Close()

	httpServer := CreateHTTPServerForRoutedMode(":0", us, "")

	// Get response once
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Run field validation tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, exists := response[tt.fieldName]
			if !exists {
				t.Fatalf("Expected field '%s' to exist in response", tt.fieldName)
			}
			tt.validate(t, value)
		})
	}
}

// verifyHealthResponse is a helper that validates the health endpoint response
func verifyHealthResponse(t *testing.T, w *httptest.ResponseRecorder, expectServers int) {
	t.Helper()

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "Content-Type 'application/json', got '%s'")

	// Check response body
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Check required fields exist
	requiredFields := []string{"status", "specVersion", "gatewayVersion", "servers"}
	for _, field := range requiredFields {
		if _, ok := response[field]; !ok {
			t.Errorf("Expected field '%s' to exist in response", field)
		}
	}

	// Check status field
	if status, ok := response["status"].(string); ok {
		if status != "healthy" && status != "unhealthy" {
			t.Errorf("Expected status 'healthy' or 'unhealthy', got '%s'", status)
		}
	} else {
		t.Error("Expected 'status' field to be a string")
	}

	// Check specVersion field
	if specVersion, ok := response["specVersion"].(string); ok {
		assert.Equal(t, MCPGatewaySpecVersion, specVersion, "specVersion '%s', got '%s'")
	} else {
		t.Error("Expected 'specVersion' field to be a string")
	}

	// Check gatewayVersion field
	if gatewayVersion, ok := response["gatewayVersion"].(string); ok {
		if gatewayVersion == "" {
			t.Error("Expected gatewayVersion to be non-empty")
		}
	} else {
		t.Error("Expected 'gatewayVersion' field to be a string")
	}

	// Check servers field
	if servers, ok := response["servers"].(map[string]interface{}); ok {
		if len(servers) != expectServers {
			t.Errorf("Expected %d servers in response, got %d", expectServers, len(servers))
		}
	} else {
		t.Error("Expected 'servers' field to be a map")
	}
}
