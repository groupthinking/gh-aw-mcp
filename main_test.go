package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildVersionString(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		gitCommit      string
		buildDate      string
		expectedParts  []string
		unexpectedPart string
	}{
		{
			name:          "all metadata present",
			version:       "v1.0.0",
			gitCommit:     "abc123",
			buildDate:     "2026-01-21T10:00:00Z",
			expectedParts: []string{"v1.0.0", "commit: abc123", "built: 2026-01-21T10:00:00Z"},
		},
		{
			name:          "only version",
			version:       "v1.0.0",
			gitCommit:     "",
			buildDate:     "",
			expectedParts: []string{"v1.0.0"},
		},
		{
			name:          "version with commit",
			version:       "v1.0.0",
			gitCommit:     "abc123",
			buildDate:     "",
			expectedParts: []string{"v1.0.0", "commit: abc123"},
		},
		{
			name:          "version with build date",
			version:       "v1.0.0",
			gitCommit:     "",
			buildDate:     "2026-01-21T10:00:00Z",
			expectedParts: []string{"v1.0.0", "built: 2026-01-21T10:00:00Z"},
		},
		{
			name:           "no version defaults to dev",
			version:        "",
			gitCommit:      "",
			buildDate:      "",
			expectedParts:  []string{"dev"},
			unexpectedPart: "commit:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set test values
			origVersion := Version
			origGitCommit := GitCommit
			origBuildDate := BuildDate
			t.Cleanup(func() {
				Version = origVersion
				GitCommit = origGitCommit
				BuildDate = origBuildDate
			})

			Version = tt.version
			GitCommit = tt.gitCommit
			BuildDate = tt.buildDate

			result := buildVersionString()

			// Check expected parts
			for _, part := range tt.expectedParts {
				assert.Contains(t, result, part, "Version string should contain: %s", part)
			}

			// Check unexpected parts
			if tt.unexpectedPart != "" {
				assert.NotContains(t, result, tt.unexpectedPart, "Version string should not contain: %s", tt.unexpectedPart)
			}

			// Ensure parts are comma-separated
			if len(tt.expectedParts) > 1 {
				assert.True(t, strings.Contains(result, ", "), "Multi-part version should be comma-separated")
			}
		})
	}
}

func TestBuildVersionString_UsesVCSInfo(t *testing.T) {
	// When ldflags are not set, buildVersionString should fall back to VCS info from runtime/debug
	// This test verifies the code structure, actual VCS extraction depends on build settings

	// Set empty values to trigger VCS fallback
	origVersion := Version
	origGitCommit := GitCommit
	origBuildDate := BuildDate
	t.Cleanup(func() {
		Version = origVersion
		GitCommit = origGitCommit
		BuildDate = origBuildDate
	})

	Version = ""
	GitCommit = ""
	BuildDate = ""

	result := buildVersionString()

	// Should at least have "dev"
	assert.Contains(t, result, "dev", "Should contain 'dev' when Version is empty")

	// May or may not have commit/built info depending on VCS availability
	// Just verify it doesn't panic or return empty string
	assert.NotEmpty(t, result, "Should return non-empty string")
}
