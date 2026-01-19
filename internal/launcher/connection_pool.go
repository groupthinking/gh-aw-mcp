package launcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	"github.com/githubnext/gh-aw-mcpg/internal/mcp"
)

var logPool = logger.New("launcher:pool")

// ConnectionKey uniquely identifies a connection by backend and session
type ConnectionKey struct {
	BackendID string
	SessionID string
}

// ConnectionMetadata tracks information about a pooled connection
type ConnectionMetadata struct {
	Connection  *mcp.Connection
	CreatedAt   time.Time
	LastUsedAt  time.Time
	RequestCount int
	ErrorCount  int
	State       ConnectionState
}

// ConnectionState represents the state of a pooled connection
type ConnectionState string

const (
	ConnectionStateActive ConnectionState = "active"
	ConnectionStateIdle   ConnectionState = "idle"
	ConnectionStateClosed ConnectionState = "closed"
)

// SessionConnectionPool manages connections keyed by (backend, session)
type SessionConnectionPool struct {
	connections map[ConnectionKey]*ConnectionMetadata
	mu          sync.RWMutex
	ctx         context.Context
}

// NewSessionConnectionPool creates a new connection pool
func NewSessionConnectionPool(ctx context.Context) *SessionConnectionPool {
	logPool.Print("Creating new session connection pool")
	return &SessionConnectionPool{
		connections: make(map[ConnectionKey]*ConnectionMetadata),
		ctx:         ctx,
	}
}

// Get retrieves a connection from the pool
func (p *SessionConnectionPool) Get(backendID, sessionID string) (*mcp.Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := ConnectionKey{BackendID: backendID, SessionID: sessionID}
	metadata, exists := p.connections[key]
	
	if !exists {
		logPool.Printf("Connection not found: backend=%s, session=%s", backendID, sessionID)
		return nil, false
	}

	if metadata.State == ConnectionStateClosed {
		logPool.Printf("Connection is closed: backend=%s, session=%s", backendID, sessionID)
		return nil, false
	}

	logPool.Printf("Reusing connection: backend=%s, session=%s, requests=%d", 
		backendID, sessionID, metadata.RequestCount)
	
	// Update last used time (need write lock for this)
	p.mu.RUnlock()
	p.mu.Lock()
	metadata.LastUsedAt = time.Now()
	metadata.RequestCount++
	p.mu.Unlock()
	p.mu.RLock()

	return metadata.Connection, true
}

// Set adds or updates a connection in the pool
func (p *SessionConnectionPool) Set(backendID, sessionID string, conn *mcp.Connection) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := ConnectionKey{BackendID: backendID, SessionID: sessionID}
	
	// Check if connection already exists
	if existing, exists := p.connections[key]; exists {
		logPool.Printf("Updating existing connection: backend=%s, session=%s", backendID, sessionID)
		existing.Connection = conn
		existing.LastUsedAt = time.Now()
		existing.State = ConnectionStateActive
		return
	}

	// Create new metadata
	metadata := &ConnectionMetadata{
		Connection:   conn,
		CreatedAt:    time.Now(),
		LastUsedAt:   time.Now(),
		RequestCount: 0,
		ErrorCount:   0,
		State:        ConnectionStateActive,
	}

	p.connections[key] = metadata
	logPool.Printf("Added new connection to pool: backend=%s, session=%s", backendID, sessionID)
}

// Delete removes a connection from the pool
func (p *SessionConnectionPool) Delete(backendID, sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := ConnectionKey{BackendID: backendID, SessionID: sessionID}
	
	if metadata, exists := p.connections[key]; exists {
		metadata.State = ConnectionStateClosed
		delete(p.connections, key)
		logPool.Printf("Deleted connection from pool: backend=%s, session=%s", backendID, sessionID)
	}
}

// Size returns the number of connections in the pool
func (p *SessionConnectionPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.connections)
}

// GetMetadata returns metadata for a connection (for testing/monitoring)
func (p *SessionConnectionPool) GetMetadata(backendID, sessionID string) (*ConnectionMetadata, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := ConnectionKey{BackendID: backendID, SessionID: sessionID}
	metadata, exists := p.connections[key]
	return metadata, exists
}

// RecordError increments the error count for a connection
func (p *SessionConnectionPool) RecordError(backendID, sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := ConnectionKey{BackendID: backendID, SessionID: sessionID}
	if metadata, exists := p.connections[key]; exists {
		metadata.ErrorCount++
		logPool.Printf("Recorded error for connection: backend=%s, session=%s, errors=%d", 
			backendID, sessionID, metadata.ErrorCount)
	}
}

// List returns all connection keys in the pool (for monitoring/debugging)
func (p *SessionConnectionPool) List() []ConnectionKey {
	p.mu.RLock()
	defer p.mu.RUnlock()

	keys := make([]ConnectionKey, 0, len(p.connections))
	for key := range p.connections {
		keys = append(keys, key)
	}
	return keys
}

// String returns a string representation of the connection key
func (k ConnectionKey) String() string {
	return fmt.Sprintf("%s/%s", k.BackendID, k.SessionID)
}
