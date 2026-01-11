package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

var logValidation = logger.New("config:validation")

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
var varExprPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandVariables expands variable expressions in a string
// Returns the expanded string and error if any variable is undefined
func expandVariables(value, jsonPath string) (string, error) {
	logValidation.Printf("Expanding variables: jsonPath=%s, value=%s", jsonPath, value)
	var undefinedVars []string

	result := varExprPattern.ReplaceAllStringFunc(value, func(match string) string {
		// Extract variable name (remove ${ and })
		varName := match[2 : len(match)-1]

		if envValue, exists := os.LookupEnv(varName); exists {
			logValidation.Printf("Variable expanded: %s=%s", varName, envValue)
			return envValue
		}

		// Track undefined variable
		logValidation.Printf("Undefined variable: %s", varName)
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

// validateMounts validates mount specifications
func validateMounts(mounts []string, jsonPath string) error {
	for i, mount := range mounts {
		parts := strings.Split(mount, ":")
		if len(parts) != 3 {
			return &ValidationError{
				Field:      "mounts",
				Message:    fmt.Sprintf("invalid mount format '%s' (expected 'source:dest:mode')", mount),
				JSONPath:   fmt.Sprintf("%s.mounts[%d]", jsonPath, i),
				Suggestion: "Use format 'source:dest:mode' where mode is 'ro' (read-only) or 'rw' (read-write)",
			}
		}

		source, dest, mode := parts[0], parts[1], parts[2]

		// Validate source is not empty
		if source == "" {
			return &ValidationError{
				Field:      "mounts",
				Message:    fmt.Sprintf("mount source cannot be empty in '%s'", mount),
				JSONPath:   fmt.Sprintf("%s.mounts[%d]", jsonPath, i),
				Suggestion: "Provide a valid source path",
			}
		}

		// Validate dest is not empty
		if dest == "" {
			return &ValidationError{
				Field:      "mounts",
				Message:    fmt.Sprintf("mount destination cannot be empty in '%s'", mount),
				JSONPath:   fmt.Sprintf("%s.mounts[%d]", jsonPath, i),
				Suggestion: "Provide a valid destination path",
			}
		}

		// Validate mode
		if mode != "ro" && mode != "rw" {
			return &ValidationError{
				Field:      "mounts",
				Message:    fmt.Sprintf("invalid mount mode '%s' (must be 'ro' or 'rw')", mode),
				JSONPath:   fmt.Sprintf("%s.mounts[%d]", jsonPath, i),
				Suggestion: "Use 'ro' for read-only or 'rw' for read-write",
			}
		}
	}
	return nil
}

// validateStdioServer validates a stdio server configuration
func validateStdioServer(name string, server *StdinServerConfig) error {
	logValidation.Printf("Validating stdio server: name=%s, type=%s", name, server.Type)
	jsonPath := fmt.Sprintf("mcpServers.%s", name)

	// Validate type (empty defaults to stdio)
	if server.Type == "" {
		server.Type = "stdio"
	}

	// Normalize "local" to "stdio"
	if server.Type == "local" {
		logValidation.Print("Normalizing 'local' type to 'stdio'")
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

	// For stdio servers, container is required
	if server.Type == "stdio" || server.Type == "local" {
		if server.Container == "" {
			return &ValidationError{
				Field:      "container",
				Message:    "'container' is required for stdio servers",
				JSONPath:   jsonPath,
				Suggestion: "Add a 'container' field (e.g., \"ghcr.io/owner/image:tag\")",
			}
		}

		// Reject unsupported 'command' field
		if server.Command != "" {
			return &ValidationError{
				Field:      "command",
				Message:    "'command' field is not supported (stdio servers must use 'container')",
				JSONPath:   jsonPath,
				Suggestion: "Remove 'command' field and use 'container' instead",
			}
		}

		// Validate mounts if provided
		if len(server.Mounts) > 0 {
			if err := validateMounts(server.Mounts, jsonPath); err != nil {
				return err
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

	// Validate timeout values (minimum 1 per schema)
	if gateway.StartupTimeout != nil && *gateway.StartupTimeout < 1 {
		return &ValidationError{
			Field:      "startupTimeout",
			Message:    fmt.Sprintf("startupTimeout must be at least 1, got %d", *gateway.StartupTimeout),
			JSONPath:   "gateway.startupTimeout",
			Suggestion: "Use a positive number of seconds (e.g., 30)",
		}
	}

	if gateway.ToolTimeout != nil && *gateway.ToolTimeout < 1 {
		return &ValidationError{
			Field:      "toolTimeout",
			Message:    fmt.Sprintf("toolTimeout must be at least 1, got %d", *gateway.ToolTimeout),
			JSONPath:   "gateway.toolTimeout",
			Suggestion: "Use a positive number of seconds (e.g., 60)",
		}
	}

	return nil
}
