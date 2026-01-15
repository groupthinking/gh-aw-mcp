package rules

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPortRange(t *testing.T) {
	tests := []struct {
		name      string
		port      int
		jsonPath  string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid port 8080",
			port:      8080,
			jsonPath:  "gateway.port",
			shouldErr: false,
		},
		{
			name:      "valid port 1",
			port:      1,
			jsonPath:  "gateway.port",
			shouldErr: false,
		},
		{
			name:      "valid port 65535",
			port:      65535,
			jsonPath:  "gateway.port",
			shouldErr: false,
		},
		{
			name:      "invalid port 0",
			port:      0,
			jsonPath:  "gateway.port",
			shouldErr: true,
			errMsg:    "port must be between 1 and 65535",
		},
		{
			name:      "invalid port 65536",
			port:      65536,
			jsonPath:  "gateway.port",
			shouldErr: true,
			errMsg:    "port must be between 1 and 65535",
		},
		{
			name:      "invalid negative port",
			port:      -1,
			jsonPath:  "gateway.port",
			shouldErr: true,
			errMsg:    "port must be between 1 and 65535",
		},
		{
			name:      "invalid port 100000",
			port:      100000,
			jsonPath:  "gateway.port",
			shouldErr: true,
			errMsg:    "port must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PortRange(tt.port, tt.jsonPath)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if !strings.Contains(err.Message, tt.errMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errMsg, err.Message)
				}
				if err.JSONPath != tt.jsonPath {
					t.Errorf("Expected JSONPath %q, got %q", tt.jsonPath, err.JSONPath)
				}
			} else {
				assert.Nil(t, err, "Unexpected error")
			}
		})
	}
}

func TestTimeoutPositive(t *testing.T) {
	tests := []struct {
		name      string
		timeout   int
		fieldName string
		jsonPath  string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid timeout 30",
			timeout:   30,
			fieldName: "startupTimeout",
			jsonPath:  "gateway.startupTimeout",
			shouldErr: false,
		},
		{
			name:      "valid timeout 1",
			timeout:   1,
			fieldName: "toolTimeout",
			jsonPath:  "gateway.toolTimeout",
			shouldErr: false,
		},
		{
			name:      "valid large timeout",
			timeout:   3600,
			fieldName: "startupTimeout",
			jsonPath:  "gateway.startupTimeout",
			shouldErr: false,
		},
		{
			name:      "invalid timeout 0",
			timeout:   0,
			fieldName: "startupTimeout",
			jsonPath:  "gateway.startupTimeout",
			shouldErr: true,
			errMsg:    "startupTimeout must be at least 1",
		},
		{
			name:      "invalid negative timeout",
			timeout:   -10,
			fieldName: "toolTimeout",
			jsonPath:  "gateway.toolTimeout",
			shouldErr: true,
			errMsg:    "toolTimeout must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TimeoutPositive(tt.timeout, tt.fieldName, tt.jsonPath)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if !strings.Contains(err.Message, tt.errMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errMsg, err.Message)
				}
				if err.JSONPath != tt.jsonPath {
					t.Errorf("Expected JSONPath %q, got %q", tt.jsonPath, err.JSONPath)
				}
				if err.Field != tt.fieldName {
					t.Errorf("Expected Field %q, got %q", tt.fieldName, err.Field)
				}
			} else {
				assert.Nil(t, err, "Unexpected error")
			}
		})
	}
}

func TestMountFormat(t *testing.T) {
	tests := []struct {
		name      string
		mount     string
		jsonPath  string
		index     int
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid ro mount",
			mount:     "/host/path:/container/path:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: false,
		},
		{
			name:      "valid rw mount",
			mount:     "/var/data:/app/data:rw",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: false,
		},
		{
			name:      "valid mount with special chars",
			mount:     "/home/user/my-app:/app/data:ro",
			jsonPath:  "mcpServers.github",
			index:     1,
			shouldErr: false,
		},
		{
			name:      "invalid format - missing mode",
			mount:     "/host/path:/container/path",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "invalid mount format",
		},
		{
			name:      "invalid format - too many colons",
			mount:     "/host/path:/container/path:ro:extra",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "invalid mount format",
		},
		{
			name:      "invalid format - empty source",
			mount:     ":/container/path:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount source cannot be empty",
		},
		{
			name:      "invalid format - empty dest",
			mount:     "/host/path::ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount destination cannot be empty",
		},
		{
			name:      "invalid mode",
			mount:     "/host/path:/container/path:invalid",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "invalid mount mode",
		},
		{
			name:      "invalid mode - uppercase",
			mount:     "/host/path:/container/path:RO",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "invalid mount mode",
		},
		{
			name:      "invalid source - relative path",
			mount:     "relative/path:/container/path:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount source must be an absolute path",
		},
		{
			name:      "invalid dest - relative path",
			mount:     "/host/path:relative/path:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount destination must be an absolute path",
		},
		{
			name:      "invalid source - dot relative",
			mount:     "./config:/app/config:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount source must be an absolute path",
		},
		{
			name:      "invalid dest - dot relative",
			mount:     "/host/config:./config:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount destination must be an absolute path",
		},
		{
			name:      "invalid source - parent relative",
			mount:     "../config:/app/config:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount source must be an absolute path",
		},
		{
			name:      "invalid dest - parent relative",
			mount:     "/host/config:../config:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: true,
			errMsg:    "mount destination must be an absolute path",
		},
		{
			name:      "valid mount - root paths",
			mount:     "/:/root:ro",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: false,
		},
		{
			name:      "valid mount - deep nested paths",
			mount:     "/var/lib/docker/volumes/data:/app/data/volumes:rw",
			jsonPath:  "mcpServers.github",
			index:     0,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MountFormat(tt.mount, tt.jsonPath, tt.index)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if !strings.Contains(err.Message, tt.errMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errMsg, err.Message)
				}
				expectedPath := "mcpServers.github.mounts[0]"
				if tt.index != 0 {
					expectedPath = "mcpServers.github.mounts[1]"
				}
				if err.JSONPath != expectedPath {
					t.Errorf("Expected JSONPath %q, got %q", expectedPath, err.JSONPath)
				}
			} else {
				assert.Nil(t, err, "Unexpected error")
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name       string
		valErr     *ValidationError
		wantSubstr []string
	}{
		{
			name: "error with suggestion",
			valErr: &ValidationError{
				Field:      "port",
				Message:    "port must be between 1 and 65535",
				JSONPath:   "gateway.port",
				Suggestion: "Use a valid port number",
			},
			wantSubstr: []string{
				"Configuration error at gateway.port",
				"port must be between 1 and 65535",
				"Suggestion: Use a valid port number",
			},
		},
		{
			name: "error without suggestion",
			valErr: &ValidationError{
				Field:    "timeout",
				Message:  "timeout must be positive",
				JSONPath: "gateway.startupTimeout",
			},
			wantSubstr: []string{
				"Configuration error at gateway.startupTimeout",
				"timeout must be positive",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.valErr.Error()

			for _, substr := range tt.wantSubstr {
				if !strings.Contains(errStr, substr) {
					t.Errorf("Error string should contain %q, got: %s", substr, errStr)
				}
			}
		})
	}
}

func TestUnsupportedType(t *testing.T) {
	tests := []struct {
		name       string
		fieldName  string
		actualType string
		jsonPath   string
		suggestion string
		wantSubstr []string
	}{
		{
			name:       "unsupported server type",
			fieldName:  "type",
			actualType: "grpc",
			jsonPath:   "mcpServers.github",
			suggestion: "Use 'stdio' for standard input/output transport or 'http' for HTTP transport",
			wantSubstr: []string{
				"type",
				"unsupported server type 'grpc'",
				"mcpServers.github.type",
				"Use 'stdio'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UnsupportedType(tt.fieldName, tt.actualType, tt.jsonPath, tt.suggestion)

			if err.Field != tt.fieldName {
				t.Errorf("Expected Field %q, got %q", tt.fieldName, err.Field)
			}
			if !strings.Contains(err.Message, tt.actualType) {
				t.Errorf("Expected Message to contain %q, got %q", tt.actualType, err.Message)
			}
			if !strings.Contains(err.JSONPath, tt.jsonPath) {
				t.Errorf("Expected JSONPath to contain %q, got %q", tt.jsonPath, err.JSONPath)
			}
			if err.Suggestion != tt.suggestion {
				t.Errorf("Expected Suggestion %q, got %q", tt.suggestion, err.Suggestion)
			}

			errStr := err.Error()
			for _, substr := range tt.wantSubstr {
				if !strings.Contains(errStr, substr) {
					t.Errorf("Error string should contain %q, got: %s", substr, errStr)
				}
			}
		})
	}
}

func TestUndefinedVariable(t *testing.T) {
	tests := []struct {
		name       string
		varName    string
		jsonPath   string
		wantSubstr []string
	}{
		{
			name:     "undefined env variable",
			varName:  "MY_VAR",
			jsonPath: "mcpServers.github.env.TOKEN",
			wantSubstr: []string{
				"undefined environment variable referenced: MY_VAR",
				"mcpServers.github.env.TOKEN",
				"Set the environment variable MY_VAR",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UndefinedVariable(tt.varName, tt.jsonPath)

			if err.Field != "env variable" {
				t.Errorf("Expected Field 'env variable', got %q", err.Field)
			}
			if !strings.Contains(err.Message, tt.varName) {
				t.Errorf("Expected Message to contain %q, got %q", tt.varName, err.Message)
			}
			if err.JSONPath != tt.jsonPath {
				t.Errorf("Expected JSONPath %q, got %q", tt.jsonPath, err.JSONPath)
			}
			if !strings.Contains(err.Suggestion, tt.varName) {
				t.Errorf("Expected Suggestion to contain %q, got %q", tt.varName, err.Suggestion)
			}

			errStr := err.Error()
			for _, substr := range tt.wantSubstr {
				if !strings.Contains(errStr, substr) {
					t.Errorf("Error string should contain %q, got: %s", substr, errStr)
				}
			}
		})
	}
}

func TestMissingRequired(t *testing.T) {
	tests := []struct {
		name       string
		fieldName  string
		serverType string
		jsonPath   string
		suggestion string
		wantSubstr []string
	}{
		{
			name:       "missing container field",
			fieldName:  "container",
			serverType: "stdio",
			jsonPath:   "mcpServers.github",
			suggestion: "Add a 'container' field (e.g., \"ghcr.io/owner/image:tag\")",
			wantSubstr: []string{
				"container",
				"'container' is required",
				"stdio servers",
				"mcpServers.github",
			},
		},
		{
			name:       "missing url field",
			fieldName:  "url",
			serverType: "HTTP",
			jsonPath:   "mcpServers.httpServer",
			suggestion: "Add a 'url' field (e.g., \"https://example.com/mcp\")",
			wantSubstr: []string{
				"url",
				"'url' is required",
				"HTTP servers",
				"mcpServers.httpServer",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MissingRequired(tt.fieldName, tt.serverType, tt.jsonPath, tt.suggestion)

			if err.Field != tt.fieldName {
				t.Errorf("Expected Field %q, got %q", tt.fieldName, err.Field)
			}
			if !strings.Contains(err.Message, tt.fieldName) {
				t.Errorf("Expected Message to contain %q, got %q", tt.fieldName, err.Message)
			}
			if !strings.Contains(err.Message, tt.serverType) {
				t.Errorf("Expected Message to contain %q, got %q", tt.serverType, err.Message)
			}
			if err.JSONPath != tt.jsonPath {
				t.Errorf("Expected JSONPath %q, got %q", tt.jsonPath, err.JSONPath)
			}
			if err.Suggestion != tt.suggestion {
				t.Errorf("Expected Suggestion %q, got %q", tt.suggestion, err.Suggestion)
			}

			errStr := err.Error()
			for _, substr := range tt.wantSubstr {
				if !strings.Contains(errStr, substr) {
					t.Errorf("Error string should contain %q, got: %s", substr, errStr)
				}
			}
		})
	}
}

func TestUnsupportedField(t *testing.T) {
	tests := []struct {
		name       string
		fieldName  string
		message    string
		jsonPath   string
		suggestion string
		wantSubstr []string
	}{
		{
			name:       "unsupported command field",
			fieldName:  "command",
			message:    "'command' field is not supported (stdio servers must use 'container')",
			jsonPath:   "mcpServers.github",
			suggestion: "Remove 'command' field and use 'container' instead",
			wantSubstr: []string{
				"command",
				"not supported",
				"mcpServers.github",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UnsupportedField(tt.fieldName, tt.message, tt.jsonPath, tt.suggestion)

			if err.Field != tt.fieldName {
				t.Errorf("Expected Field %q, got %q", tt.fieldName, err.Field)
			}
			if err.Message != tt.message {
				t.Errorf("Expected Message %q, got %q", tt.message, err.Message)
			}
			if err.JSONPath != tt.jsonPath {
				t.Errorf("Expected JSONPath %q, got %q", tt.jsonPath, err.JSONPath)
			}
			if err.Suggestion != tt.suggestion {
				t.Errorf("Expected Suggestion %q, got %q", tt.suggestion, err.Suggestion)
			}

			errStr := err.Error()
			for _, substr := range tt.wantSubstr {
				if !strings.Contains(errStr, substr) {
					t.Errorf("Error string should contain %q, got: %s", substr, errStr)
				}
			}
		})
	}
}

func TestAppendConfigDocsFooter(t *testing.T) {
	var sb strings.Builder
	AppendConfigDocsFooter(&sb)

	result := sb.String()

	wantSubstr := []string{
		"Please check your configuration",
		ConfigSpecURL,
		"JSON Schema reference",
		SchemaURL,
	}

	for _, substr := range wantSubstr {
		if !strings.Contains(result, substr) {
			t.Errorf("Footer should contain %q, got: %s", substr, result)
		}
	}
}

func TestDocumentationURLConstants(t *testing.T) {
	if ConfigSpecURL == "" {
		t.Error("ConfigSpecURL should not be empty")
	}
	if SchemaURL == "" {
		t.Error("SchemaURL should not be empty")
	}
	if !strings.HasPrefix(ConfigSpecURL, "https://") {
		t.Errorf("ConfigSpecURL should start with https://, got: %s", ConfigSpecURL)
	}
	if !strings.HasPrefix(SchemaURL, "https://") {
		t.Errorf("SchemaURL should start with https://, got: %s", SchemaURL)
	}
}
