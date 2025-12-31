package guard

import (
	"context"
	"testing"

	"github.com/githubnext/gh-aw-mcpg/internal/difc"
)

func TestNoopGuard(t *testing.T) {
	guard := NewNoopGuard()

	t.Run("Name returns noop", func(t *testing.T) {
		if guard.Name() != "noop" {
			t.Errorf("Expected name to be 'noop', got %s", guard.Name())
		}
	})

	t.Run("LabelResource returns empty labels", func(t *testing.T) {
		ctx := context.Background()
		caps := difc.NewCapabilities()

		resource, operation, err := guard.LabelResource(ctx, "test_tool", map[string]interface{}{}, nil, caps)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if resource == nil {
			t.Fatal("Expected resource to be non-nil")
		}

		if !resource.Secrecy.Label.IsEmpty() {
			t.Errorf("Expected empty secrecy labels")
		}

		if !resource.Integrity.Label.IsEmpty() {
			t.Errorf("Expected empty integrity labels")
		}

		if operation != difc.OperationWrite {
			t.Errorf("Expected OperationWrite, got %v", operation)
		}
	})

	t.Run("LabelResponse returns nil", func(t *testing.T) {
		ctx := context.Background()
		caps := difc.NewCapabilities()

		labeledData, err := guard.LabelResponse(ctx, "test_tool", map[string]interface{}{}, nil, caps)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if labeledData != nil {
			t.Errorf("Expected nil labeled data")
		}
	})
}

func TestGuardRegistry(t *testing.T) {
	t.Run("Register and Get guard", func(t *testing.T) {
		registry := NewRegistry()
		guard := NewNoopGuard()

		registry.Register("test-server", guard)

		retrieved := registry.Get("test-server")
		if retrieved != guard {
			t.Errorf("Expected to retrieve same guard instance")
		}
	})

	t.Run("Get non-existent guard returns noop", func(t *testing.T) {
		registry := NewRegistry()

		guard := registry.Get("non-existent")
		if guard.Name() != "noop" {
			t.Errorf("Expected noop guard for non-existent server, got %s", guard.Name())
		}
	})

	t.Run("Has checks guard existence", func(t *testing.T) {
		registry := NewRegistry()
		guard := NewNoopGuard()

		if registry.Has("test-server") {
			t.Errorf("Expected Has to return false for non-existent guard")
		}

		registry.Register("test-server", guard)

		if !registry.Has("test-server") {
			t.Errorf("Expected Has to return true for registered guard")
		}
	})

	t.Run("List returns all server IDs", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register("server1", NewNoopGuard())
		registry.Register("server2", NewNoopGuard())

		list := registry.List()
		if len(list) != 2 {
			t.Errorf("Expected 2 servers, got %d", len(list))
		}
	})

	t.Run("GetGuardInfo returns guard names", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register("server1", NewNoopGuard())

		info := registry.GetGuardInfo()
		if info["server1"] != "noop" {
			t.Errorf("Expected guard name 'noop', got %s", info["server1"])
		}
	})
}

func TestCreateGuard(t *testing.T) {
	t.Run("Create noop guard", func(t *testing.T) {
		guard, err := CreateGuard("noop")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if guard.Name() != "noop" {
			t.Errorf("Expected noop guard, got %s", guard.Name())
		}
	})

	t.Run("Create empty string returns noop", func(t *testing.T) {
		guard, err := CreateGuard("")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if guard.Name() != "noop" {
			t.Errorf("Expected noop guard, got %s", guard.Name())
		}
	})

	t.Run("Create unknown guard returns error", func(t *testing.T) {
		_, err := CreateGuard("unknown-guard-type")
		if err == nil {
			t.Errorf("Expected error for unknown guard type")
		}
	})
}

func TestContextHelpers(t *testing.T) {
	t.Run("GetAgentIDFromContext returns default", func(t *testing.T) {
		ctx := context.Background()
		agentID := GetAgentIDFromContext(ctx)

		if agentID != "default" {
			t.Errorf("Expected 'default', got %s", agentID)
		}
	})

	t.Run("SetAgentIDInContext and retrieve", func(t *testing.T) {
		ctx := context.Background()
		ctx = SetAgentIDInContext(ctx, "test-agent")

		agentID := GetAgentIDFromContext(ctx)
		if agentID != "test-agent" {
			t.Errorf("Expected 'test-agent', got %s", agentID)
		}
	})

	t.Run("ExtractAgentIDFromAuthHeader Bearer", func(t *testing.T) {
		agentID := ExtractAgentIDFromAuthHeader("Bearer test-token-123")
		if agentID != "test-token-123" {
			t.Errorf("Expected 'test-token-123', got %s", agentID)
		}
	})

	t.Run("ExtractAgentIDFromAuthHeader Agent", func(t *testing.T) {
		agentID := ExtractAgentIDFromAuthHeader("Agent my-agent-id")
		if agentID != "my-agent-id" {
			t.Errorf("Expected 'my-agent-id', got %s", agentID)
		}
	})

	t.Run("ExtractAgentIDFromAuthHeader empty", func(t *testing.T) {
		agentID := ExtractAgentIDFromAuthHeader("")
		if agentID != "default" {
			t.Errorf("Expected 'default', got %s", agentID)
		}
	})
}
