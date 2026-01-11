package logger_test

import (
	"fmt"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

func ExampleNew() {
	// Create a logger for a specific namespace
	// Note: Set DEBUG environment variable before running your application:
	//   DEBUG=app:* go run main.go
	log := logger.New("app:feature")

	// Check if logger is enabled
	if log.Enabled() {
		fmt.Println("Logger is enabled")
	} else {
		fmt.Println("Logger is disabled (set DEBUG=app:* to enable)")
	}

	// Output: Logger is disabled (set DEBUG=app:* to enable)
}

func ExampleLogger_Printf() {
	// Printf uses standard fmt.Printf formatting
	// Note: Set DEBUG=* before running to see output:
	//   DEBUG=* go run main.go
	log := logger.New("app:feature")

	log.Printf("Processing %d items", 42)
	// When DEBUG=* is set, this writes to stderr:
	// app:feature Processing 42 items +Xms
}

func ExampleLogger_Print() {
	// Print concatenates arguments like fmt.Sprint
	// Note: Set DEBUG=* before running to see output:
	//   DEBUG=* go run main.go
	log := logger.New("app:feature")

	log.Print("Processing", " ", "items")
	// When DEBUG=* is set, this writes to stderr:
	// app:feature Processing items +Xms
}

func ExampleNew_patterns() {
	// Example patterns for DEBUG environment variable
	// Set these before running your application:

	// Enable all loggers
	// DEBUG=*

	// Enable all loggers in workflow namespace
	// DEBUG=workflow:*

	// Enable multiple namespaces
	// DEBUG=workflow:*,cli:*

	// Enable all except specific patterns
	// DEBUG=*,-workflow:test

	// Enable namespace but exclude specific loggers
	// DEBUG=workflow:*,-workflow:cache

	fmt.Println("See comments for DEBUG pattern examples")
	// Output: See comments for DEBUG pattern examples
}
