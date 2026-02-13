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
