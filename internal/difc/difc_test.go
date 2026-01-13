package difc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabelOperations(t *testing.T) {
	t.Run("SecrecyLabel flow checks", func(t *testing.T) {
		// Create labels
		l1 := NewSecrecyLabel()
		l1.Label.Add("tag1")
		l1.Label.Add("tag2")

		l2 := NewSecrecyLabel()
		l2.Label.Add("tag1")
		l2.Label.Add("tag2")
		l2.Label.Add("tag3")

		// l1 should flow to l2 (l1 ⊆ l2)
		assert.True(t, l1.CanFlowTo(l2), "Expected l1 to flow to l2")

		// l2 should NOT flow to l1 (l2 has extra tags)
		assert.False(t, l2.CanFlowTo(l1), "Expected l2 NOT to flow to l1")
	})

	t.Run("IntegrityLabel flow checks", func(t *testing.T) {
		// Create labels
		l1 := NewIntegrityLabel()
		l1.Label.Add("trust1")
		l1.Label.Add("trust2")

		l2 := NewIntegrityLabel()
		l2.Label.Add("trust1")

		// l1 should flow to l2 (l1 ⊇ l2)
		assert.True(t, l1.CanFlowTo(l2), "Expected l1 to flow to l2")

		// l2 should NOT flow to l1 (l2 missing trust2)
		assert.False(t, l2.CanFlowTo(l1), "Expected l2 NOT to flow to l1")
	})

	t.Run("Empty labels flow to everything", func(t *testing.T) {
		empty := NewSecrecyLabel()
		withTags := NewSecrecyLabel()
		withTags.Label.Add("tag1")

		// Empty should flow to anything
		assert.True(t, empty.CanFlowTo(withTags), "Expected empty to flow to withTags")

		// withTags should NOT flow to empty
		assert.False(t, withTags.CanFlowTo(empty), "Expected withTags NOT to flow to empty")
	})
}

func TestEvaluator(t *testing.T) {
	eval := NewEvaluator()

	t.Run("Read operation - secrecy check", func(t *testing.T) {
		// Agent with no secrecy tags tries to read data with secrecy requirements
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()

		resource := NewLabeledResource("private-file")
		resource.Secrecy.Label.Add("private")

		result := eval.Evaluate(agentSecrecy, agentIntegrity, resource, OperationRead)

		assert.False(t, result.IsAllowed(), "Expected access to be denied for read with insufficient secrecy")

		assert.False(t, len(result.SecrecyToAdd) == 0, "Expected SecrecyToAdd to contain required tags")
	})

	t.Run("Read operation - allowed with matching labels", func(t *testing.T) {
		// Agent with secrecy tag can read data with that tag
		agentSecrecy := NewSecrecyLabel()
		agentSecrecy.Label.Add("private")
		agentIntegrity := NewIntegrityLabel()

		resource := NewLabeledResource("private-file")
		resource.Secrecy.Label.Add("private")

		result := eval.Evaluate(agentSecrecy, agentIntegrity, resource, OperationRead)

		if !result.IsAllowed() {
			t.Errorf("Expected access to be allowed: %s", result.Reason)
		}
	})

	t.Run("Write operation - integrity check", func(t *testing.T) {
		// Agent without integrity tries to write to high-integrity resource
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()

		resource := NewLabeledResource("production-database")
		resource.Integrity.Label.Add("production")

		result := eval.Evaluate(agentSecrecy, agentIntegrity, resource, OperationWrite)

		assert.False(t, result.IsAllowed(), "Expected access to be denied for write with insufficient integrity")

		assert.False(t, len(result.IntegrityToDrop) == 0, "Expected IntegrityToDrop to contain required tags")
	})

	t.Run("Write operation - allowed with matching integrity", func(t *testing.T) {
		// Agent with production integrity can write to production resource
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()
		agentIntegrity.Label.Add("production")

		resource := NewLabeledResource("production-database")
		resource.Integrity.Label.Add("production")

		result := eval.Evaluate(agentSecrecy, agentIntegrity, resource, OperationWrite)

		if !result.IsAllowed() {
			t.Errorf("Expected access to be allowed: %s", result.Reason)
		}
	})

	t.Run("Empty resource allows all operations", func(t *testing.T) {
		// NoopGuard returns empty labels - should allow everything
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()

		resource := NewLabeledResource("noop-resource")
		// No tags added = no restrictions

		// Both read and write should be allowed
		readResult := eval.Evaluate(agentSecrecy, agentIntegrity, resource, OperationRead)
		writeResult := eval.Evaluate(agentSecrecy, agentIntegrity, resource, OperationWrite)

		if !readResult.IsAllowed() {
			t.Errorf("Expected read to be allowed for empty resource: %s", readResult.Reason)
		}
		if !writeResult.IsAllowed() {
			t.Errorf("Expected write to be allowed for empty resource: %s", writeResult.Reason)
		}
	})
}

func TestAgentRegistry(t *testing.T) {
	registry := NewAgentRegistry()

	t.Run("GetOrCreate creates new agent", func(t *testing.T) {
		agent := registry.GetOrCreate("agent-1")
		if agent.AgentID != "agent-1" {
			t.Errorf("Expected agent ID to be 'agent-1', got %s", agent.AgentID)
		}

		// Should have empty labels initially
		assert.True(t, agent.Secrecy.Label.IsEmpty(), "Expected new agent to have empty secrecy labels")
		assert.True(t, agent.Integrity.Label.IsEmpty(), "Expected new agent to have empty integrity labels")
	})

	t.Run("GetOrCreate returns existing agent", func(t *testing.T) {
		agent1 := registry.GetOrCreate("agent-2")
		agent1.Secrecy.Label.Add("secret")

		agent2 := registry.GetOrCreate("agent-2")
		assert.Equal(t, agent2, agent1, "to get same agent instance")

		assert.True(t, agent2.Secrecy.Label.Contains("secret"), "Expected agent to retain added tags")
	})

	t.Run("AccumulateFromRead updates agent labels", func(t *testing.T) {
		agent := registry.GetOrCreate("agent-3")

		resource := NewLabeledResource("data-source")
		resource.Secrecy.Label.Add("confidential")
		resource.Integrity.Label.Add("verified")

		agent.AccumulateFromRead(resource)

		assert.True(t, agent.Secrecy.Label.Contains("confidential"), "Expected agent to gain secrecy tag from read")
		assert.True(t, agent.Integrity.Label.Contains("verified"), "Expected agent to gain integrity tag from read")
	})
}

func TestCollectionFiltering(t *testing.T) {
	eval := NewEvaluator()

	t.Run("FilterCollection filters inaccessible items", func(t *testing.T) {
		// Agent with limited clearance
		agentSecrecy := NewSecrecyLabel()
		agentSecrecy.Label.Add("public")
		agentIntegrity := NewIntegrityLabel()

		// Create collection with mixed access
		collection := &CollectionLabeledData{
			Items: []LabeledItem{
				{
					Data: map[string]string{"name": "public-item"},
					Labels: &LabeledResource{
						Description: "public item",
						Secrecy:     *NewSecrecyLabelWithTags([]Tag{"public"}),
						Integrity:   *NewIntegrityLabel(),
					},
				},
				{
					Data: map[string]string{"name": "secret-item"},
					Labels: &LabeledResource{
						Description: "secret item",
						Secrecy:     *NewSecrecyLabelWithTags([]Tag{"secret"}),
						Integrity:   *NewIntegrityLabel(),
					},
				},
			},
		}

		filtered := eval.FilterCollection(agentSecrecy, agentIntegrity, collection, OperationRead)

		if filtered.GetAccessibleCount() != 1 {
			t.Errorf("Expected 1 accessible item, got %d", filtered.GetAccessibleCount())
		}
		if filtered.GetFilteredCount() != 1 {
			t.Errorf("Expected 1 filtered item, got %d", filtered.GetFilteredCount())
		}
	})
}
