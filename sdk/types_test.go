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

func TestDecision_Validate_Valid(t *testing.T) {
	tests := []struct {
		name     string
		severity int
	}{
		{"zero", 0},
		{"mid", 50},
		{"max", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Decision{Severity: tt.severity}
			if err := d.Validate(); err != nil {
				t.Errorf("unexpected error for severity %d: %v", tt.severity, err)
			}
		})
	}
}

func TestDecision_Validate_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		severity int
	}{
		{"negative", -1},
		{"too high", 101},
		{"way too high", 999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Decision{Severity: tt.severity}
			err := d.Validate()
			if err == nil {
				t.Fatalf("expected error for severity %d, got nil", tt.severity)
			}
		})
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
