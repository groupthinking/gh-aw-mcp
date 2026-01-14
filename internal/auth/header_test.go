package auth

import (
	"testing"
)

func TestParseAuthHeader(t *testing.T) {
	tests := []struct {
		name        string
		authHeader  string
		wantAPIKey  string
		wantAgentID string
		wantErr     error
	}{
		{
			name:        "Empty header",
			authHeader:  "",
			wantAPIKey:  "",
			wantAgentID: "",
			wantErr:     ErrMissingAuthHeader,
		},
		{
			name:        "Plain API key (MCP spec 7.1)",
			authHeader:  "my-secret-api-key",
			wantAPIKey:  "my-secret-api-key",
			wantAgentID: "my-secret-api-key",
			wantErr:     nil,
		},
		{
			name:        "Bearer token (backward compatibility)",
			authHeader:  "Bearer my-token-123",
			wantAPIKey:  "my-token-123",
			wantAgentID: "my-token-123",
			wantErr:     nil,
		},
		{
			name:        "Agent format",
			authHeader:  "Agent agent-123",
			wantAPIKey:  "agent-123",
			wantAgentID: "agent-123",
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAPIKey, gotAgentID, gotErr := ParseAuthHeader(tt.authHeader)

			if gotErr != tt.wantErr {
				t.Errorf("ParseAuthHeader() error = %v, wantErr %v", gotErr, tt.wantErr)
				return
			}

			if gotAPIKey != tt.wantAPIKey {
				t.Errorf("ParseAuthHeader() gotAPIKey = %v, want %v", gotAPIKey, tt.wantAPIKey)
			}

			if gotAgentID != tt.wantAgentID {
				t.Errorf("ParseAuthHeader() gotAgentID = %v, want %v", gotAgentID, tt.wantAgentID)
			}
		})
	}
}

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		provided string
		expected string
		want     bool
	}{
		{
			name:     "Matching keys",
			provided: "my-secret-key",
			expected: "my-secret-key",
			want:     true,
		},
		{
			name:     "Non-matching keys",
			provided: "wrong-key",
			expected: "correct-key",
			want:     false,
		},
		{
			name:     "Empty expected (auth disabled)",
			provided: "any-key",
			expected: "",
			want:     true,
		},
		{
			name:     "Empty provided with expected",
			provided: "",
			expected: "required-key",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateAPIKey(tt.provided, tt.expected); got != tt.want {
				t.Errorf("ValidateAPIKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
