package guard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/githubnext/gh-aw-mcpg/internal/difc"
)

func TestNoopGuard(t *testing.T) {
	guard := NewNoopGuard()

	t.Run("Name returns noop", func(t *testing.T) {
		assert.Equal(t, "noop", guard.Name())
	})

	t.Run("LabelResource returns empty labels", func(t *testing.T) {
		ctx := context.Background()
		caps := difc.NewCapabilities()

		resource, operation, err := guard.LabelResource(ctx, "test_tool", map[string]interface{}{}, nil, caps)
		require.NoError(t, err)

		require.NotNil(t, resource)

		assert.True(t, resource.Secrecy.Label.IsEmpty(), "Expected empty secrecy labels")

		assert.True(t, resource.Integrity.Label.IsEmpty(), "Expected empty integrity labels")

		assert.Equal(t, difc.OperationWrite, operation)
	})

	t.Run("LabelResponse returns nil", func(t *testing.T) {
		ctx := context.Background()
		caps := difc.NewCapabilities()

		labeledData, err := guard.LabelResponse(ctx, "test_tool", map[string]interface{}{}, nil, caps)
		require.NoError(t, err)

		assert.Nil(t, labeledData)
	})
}

func TestGuardRegistry(t *testing.T) {
	t.Run("Register and Get guard", func(t *testing.T) {
		registry := NewRegistry()
		guard := NewNoopGuard()

		registry.Register("test-server", guard)

		retrieved := registry.Get("test-server")
		assert.Equal(t, guard, retrieved)
	})

	t.Run("Get non-existent guard returns noop", func(t *testing.T) {
		registry := NewRegistry()

		guard := registry.Get("non-existent")
		assert.Equal(t, "noop", guard.Name())
	})

	t.Run("Has checks guard existence", func(t *testing.T) {
		registry := NewRegistry()
		guard := NewNoopGuard()

		assert.False(t, registry.Has("test-server"))

		registry.Register("test-server", guard)

		assert.True(t, registry.Has("test-server"))
	})

	t.Run("List returns all server IDs", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register("server1", NewNoopGuard())
		registry.Register("server2", NewNoopGuard())

		list := registry.List()
		assert.Len(t, list, 2)
	})

	t.Run("GetGuardInfo returns guard names", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register("server1", NewNoopGuard())

		info := registry.GetGuardInfo()
		assert.Equal(t, "noop", info["server1"])
	})
}

func TestCreateGuard(t *testing.T) {
	t.Run("Create noop guard", func(t *testing.T) {
		guard, err := CreateGuard("noop")
		require.NoError(t, err)

		assert.Equal(t, "noop", guard.Name())
	})

	t.Run("Create empty string returns noop", func(t *testing.T) {
		guard, err := CreateGuard("")
		require.NoError(t, err)

		assert.Equal(t, "noop", guard.Name())
	})

	t.Run("Create unknown guard returns error", func(t *testing.T) {
		_, err := CreateGuard("unknown-guard-type")
		require.Error(t, err)
	})
}

func TestContextHelpers(t *testing.T) {
	t.Run("GetAgentIDFromContext returns default", func(t *testing.T) {
		ctx := context.Background()
		agentID := GetAgentIDFromContext(ctx)

		assert.Equal(t, "default", agentID)
	})

	t.Run("SetAgentIDInContext and retrieve", func(t *testing.T) {
		ctx := context.Background()
		ctx = SetAgentIDInContext(ctx, "test-agent")

		agentID := GetAgentIDFromContext(ctx)
		assert.Equal(t, "test-agent", agentID)
	})

	t.Run("ExtractAgentIDFromAuthHeader Bearer", func(t *testing.T) {
		agentID := ExtractAgentIDFromAuthHeader("Bearer test-token-123")
		assert.Equal(t, "test-token-123", agentID)
	})

	t.Run("ExtractAgentIDFromAuthHeader Agent", func(t *testing.T) {
		agentID := ExtractAgentIDFromAuthHeader("Agent my-agent-id")
		assert.Equal(t, "my-agent-id", agentID)
	})

	t.Run("ExtractAgentIDFromAuthHeader empty", func(t *testing.T) {
		agentID := ExtractAgentIDFromAuthHeader("")
		assert.Equal(t, "default", agentID)
	})
}
