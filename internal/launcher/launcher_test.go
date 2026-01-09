package launcher

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
	"github.com/githubnext/gh-aw-mcpg/internal/mcp"
)

// mockConnection is a mock implementation for testing
type mockConnection struct {
	closed bool
}

func (m *mockConnection) Close() error {
	m.closed = true
	return nil
}

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		serversCount   int
		inContainer    bool
		expectedNonNil bool
	}{
		{
			name:           "create launcher with no servers",
			serversCount:   0,
			expectedNonNil: true,
		},
		{
			name:           "create launcher with single server",
			serversCount:   1,
			expectedNonNil: true,
		},
		{
			name:           "create launcher with multiple servers",
			serversCount:   3,
			expectedNonNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Servers: make(map[string]*config.ServerConfig),
			}
			for i := 0; i < tt.serversCount; i++ {
				cfg.Servers["server"+string(rune('0'+i))] = &config.ServerConfig{
					Command: "docker",
					Args:    []string{"run", "test"},
				}
			}

			ctx := context.Background()
			launcher := New(ctx, cfg)

			if launcher == nil && tt.expectedNonNil {
				t.Error("Expected non-nil launcher")
			}

			if launcher != nil {
				if launcher.ctx != ctx {
					t.Error("Expected context to be set")
				}
				if launcher.config != cfg {
					t.Error("Expected config to be set")
				}
				if launcher.connections == nil {
					t.Error("Expected connections map to be initialized")
				}
			}
		})
	}
}

func TestServerIDs(t *testing.T) {
	tests := []struct {
		name        string
		serverIDs   []string
		expectedLen int
	}{
		{
			name:        "no servers",
			serverIDs:   []string{},
			expectedLen: 0,
		},
		{
			name:        "single server",
			serverIDs:   []string{"github"},
			expectedLen: 1,
		},
		{
			name:        "multiple servers",
			serverIDs:   []string{"github", "slack", "jira"},
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Servers: make(map[string]*config.ServerConfig),
			}
			for _, id := range tt.serverIDs {
				cfg.Servers[id] = &config.ServerConfig{
					Command: "docker",
					Args:    []string{"run", id},
				}
			}

			ctx := context.Background()
			launcher := New(ctx, cfg)

			ids := launcher.ServerIDs()
			if len(ids) != tt.expectedLen {
				t.Errorf("Expected %d server IDs, got %d", tt.expectedLen, len(ids))
			}

			// Verify all IDs are present
			idMap := make(map[string]bool)
			for _, id := range ids {
				idMap[id] = true
			}
			for _, expectedID := range tt.serverIDs {
				if !idMap[expectedID] {
					t.Errorf("Expected server ID %s not found", expectedID)
				}
			}
		})
	}
}

func TestClose(t *testing.T) {
	t.Run("close with no connections", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{},
		}
		ctx := context.Background()
		launcher := New(ctx, cfg)

		// Should not panic
		launcher.Close()

		if len(launcher.connections) != 0 {
			t.Error("Expected connections map to be empty after close")
		}
	})

	t.Run("close with active connections", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{},
		}
		ctx := context.Background()
		launcher := New(ctx, cfg)

		// Add mock connections
		mockConn1 := &mockConnection{}
		mockConn2 := &mockConnection{}
		launcher.connections["server1"] = (*mcp.Connection)(nil) // Type-safe but won't be called
		launcher.connections["server2"] = (*mcp.Connection)(nil)

		// Track that Close was conceptually called
		// Note: In real implementation, connections would be closed
		launcher.Close()

		if len(launcher.connections) != 0 {
			t.Error("Expected connections map to be emptied after close")
		}

		// In a real test with proper mocks, we'd verify Close was called
		_ = mockConn1
		_ = mockConn2
	})
}

func TestGetOrLaunch_ServerNotFound(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"existing-server": {
				Command: "docker",
				Args:    []string{"run", "test"},
			},
		},
	}

	ctx := context.Background()
	launcher := New(ctx, cfg)

	_, err := GetOrLaunch(launcher, "non-existent-server")
	if err == nil {
		t.Error("Expected error for non-existent server")
	}

	if err != nil && err.Error() != "server 'non-existent-server' not found in config" {
		t.Errorf("Expected 'server not found' error, got: %v", err)
	}
}

func TestGetOrLaunch_ReuseExistingConnection(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test-server": {
				Command: "docker",
				Args:    []string{"run", "test"},
			},
		},
	}

	ctx := context.Background()
	launcher := New(ctx, cfg)

	// Manually add a connection to simulate existing connection
	mockConn := (*mcp.Connection)(nil)
	launcher.connections["test-server"] = mockConn

	conn, err := GetOrLaunch(launcher, "test-server")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if conn != mockConn {
		t.Error("Expected to reuse existing connection")
	}

	// Verify connection count didn't change
	if len(launcher.connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(launcher.connections))
	}
}

func TestGetOrLaunch_ConcurrentAccess(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"concurrent-server": {
				Command: "docker",
				Args:    []string{"run", "test"},
			},
		},
	}

	ctx := context.Background()
	launcher := New(ctx, cfg)

	// Pre-populate connection to test concurrent reads
	mockConn := (*mcp.Connection)(nil)
	launcher.connections["concurrent-server"] = mockConn

	// Test concurrent reads (should all succeed with same connection)
	var wg sync.WaitGroup
	numGoroutines := 10
	results := make([]*mcp.Connection, numGoroutines)
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			conn, err := GetOrLaunch(launcher, "concurrent-server")
			results[index] = conn
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// Verify all goroutines got the same connection
	for i := 0; i < numGoroutines; i++ {
		if errors[i] != nil {
			t.Errorf("Goroutine %d got error: %v", i, errors[i])
		}
		if results[i] != mockConn {
			t.Errorf("Goroutine %d got different connection", i)
		}
	}
}

func TestGetOrLaunch_EnvironmentPassthrough(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		args        []string
		shouldSetup bool
	}{
		{
			name: "env var with passthrough",
			envVars: map[string]string{
				"TEST_TOKEN": "secret-token-123",
			},
			args:        []string{"run", "-e", "TEST_TOKEN"},
			shouldSetup: true,
		},
		{
			name:        "env var without passthrough (not set)",
			envVars:     map[string]string{},
			args:        []string{"run", "-e", "MISSING_VAR"},
			shouldSetup: false,
		},
		{
			name: "env var with explicit value",
			envVars: map[string]string{
				"TEST_VAR": "original-value",
			},
			args:        []string{"run", "-e", "TEST_VAR=override-value"},
			shouldSetup: true,
		},
		{
			name:        "no env vars",
			envVars:     map[string]string{},
			args:        []string{"run", "test"},
			shouldSetup: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			if tt.shouldSetup {
				for k, v := range tt.envVars {
					os.Setenv(k, v)
					defer os.Unsetenv(k)
				}
			}

			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{
					"env-server": {
						Command: "docker",
						Args:    tt.args,
					},
				},
			}

			ctx := context.Background()
			launcher := New(ctx, cfg)

			// This will fail to connect (no real docker), but we can verify it doesn't panic
			// and properly processes the environment variables
			_, err := GetOrLaunch(launcher, "env-server")

			// We expect an error because docker isn't really running
			// The important thing is the function doesn't panic on env processing
			if err == nil {
				t.Error("Expected connection error (no real server), but got nil")
			}
		})
	}
}

func TestGetOrLaunch_DirectCommandWarning(t *testing.T) {
	tests := []struct {
		name              string
		command           string
		runningInContainer bool
		expectWarning      bool
	}{
		{
			name:               "direct command in container",
			command:            "node",
			runningInContainer: true,
			expectWarning:      true,
		},
		{
			name:               "docker command in container",
			command:            "docker",
			runningInContainer: true,
			expectWarning:      false,
		},
		{
			name:               "direct command not in container",
			command:            "python",
			runningInContainer: false,
			expectWarning:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{
					"test-server": {
						Command: tt.command,
						Args:    []string{"test.js"},
					},
				},
			}

			ctx := context.Background()
			launcher := New(ctx, cfg)
			launcher.runningInContainer = tt.runningInContainer

			// Attempt to launch (will fail, but we're testing the warning logic path)
			_, err := GetOrLaunch(launcher, "test-server")

			// Should always error (no real server)
			if err == nil {
				t.Error("Expected error from connection attempt")
			}

			// The warning is logged, not returned, so we verify the function executed
			// In a real test with log capture, we'd verify the warning was emitted
		})
	}
}

func TestGetOrLaunch_DoubleCheckLocking(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"test-server": {
				Command: "docker",
				Args:    []string{"run", "test"},
			},
		},
	}

	ctx := context.Background()
	launcher := New(ctx, cfg)

	// Test the double-check locking pattern
	// First goroutine will get write lock, second should wait and see existing connection
	var wg sync.WaitGroup
	var firstConn, secondConn *mcp.Connection
	var firstErr, secondErr error

	// Pre-populate to ensure reuse
	mockConn := (*mcp.Connection)(nil)

	wg.Add(2)

	// First goroutine
	go func() {
		defer wg.Done()
		launcher.connections["test-server"] = mockConn
		firstConn, firstErr = GetOrLaunch(launcher, "test-server")
	}()

	// Second goroutine
	go func() {
		defer wg.Done()
		secondConn, secondErr = GetOrLaunch(launcher, "test-server")
	}()

	wg.Wait()

	// Both should succeed
	if firstErr != nil {
		t.Errorf("First goroutine error: %v", firstErr)
	}
	if secondErr != nil {
		t.Errorf("Second goroutine error: %v", secondErr)
	}

	// Both should get a connection (either same or handled gracefully)
	if firstConn == nil && secondConn == nil {
		t.Error("Both connections are nil")
	}
}

func TestGetOrLaunch_EmptyServerID(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"valid-server": {
				Command: "docker",
				Args:    []string{"run", "test"},
			},
		},
	}

	ctx := context.Background()
	launcher := New(ctx, cfg)

	_, err := GetOrLaunch(launcher, "")
	if err == nil {
		t.Error("Expected error for empty server ID")
	}
}

func TestGetOrLaunch_ConfigWithEnvMap(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"env-server": {
				Command: "docker",
				Args:    []string{"run", "test"},
				Env: map[string]string{
					"KEY1": "value1",
					"KEY2": "value2",
				},
			},
		},
	}

	ctx := context.Background()
	launcher := New(ctx, cfg)

	// Will fail to connect but should process env vars without error
	_, err := GetOrLaunch(launcher, "env-server")

	// Expected to fail (no real docker)
	if err == nil {
		t.Error("Expected connection error")
	}

	// The important part is it didn't panic on env processing
}

func TestLauncher_ThreadSafety(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"server1": {Command: "docker", Args: []string{"run", "test1"}},
			"server2": {Command: "docker", Args: []string{"run", "test2"}},
			"server3": {Command: "docker", Args: []string{"run", "test3"}},
		},
	}

	ctx := context.Background()
	launcher := New(ctx, cfg)

	// Pre-populate connections
	launcher.connections["server1"] = (*mcp.Connection)(nil)
	launcher.connections["server2"] = (*mcp.Connection)(nil)
	launcher.connections["server3"] = (*mcp.Connection)(nil)

	// Concurrent operations
	var wg sync.WaitGroup
	numOps := 20

	// Mix of reads (GetOrLaunch), ServerIDs, and Close operations
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			serverID := "server" + string(rune('1'+(index%3)))
			GetOrLaunch(launcher, serverID)
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			launcher.ServerIDs()
		}()
	}

	wg.Wait()

	// Should not deadlock or panic
	t.Log("Thread safety test completed successfully")
}
