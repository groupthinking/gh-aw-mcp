package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/githubnext/gh-aw-mcpg/internal/config/rules"
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

// ValidationError is an alias for rules.ValidationError for backward compatibility
type ValidationError = rules.ValidationError

// Variable expression pattern: ${VARIABLE_NAME}
var varExprPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

var log = logger.New("config:validation")

// expandVariables expands variable expressions in a string
// Returns the expanded string and error if any variable is undefined
func expandVariables(value, jsonPath string) (string, error) {
	log.Printf("Expanding variables: jsonPath=%s, value=%s", jsonPath, value)
	var undefinedVars []string

	result := varExprPattern.ReplaceAllStringFunc(value, func(match string) string {
		// Extract variable name (remove ${ and })
		varName := match[2 : len(match)-1]

		if envValue, exists := os.LookupEnv(varName); exists {
			log.Printf("Expanded variable: %s (found in environment)", varName)
			return envValue
		}

		// Track undefined variable
		undefinedVars = append(undefinedVars, varName)
		log.Printf("Undefined variable: %s", varName)
		return match // Keep original if undefined
	})

	if len(undefinedVars) > 0 {
		log.Printf("Variable expansion failed: undefined variables=%v", undefinedVars)
		return "", &ValidationError{
			Field:      "env variable",
			Message:    fmt.Sprintf("undefined environment variable referenced: %s", undefinedVars[0]),
			JSONPath:   jsonPath,
			Suggestion: fmt.Sprintf("Set the environment variable %s before starting the gateway", undefinedVars[0]),
		}
	}

	log.Printf("Variable expansion completed: result=%s", result)
	return result, nil
}

// expandEnvVariables expands all variable expressions in an env map
func expandEnvVariables(env map[string]string, serverName string) (map[string]string, error) {
	log.Printf("Expanding env variables for server: %s, count=%d", serverName, len(env))
	result := make(map[string]string, len(env))

	for key, value := range env {
		jsonPath := fmt.Sprintf("mcpServers.%s.env.%s", serverName, key)

		expanded, err := expandVariables(value, jsonPath)
		if err != nil {
			return nil, err
		}

		result[key] = expanded
	}

	log.Printf("Env variable expansion completed for server: %s", serverName)
	return result, nil
}

// validateMounts validates mount specifications using centralized rules
func validateMounts(mounts []string, jsonPath string) error {
	for i, mount := range mounts {
		if err := rules.MountFormat(mount, jsonPath, i); err != nil {
			return err
		}
	}
	return nil
}

// validateServerConfig validates a server configuration (stdio or HTTP)
func validateServerConfig(name string, server *StdinServerConfig) error {
	log.Printf("Validating server config: name=%s, type=%s", name, server.Type)
	jsonPath := fmt.Sprintf("mcpServers.%s", name)

	// Validate type (empty defaults to stdio)
	if server.Type == "" {
		server.Type = "stdio"
		log.Printf("Server type empty, defaulting to stdio: name=%s", name)
	}

	// Normalize "local" to "stdio"
	if server.Type == "local" {
		server.Type = "stdio"
		log.Printf("Server type normalized from 'local' to 'stdio': name=%s", name)
	}

	// Validate known types
	if server.Type != "stdio" && server.Type != "http" {
		log.Printf("Invalid server type: name=%s, type=%s", name, server.Type)
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
			log.Printf("Validation failed: stdio server missing container field, name=%s", name)
			return &ValidationError{
				Field:      "container",
				Message:    "'container' is required for stdio servers",
				JSONPath:   jsonPath,
				Suggestion: "Add a 'container' field (e.g., \"ghcr.io/owner/image:tag\")",
			}
		}

		// Reject unsupported 'command' field
		if server.Command != "" {
			log.Printf("Validation failed: stdio server has unsupported command field, name=%s", name)
			return &ValidationError{
				Field:      "command",
				Message:    "'command' field is not supported (stdio servers must use 'container')",
				JSONPath:   jsonPath,
				Suggestion: "Remove 'command' field and use 'container' instead",
			}
		}

		// Validate mounts if provided
		if len(server.Mounts) > 0 {
			log.Printf("Validating mounts for server: name=%s, mount_count=%d", name, len(server.Mounts))
			if err := validateMounts(server.Mounts, jsonPath); err != nil {
				return err
			}
		}
	}

	// For HTTP servers, url is required
	if server.Type == "http" {
		if server.URL == "" {
			log.Printf("Validation failed: HTTP server missing url field, name=%s", name)
			return &ValidationError{
				Field:      "url",
				Message:    "'url' is required for HTTP servers",
				JSONPath:   jsonPath,
				Suggestion: "Add a 'url' field (e.g., \"https://example.com/mcp\")",
			}
		}
	}

	log.Printf("Server config validation passed: name=%s", name)
	return nil
}

// validateGatewayConfig validates gateway configuration
func validateGatewayConfig(gateway *StdinGatewayConfig) error {
	if gateway == nil {
		log.Print("No gateway config to validate")
		return nil
	}

	log.Print("Validating gateway configuration")

	// Validate port range using centralized rules
	if gateway.Port != nil {
		log.Printf("Validating gateway port: %d", *gateway.Port)
		if err := rules.PortRange(*gateway.Port, "gateway.port"); err != nil {
			return err
		}
	}

	// Validate timeout values using centralized rules
	if gateway.StartupTimeout != nil {
		log.Printf("Validating startup timeout: %d", *gateway.StartupTimeout)
		if err := rules.TimeoutPositive(*gateway.StartupTimeout, "startupTimeout", "gateway.startupTimeout"); err != nil {
			return err
		}
	}

	if gateway.ToolTimeout != nil {
		log.Printf("Validating tool timeout: %d", *gateway.ToolTimeout)
		if err := rules.TimeoutPositive(*gateway.ToolTimeout, "toolTimeout", "gateway.toolTimeout"); err != nil {
			return err
		}
	}

	log.Print("Gateway config validation passed")
	return nil
}
