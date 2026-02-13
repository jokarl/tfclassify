package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

func TestSensitiveAnalyzer_DetectsChange(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_key_vault_secret.password",
				Type:    "azurerm_key_vault_secret",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"value": "old-secret",
				},
				After: map[string]interface{}{
					"value": "new-secret",
				},
				BeforeSensitive: map[string]interface{}{
					"value": true,
				},
				AfterSensitive: map[string]interface{}{
					"value": true,
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0].decision
	if decision.Severity != 70 {
		t.Errorf("expected severity 70, got %d", decision.Severity)
	}

	if !strings.Contains(decision.Reason, "value") {
		t.Errorf("expected reason to mention 'value' attribute, got: %s", decision.Reason)
	}
}

func TestSensitiveAnalyzer_NoSensitive(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_resource_group.example",
				Type:    "azurerm_resource_group",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"name": "old-name",
				},
				After: map[string]interface{}{
					"name": "new-name",
				},
				BeforeSensitive: nil,
				AfterSensitive:  nil,
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for non-sensitive change, got %d", len(runner.decisions))
	}
}

func TestSensitiveAnalyzer_NoValueExposed(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)

	sensitiveValue := "super-secret-password-12345"

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_key_vault_secret.password",
				Type:    "azurerm_key_vault_secret",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"value": sensitiveValue,
				},
				After: map[string]interface{}{
					"value": "new-secret",
				},
				BeforeSensitive: map[string]interface{}{
					"value": true,
				},
				AfterSensitive: map[string]interface{}{
					"value": true,
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0].decision

	// The reason should NOT contain the actual sensitive value
	if strings.Contains(decision.Reason, sensitiveValue) {
		t.Errorf("reason should NOT expose sensitive value, got: %s", decision.Reason)
	}

	// The reason should mention the attribute name
	if !strings.Contains(decision.Reason, "value") {
		t.Errorf("reason should mention attribute name 'value', got: %s", decision.Reason)
	}
}

func TestSensitiveAnalyzer_Disabled(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: false}
	analyzer := NewSensitiveAnalyzer(config)

	if analyzer.Enabled() {
		t.Error("expected analyzer to be disabled")
	}
}

func TestSensitiveAnalyzer_GetResourceChangesError(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)
	runner := &mockRunner{err: errors.New("test error")}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSensitiveAnalyzer_EmitDecisionError(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address:         "aws_db_instance.main",
				Actions:         []string{"update"},
				Before:          map[string]interface{}{"password": "old"},
				After:           map[string]interface{}{"password": "new"},
				BeforeSensitive: map[string]interface{}{"password": true},
				AfterSensitive:  map[string]interface{}{"password": true},
			},
		},
		emitErr: errors.New("emit error"),
	}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSensitiveAnalyzer_ResourcePatterns(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)

	patterns := analyzer.ResourcePatterns()
	if len(patterns) != 1 || patterns[0] != "*" {
		t.Errorf("expected patterns [*], got %v", patterns)
	}
}

func TestSensitiveAnalyzer_Name(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)

	if analyzer.Name() != "sensitive" {
		t.Errorf("expected name 'sensitive', got %q", analyzer.Name())
	}
}

func TestSensitiveAnalyzer_NewSensitiveAttribute(t *testing.T) {
	// Test case: attribute becomes sensitive in after state
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "aws_db_instance.main",
				Type:    "aws_db_instance",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"password": "old-pass",
				},
				After: map[string]interface{}{
					"password": "new-pass",
				},
				BeforeSensitive: nil, // was not sensitive
				AfterSensitive: map[string]interface{}{
					"password": true, // now sensitive
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	if !strings.Contains(runner.decisions[0].decision.Reason, "password") {
		t.Errorf("expected reason to mention password, got: %s", runner.decisions[0].decision.Reason)
	}
}

func TestSensitiveAnalyzer_AttributeRemoved(t *testing.T) {
	// Test case: sensitive attribute removed
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "aws_db_instance.main",
				Type:    "aws_db_instance",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"password": "old-pass",
				},
				After:           map[string]interface{}{}, // password removed
				BeforeSensitive: map[string]interface{}{"password": true},
				AfterSensitive:  nil,
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for removed sensitive attr, got %d", len(runner.decisions))
	}
}

func TestSensitiveAnalyzer_SameValue(t *testing.T) {
	// Test case: sensitive attribute exists but value unchanged
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "aws_db_instance.main",
				Type:    "aws_db_instance",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"password": "same-pass",
					"name":     "old-name",
				},
				After: map[string]interface{}{
					"password": "same-pass", // unchanged
					"name":     "new-name",  // changed but not sensitive
				},
				BeforeSensitive: map[string]interface{}{"password": true},
				AfterSensitive:  map[string]interface{}{"password": true},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for unchanged sensitive attr, got %d", len(runner.decisions))
	}
}

func TestSensitiveAnalyzer_MultipleSensitive(t *testing.T) {
	// Test case: multiple sensitive attributes
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "aws_db_instance.main",
				Type:    "aws_db_instance",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"password": "old-pass",
					"api_key":  "old-key",
				},
				After: map[string]interface{}{
					"password": "new-pass",
					"api_key":  "new-key",
				},
				BeforeSensitive: map[string]interface{}{
					"password": true,
					"api_key":  true,
				},
				AfterSensitive: map[string]interface{}{
					"password": true,
					"api_key":  true,
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision (aggregated), got %d", len(runner.decisions))
	}

	decision := runner.decisions[0].decision
	if !strings.Contains(decision.Reason, "password") || !strings.Contains(decision.Reason, "api_key") {
		t.Errorf("expected reason to mention both attrs, got: %s", decision.Reason)
	}
}

func TestAsBoolMap_Nil(t *testing.T) {
	result := asBoolMap(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestAsBoolMap_NotMap(t *testing.T) {
	result := asBoolMap("not a map")
	if result != nil {
		t.Errorf("expected nil for non-map input, got %v", result)
	}

	result = asBoolMap([]string{"a", "b"})
	if result != nil {
		t.Errorf("expected nil for slice input, got %v", result)
	}

	result = asBoolMap(42)
	if result != nil {
		t.Errorf("expected nil for int input, got %v", result)
	}
}

func TestAsBoolMap_ValidMap(t *testing.T) {
	input := map[string]interface{}{"password": true}
	result := asBoolMap(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["password"] != true {
		t.Errorf("expected password=true, got %v", result["password"])
	}
}

func TestHasAttributeChanged_BothNil(t *testing.T) {
	result := hasAttributeChanged("test", nil, nil)
	if result {
		t.Error("expected false when both before and after are nil")
	}
}

func TestHasAttributeChanged_AttributeAdded(t *testing.T) {
	before := map[string]interface{}{}
	after := map[string]interface{}{"password": "new"}
	result := hasAttributeChanged("password", before, after)
	if !result {
		t.Error("expected true when attribute is added")
	}
}

func TestHasAttributeChanged_AttributeRemoved(t *testing.T) {
	before := map[string]interface{}{"password": "old"}
	after := map[string]interface{}{}
	result := hasAttributeChanged("password", before, after)
	if !result {
		t.Error("expected true when attribute is removed")
	}
}

func TestHasAttributeChanged_ValueChanged(t *testing.T) {
	before := map[string]interface{}{"password": "old"}
	after := map[string]interface{}{"password": "new"}
	result := hasAttributeChanged("password", before, after)
	if !result {
		t.Error("expected true when value changed")
	}
}

func TestHasAttributeChanged_ValueUnchanged(t *testing.T) {
	before := map[string]interface{}{"password": "same"}
	after := map[string]interface{}{"password": "same"}
	result := hasAttributeChanged("password", before, after)
	if result {
		t.Error("expected false when value unchanged")
	}
}
