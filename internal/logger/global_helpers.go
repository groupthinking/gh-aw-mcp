// Package logger provides structured logging for the MCP Gateway.
//
// This file contains helper functions for managing global logger state with proper
// mutex handling. These helpers encapsulate common patterns for initializing and
// closing global loggers (FileLogger, JSONLLogger, MarkdownLogger) to reduce code
// duplication while maintaining thread safety.
//
// Functions in this file follow a consistent pattern:
//
// - init*: Initialize a global logger with proper locking and cleanup of any existing logger
// - close*: Close and clear a global logger with proper locking
//
// These helpers are used internally by the logger package and should not be called
// directly by external code. Use the public Init* and Close* functions instead.
package logger

// This file contains helper functions that encapsulate the common patterns
// for global logger initialization and cleanup. These helpers reduce code
// duplication while maintaining type safety.

// initGlobalFileLogger is a helper that encapsulates the common pattern for
// initializing a global FileLogger with proper mutex handling.
func initGlobalFileLogger(logger *FileLogger) {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()

	if globalFileLogger != nil {
		globalFileLogger.Close()
	}
	globalFileLogger = logger
}

// closeGlobalFileLogger is a helper that encapsulates the common pattern for
// closing and clearing a global FileLogger with proper mutex handling.
func closeGlobalFileLogger() error {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()

	if globalFileLogger != nil {
		err := globalFileLogger.Close()
		globalFileLogger = nil
		return err
	}
	return nil
}

// initGlobalJSONLLogger is a helper that encapsulates the common pattern for
// initializing a global JSONLLogger with proper mutex handling.
func initGlobalJSONLLogger(logger *JSONLLogger) {
	globalJSONLMu.Lock()
	defer globalJSONLMu.Unlock()

	if globalJSONLLogger != nil {
		globalJSONLLogger.Close()
	}
	globalJSONLLogger = logger
}

// closeGlobalJSONLLogger is a helper that encapsulates the common pattern for
// closing and clearing a global JSONLLogger with proper mutex handling.
func closeGlobalJSONLLogger() error {
	globalJSONLMu.Lock()
	defer globalJSONLMu.Unlock()

	if globalJSONLLogger != nil {
		err := globalJSONLLogger.Close()
		globalJSONLLogger = nil
		return err
	}
	return nil
}

// initGlobalMarkdownLogger is a helper that encapsulates the common pattern for
// initializing a global MarkdownLogger with proper mutex handling.
func initGlobalMarkdownLogger(logger *MarkdownLogger) {
	globalMarkdownMu.Lock()
	defer globalMarkdownMu.Unlock()

	if globalMarkdownLogger != nil {
		globalMarkdownLogger.Close()
	}
	globalMarkdownLogger = logger
}

// closeGlobalMarkdownLogger is a helper that encapsulates the common pattern for
// closing and clearing a global MarkdownLogger with proper mutex handling.
func closeGlobalMarkdownLogger() error {
	globalMarkdownMu.Lock()
	defer globalMarkdownMu.Unlock()

	if globalMarkdownLogger != nil {
		err := globalMarkdownLogger.Close()
		globalMarkdownLogger = nil
		return err
	}
	return nil
}
