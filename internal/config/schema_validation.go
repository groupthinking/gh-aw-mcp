package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed schemas/mcp-gateway-config.schema.json
var schemaJSON []byte

var (
	// Compile regex patterns from schema for additional validation
	containerPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9./_-]*(:([a-zA-Z0-9._-]+|latest))?$`)
	urlPattern       = regexp.MustCompile(`^https?://.+`)
	mountPattern     = regexp.MustCompile(`^[^:]+:[^:]+:(ro|rw)$`)
	domainVarPattern = regexp.MustCompile(`^\$\{[A-Z_][A-Z0-9_]*\}$`)
	
	// gatewayVersion stores the version string to include in error messages
	gatewayVersion = "dev"
)

// SetVersion sets the gateway version for error reporting
func SetVersion(version string) {
	if version != "" {
		gatewayVersion = version
	}
}

// validateJSONSchema validates the raw JSON configuration against the JSON schema
func validateJSONSchema(data []byte) error {
	// Parse the schema
	var schemaData interface{}
	if err := json.Unmarshal(schemaJSON, &schemaData); err != nil {
		return fmt.Errorf("failed to parse embedded schema: %w", err)
	}

	// Compile the schema
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft7

	// Add the schema with its $id
	schemaURL := "https://github.com/githubnext/gh-aw/blob/main/docs/public/schemas/mcp-gateway-config.schema.json"
	if err := compiler.AddResource(schemaURL, strings.NewReader(string(schemaJSON))); err != nil {
		return fmt.Errorf("failed to add schema resource: %w", err)
	}

	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	// Parse the configuration
	var configObj interface{}
	if err := json.Unmarshal(data, &configObj); err != nil {
		return fmt.Errorf("failed to parse configuration JSON: %w", err)
	}

	// Validate the configuration
	if err := schema.Validate(configObj); err != nil {
		return formatSchemaError(err)
	}

	return nil
}

// formatSchemaError formats JSON schema validation errors to be user-friendly
func formatSchemaError(err error) error {
	if err == nil {
		return nil
	}

	// The jsonschema library returns a ValidationError type with detailed info
	if ve, ok := err.(*jsonschema.ValidationError); ok {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Configuration validation error (MCP Gateway version: %s):\n\n", gatewayVersion))

		// Recursively format all errors
		formatValidationErrorRecursive(ve, &sb, 0)

		sb.WriteString("\n\nPlease check your configuration against the MCP Gateway specification at:")
		sb.WriteString("\nhttps://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md")
		sb.WriteString("\n\nJSON Schema reference:")
		sb.WriteString("\nhttps://github.com/githubnext/gh-aw/blob/main/docs/public/schemas/mcp-gateway-config.schema.json")

		return fmt.Errorf("%s", sb.String())
	}

	return fmt.Errorf("configuration validation error (version: %s): %s", gatewayVersion, err.Error())
}

// formatValidationErrorRecursive recursively formats validation errors with proper indentation
func formatValidationErrorRecursive(ve *jsonschema.ValidationError, sb *strings.Builder, depth int) {
	indent := strings.Repeat("  ", depth)
	
	// Format location and message
	location := ve.InstanceLocation
	if location == "" {
		location = "<root>"
	}
	sb.WriteString(fmt.Sprintf("%sLocation: %s\n", indent, location))
	sb.WriteString(fmt.Sprintf("%sError: %s\n", indent, ve.Message))
	
	// Add detailed context based on the error message
	context := formatErrorContext(ve, indent)
	if context != "" {
		sb.WriteString(context)
	}
	
	// Recursively process nested causes
	if len(ve.Causes) > 0 {
		for _, cause := range ve.Causes {
			formatValidationErrorRecursive(cause, sb, depth+1)
		}
	}
	
	// Add spacing between sibling errors at the same level
	if depth == 0 {
		sb.WriteString("\n")
	}
}

// formatErrorContext provides additional context about what caused the validation error
func formatErrorContext(ve *jsonschema.ValidationError, prefix string) string {
	var sb strings.Builder
	msg := ve.Message
	
	// For additional properties errors, explain what's wrong
	if strings.Contains(msg, "additionalProperties") || strings.Contains(msg, "additional property") {
		sb.WriteString(fmt.Sprintf("%sDetails: Configuration contains field(s) that are not defined in the schema\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Check for typos in field names or remove unsupported fields\n", prefix))
	}
	
	// For type errors, show the mismatch
	if strings.Contains(msg, "expected") && (strings.Contains(msg, "but got") || strings.Contains(msg, "type")) {
		sb.WriteString(fmt.Sprintf("%sDetails: Type mismatch - the value type doesn't match what's expected\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Verify the value is the correct type (string, number, boolean, object, array)\n", prefix))
	}
	
	// For enum errors (invalid values from a set of allowed values)
	if strings.Contains(msg, "value must be one of") || strings.Contains(msg, "must be") {
		sb.WriteString(fmt.Sprintf("%sDetails: Invalid value - the field has a restricted set of allowed values\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Check the documentation for the list of valid values\n", prefix))
	}
	
	// For missing required properties
	if strings.Contains(msg, "missing properties") || strings.Contains(msg, "required") {
		sb.WriteString(fmt.Sprintf("%sDetails: Required field(s) are missing\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Add the required field(s) to your configuration\n", prefix))
	}
	
	// For pattern validation failures (regex patterns)
	if strings.Contains(msg, "does not match pattern") || strings.Contains(msg, "pattern") {
		sb.WriteString(fmt.Sprintf("%sDetails: Value format is incorrect\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → The value must match a specific format or pattern\n", prefix))
	}
	
	// For minimum/maximum constraint violations
	if strings.Contains(msg, "must be >=") || strings.Contains(msg, "must be <=") || strings.Contains(msg, "minimum") || strings.Contains(msg, "maximum") {
		sb.WriteString(fmt.Sprintf("%sDetails: Value is outside the allowed range\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Adjust the value to be within the valid range\n", prefix))
	}
	
	// For oneOf errors (typically type selection issues)
	if strings.Contains(msg, "doesn't validate with any of") || strings.Contains(msg, "oneOf") {
		sb.WriteString(fmt.Sprintf("%sDetails: Configuration doesn't match any of the expected formats\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  → Review the structure and ensure it matches one of the valid configuration types\n", prefix))
	}
	
	// Add keyword location if it provides useful context
	if ve.KeywordLocation != "" && ve.KeywordLocation != ve.InstanceLocation {
		sb.WriteString(fmt.Sprintf("%sSchema location: %s\n", prefix, ve.KeywordLocation))
	}
	
	return sb.String()
}

// validateStringPatterns validates string fields against regex patterns from the schema
// This provides additional validation beyond the JSON schema validation
func validateStringPatterns(stdinCfg *StdinConfig) error {
	// Validate server configurations
	for name, server := range stdinCfg.MCPServers {
		jsonPath := fmt.Sprintf("mcpServers.%s", name)

		// Validate container pattern for stdio servers
		if server.Type == "" || server.Type == "stdio" || server.Type == "local" {
			if server.Container != "" && !containerPattern.MatchString(server.Container) {
				return &ValidationError{
					Field:      "container",
					Message:    fmt.Sprintf("container image '%s' does not match required pattern", server.Container),
					JSONPath:   fmt.Sprintf("%s.container", jsonPath),
					Suggestion: "Use a valid container image format (e.g., 'ghcr.io/owner/image:tag' or 'owner/image:latest')",
				}
			}

			// Validate mount patterns
			for i, mount := range server.Mounts {
				if !mountPattern.MatchString(mount) {
					return &ValidationError{
						Field:      "mounts",
						Message:    fmt.Sprintf("mount '%s' does not match required pattern", mount),
						JSONPath:   fmt.Sprintf("%s.mounts[%d]", jsonPath, i),
						Suggestion: "Use format 'source:dest:mode' where mode is 'ro' or 'rw'",
					}
				}
			}

			// Validate entrypoint is not empty if provided
			if server.Entrypoint != "" && len(strings.TrimSpace(server.Entrypoint)) == 0 {
				return &ValidationError{
					Field:      "entrypoint",
					Message:    "entrypoint cannot be empty or whitespace only",
					JSONPath:   fmt.Sprintf("%s.entrypoint", jsonPath),
					Suggestion: "Provide a valid entrypoint path or remove the field",
				}
			}
		}

		// Validate URL pattern for HTTP servers
		if server.Type == "http" {
			if server.URL != "" && !urlPattern.MatchString(server.URL) {
				return &ValidationError{
					Field:      "url",
					Message:    fmt.Sprintf("url '%s' does not match required pattern", server.URL),
					JSONPath:   fmt.Sprintf("%s.url", jsonPath),
					Suggestion: "Use a valid HTTP or HTTPS URL (e.g., 'https://api.example.com/mcp')",
				}
			}
		}
	}

	// Validate gateway configuration patterns
	if stdinCfg.Gateway != nil {
		// Validate port: must be integer 1-65535 or variable expression
		if stdinCfg.Gateway.Port != nil {
			port := *stdinCfg.Gateway.Port
			if port < 1 || port > 65535 {
				return &ValidationError{
					Field:      "port",
					Message:    fmt.Sprintf("port must be between 1 and 65535, got %d", port),
					JSONPath:   "gateway.port",
					Suggestion: "Use a valid port number (e.g., 8080)",
				}
			}
		}

		// Validate domain: must be "localhost", "host.docker.internal", or variable expression
		if stdinCfg.Gateway.Domain != "" {
			domain := stdinCfg.Gateway.Domain
			if domain != "localhost" && domain != "host.docker.internal" && !domainVarPattern.MatchString(domain) {
				return &ValidationError{
					Field:      "domain",
					Message:    fmt.Sprintf("domain '%s' must be 'localhost', 'host.docker.internal', or a variable expression", domain),
					JSONPath:   "gateway.domain",
					Suggestion: "Use 'localhost', 'host.docker.internal', or a variable like '${MCP_GATEWAY_DOMAIN}'",
				}
			}
		}

		// Validate timeouts are positive
		if stdinCfg.Gateway.StartupTimeout != nil && *stdinCfg.Gateway.StartupTimeout < 1 {
			return &ValidationError{
				Field:      "startupTimeout",
				Message:    fmt.Sprintf("startupTimeout must be at least 1, got %d", *stdinCfg.Gateway.StartupTimeout),
				JSONPath:   "gateway.startupTimeout",
				Suggestion: "Use a positive number of seconds (e.g., 30)",
			}
		}

		if stdinCfg.Gateway.ToolTimeout != nil && *stdinCfg.Gateway.ToolTimeout < 1 {
			return &ValidationError{
				Field:      "toolTimeout",
				Message:    fmt.Sprintf("toolTimeout must be at least 1, got %d", *stdinCfg.Gateway.ToolTimeout),
				JSONPath:   "gateway.toolTimeout",
				Suggestion: "Use a positive number of seconds (e.g., 60)",
			}
		}
	}

	return nil
}
