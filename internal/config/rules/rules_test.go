package rules

import (
	"strings"
	"testing"
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
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
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
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
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
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
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
