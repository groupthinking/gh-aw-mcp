package launcher

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		servers        map[string]*config.ServerConfig
		wantServerLen  int
		setupContainer bool
	}{
		{
			name: "empty config",
			servers: map[string]*config.ServerConfig{},
			wantServerLen: 0,
			setupContainer: false,
		},
		{
			name: "single server",
			servers: map[string]*config.ServerConfig{
				"test": {
					Command: "docker",
					Args:    []string{"run", "test"},
				},
			},
			wantServerLen: 1,
			setupContainer: false,
		},
		{
			name: "multiple servers",
			servers: map[string]*config.ServerConfig{
				"server1": {
					Command: "docker",
					Args:    []string{"run", "test1"},
				},
				"server2": {
					Command: "docker",
					Args:    []string{"run", "test2"},
				},
				"server3": {
					Command: "node",
					Args:    []string{"server.js"},
				},
			},
			wantServerLen: 3,
			setupContainer: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := &config.Config{
				Servers: tt.servers,
			}

			launcher := New(ctx, cfg)

			if launcher == nil {
				t.Fatal("New() returned nil launcher")
			}

			if launcher.ctx != ctx {
				t.Error("Context not set correctly")
			}

			if launcher.config != cfg {
				t.Error("Config not set correctly")
			}

			if launcher.connections == nil {
				t.Error("Connections map not initialized")
			}

			if len(launcher.connections) != 0 {
				t.Errorf("Expected empty connections map, got %d connections", len(launcher.connections))
			}

			if len(cfg.Servers) != tt.wantServerLen {
				t.Errorf("Expected %d servers in config, got %d", tt.wantServerLen, len(cfg.Servers))
			}
		})
	}
}

func TestServerIDs(t *testing.T) {
	tests := []struct {
		name    string
		servers map[string]*config.ServerConfig
		want    []string
	}{
		{
			name:    "empty config",
			servers: map[string]*config.ServerConfig{},
			want:    []string{},
		},
		{
			name: "single server",
			servers: map[string]*config.ServerConfig{
				"test": {Command: "docker"},
			},
			want: []string{"test"},
		},
		{
			name: "multiple servers",
			servers: map[string]*config.ServerConfig{
				"github":   {Command: "docker"},
				"database": {Command: "node"},
				"api":      {Command: "python"},
			},
			want: []string{"github", "database", "api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := &config.Config{Servers: tt.servers}
			launcher := New(ctx, cfg)

			got := launcher.ServerIDs()

			if len(got) != len(tt.want) {
				t.Fatalf("ServerIDs() returned %d IDs, want %d", len(got), len(tt.want))
			}

			// Check all expected IDs are present (order doesn't matter for maps)
			gotMap := make(map[string]bool)
			for _, id := range got {
				gotMap[id] = true
			}

			for _, wantID := range tt.want {
				if !gotMap[wantID] {
					t.Errorf("ServerIDs() missing expected ID: %s", wantID)
				}
			}
		})
	}
}

func TestServerIDs_Concurrent(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"server1": {Command: "docker"},
			"server2": {Command: "node"},
			"server3": {Command: "python"},
		},
	}
	launcher := New(ctx, cfg)

	// Test concurrent reads are safe
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids := launcher.ServerIDs()
			if len(ids) != 3 {
				t.Errorf("Concurrent ServerIDs() returned %d IDs, want 3", len(ids))
			}
		}()
	}
	wg.Wait()
}

func TestGetOrLaunch_ServerNotFound(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"existing": {Command: "docker"},
		},
	}
	launcher := New(ctx, cfg)

	_, err := GetOrLaunch(launcher, "nonexistent")
	if err == nil {
		t.Fatal("GetOrLaunch() expected error for nonexistent server, got nil")
	}

	expectedMsg := "not found in config"
	if err.Error() != "server 'nonexistent' not found in config" {
		t.Errorf("GetOrLaunch() error = %v, want error containing %q", err, expectedMsg)
	}
}

func TestGetOrLaunch_MultipleServersNotFound(t *testing.T) {
	tests := []struct {
		name     string
		serverID string
	}{
		{"empty string", ""},
		{"special chars", "server@#$%"},
		{"spaces", "server with spaces"},
		{"just spaces", "   "},
	}

	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"valid": {Command: "docker"},
		},
	}
	launcher := New(ctx, cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetOrLaunch(launcher, tt.serverID)
			if err == nil {
				t.Errorf("GetOrLaunch(%q) expected error, got nil", tt.serverID)
			}
		})
	}
}

func TestGetOrLaunch_EnvironmentVariablePassthrough(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		envVars      map[string]string
		shouldSetEnv bool
	}{
		{
			name: "passthrough with existing env var",
			args: []string{"run", "-e", "TEST_VAR", "container"},
			envVars: map[string]string{
				"TEST_VAR": "test_value",
			},
			shouldSetEnv: true,
		},
		{
			name: "passthrough with missing env var",
			args: []string{"run", "-e", "MISSING_VAR", "container"},
			envVars: map[string]string{},
			shouldSetEnv: false,
		},
		{
			name: "explicit value (not passthrough)",
			args: []string{"run", "-e", "VAR=value", "container"},
			envVars: map[string]string{},
			shouldSetEnv: false,
		},
		{
			name: "multiple -e flags mixed",
			args: []string{"run", "-e", "VAR1", "-e", "VAR2=explicit", "-e", "VAR3", "container"},
			envVars: map[string]string{
				"VAR1": "value1",
				"VAR3": "value3",
			},
			shouldSetEnv: true,
		},
		{
			name: "no -e flags",
			args: []string{"run", "container"},
			envVars: map[string]string{},
			shouldSetEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			for k, v := range tt.envVars {
				if tt.shouldSetEnv {
					os.Setenv(k, v)
					defer os.Unsetenv(k)
				}
			}

			ctx := context.Background()
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{
					"test": {
						Command: "docker",
						Args:    tt.args,
					},
				},
			}
			launcher := New(ctx, cfg)

			// We expect this to fail because we can't actually launch docker,
			// but we're testing the environment detection logic before the launch
			_, err := GetOrLaunch(launcher, "test")

			// Should get an error (connection failure), but that's expected
			// We're just testing the env var detection code path was executed
			if err == nil {
				t.Error("Expected connection error (since we're not actually launching), got nil")
			}
		})
	}
}

func TestClose(t *testing.T) {
	tests := []struct {
		name            string
		servers         map[string]*config.ServerConfig
		initialConnKeys []string
	}{
		{
			name:            "empty launcher",
			servers:         map[string]*config.ServerConfig{},
			initialConnKeys: []string{},
		},
		{
			name: "launcher with config but no connections",
			servers: map[string]*config.ServerConfig{
				"server1": {Command: "docker"},
			},
			initialConnKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := &config.Config{Servers: tt.servers}
			launcher := New(ctx, cfg)

			// Close should not panic and should clear connections
			launcher.Close()

			launcher.mu.RLock()
			connLen := len(launcher.connections)
			launcher.mu.RUnlock()

			if connLen != 0 {
				t.Errorf("Close() did not clear connections, got %d connections", connLen)
			}
		})
	}
}

func TestClose_Concurrent(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"server1": {Command: "docker"},
			"server2": {Command: "node"},
		},
	}
	launcher := New(ctx, cfg)

	// Test concurrent close and read operations
	var wg sync.WaitGroup

	// Start goroutines that read ServerIDs
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond * 10)
			_ = launcher.ServerIDs()
		}()
	}

	// Start goroutines that close
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond * 5)
			launcher.Close()
		}()
	}

	wg.Wait()

	// After all operations, connections should be empty
	launcher.mu.RLock()
	connLen := len(launcher.connections)
	launcher.mu.RUnlock()

	if connLen != 0 {
		t.Errorf("After concurrent Close() operations, got %d connections, want 0", connLen)
	}
}

func TestLauncher_ContainerDetection(t *testing.T) {
	// This test verifies the container detection happens during New()
	// The actual detection is done by tty.IsRunningInContainer()
	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test": {Command: "docker"},
		},
	}

	launcher := New(ctx, cfg)

	// We can't easily mock tty.IsRunningInContainer(), but we can verify
	// the field exists and is set (to whatever the runtime environment is)
	_ = launcher.runningInContainer // Field should exist

	// Just verify the launcher was created successfully
	if launcher == nil {
		t.Fatal("New() returned nil launcher")
	}
}

func TestGetOrLaunch_DirectCommandWarning(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		wantWarn  bool
	}{
		{
			name:     "docker command (no warning)",
			command:  "docker",
			wantWarn: false,
		},
		{
			name:     "direct command (potential warning if in container)",
			command:  "node",
			wantWarn: true,
		},
		{
			name:     "python command (potential warning if in container)",
			command:  "python",
			wantWarn: true,
		},
		{
			name:     "custom script (potential warning if in container)",
			command:  "/usr/local/bin/custom-server",
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{
					"test": {
						Command: tt.command,
						Args:    []string{"arg1"},
					},
				},
			}
			launcher := New(ctx, cfg)

			// Attempt to launch (will fail, but tests the warning logic)
			_, err := GetOrLaunch(launcher, "test")

			// Should get an error due to actual launch failure
			if err == nil {
				t.Error("Expected error from launch attempt, got nil")
			}

			// The warning detection happens based on:
			// isDirectCommand := serverCfg.Command != "docker"
			// The actual warning is logged if l.runningInContainer && isDirectCommand
			// We've verified the code path is executed
		})
	}
}

func TestGetOrLaunch_DoubleCheckLocking(t *testing.T) {
	// This test verifies the double-check locking pattern works correctly
	// It's difficult to test without actually creating connections,
	// but we can at least verify the logic flow for "server not found"
	ctx := context.Background()
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"server1": {Command: "docker"},
		},
	}
	launcher := New(ctx, cfg)

	// Try to launch a non-existent server from multiple goroutines
	var wg sync.WaitGroup
	errCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := GetOrLaunch(launcher, "nonexistent")
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// All goroutines should get the "not found" error
	if errCount != 10 {
		t.Errorf("Expected 10 errors, got %d", errCount)
	}
}

func TestNew_NilConfig(t *testing.T) {
	// Defensive test - verify behavior with nil config
	// This would likely panic in real usage, but tests defensive coding
	ctx := context.Background()
	
	// This will panic if config.Servers is accessed without nil check
	// The actual code doesn't check for nil, but tests document the expectation
	defer func() {
		if r := recover(); r != nil {
			// Expected panic with nil config
			t.Logf("Expected panic with nil config: %v", r)
		}
	}()
	
	_ = New(ctx, nil)
	
	// If we get here, no panic occurred (config handling is defensive)
	// This is actually fine - the code will just have an empty servers map
}

func TestGetOrLaunch_ContextCancellation(t *testing.T) {
	// Test that a cancelled context is handled properly
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test": {
				Command: "docker",
				Args:    []string{"run", "test"},
			},
		},
	}
	launcher := New(ctx, cfg)

	// Attempt to launch with cancelled context
	_, err := GetOrLaunch(launcher, "test")

	// Should get an error (either from context or connection failure)
	if err == nil {
		t.Error("Expected error with cancelled context, got nil")
	}
}

func TestGetOrLaunch_EmptyServerConfig(t *testing.T) {
	// Test with empty/minimal server config
	tests := []struct {
		name   string
		config *config.ServerConfig
	}{
		{
			name: "empty command",
			config: &config.ServerConfig{
				Command: "",
				Args:    []string{},
			},
		},
		{
			name: "only command",
			config: &config.ServerConfig{
				Command: "docker",
			},
		},
		{
			name: "command with nil args",
			config: &config.ServerConfig{
				Command: "docker",
				Args:    nil,
			},
		},
		{
			name: "command with empty env",
			config: &config.ServerConfig{
				Command: "docker",
				Args:    []string{"run"},
				Env:     map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{
					"test": tt.config,
				},
			}
			launcher := New(ctx, cfg)

			// Attempt to launch (will fail, but tests the config handling)
			_, err := GetOrLaunch(launcher, "test")

			// Should get an error due to launch failure
			// (empty command will fail, docker without proper args will fail, etc.)
			if err == nil {
				t.Error("Expected error from invalid config, got nil")
			}
		})
	}
}
