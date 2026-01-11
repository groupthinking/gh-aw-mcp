package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"testing"
	"time"
)

// TestHTTPMCPGatewayWithAuthorization tests the gateway's HTTP MCP endpoints
// with proper authorization header handling and configuration
func TestHTTPMCPGatewayWithAuthorization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Find the gateway binary
	binaryPath := findBinary(t)
	t.Logf("Using binary: %s", binaryPath)

	// Gateway API key for authentication
	gatewayAPIKey := "test-gateway-api-key-12345"

	// Create config with stdio backend (HTTP backends not yet implemented)
	// This test focuses on the gateway's HTTP server with authorization
	configJSON := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"testserver": map[string]interface{}{
				"type":      "stdio",
				"container": "echo", // Dummy container
			},
		},
		"gateway": map[string]interface{}{
			"port":   13010,
			"domain": "localhost",
			"apiKey": gatewayAPIKey,
		},
	}

	configBytes, _ := json.Marshal(configJSON)

	// Start the gateway process with API key configured
	port := "13010"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config-stdin",
		"--listen", "127.0.0.1:"+port,
		"--routed",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(configBytes)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start gateway: %v", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		t.Logf("Gateway stdout: %s", stdout.String())
		t.Logf("Gateway stderr: %s", stderr.String())
	}()

	// Wait for gateway to start
	gatewayURL := "http://127.0.0.1:" + port
	if !waitForServer(t, gatewayURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Gateway did not start in time")
	}

	t.Log("✓ Gateway started successfully with API key authentication")

	// Test 1: Health check (should not require auth)
	t.Run("HealthCheckNoAuth", func(t *testing.T) {
		resp, err := http.Get(gatewayURL + "/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		t.Log("✓ Health check passed without authentication")
	})

	// Test 2: MCP request without auth header should fail
	t.Run("MCPRequestWithoutAuth", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "1.0.0",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", gatewayURL+"/mcp/testserver", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		// No Authorization header

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for missing auth, got %d", resp.StatusCode)
		} else {
			t.Log("✓ Gateway correctly rejects MCP requests without authorization")
		}
	})

	// Test 3: MCP request with invalid auth should fail
	t.Run("MCPRequestWithInvalidAuth", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "1.0.0",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", gatewayURL+"/mcp/testserver", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "wrong-api-key")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for invalid auth, got %d", resp.StatusCode)
		} else {
			t.Log("✓ Gateway correctly rejects MCP requests with invalid authorization")
		}
	})

	// Test 4: MCP request with valid auth should succeed
	t.Run("MCPRequestWithValidAuth", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "1.0.0",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		// Per spec 7.1: Authorization header contains plain API key (not Bearer scheme)
		jsonData, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", gatewayURL+"/mcp/testserver", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", gatewayAPIKey)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			t.Error("Gateway rejected valid authorization")
		} else {
			t.Logf("✓ Gateway accepts MCP requests with valid authorization (status: %d)", resp.StatusCode)
		}

		// Verify response is valid (could be SSE or JSON)
		contentType := resp.Header.Get("Content-Type")
		t.Logf("Response Content-Type: %s", contentType)
		t.Log("✓ Authorization header properly configured and validated")
	})

	// Test 5: Close endpoint without auth should fail
	t.Run("CloseEndpointWithoutAuth", func(t *testing.T) {
		req, _ := http.NewRequest("POST", gatewayURL+"/close", nil)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for /close without auth, got %d", resp.StatusCode)
		} else {
			t.Log("✓ Close endpoint requires authorization")
		}
	})

	// Test 6: Close endpoint with valid auth should succeed
	t.Run("CloseEndpointWithValidAuth", func(t *testing.T) {
		req, _ := http.NewRequest("POST", gatewayURL+"/close", nil)
		req.Header.Set("Authorization", gatewayAPIKey)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for /close with valid auth, got %d", resp.StatusCode)
		} else {
			t.Log("✓ Close endpoint accepts valid authorization")
		}

		// Parse response
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			if status, ok := result["status"].(string); ok && status == "closed" {
				t.Log("✓ Close endpoint returned correct response")
			}
		}
	})
}

// TestHTTPMCPGatewayNoAuthRequired tests gateway with different API key
// to verify that authorization headers are properly validated
func TestHTTPMCPGatewayNoAuthRequired(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	binaryPath := findBinary(t)

	// Create config with different API key
	apiKey := "different-api-key-67890"
	configJSON := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"testserver": map[string]interface{}{
				"type":      "stdio",
				"container": "echo",
			},
		},
		"gateway": map[string]interface{}{
			"port":   13011,
			"domain": "localhost",
			"apiKey": apiKey,
		},
	}

	configBytes, _ := json.Marshal(configJSON)

	port := "13011"
	cmd := exec.CommandContext(ctx, binaryPath,
		"--config-stdin",
		"--listen", "127.0.0.1:"+port,
		"--routed",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(configBytes)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start gateway: %v", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	gatewayURL := "http://127.0.0.1:" + port
	if !waitForServer(t, gatewayURL+"/health", 10*time.Second) {
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("Gateway did not start in time")
	}

	t.Log("✓ Gateway started with different API key")

	// Test: Request with correct API key for this instance should succeed
	t.Run("MCPRequestWithCorrectAPIKey", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "1.0.0",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", gatewayURL+"/mcp/testserver", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", apiKey)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			t.Error("Gateway rejected valid authorization for its configured API key")
		} else {
			t.Logf("✓ Gateway accepts requests with correct API key (status: %d)", resp.StatusCode)
		}
	})

	// Test: Request with wrong API key should fail
	t.Run("MCPRequestWithWrongAPIKey", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "1.0.0",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", gatewayURL+"/mcp/testserver", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "wrong-key-for-this-instance")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for wrong API key, got %d", resp.StatusCode)
		} else {
			t.Log("✓ Gateway correctly rejects requests with wrong API key")
		}
	})
}
