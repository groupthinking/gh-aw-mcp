package mcp

import (
	"os"
	"reflect"
	"testing"
)

func TestExpandDockerEnvArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envSetup map[string]string
		want     []string
	}{
		{
			name: "no -e flags",
			args: []string{"run", "--rm", "image"},
			want: []string{"run", "--rm", "image"},
		},
		{
			name: "single -e with explicit value",
			args: []string{"run", "-e", "VAR=value", "image"},
			want: []string{"run", "-e", "VAR=value", "image"},
		},
		{
			name:     "single -e with passthrough variable (exists)",
			args:     []string{"run", "-e", "TEST_VAR", "image"},
			envSetup: map[string]string{"TEST_VAR": "test_value"},
			want:     []string{"run", "-e", "TEST_VAR=test_value", "image"},
		},
		{
			name:     "single -e with passthrough variable (not exists)",
			args:     []string{"run", "-e", "MISSING_VAR", "image"},
			envSetup: map[string]string{},
			want:     []string{"run", "-e", "MISSING_VAR", "image"},
		},
		{
			name: "multiple -e flags mixed",
			args: []string{"run", "-e", "VAR1=explicit", "-e", "VAR2", "-e", "VAR3=another", "image"},
			envSetup: map[string]string{
				"VAR2": "passthrough_value",
			},
			want: []string{"run", "-e", "VAR1=explicit", "-e", "VAR2=passthrough_value", "-e", "VAR3=another", "image"},
		},
		{
			name:     "empty passthrough variable",
			args:     []string{"run", "-e", "EMPTY_VAR", "image"},
			envSetup: map[string]string{"EMPTY_VAR": ""},
			want:     []string{"run", "-e", "EMPTY_VAR=", "image"},
		},
		{
			name: "-e at end of args (incomplete)",
			args: []string{"run", "image", "-e"},
			want: []string{"run", "image", "-e"},
		},
		{
			name:     "passthrough with special characters",
			args:     []string{"run", "-e", "SPECIAL_VAR", "image"},
			envSetup: map[string]string{"SPECIAL_VAR": "value with spaces!@#$%"},
			want:     []string{"run", "-e", "SPECIAL_VAR=value with spaces!@#$%", "image"},
		},
		{
			name: "multiple -e flags all explicit",
			args: []string{"run", "-e", "A=1", "-e", "B=2", "-e", "C=3", "image"},
			want: []string{"run", "-e", "A=1", "-e", "B=2", "-e", "C=3", "image"},
		},
		{
			name:     "multiple -e flags all passthrough",
			args:     []string{"run", "-e", "X", "-e", "Y", "-e", "Z", "image"},
			envSetup: map[string]string{"X": "x_val", "Y": "y_val", "Z": "z_val"},
			want:     []string{"run", "-e", "X=x_val", "-e", "Y=y_val", "-e", "Z=z_val", "image"},
		},
		{
			name:     "passthrough with equals sign in value",
			args:     []string{"run", "-e", "URL_VAR", "image"},
			envSetup: map[string]string{"URL_VAR": "https://example.com?param=value"},
			want:     []string{"run", "-e", "URL_VAR=https://example.com?param=value", "image"},
		},
		{
			name: "args without any docker flags",
			args: []string{"go", "test", "-v", "./..."},
			want: []string{"go", "test", "-v", "./..."},
		},
		{
			name: "empty args",
			args: []string{},
			want: []string{},
		},
		{
			name: "-e flag with empty string value",
			args: []string{"run", "-e", "", "image"},
			want: []string{"run", "-e", "", "image"},
		},
		{
			name:     "complex docker command",
			args:     []string{"run", "--rm", "-i", "-e", "TOKEN", "-v", "/path:/mount", "--name", "container", "image:tag"},
			envSetup: map[string]string{"TOKEN": "secret123"},
			want:     []string{"run", "--rm", "-i", "-e", "TOKEN=secret123", "-v", "/path:/mount", "--name", "container", "image:tag"},
		},
		{
			name:     "passthrough with newlines and tabs",
			args:     []string{"run", "-e", "MULTILINE_VAR", "image"},
			envSetup: map[string]string{"MULTILINE_VAR": "line1\nline2\ttab"},
			want:     []string{"run", "-e", "MULTILINE_VAR=line1\nline2\ttab", "image"},
		},
		{
			name: "non -e flag followed by value",
			args: []string{"run", "-v", "volume", "-e", "VAR=val", "image"},
			want: []string{"run", "-v", "volume", "-e", "VAR=val", "image"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment
			originalEnv := make(map[string]string)
			for key := range tt.envSetup {
				if val, exists := os.LookupEnv(key); exists {
					originalEnv[key] = val
				}
			}

			// Set up test environment
			for key, value := range tt.envSetup {
				os.Setenv(key, value)
			}

			// Run the function
			got := expandDockerEnvArgs(tt.args)

			// Restore original environment
			for key := range tt.envSetup {
				if originalVal, existed := originalEnv[key]; existed {
					os.Setenv(key, originalVal)
				} else {
					os.Unsetenv(key)
				}
			}

			// Compare results
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expandDockerEnvArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsEqual(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{
			name: "empty string",
			s:    "",
			want: false,
		},
		{
			name: "no equals",
			s:    "VARIABLE_NAME",
			want: false,
		},
		{
			name: "has equals at start",
			s:    "=value",
			want: true,
		},
		{
			name: "has equals in middle",
			s:    "VAR=value",
			want: true,
		},
		{
			name: "has equals at end",
			s:    "VAR=",
			want: true,
		},
		{
			name: "multiple equals",
			s:    "VAR=value=extra",
			want: true,
		},
		{
			name: "special characters no equals",
			s:    "VAR_NAME-123",
			want: false,
		},
		{
			name: "single equals character",
			s:    "=",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsEqual(tt.s)
			if got != tt.want {
				t.Errorf("containsEqual(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestExpandDockerEnvArgs_EdgeCases(t *testing.T) {
	t.Run("preserve order of non-e flags", func(t *testing.T) {
		args := []string{"run", "--rm", "-i", "-e", "VAR=val", "--name", "test", "image"}
		want := []string{"run", "--rm", "-i", "-e", "VAR=val", "--name", "test", "image"}
		got := expandDockerEnvArgs(args)

		if !reflect.DeepEqual(got, want) {
			t.Errorf("expandDockerEnvArgs() = %v, want %v", got, want)
		}
	})

	t.Run("multiple -e flags in sequence", func(t *testing.T) {
		os.Setenv("TEST_VAR1", "value1")
		os.Setenv("TEST_VAR2", "value2")
		defer os.Unsetenv("TEST_VAR1")
		defer os.Unsetenv("TEST_VAR2")

		args := []string{"-e", "TEST_VAR1", "-e", "TEST_VAR2"}
		want := []string{"-e", "TEST_VAR1=value1", "-e", "TEST_VAR2=value2"}
		got := expandDockerEnvArgs(args)

		if !reflect.DeepEqual(got, want) {
			t.Errorf("expandDockerEnvArgs() = %v, want %v", got, want)
		}
	})

	t.Run("nil args slice", func(t *testing.T) {
		// Should not panic
		got := expandDockerEnvArgs(nil)
		if got == nil {
			t.Error("expandDockerEnvArgs(nil) returned nil, expected empty slice")
		}
		if len(got) != 0 {
			t.Errorf("expandDockerEnvArgs(nil) = %v, want []", got)
		}
	})

	t.Run("large number of arguments", func(t *testing.T) {
		args := make([]string, 100)
		for i := 0; i < 100; i++ {
			if i%2 == 0 {
				args[i] = "-e"
			} else {
				args[i] = "VAR=value"
			}
		}

		got := expandDockerEnvArgs(args)
		if len(got) != 100 {
			t.Errorf("expandDockerEnvArgs() returned %d args, want 100", len(got))
		}
	})
}

func TestExpandDockerEnvArgs_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envSetup map[string]string
		want     []string
	}{
		{
			name: "GitHub MCP server scenario",
			args: []string{
				"run",
				"--rm",
				"-e",
				"GITHUB_PERSONAL_ACCESS_TOKEN",
				"-i",
				"ghcr.io/github/github-mcp-server:latest",
			},
			envSetup: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_1234567890abcdef",
			},
			want: []string{
				"run",
				"--rm",
				"-e",
				"GITHUB_PERSONAL_ACCESS_TOKEN=ghp_1234567890abcdef",
				"-i",
				"ghcr.io/github/github-mcp-server:latest",
			},
		},
		{
			name: "multiple env vars for API service",
			args: []string{
				"run",
				"--rm",
				"-e",
				"API_KEY",
				"-e",
				"API_SECRET",
				"-e",
				"DEBUG=true",
				"-p",
				"8080:8080",
				"api-server:latest",
			},
			envSetup: map[string]string{
				"API_KEY":    "key123",
				"API_SECRET": "secret456",
			},
			want: []string{
				"run",
				"--rm",
				"-e",
				"API_KEY=key123",
				"-e",
				"API_SECRET=secret456",
				"-e",
				"DEBUG=true",
				"-p",
				"8080:8080",
				"api-server:latest",
			},
		},
		{
			name: "docker compose style with network",
			args: []string{
				"run",
				"--network",
				"host",
				"-e",
				"DATABASE_URL",
				"--name",
				"app",
				"myapp:v1",
			},
			envSetup: map[string]string{
				"DATABASE_URL": "postgresql://user:pass@localhost/db",
			},
			want: []string{
				"run",
				"--network",
				"host",
				"-e",
				"DATABASE_URL=postgresql://user:pass@localhost/db",
				"--name",
				"app",
				"myapp:v1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and set environment
			originalEnv := make(map[string]string)
			for key := range tt.envSetup {
				if val, exists := os.LookupEnv(key); exists {
					originalEnv[key] = val
				}
			}

			for key, value := range tt.envSetup {
				os.Setenv(key, value)
			}

			// Test
			got := expandDockerEnvArgs(tt.args)

			// Restore environment
			for key := range tt.envSetup {
				if originalVal, existed := originalEnv[key]; existed {
					os.Setenv(key, originalVal)
				} else {
					os.Unsetenv(key)
				}
			}

			// Verify
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expandDockerEnvArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkExpandDockerEnvArgs(b *testing.B) {
	args := []string{"run", "-e", "VAR1", "-e", "VAR2=explicit", "-e", "VAR3", "image"}
	os.Setenv("VAR1", "value1")
	os.Setenv("VAR3", "value3")
	defer os.Unsetenv("VAR1")
	defer os.Unsetenv("VAR3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expandDockerEnvArgs(args)
	}
}

func BenchmarkContainsEqual(b *testing.B) {
	testStrings := []string{
		"VAR_NAME",
		"VAR=value",
		"",
		"LONG_VARIABLE_NAME_WITHOUT_EQUALS",
		"SHORT=V",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range testStrings {
			containsEqual(s)
		}
	}
}
