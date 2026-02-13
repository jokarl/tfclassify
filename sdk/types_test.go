package sdk

import (
	"testing"
)

func TestDecision_SeverityRange(t *testing.T) {
	tests := []struct {
		severity int
	}{
		{0},
		{50},
		{100},
	}

	for _, tt := range tests {
		decision := Decision{
			Severity: tt.severity,
		}

		if decision.Severity != tt.severity {
			t.Errorf("expected severity %d, got %d", tt.severity, decision.Severity)
		}
	}
}

func TestResourceChange_BeforeAfterTypes(t *testing.T) {
	rc := ResourceChange{
		Address: "test.resource",
		Type:    "test_resource",
		Before: map[string]interface{}{
			"nested": map[string]interface{}{
				"key": "value",
			},
		},
		After: map[string]interface{}{
			"nested": map[string]interface{}{
				"key": "new-value",
			},
		},
	}

	// Check Before
	if rc.Before == nil {
		t.Fatal("expected Before to be non-nil")
	}

	nested, ok := rc.Before["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("expected nested to be a map")
	}

	if nested["key"] != "value" {
		t.Errorf("expected nested.key to be 'value', got %v", nested["key"])
	}

	// Check After
	if rc.After == nil {
		t.Fatal("expected After to be non-nil")
	}

	nestedAfter, ok := rc.After["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("expected nested to be a map")
	}

	if nestedAfter["key"] != "new-value" {
		t.Errorf("expected nested.key to be 'new-value', got %v", nestedAfter["key"])
	}
}

func TestDecision_Metadata(t *testing.T) {
	decision := Decision{
		Classification: "critical",
		Reason:         "sensitive change detected",
		Severity:       80,
		Metadata: map[string]interface{}{
			"affected_attribute": "password",
			"change_type":        "update",
		},
	}

	if decision.Metadata == nil {
		t.Fatal("expected Metadata to be non-nil")
	}

	if decision.Metadata["affected_attribute"] != "password" {
		t.Errorf("expected affected_attribute to be 'password', got %v",
			decision.Metadata["affected_attribute"])
	}
}
