package mcp

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

// TestContainsEqual tests the containsEqual helper function
func TestContainsEqual(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "no equal sign",
			input: "VARIABLE",
			want:  false,
		},
		{
			name:  "contains equal sign",
			input: "VAR=value",
			want:  true,
		},
		{
			name:  "equal sign at start",
			input: "=value",
			want:  true,
		},
		{
			name:  "equal sign at end",
			input: "VAR=",
			want:  true,
		},
		{
			name:  "multiple equal signs",
			input: "VAR=value=another",
			want:  true,
		},
		{
			name:  "single equal sign",
			input: "=",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsEqual(tt.input)
			if got != tt.want {
				t.Errorf("containsEqual(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestExpandDockerEnvArgs tests environment variable expansion for Docker args
func TestExpandDockerEnvArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		envVars map[string]string
		want    []string
	}{
		{
			name:    "empty args",
			args:    []string{},
			envVars: map[string]string{},
			want:    []string{},
		},
		{
			name: "no -e flags",
			args: []string{"run", "--rm", "container"},
			envVars: map[string]string{
				"TEST": "value",
			},
			want: []string{"run", "--rm", "container"},
		},
		{
			name: "single -e with explicit value (no expansion)",
			args: []string{"-e", "VAR=explicit"},
			envVars: map[string]string{
				"VAR": "should_not_use_this",
			},
			want: []string{"-e", "VAR=explicit"},
		},
		{
			name: "single -e with variable reference (expansion)",
			args: []string{"-e", "MY_VAR"},
			envVars: map[string]string{
				"MY_VAR": "expanded_value",
			},
			want: []string{"-e", "MY_VAR=expanded_value"},
		},
		{
			name: "multiple -e flags mixed",
			args: []string{"run", "-e", "VAR1", "-e", "VAR2=explicit", "-e", "VAR3"},
			envVars: map[string]string{
				"VAR1": "value1",
				"VAR2": "should_not_use",
				"VAR3": "value3",
			},
			want: []string{"run", "-e", "VAR1=value1", "-e", "VAR2=explicit", "-e", "VAR3=value3"},
		},
		{
			name: "variable not in environment (no expansion)",
			args: []string{"-e", "MISSING_VAR"},
			envVars: map[string]string{
				"OTHER_VAR": "value",
			},
			want: []string{"-e", "MISSING_VAR"},
		},
		{
			name: "empty variable value",
			args: []string{"-e", "EMPTY_VAR"},
			envVars: map[string]string{
				"EMPTY_VAR": "",
			},
			want: []string{"-e", "EMPTY_VAR="},
		},
		{
			name: "complex Docker command",
			args: []string{
				"run", "--rm",
				"-e", "GITHUB_TOKEN",
				"-e", "CUSTOM_VAR=hardcoded",
				"-i", "container:latest",
			},
			envVars: map[string]string{
				"GITHUB_TOKEN": "ghp_secret123",
				"CUSTOM_VAR":   "should_not_use",
			},
			want: []string{
				"run", "--rm",
				"-e", "GITHUB_TOKEN=ghp_secret123",
				"-e", "CUSTOM_VAR=hardcoded",
				"-i", "container:latest",
			},
		},
		{
			name: "trailing -e without value",
			args: []string{"run", "-e"},
			envVars: map[string]string{
				"VAR": "value",
			},
			want: []string{"run", "-e"},
		},
		{
			name: "variable with special characters in value",
			args: []string{"-e", "SPECIAL"},
			envVars: map[string]string{
				"SPECIAL": "value=with=equals;and;semicolons",
			},
			want: []string{"-e", "SPECIAL=value=with=equals;and;semicolons"},
		},
		{
			name: "variable reference that is empty string",
			args: []string{"-e", ""},
			envVars: map[string]string{
				"": "value",
			},
			want: []string{"-e", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for this test
			for k, v := range tt.envVars {
				oldVal, existed := os.LookupEnv(k)
				os.Setenv(k, v)
				defer func(key, old string, existed bool) {
					if existed {
						os.Setenv(key, old)
					} else {
						os.Unsetenv(key)
					}
				}(k, oldVal, existed)
			}

			got := expandDockerEnvArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expandDockerEnvArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// TestExpandDockerEnvArgs_Concurrent tests thread safety of expandDockerEnvArgs
func TestExpandDockerEnvArgs_Concurrent(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_VAR_1", "value1")
	os.Setenv("TEST_VAR_2", "value2")
	defer os.Unsetenv("TEST_VAR_1")
	defer os.Unsetenv("TEST_VAR_2")

	args1 := []string{"-e", "TEST_VAR_1"}
	args2 := []string{"-e", "TEST_VAR_2"}

	done := make(chan bool, 2)

	// Run expansions concurrently
	go func() {
		for i := 0; i < 100; i++ {
			result := expandDockerEnvArgs(args1)
			expected := []string{"-e", "TEST_VAR_1=value1"}
			if !reflect.DeepEqual(result, expected) {
				t.Errorf("Concurrent call 1: got %v, want %v", result, expected)
			}
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			result := expandDockerEnvArgs(args2)
			expected := []string{"-e", "TEST_VAR_2=value2"}
			if !reflect.DeepEqual(result, expected) {
				t.Errorf("Concurrent call 2: got %v, want %v", result, expected)
			}
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done
}

// TestNewConnection_InvalidCommand tests error handling for invalid commands
func TestNewConnection_InvalidCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        []string
		env         map[string]string
		wantErrMsg  string
	}{
		{
			name:       "command not found",
			command:    "nonexistent_command_xyz123",
			args:       []string{},
			env:        map[string]string{},
			wantErrMsg: "failed to connect",
		},
		{
			name:       "empty command",
			command:    "",
			args:       []string{},
			env:        map[string]string{},
			wantErrMsg: "failed to connect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			conn, err := NewConnection(ctx, tt.command, tt.args, tt.env)
			if err == nil {
				if conn != nil {
					conn.Close()
				}
				t.Fatal("NewConnection() expected error, got nil")
			}

			if conn != nil {
				t.Errorf("NewConnection() returned non-nil connection on error")
			}

			if tt.wantErrMsg != "" && !containsString(err.Error(), tt.wantErrMsg) {
				t.Errorf("NewConnection() error = %v, want error containing %q", err, tt.wantErrMsg)
			}
		})
	}
}

// TestNewConnection_ContextCancellation tests context cancellation handling
func TestNewConnection_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Use a command that would block if not for cancellation
	conn, err := NewConnection(ctx, "sleep", []string{"10"}, nil)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("NewConnection() with cancelled context should return error")
	}

	if conn != nil {
		t.Errorf("NewConnection() returned non-nil connection with cancelled context")
	}
}

// TestConnection_Close tests connection closure
func TestConnection_Close(t *testing.T) {
	tests := []struct {
		name       string
		setupConn  func() *Connection
		wantErr    bool
	}{
		{
			name: "close connection with nil session",
			setupConn: func() *Connection {
				ctx, cancel := context.WithCancel(context.Background())
				return &Connection{
					ctx:     ctx,
					cancel:  cancel,
					session: nil,
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := tt.setupConn()
			err := conn.Close()
			if (err != nil) != tt.wantErr {
				t.Errorf("Close() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCallToolParams_JSON tests JSON marshaling/unmarshaling
func TestCallToolParams_JSON(t *testing.T) {
	tests := []struct {
		name    string
		params  CallToolParams
		wantErr bool
	}{
		{
			name: "valid params with arguments",
			params: CallToolParams{
				Name: "test_tool",
				Arguments: map[string]interface{}{
					"param1": "value1",
					"param2": 42,
					"param3": true,
				},
			},
			wantErr: false,
		},
		{
			name: "params without arguments",
			params: CallToolParams{
				Name:      "simple_tool",
				Arguments: nil,
			},
			wantErr: false,
		},
		{
			name: "params with empty arguments",
			params: CallToolParams{
				Name:      "another_tool",
				Arguments: map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "params with nested arguments",
			params: CallToolParams{
				Name: "complex_tool",
				Arguments: map[string]interface{}{
					"nested": map[string]interface{}{
						"inner": "value",
					},
					"array": []interface{}{1, 2, 3},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Unmarshal back
			var got CallToolParams
			err = json.Unmarshal(data, &got)
			if err != nil {
				t.Errorf("json.Unmarshal() error = %v", err)
				return
			}

			// Compare
			if got.Name != tt.params.Name {
				t.Errorf("Name = %v, want %v", got.Name, tt.params.Name)
			}

			// Deep equal for arguments
			if !reflect.DeepEqual(got.Arguments, tt.params.Arguments) {
				t.Errorf("Arguments = %v, want %v", got.Arguments, tt.params.Arguments)
			}
		})
	}
}

// TestResponse_JSON tests Response JSON marshaling
func TestResponse_JSON(t *testing.T) {
	tests := []struct {
		name     string
		response Response
		wantErr  bool
	}{
		{
			name: "successful response",
			response: Response{
				JSONRPC: "2.0",
				ID:      1,
				Result:  json.RawMessage(`{"key":"value"}`),
			},
			wantErr: false,
		},
		{
			name: "error response",
			response: Response{
				JSONRPC: "2.0",
				ID:      2,
				Error: &ResponseError{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			wantErr: false,
		},
		{
			name: "response with string ID",
			response: Response{
				JSONRPC: "2.0",
				ID:      "request-123",
				Result:  json.RawMessage(`[]`),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Unmarshal and verify structure
			var got Response
			err = json.Unmarshal(data, &got)
			if err != nil {
				t.Errorf("json.Unmarshal() error = %v", err)
				return
			}

			if got.JSONRPC != tt.response.JSONRPC {
				t.Errorf("JSONRPC = %v, want %v", got.JSONRPC, tt.response.JSONRPC)
			}
		})
	}
}

// TestResponseError_JSON tests ResponseError JSON marshaling
func TestResponseError_JSON(t *testing.T) {
	tests := []struct {
		name    string
		err     ResponseError
		wantErr bool
	}{
		{
			name: "error with code and message",
			err: ResponseError{
				Code:    -32700,
				Message: "Parse error",
			},
			wantErr: false,
		},
		{
			name: "error with data",
			err: ResponseError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    json.RawMessage(`{"param":"value"}`),
			},
			wantErr: false,
		},
		{
			name: "error with empty message",
			err: ResponseError{
				Code:    -32603,
				Message: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.err)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			var got ResponseError
			err = json.Unmarshal(data, &got)
			if err != nil {
				t.Errorf("json.Unmarshal() error = %v", err)
				return
			}

			if got.Code != tt.err.Code {
				t.Errorf("Code = %v, want %v", got.Code, tt.err.Code)
			}
			if got.Message != tt.err.Message {
				t.Errorf("Message = %v, want %v", got.Message, tt.err.Message)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
