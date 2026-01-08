package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ValidationError represents a configuration validation error with context
type ValidationError struct {
	Field      string
	Message    string
	JSONPath   string
	Suggestion string
}

func (e *ValidationError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Configuration error at %s: %s", e.JSONPath, e.Message))
	if e.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("\nSuggestion: %s", e.Suggestion))
	}
	return sb.String()
}

// Variable expression pattern: ${VARIABLE_NAME}
var varExprPattern = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

// expandVariables expands variable expressions in a string
// Returns the expanded string and error if any variable is undefined
func expandVariables(value, jsonPath string) (string, error) {
	var undefinedVars []string

	result := varExprPattern.ReplaceAllStringFunc(value, func(match string) string {
		// Extract variable name (remove ${ and })
		varName := match[2 : len(match)-1]

		if envValue, exists := os.LookupEnv(varName); exists {
			return envValue
		}

		// Track undefined variable
		undefinedVars = append(undefinedVars, varName)
		return match // Keep original if undefined
	})

	if len(undefinedVars) > 0 {
		return "", &ValidationError{
			Field:      "env variable",
			Message:    fmt.Sprintf("undefined environment variable referenced: %s", undefinedVars[0]),
			JSONPath:   jsonPath,
			Suggestion: fmt.Sprintf("Set the environment variable %s before starting the gateway", undefinedVars[0]),
		}
	}

	return result, nil
}

// expandEnvVariables expands all variable expressions in an env map
func expandEnvVariables(env map[string]string, serverName string) (map[string]string, error) {
	result := make(map[string]string, len(env))

	for key, value := range env {
		jsonPath := fmt.Sprintf("mcpServers.%s.env.%s", serverName, key)

		expanded, err := expandVariables(value, jsonPath)
		if err != nil {
			return nil, err
		}

		result[key] = expanded
	}

	return result, nil
}

// validateStdioServer validates a stdio server configuration
func validateStdioServer(name string, server *StdinServerConfig) error {
	jsonPath := fmt.Sprintf("mcpServers.%s", name)

	// Validate type (empty defaults to stdio)
	if server.Type == "" {
		server.Type = "stdio"
	}

	// Normalize "local" to "stdio"
	if server.Type == "local" {
		server.Type = "stdio"
	}

	// Validate known types
	if server.Type != "stdio" && server.Type != "http" {
		return &ValidationError{
			Field:      "type",
			Message:    fmt.Sprintf("unsupported server type '%s'", server.Type),
			JSONPath:   fmt.Sprintf("%s.type", jsonPath),
			Suggestion: "Use 'stdio' for standard input/output transport or 'http' for HTTP transport",
		}
	}

	// For stdio servers, either command or container is required
	if server.Type == "stdio" || server.Type == "local" {
		if server.Command == "" && server.Container == "" {
			return &ValidationError{
				Field:      "command/container",
				Message:    "either 'command' or 'container' is required for stdio servers",
				JSONPath:   jsonPath,
				Suggestion: "Add a 'command' field (e.g., \"node\") or 'container' field (e.g., \"ghcr.io/owner/image:tag\")",
			}
		}

		// Ensure command and container are mutually exclusive
		if server.Command != "" && server.Container != "" {
			return &ValidationError{
				Field:      "command/container",
				Message:    "'command' and 'container' are mutually exclusive",
				JSONPath:   jsonPath,
				Suggestion: "Remove either 'command' or 'container' field",
			}
		}

		// Validate entrypointArgs only allowed with container
		if len(server.EntrypointArgs) > 0 && server.Container == "" {
			return &ValidationError{
				Field:      "entrypointArgs",
				Message:    "'entrypointArgs' is only valid when 'container' is specified",
				JSONPath:   jsonPath,
				Suggestion: "Remove 'entrypointArgs' or add 'container' field",
			}
		}
	}

	// For HTTP servers, url is required
	if server.Type == "http" {
		if server.URL == "" {
			return &ValidationError{
				Field:      "url",
				Message:    "'url' is required for HTTP servers",
				JSONPath:   jsonPath,
				Suggestion: "Add a 'url' field (e.g., \"https://example.com/mcp\")",
			}
		}
	}

	return nil
}

// validateGatewayConfig validates gateway configuration
func validateGatewayConfig(gateway *StdinGatewayConfig) error {
	if gateway == nil {
		return nil
	}

	// Validate port range
	if gateway.Port != nil {
		if *gateway.Port < 1 || *gateway.Port > 65535 {
			return &ValidationError{
				Field:      "port",
				Message:    fmt.Sprintf("port must be between 1 and 65535, got %d", *gateway.Port),
				JSONPath:   "gateway.port",
				Suggestion: "Use a valid port number (e.g., 8080)",
			}
		}
	}

	// Validate timeout values
	if gateway.StartupTimeout != nil && *gateway.StartupTimeout < 0 {
		return &ValidationError{
			Field:      "startupTimeout",
			Message:    fmt.Sprintf("startupTimeout must be non-negative, got %d", *gateway.StartupTimeout),
			JSONPath:   "gateway.startupTimeout",
			Suggestion: "Use a positive number of seconds (e.g., 30)",
		}
	}

	if gateway.ToolTimeout != nil && *gateway.ToolTimeout < 0 {
		return &ValidationError{
			Field:      "toolTimeout",
			Message:    fmt.Sprintf("toolTimeout must be non-negative, got %d", *gateway.ToolTimeout),
			JSONPath:   "gateway.toolTimeout",
			Suggestion: "Use a positive number of seconds (e.g., 60)",
		}
	}

	return nil
}
