package difc

import (
	"strings"
	"testing"
)

func TestFormatViolationError(t *testing.T) {
	tests := []struct {
		name               string
		result             *EvaluationResult
		agentSecrecy       *SecrecyLabel
		agentIntegrity     *IntegrityLabel
		resource           *LabeledResource
		wantErr            bool
		wantContains       []string
		wantNotContains    []string
	}{
		{
			name: "allowed access returns nil",
			result: &EvaluationResult{
				Decision:        AccessAllow,
				SecrecyToAdd:    []Tag{},
				IntegrityToDrop: []Tag{},
				Reason:          "",
			},
			agentSecrecy:   NewSecrecyLabel(),
			agentIntegrity: NewIntegrityLabel(),
			resource:       NewLabeledResource("test-resource"),
			wantErr:        false,
		},
		{
			name: "secrecy violation with single tag",
			result: &EvaluationResult{
				Decision:        AccessDeny,
				SecrecyToAdd:    []Tag{"private"},
				IntegrityToDrop: []Tag{},
				Reason:          "Agent needs more secrecy tags",
			},
			agentSecrecy:   NewSecrecyLabel(),
			agentIntegrity: NewIntegrityLabel(),
			resource: func() *LabeledResource {
				r := NewLabeledResource("private-file")
				r.Secrecy.Label.Add("private")
				return r
			}(),
			wantErr: true,
			wantContains: []string{
				"DIFC Violation",
				"Agent needs more secrecy tags",
				"Required Action: Add secrecy tags [private]",
				"Agent will be restricted from writing to resources that lack these tags",
				"public resources",
				"public repositories, public internet",
				"handling sensitive information",
				"Current Agent Labels:",
				"Resource Requirements:",
			},
		},
		{
			name: "secrecy violation with multiple tags",
			result: &EvaluationResult{
				Decision:        AccessDeny,
				SecrecyToAdd:    []Tag{"private", "confidential"},
				IntegrityToDrop: []Tag{},
				Reason:          "Multiple secrecy tags required",
			},
			agentSecrecy:   NewSecrecyLabel(),
			agentIntegrity: NewIntegrityLabel(),
			resource: func() *LabeledResource {
				r := NewLabeledResource("secret-data")
				r.Secrecy.Label.Add("private")
				r.Secrecy.Label.Add("confidential")
				return r
			}(),
			wantErr: true,
			wantContains: []string{
				"DIFC Violation",
				"Multiple secrecy tags required",
				"Required Action: Add secrecy tags",
				"private",
				"confidential",
				"Implications of adding secrecy tags:",
				"marked as handling sensitive information",
			},
		},
		{
			name: "integrity violation with single tag",
			result: &EvaluationResult{
				Decision:        AccessDeny,
				SecrecyToAdd:    []Tag{},
				IntegrityToDrop: []Tag{"production"},
				Reason:          "Agent lacks production integrity",
			},
			agentSecrecy: NewSecrecyLabel(),
			agentIntegrity: func() *IntegrityLabel {
				l := NewIntegrityLabel()
				l.Label.Add("production")
				return l
			}(),
			resource: func() *LabeledResource {
				r := NewLabeledResource("prod-db")
				r.Integrity.Label.Add("verified")
				return r
			}(),
			wantErr: true,
			wantContains: []string{
				"DIFC Violation",
				"Agent lacks production integrity",
				"Required Action: Drop integrity tags [production]",
				"Implications of dropping integrity tags:",
				"no longer be able to write to high-integrity resources",
				"cannot write to resources requiring tags: [production]",
				"influenced by lower-integrity data",
				"less trustworthy",
			},
		},
		{
			name: "integrity violation with multiple tags",
			result: &EvaluationResult{
				Decision:        AccessDeny,
				SecrecyToAdd:    []Tag{},
				IntegrityToDrop: []Tag{"verified", "trusted"},
				Reason:          "Multiple integrity tags need dropping",
			},
			agentSecrecy: NewSecrecyLabel(),
			agentIntegrity: func() *IntegrityLabel {
				l := NewIntegrityLabel()
				l.Label.Add("verified")
				l.Label.Add("trusted")
				return l
			}(),
			resource: NewLabeledResource("untrusted-source"),
			wantErr:  true,
			wantContains: []string{
				"DIFC Violation",
				"Multiple integrity tags need dropping",
				"Drop integrity tags",
				"verified",
				"trusted",
			},
		},
		{
			name: "both secrecy and integrity violations",
			result: &EvaluationResult{
				Decision:        AccessDeny,
				SecrecyToAdd:    []Tag{"secret"},
				IntegrityToDrop: []Tag{"trusted"},
				Reason:          "Both secrecy and integrity violated",
			},
			agentSecrecy: NewSecrecyLabel(),
			agentIntegrity: func() *IntegrityLabel {
				l := NewIntegrityLabel()
				l.Label.Add("trusted")
				return l
			}(),
			resource: func() *LabeledResource {
				r := NewLabeledResource("complex-resource")
				r.Secrecy.Label.Add("secret")
				return r
			}(),
			wantErr: true,
			wantContains: []string{
				"DIFC Violation",
				"Both secrecy and integrity violated",
				"Required Action: Add secrecy tags [secret]",
				"Required Action: Drop integrity tags [trusted]",
				"Implications of adding secrecy tags:",
				"Implications of dropping integrity tags:",
				"Current Agent Labels:",
				"Resource Requirements:",
			},
		},
		{
			name: "agent with existing tags",
			result: &EvaluationResult{
				Decision:        AccessDeny,
				SecrecyToAdd:    []Tag{"top-secret"},
				IntegrityToDrop: []Tag{},
				Reason:          "Need higher secrecy clearance",
			},
			agentSecrecy: func() *SecrecyLabel {
				l := NewSecrecyLabel()
				l.Label.Add("confidential")
				return l
			}(),
			agentIntegrity: func() *IntegrityLabel {
				l := NewIntegrityLabel()
				l.Label.Add("verified")
				return l
			}(),
			resource: func() *LabeledResource {
				r := NewLabeledResource("classified-doc")
				r.Secrecy.Label.Add("confidential")
				r.Secrecy.Label.Add("top-secret")
				r.Integrity.Label.Add("verified")
				return r
			}(),
			wantErr: true,
			wantContains: []string{
				"DIFC Violation",
				"Need higher secrecy clearance",
				"top-secret",
				"Current Agent Labels:",
				"Secrecy: [confidential]",
				"Integrity: [verified]",
				"Resource Requirements:",
				"Secrecy:",
				"confidential",
				"top-secret",
			},
		},
		{
			name: "empty agent labels",
			result: &EvaluationResult{
				Decision:        AccessDeny,
				SecrecyToAdd:    []Tag{"private"},
				IntegrityToDrop: []Tag{},
				Reason:          "Empty agent cannot access private resource",
			},
			agentSecrecy:   NewSecrecyLabel(),
			agentIntegrity: NewIntegrityLabel(),
			resource: func() *LabeledResource {
				r := NewLabeledResource("private-data")
				r.Secrecy.Label.Add("private")
				return r
			}(),
			wantErr: true,
			wantContains: []string{
				"DIFC Violation",
				"Empty agent cannot access private resource",
				"Secrecy: []",
				"Integrity: []",
			},
		},
		{
			name: "empty resource labels",
			result: &EvaluationResult{
				Decision:        AccessDeny,
				SecrecyToAdd:    []Tag{"public"},
				IntegrityToDrop: []Tag{},
				Reason:          "Test with empty resource",
			},
			agentSecrecy: func() *SecrecyLabel {
				l := NewSecrecyLabel()
				l.Label.Add("public")
				return l
			}(),
			agentIntegrity: NewIntegrityLabel(),
			resource:       NewLabeledResource("public-resource"),
			wantErr:        true,
			wantContains: []string{
				"DIFC Violation",
				"Resource Requirements:",
				"Secrecy: []",
				"Integrity: []",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatViolationError(tt.result, tt.agentSecrecy, tt.agentIntegrity, tt.resource)

			if (err != nil) != tt.wantErr {
				t.Errorf("FormatViolationError() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				return
			}

			errMsg := err.Error()

			// Check that all expected strings are present
			for _, want := range tt.wantContains {
				if !strings.Contains(errMsg, want) {
					t.Errorf("FormatViolationError() error message missing expected string:\nwant: %q\ngot: %s", want, errMsg)
				}
			}

			// Check that unwanted strings are not present
			for _, unwant := range tt.wantNotContains {
				if strings.Contains(errMsg, unwant) {
					t.Errorf("FormatViolationError() error message contains unwanted string:\nunwant: %q\ngot: %s", unwant, errMsg)
				}
			}
		})
	}
}

func TestFormatViolationError_ErrorStructure(t *testing.T) {
	t.Run("error message has proper structure", func(t *testing.T) {
		result := &EvaluationResult{
			Decision:        AccessDeny,
			SecrecyToAdd:    []Tag{"private"},
			IntegrityToDrop: []Tag{},
			Reason:          "Test reason",
		}
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()
		resource := NewLabeledResource("test")

		err := FormatViolationError(result, agentSecrecy, agentIntegrity, resource)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		errMsg := err.Error()

		// Check structure: should have violation header, action, implications, and labels
		sections := []string{
			"DIFC Violation:",
			"Required Action:",
			"Implications of adding secrecy tags:",
			"Current Agent Labels:",
			"Resource Requirements:",
		}

		lastIndex := -1
		for _, section := range sections {
			index := strings.Index(errMsg, section)
			if index == -1 {
				t.Errorf("Missing section: %q", section)
				continue
			}
			if index < lastIndex {
				t.Errorf("Section %q appears out of order", section)
			}
			lastIndex = index
		}
	})

	t.Run("error message formats arrays correctly", func(t *testing.T) {
		result := &EvaluationResult{
			Decision:        AccessDeny,
			SecrecyToAdd:    []Tag{"tag1", "tag2", "tag3"},
			IntegrityToDrop: []Tag{},
			Reason:          "Multiple tags test",
		}
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()
		resource := NewLabeledResource("test")

		err := FormatViolationError(result, agentSecrecy, agentIntegrity, resource)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		errMsg := err.Error()

		// Should contain formatted array
		if !strings.Contains(errMsg, "[tag1 tag2 tag3]") && 
		   !strings.Contains(errMsg, "[tag1, tag2, tag3]") &&
		   !strings.Contains(errMsg, "tag1") {
			t.Errorf("Error message does not contain properly formatted tag array: %s", errMsg)
		}
	})
}

func TestEvaluationResult_IsAllowed(t *testing.T) {
	tests := []struct {
		name     string
		decision AccessDecision
		want     bool
	}{
		{
			name:     "allow decision returns true",
			decision: AccessAllow,
			want:     true,
		},
		{
			name:     "deny decision returns false",
			decision: AccessDeny,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &EvaluationResult{
				Decision: tt.decision,
			}
			if got := result.IsAllowed(); got != tt.want {
				t.Errorf("IsAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOperationType_String(t *testing.T) {
	tests := []struct {
		name string
		op   OperationType
		want string
	}{
		{
			name: "read operation",
			op:   OperationRead,
			want: "read",
		},
		{
			name: "write operation",
			op:   OperationWrite,
			want: "write",
		},
		{
			name: "read-write operation",
			op:   OperationReadWrite,
			want: "read-write",
		},
		{
			name: "unknown operation",
			op:   OperationType(999),
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.op.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccessDecision_String(t *testing.T) {
	tests := []struct {
		name     string
		decision AccessDecision
		want     string
	}{
		{
			name:     "allow decision",
			decision: AccessAllow,
			want:     "allow",
		},
		{
			name:     "deny decision",
			decision: AccessDeny,
			want:     "deny",
		},
		{
			name:     "unknown decision",
			decision: AccessDecision(999),
			want:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterCollection(t *testing.T) {
	eval := NewEvaluator()

	t.Run("empty collection returns empty results", func(t *testing.T) {
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()
		collection := &CollectionLabeledData{
			Items: []LabeledItem{},
		}

		filtered := eval.FilterCollection(agentSecrecy, agentIntegrity, collection, OperationRead)

		if filtered.GetAccessibleCount() != 0 {
			t.Errorf("Expected 0 accessible items, got %d", filtered.GetAccessibleCount())
		}
		if filtered.GetFilteredCount() != 0 {
			t.Errorf("Expected 0 filtered items, got %d", filtered.GetFilteredCount())
		}
		if filtered.TotalCount != 0 {
			t.Errorf("Expected TotalCount 0, got %d", filtered.TotalCount)
		}
	})

	t.Run("all accessible items", func(t *testing.T) {
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()
		collection := &CollectionLabeledData{
			Items: []LabeledItem{
				{
					Data:   map[string]string{"id": "1"},
					Labels: NewLabeledResource("item1"),
				},
				{
					Data:   map[string]string{"id": "2"},
					Labels: NewLabeledResource("item2"),
				},
			},
		}

		filtered := eval.FilterCollection(agentSecrecy, agentIntegrity, collection, OperationRead)

		if filtered.GetAccessibleCount() != 2 {
			t.Errorf("Expected 2 accessible items, got %d", filtered.GetAccessibleCount())
		}
		if filtered.GetFilteredCount() != 0 {
			t.Errorf("Expected 0 filtered items, got %d", filtered.GetFilteredCount())
		}
	})

	t.Run("all filtered items", func(t *testing.T) {
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()

		item1 := NewLabeledResource("secret1")
		item1.Secrecy.Label.Add("secret")

		item2 := NewLabeledResource("secret2")
		item2.Secrecy.Label.Add("top-secret")

		collection := &CollectionLabeledData{
			Items: []LabeledItem{
				{
					Data:   map[string]string{"id": "1"},
					Labels: item1,
				},
				{
					Data:   map[string]string{"id": "2"},
					Labels: item2,
				},
			},
		}

		filtered := eval.FilterCollection(agentSecrecy, agentIntegrity, collection, OperationRead)

		if filtered.GetAccessibleCount() != 0 {
			t.Errorf("Expected 0 accessible items, got %d", filtered.GetAccessibleCount())
		}
		if filtered.GetFilteredCount() != 2 {
			t.Errorf("Expected 2 filtered items, got %d", filtered.GetFilteredCount())
		}
	})

	t.Run("mixed accessible and filtered items", func(t *testing.T) {
		agentSecrecy := NewSecrecyLabel()
		agentSecrecy.Label.Add("public")
		agentIntegrity := NewIntegrityLabel()

		publicItem := NewLabeledResource("public")
		publicItem.Secrecy.Label.Add("public")

		privateItem := NewLabeledResource("private")
		privateItem.Secrecy.Label.Add("private")

		collection := &CollectionLabeledData{
			Items: []LabeledItem{
				{
					Data:   map[string]string{"id": "public1"},
					Labels: publicItem,
				},
				{
					Data:   map[string]string{"id": "private1"},
					Labels: privateItem,
				},
				{
					Data:   map[string]string{"id": "public2"},
					Labels: NewLabeledResource("public2"),
				},
			},
		}

		filtered := eval.FilterCollection(agentSecrecy, agentIntegrity, collection, OperationRead)

		if filtered.GetAccessibleCount() != 2 {
			t.Errorf("Expected 2 accessible items, got %d", filtered.GetAccessibleCount())
		}
		if filtered.GetFilteredCount() != 1 {
			t.Errorf("Expected 1 filtered item, got %d", filtered.GetFilteredCount())
		}
		if filtered.TotalCount != 3 {
			t.Errorf("Expected TotalCount 3, got %d", filtered.TotalCount)
		}
	})

	t.Run("filter reason is set", func(t *testing.T) {
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()
		collection := &CollectionLabeledData{
			Items: []LabeledItem{},
		}

		filtered := eval.FilterCollection(agentSecrecy, agentIntegrity, collection, OperationRead)

		if filtered.FilterReason != "DIFC policy" {
			t.Errorf("Expected FilterReason 'DIFC policy', got %q", filtered.FilterReason)
		}
	})

	t.Run("write operation filtering", func(t *testing.T) {
		agentSecrecy := NewSecrecyLabel()
		agentIntegrity := NewIntegrityLabel()
		agentIntegrity.Label.Add("trusted")

		lowIntegrityResource := NewLabeledResource("low")
		
		highIntegrityResource := NewLabeledResource("high")
		highIntegrityResource.Integrity.Label.Add("verified")

		collection := &CollectionLabeledData{
			Items: []LabeledItem{
				{
					Data:   map[string]string{"id": "1"},
					Labels: lowIntegrityResource,
				},
				{
					Data:   map[string]string{"id": "2"},
					Labels: highIntegrityResource,
				},
			},
		}

		filtered := eval.FilterCollection(agentSecrecy, agentIntegrity, collection, OperationWrite)

		// Agent with "trusted" cannot write to resource requiring "verified"
		if filtered.GetAccessibleCount() != 1 {
			t.Errorf("Expected 1 accessible item for write, got %d", filtered.GetAccessibleCount())
		}
		if filtered.GetFilteredCount() != 1 {
			t.Errorf("Expected 1 filtered item for write, got %d", filtered.GetFilteredCount())
		}
	})
}
