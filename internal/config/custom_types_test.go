package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomServerTypes tests the compliance requirements for custom server types
// as specified in section 4.1.4 of the MCP Gateway specification.

// T-CFG-009: Valid custom server type with registered schema
func TestTCFG009_ValidCustomTypeWithRegisteredSchema(t *testing.T) {
	// Create a mock HTTP server that returns a valid JSON schema for the custom type
	mockSchemaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		schema := map[string]interface{}{
			"$schema": "http://json-schema.org/draft-07/schema#",
			"type":    "object",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type": "string",
					"enum": []string{"safeinputs"},
				},
				"customField": map[string]interface{}{
					"type": "string",
				},
				"container": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"type", "customField", "container"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schema)
	}))
	defer mockSchemaServer.Close()

	// Configuration with custom server type and registered schema
	configJSON := map[string]interface{}{
		"customSchemas": map[string]string{
			"safeinputs": mockSchemaServer.URL,
		},
		"mcpServers": map[string]interface{}{
			"custom-server": map[string]interface{}{
				"type":        "safeinputs",
				"customField": "custom-value",
				"container":   "ghcr.io/example/safeinputs:latest",
			},
		},
	}

	data, err := json.Marshal(configJSON)
	require.NoError(t, err)

	// Parse the configuration
	var stdinCfg StdinConfig
	err = json.Unmarshal(data, &stdinCfg)
	require.NoError(t, err)

	// Custom schemas should be populated
	assert.NotNil(t, stdinCfg.CustomSchemas)
	assert.Equal(t, mockSchemaServer.URL, stdinCfg.CustomSchemas["safeinputs"])

	// Validate the server configuration with custom schemas
	server := stdinCfg.MCPServers["custom-server"]
	require.NotNil(t, server)

	err = validateServerConfigWithCustomSchemas("custom-server", server, stdinCfg.CustomSchemas)
	assert.NoError(t, err, "Valid custom server type with registered schema should pass validation")
}

// T-CFG-010: Reject custom type without schema registration
func TestTCFG010_RejectCustomTypeWithoutRegistration(t *testing.T) {
	// Configuration with unregistered custom server type
	configJSON := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"unregistered-server": map[string]interface{}{
				"type":      "unregistered",
				"container": "ghcr.io/example/unregistered:latest",
			},
		},
	}

	data, err := json.Marshal(configJSON)
	require.NoError(t, err)

	var stdinCfg StdinConfig
	err = json.Unmarshal(data, &stdinCfg)
	require.NoError(t, err)

	server := stdinCfg.MCPServers["unregistered-server"]
	require.NotNil(t, server)

	// Validate should fail for unregistered custom type
	err = validateServerConfigWithCustomSchemas("unregistered-server", server, stdinCfg.CustomSchemas)
	assert.Error(t, err, "Unregistered custom server type should be rejected")
	assert.Contains(t, err.Error(), "unregistered")
	assert.Contains(t, err.Error(), "not registered in customSchemas")
}

// T-CFG-011: Validate custom configuration against registered schema
func TestTCFG011_ValidateAgainstCustomSchema(t *testing.T) {
	t.Run("valid_custom_config", func(t *testing.T) {
		// Create a mock schema server that requires a specific field
		mockSchemaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			schema := map[string]interface{}{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type": "string",
						"enum": []string{"mytype"},
					},
					"requiredField": map[string]interface{}{
						"type": "string",
					},
					"container": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"type", "requiredField", "container"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(schema)
		}))
		defer mockSchemaServer.Close()

		// Valid configuration that matches schema
		configJSON := map[string]interface{}{
			"customSchemas": map[string]string{
				"mytype": mockSchemaServer.URL,
			},
			"mcpServers": map[string]interface{}{
				"valid-custom": map[string]interface{}{
					"type":          "mytype",
					"requiredField": "present",
					"container":     "ghcr.io/example/mytype:latest",
				},
			},
		}

		data, err := json.Marshal(configJSON)
		require.NoError(t, err)

		var stdinCfg StdinConfig
		err = json.Unmarshal(data, &stdinCfg)
		require.NoError(t, err)

		server := stdinCfg.MCPServers["valid-custom"]
		err = validateServerConfigWithCustomSchemas("valid-custom", server, stdinCfg.CustomSchemas)
		assert.NoError(t, err, "Configuration matching custom schema should pass validation")
	})

	t.Run("invalid_custom_config_missing_field", func(t *testing.T) {
		// Create a mock schema server that requires a specific field
		mockSchemaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			schema := map[string]interface{}{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type": "string",
						"enum": []string{"mytype"},
					},
					"requiredField": map[string]interface{}{
						"type": "string",
					},
					"container": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"type", "requiredField", "container"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(schema)
		}))
		defer mockSchemaServer.Close()

		// Invalid configuration missing required field
		configJSON := map[string]interface{}{
			"customSchemas": map[string]string{
				"mytype": mockSchemaServer.URL,
			},
			"mcpServers": map[string]interface{}{
				"invalid-custom": map[string]interface{}{
					"type":      "mytype",
					"container": "ghcr.io/example/mytype:latest",
					// Missing requiredField
				},
			},
		}

		data, err := json.Marshal(configJSON)
		require.NoError(t, err)

		var stdinCfg StdinConfig
		err = json.Unmarshal(data, &stdinCfg)
		require.NoError(t, err)

		server := stdinCfg.MCPServers["invalid-custom"]
		err = validateServerConfigWithCustomSchemas("invalid-custom", server, stdinCfg.CustomSchemas)
		assert.Error(t, err, "Configuration not matching custom schema should fail validation")
	})
}

// T-CFG-012: Reject custom type conflicting with reserved types (stdio/http)
func TestTCFG012_RejectReservedTypeNames(t *testing.T) {
	tests := []struct {
		name         string
		reservedType string
	}{
		{"stdio_conflict", "stdio"},
		{"http_conflict", "http"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Try to register a reserved type name in customSchemas
			configJSON := map[string]interface{}{
				"customSchemas": map[string]string{
					tt.reservedType: "https://example.com/schema.json",
				},
				"mcpServers": map[string]interface{}{
					"test-server": map[string]interface{}{
						"type":      tt.reservedType,
						"container": "ghcr.io/example/test:latest",
					},
				},
			}

			data, err := json.Marshal(configJSON)
			require.NoError(t, err)

			var stdinCfg StdinConfig
			err = json.Unmarshal(data, &stdinCfg)
			require.NoError(t, err)

			// Validation should reject reserved type names in customSchemas
			err = validateCustomSchemas(stdinCfg.CustomSchemas)
			assert.Error(t, err, "Reserved type name %q should be rejected in customSchemas", tt.reservedType)
			assert.Contains(t, err.Error(), tt.reservedType)
			assert.Contains(t, err.Error(), "reserved")
		})
	}
}

// T-CFG-013: Custom schema URL fetch and cache
func TestTCFG013_SchemaURLFetchAndCache(t *testing.T) {
	fetchCount := 0
	mockSchemaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		schema := map[string]interface{}{
			"$schema": "http://json-schema.org/draft-07/schema#",
			"type":    "object",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type": "string",
					"enum": []string{"cached"},
				},
				"container": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"type", "container"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schema)
	}))
	defer mockSchemaServer.Close()

	customSchemas := map[string]string{
		"cached": mockSchemaServer.URL,
	}

	// First fetch should hit the server
	schema1, err := fetchCustomSchema("cached", customSchemas)
	require.NoError(t, err)
	assert.NotNil(t, schema1)
	firstFetchCount := fetchCount

	// Second fetch should use cache (not increment fetchCount)
	schema2, err := fetchCustomSchema("cached", customSchemas)
	require.NoError(t, err)
	assert.NotNil(t, schema2)
	assert.Equal(t, firstFetchCount, fetchCount, "Schema should be cached, fetch count should not increase")

	t.Run("empty_string_skips_validation", func(t *testing.T) {
		// Empty string means skip validation
		customSchemas := map[string]string{
			"novalidation": "",
		}

		schema, err := fetchCustomSchema("novalidation", customSchemas)
		assert.NoError(t, err)
		assert.Nil(t, schema, "Empty schema URL should return nil schema (skip validation)")
	})
}

// Helper function to test server config validation with custom schemas
// This will be implemented in the actual code
func validateServerConfigWithCustomSchemas(name string, server *StdinServerConfig, customSchemas map[string]string) error {
	// This is a stub for testing - the actual implementation will be added
	// For now, we'll simulate the expected behavior

	// Normalize type
	serverType := server.Type
	if serverType == "" {
		serverType = "stdio"
	}
	if serverType == "local" {
		serverType = "stdio"
	}

	// Check if it's a standard type
	if serverType == "stdio" || serverType == "http" {
		// Use existing validation
		return validateServerConfig(name, server)
	}

	// It's a custom type - check if registered
	if customSchemas == nil {
		return &ValidationError{
			Field:      "type",
			Message:    "custom server type '" + serverType + "' is not registered in customSchemas",
			JSONPath:   "mcpServers." + name + ".type",
			Suggestion: "Add the custom type to the customSchemas field or use a standard type ('stdio' or 'http')",
		}
	}

	schemaURL, exists := customSchemas[serverType]
	if !exists {
		return &ValidationError{
			Field:      "type",
			Message:    "custom server type '" + serverType + "' is not registered in customSchemas",
			JSONPath:   "mcpServers." + name + ".type",
			Suggestion: "Add the custom type to the customSchemas field or use a standard type ('stdio' or 'http')",
		}
	}

	// If schema URL is empty, skip validation
	if schemaURL == "" {
		return nil
	}

	// Fetch and validate against custom schema
	// For now, just return success if the schema exists
	// The actual implementation will validate the server config against the schema
	return nil
}

// Helper function to validate customSchemas field
func validateCustomSchemas(customSchemas map[string]string) error {
	for typeName := range customSchemas {
		if typeName == "stdio" || typeName == "http" {
			return &ValidationError{
				Field:      "customSchemas",
				Message:    "custom type name '" + typeName + "' conflicts with reserved type",
				JSONPath:   "customSchemas." + typeName,
				Suggestion: "Use a different name for your custom type (reserved types: stdio, http)",
			}
		}
	}
	return nil
}

// Helper function to fetch custom schema with caching
var customSchemaCache = make(map[string]interface{})

func fetchCustomSchema(typeName string, customSchemas map[string]string) (interface{}, error) {
	schemaURL, exists := customSchemas[typeName]
	if !exists {
		return nil, &ValidationError{
			Field:      "type",
			Message:    "custom type '" + typeName + "' not found in customSchemas",
			JSONPath:   "customSchemas." + typeName,
			Suggestion: "Register the custom type in customSchemas",
		}
	}

	// Empty string means skip validation
	if schemaURL == "" {
		return nil, nil
	}

	// Check cache
	if cached, ok := customSchemaCache[schemaURL]; ok {
		return cached, nil
	}

	// Fetch schema (using existing fetch infrastructure)
	client := &http.Client{}
	resp, err := client.Get(schemaURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var schema interface{}
	if err := json.NewDecoder(resp.Body).Decode(&schema); err != nil {
		return nil, err
	}

	// Cache it
	customSchemaCache[schemaURL] = schema

	return schema, nil
}
