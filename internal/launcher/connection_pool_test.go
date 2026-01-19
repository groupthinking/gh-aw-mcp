package launcher

import (
	"context"
	"testing"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionConnectionPool(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	require.NotNil(t, pool)
	assert.NotNil(t, pool.connections)
	assert.Equal(t, 0, pool.Size())
}

func TestConnectionPoolSetAndGet(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	// Create a mock connection
	mockConn := &mcp.Connection{}
	
	// Set a connection
	pool.Set("backend1", "session1", mockConn)
	
	// Verify size
	assert.Equal(t, 1, pool.Size())
	
	// Get the connection
	conn, exists := pool.Get("backend1", "session1")
	assert.True(t, exists)
	assert.Equal(t, mockConn, conn)
	
	// Verify metadata was created
	metadata, found := pool.GetMetadata("backend1", "session1")
	assert.True(t, found)
	assert.Equal(t, mockConn, metadata.Connection)
	assert.Equal(t, ConnectionStateActive, metadata.State)
	assert.Equal(t, 1, metadata.RequestCount) // Get increments count
}

func TestConnectionPoolGetNonExistent(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	// Try to get non-existent connection
	conn, exists := pool.Get("backend1", "session1")
	assert.False(t, exists)
	assert.Nil(t, conn)
}

func TestConnectionPoolDelete(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	mockConn := &mcp.Connection{}
	pool.Set("backend1", "session1", mockConn)
	
	assert.Equal(t, 1, pool.Size())
	
	// Delete the connection
	pool.Delete("backend1", "session1")
	
	assert.Equal(t, 0, pool.Size())
	
	// Verify it's no longer accessible
	conn, exists := pool.Get("backend1", "session1")
	assert.False(t, exists)
	assert.Nil(t, conn)
}

func TestConnectionPoolMultipleConnections(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	conn1 := &mcp.Connection{}
	conn2 := &mcp.Connection{}
	conn3 := &mcp.Connection{}
	
	// Add multiple connections with different backend/session combinations
	pool.Set("backend1", "session1", conn1)
	pool.Set("backend1", "session2", conn2)
	pool.Set("backend2", "session1", conn3)
	
	assert.Equal(t, 3, pool.Size())
	
	// Verify each connection is retrievable
	c1, exists := pool.Get("backend1", "session1")
	assert.True(t, exists)
	assert.Equal(t, conn1, c1)
	
	c2, exists := pool.Get("backend1", "session2")
	assert.True(t, exists)
	assert.Equal(t, conn2, c2)
	
	c3, exists := pool.Get("backend2", "session1")
	assert.True(t, exists)
	assert.Equal(t, conn3, c3)
}

func TestConnectionPoolUpdateExisting(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	conn1 := &mcp.Connection{}
	conn2 := &mcp.Connection{}
	
	// Set initial connection
	pool.Set("backend1", "session1", conn1)
	
	// Get metadata
	metadata1, _ := pool.GetMetadata("backend1", "session1")
	createdAt1 := metadata1.CreatedAt
	lastUsed1 := metadata1.LastUsedAt
	
	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)
	
	// Update with new connection
	pool.Set("backend1", "session1", conn2)
	
	// Verify size didn't change
	assert.Equal(t, 1, pool.Size())
	
	// Verify connection was updated
	conn, exists := pool.Get("backend1", "session1")
	assert.True(t, exists)
	assert.Equal(t, conn2, conn)
	
	// Verify metadata
	metadata2, _ := pool.GetMetadata("backend1", "session1")
	assert.Equal(t, createdAt1, metadata2.CreatedAt) // Created time should remain same
	assert.True(t, metadata2.LastUsedAt.After(lastUsed1) || metadata2.LastUsedAt.Equal(lastUsed1)) // Last used should update or be equal
}

func TestConnectionPoolRequestCount(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	mockConn := &mcp.Connection{}
	pool.Set("backend1", "session1", mockConn)
	
	// Get metadata before any Get calls
	metadata, _ := pool.GetMetadata("backend1", "session1")
	assert.Equal(t, 0, metadata.RequestCount)
	
	// Call Get multiple times
	pool.Get("backend1", "session1")
	pool.Get("backend1", "session1")
	pool.Get("backend1", "session1")
	
	// Verify request count increased
	metadata, _ = pool.GetMetadata("backend1", "session1")
	assert.Equal(t, 3, metadata.RequestCount)
}

func TestConnectionPoolRecordError(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	mockConn := &mcp.Connection{}
	pool.Set("backend1", "session1", mockConn)
	
	// Initial error count should be 0
	metadata, _ := pool.GetMetadata("backend1", "session1")
	assert.Equal(t, 0, metadata.ErrorCount)
	
	// Record errors
	pool.RecordError("backend1", "session1")
	pool.RecordError("backend1", "session1")
	
	// Verify error count increased
	metadata, _ = pool.GetMetadata("backend1", "session1")
	assert.Equal(t, 2, metadata.ErrorCount)
}

func TestConnectionPoolList(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	// Empty pool
	keys := pool.List()
	assert.Empty(t, keys)
	
	// Add connections
	pool.Set("backend1", "session1", &mcp.Connection{})
	pool.Set("backend2", "session2", &mcp.Connection{})
	
	keys = pool.List()
	assert.Len(t, keys, 2)
	
	// Verify keys are present (order may vary)
	keyStrings := make([]string, len(keys))
	for i, key := range keys {
		keyStrings[i] = key.String()
	}
	assert.Contains(t, keyStrings, "backend1/session1")
	assert.Contains(t, keyStrings, "backend2/session2")
}

func TestConnectionKeyString(t *testing.T) {
	key := ConnectionKey{
		BackendID: "test-backend",
		SessionID: "test-session",
	}
	
	assert.Equal(t, "test-backend/test-session", key.String())
}

func TestConnectionPoolConcurrency(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	mockConn := &mcp.Connection{}
	pool.Set("backend1", "session1", mockConn)
	
	// Run concurrent Get operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				pool.Get("backend1", "session1")
			}
			done <- true
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Verify metadata (should be 1000 requests)
	metadata, exists := pool.GetMetadata("backend1", "session1")
	assert.True(t, exists)
	assert.Equal(t, 1000, metadata.RequestCount)
}

func TestConnectionPoolDeleteNonExistent(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	// Delete non-existent connection (should not panic)
	pool.Delete("backend1", "session1")
	
	assert.Equal(t, 0, pool.Size())
}

func TestConnectionStateTransitions(t *testing.T) {
	ctx := context.Background()
	pool := NewSessionConnectionPool(ctx)
	
	mockConn := &mcp.Connection{}
	pool.Set("backend1", "session1", mockConn)
	
	// Initial state should be Active
	metadata, _ := pool.GetMetadata("backend1", "session1")
	assert.Equal(t, ConnectionStateActive, metadata.State)
	
	// Delete marks as Closed and removes
	pool.Delete("backend1", "session1")
	
	// After delete, connection should not exist
	_, exists := pool.GetMetadata("backend1", "session1")
	assert.False(t, exists)
}
