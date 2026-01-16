package mcp

import (
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

// NormalizeInputSchema fixes common schema validation issues in tool definitions
// that can cause downstream validation errors.
//
// Known issues fixed:
//  1. Missing schema: When a backend returns no inputSchema (nil), we provide
//     a default empty object schema that accepts any properties. This is required
//     by the MCP SDK's Server.AddTool method.
//  2. Object schemas without properties: When a schema declares "type": "object"
//     but is missing the required "properties" field, we add an empty properties
//     object to make it valid per JSON Schema standards.
func NormalizeInputSchema(schema map[string]interface{}, toolName string) map[string]interface{} {
	// If backend didn't provide a schema, use a default empty object schema
	// This allows the tool to be registered and clients will see it accepts any parameters
	if schema == nil {
		logger.LogWarn("backend", "Tool schema normalized: %s - backend provided no inputSchema, using default empty object schema", toolName)
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	// Check if this is an object type schema
	typeVal, hasType := schema["type"]
	if !hasType {
		return schema
	}

	typeStr, isString := typeVal.(string)
	if !isString || typeStr != "object" {
		return schema
	}

	// Check if properties field exists
	_, hasProperties := schema["properties"]
	_, hasAdditionalProperties := schema["additionalProperties"]

	// If it's an object type but missing both properties and additionalProperties,
	// add an empty properties object to make it valid
	if !hasProperties && !hasAdditionalProperties {
		logger.LogWarn("backend", "Tool schema normalized: %s - added empty properties to object type schema", toolName)

		// Create a copy of the schema to avoid modifying the original
		normalized := make(map[string]interface{})
		for k, v := range schema {
			normalized[k] = v
		}
		normalized["properties"] = map[string]interface{}{}

		return normalized
	}

	return schema
}
