//go:build tools
// +build tools

// Package tools manages tool dependencies for the project.
// This file ensures that tool dependencies are tracked in go.mod
// even though they are not directly imported by application code.
package tools

import (
	_ "github.com/stretchr/testify"
)
