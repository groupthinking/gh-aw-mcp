package logger_test

import (
	"fmt"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

// Example showing basic logger creation and usage
func ExampleNew() {
	// Create a logger for a specific namespace
	// Set DEBUG environment variable before running to enable:
	//   DEBUG=app:* go run main.go
	log := logger.New("app:feature")

	// The logger will be disabled if DEBUG is not set
	_ = log.Enabled()

	// Output:
}

// Example showing DEBUG pattern usage
func ExampleNew_patterns() {
	// Example patterns for DEBUG environment variable
	// Set these before running your application:

	// Enable all loggers:           DEBUG=*
	// Enable specific namespace:    DEBUG=workflow:*
	// Enable multiple namespaces:   DEBUG=workflow:*,cli:*
	// Exclude specific patterns:    DEBUG=*,-workflow:test
	// Complex pattern:              DEBUG=workflow:*,-workflow:cache

	fmt.Println("Set DEBUG environment variable to control logger output")
	// Output: Set DEBUG environment variable to control logger output
}

// Test logger creation and enabling based on DEBUG environment variable
func TestNew_WithDebugEnv(t *testing.T) {
	tests := []struct {
		name      string
		debugEnv  string
		namespace string
		enabled   bool
	}{
		{
			name:      "wildcard enables all loggers",
			debugEnv:  "*",
			namespace: "app:feature",
			enabled:   true,
		},
		{
			name:      "exact match enables logger",
			debugEnv:  "app:feature",
			namespace: "app:feature",
			enabled:   true,
		},
		{
			name:      "namespace wildcard enables matching loggers",
			debugEnv:  "app:*",
			namespace: "app:feature",
			enabled:   true,
		},
		{
			name:      "namespace wildcard does not match different prefix",
			debugEnv:  "app:*",
			namespace: "other:feature",
			enabled:   false,
		},
		{
			name:      "multiple patterns with comma",
			debugEnv:  "app:*,other:*",
			namespace: "app:feature",
			enabled:   true,
		},
		{
			name:      "exclusion pattern disables specific logger",
			debugEnv:  "app:*,-app:skip",
			namespace: "app:skip",
			enabled:   false,
		},
		{
			name:      "exclusion does not affect other loggers",
			debugEnv:  "app:*,-app:skip",
			namespace: "app:feature",
			enabled:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use t.Setenv to set environment variable for this test
			t.Setenv("DEBUG", tt.debugEnv)

			log := logger.New(tt.namespace)
			if log.Enabled() != tt.enabled {
				t.Errorf("New(%q) with DEBUG=%q: enabled = %v, want %v",
					tt.namespace, tt.debugEnv, log.Enabled(), tt.enabled)
			}
		})
	}
}

// Test logger Printf functionality with DEBUG enabled
func TestLogger_Printf_WithDebug(t *testing.T) {
	// Set DEBUG to enable all loggers
	t.Setenv("DEBUG", "*")

	log := logger.New("test:feature")
	if !log.Enabled() {
		t.Error("Logger should be enabled with DEBUG=*")
	}

	// Note: Printf writes to stderr, so we can't easily capture the output
	// in an example test. This test just verifies it doesn't panic.
	log.Printf("Processing %d items", 42)
}

// Test logger Print functionality with DEBUG enabled
func TestLogger_Print_WithDebug(t *testing.T) {
	// Set DEBUG to enable all loggers
	t.Setenv("DEBUG", "*")

	log := logger.New("test:feature")
	if !log.Enabled() {
		t.Error("Logger should be enabled with DEBUG=*")
	}

	// Note: Print writes to stderr, so we can't easily capture the output
	// in an example test. This test just verifies it doesn't panic.
	log.Print("Processing", " ", "items")
}

// Test various DEBUG patterns
func TestDebugPatterns(t *testing.T) {
	tests := []struct {
		name      string
		debugEnv  string
		namespace string
		enabled   bool
	}{
		{
			name:      "empty DEBUG disables all loggers",
			debugEnv:  "",
			namespace: "test:logger",
			enabled:   false,
		},
		{
			name:      "wildcard-all pattern",
			debugEnv:  "*",
			namespace: "any:namespace",
			enabled:   true,
		},
		{
			name:      "suffix wildcard",
			debugEnv:  "*:logger",
			namespace: "test:logger",
			enabled:   true,
		},
		{
			name:      "suffix wildcard no match",
			debugEnv:  "*:logger",
			namespace: "test:other",
			enabled:   false,
		},
		{
			name:      "middle wildcard",
			debugEnv:  "test:*:end",
			namespace: "test:middle:end",
			enabled:   true,
		},
		{
			name:      "exclusion with wildcard",
			debugEnv:  "*,-test:*",
			namespace: "test:logger",
			enabled:   false,
		},
		{
			name:      "exclusion with wildcard allows others",
			debugEnv:  "*,-test:*",
			namespace: "other:logger",
			enabled:   true,
		},
		{
			name:      "spaces in patterns are trimmed",
			debugEnv:  "test:* , other:*",
			namespace: "other:logger",
			enabled:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DEBUG", tt.debugEnv)

			log := logger.New(tt.namespace)
			if log.Enabled() != tt.enabled {
				t.Errorf("New(%q) with DEBUG=%q: enabled = %v, want %v",
					tt.namespace, tt.debugEnv, log.Enabled(), tt.enabled)
			}
		})
	}
}
