package mcp

import (
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

// NormalizeInputSchema fixes common schema validation issues in tool definitions
// that can cause downstream validation errors.
//
// Known issues fixed:
//  1. Object schemas without properties: When a schema declares "type": "object"
//     but is missing the required "properties" field, we add an empty properties
//     object to make it valid per JSON Schema standards.
func NormalizeInputSchema(schema map[string]interface{}, toolName string) map[string]interface{} {
	if schema == nil {
		return schema
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
