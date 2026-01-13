package rules

import (
	"fmt"
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

// PortRange validates that a port is in the valid range (1-65535)
// Returns nil if valid, *ValidationError if invalid
func PortRange(port int, jsonPath string) *ValidationError {
	if port < 1 || port > 65535 {
		return &ValidationError{
			Field:      "port",
			Message:    fmt.Sprintf("port must be between 1 and 65535, got %d", port),
			JSONPath:   jsonPath,
			Suggestion: "Use a valid port number (e.g., 8080)",
		}
	}
	return nil
}

// TimeoutPositive validates that a timeout value is at least 1
// Returns nil if valid, *ValidationError if invalid
func TimeoutPositive(timeout int, fieldName, jsonPath string) *ValidationError {
	if timeout < 1 {
		return &ValidationError{
			Field:      fieldName,
			Message:    fmt.Sprintf("%s must be at least 1, got %d", fieldName, timeout),
			JSONPath:   jsonPath,
			Suggestion: "Use a positive number of seconds (e.g., 30)",
		}
	}
	return nil
}

// MountFormat validates a mount specification in the format "source:dest:mode"
// Returns nil if valid, *ValidationError if invalid
func MountFormat(mount, jsonPath string, index int) *ValidationError {
	parts := strings.Split(mount, ":")
	if len(parts) != 3 {
		return &ValidationError{
			Field:      "mounts",
			Message:    fmt.Sprintf("invalid mount format '%s' (expected 'source:dest:mode')", mount),
			JSONPath:   fmt.Sprintf("%s.mounts[%d]", jsonPath, index),
			Suggestion: "Use format 'source:dest:mode' where mode is 'ro' (read-only) or 'rw' (read-write)",
		}
	}

	source, dest, mode := parts[0], parts[1], parts[2]

	// Validate source is not empty
	if source == "" {
		return &ValidationError{
			Field:      "mounts",
			Message:    fmt.Sprintf("mount source cannot be empty in '%s'", mount),
			JSONPath:   fmt.Sprintf("%s.mounts[%d]", jsonPath, index),
			Suggestion: "Provide a valid source path",
		}
	}

	// Validate dest is not empty
	if dest == "" {
		return &ValidationError{
			Field:      "mounts",
			Message:    fmt.Sprintf("mount destination cannot be empty in '%s'", mount),
			JSONPath:   fmt.Sprintf("%s.mounts[%d]", jsonPath, index),
			Suggestion: "Provide a valid destination path",
		}
	}

	// Validate mode
	if mode != "ro" && mode != "rw" {
		return &ValidationError{
			Field:      "mounts",
			Message:    fmt.Sprintf("invalid mount mode '%s' (must be 'ro' or 'rw')", mode),
			JSONPath:   fmt.Sprintf("%s.mounts[%d]", jsonPath, index),
			Suggestion: "Use 'ro' for read-only or 'rw' for read-write",
		}
	}

	return nil
}
