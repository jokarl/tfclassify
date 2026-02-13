package main

import (
	"errors"
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

func TestReplaceAnalyzer_Detects(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_resource_group.example",
				Type:    "azurerm_resource_group",
				Actions: []string{"delete", "create"},
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
	if decision.Severity != 75 {
		t.Errorf("expected severity 75, got %d", decision.Severity)
	}
}

func TestReplaceAnalyzer_CreateOnly(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_resource_group.example",
				Type:    "azurerm_resource_group",
				Actions: []string{"create"},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for create-only, got %d", len(runner.decisions))
	}
}

func TestReplaceAnalyzer_DeleteOnly(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_resource_group.example",
				Type:    "azurerm_resource_group",
				Actions: []string{"delete"},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for delete-only, got %d", len(runner.decisions))
	}
}

func TestReplaceAnalyzer_Disabled(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: false}
	analyzer := NewReplaceAnalyzer(config)

	if analyzer.Enabled() {
		t.Error("expected analyzer to be disabled")
	}
}

func TestReplaceAnalyzer_GetResourceChangesError(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)
	runner := &mockRunner{err: errors.New("test error")}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestReplaceAnalyzer_EmitDecisionError(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{Address: "aws_instance.foo", Actions: []string{"delete", "create"}},
		},
		emitErr: errors.New("emit error"),
	}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestReplaceAnalyzer_ResourcePatterns(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)

	patterns := analyzer.ResourcePatterns()
	if len(patterns) != 1 || patterns[0] != "*" {
		t.Errorf("expected patterns [*], got %v", patterns)
	}
}

func TestReplaceAnalyzer_Name(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)

	if analyzer.Name() != "replace" {
		t.Errorf("expected name 'replace', got %q", analyzer.Name())
	}
}

func TestReplaceAnalyzer_CreateDelete(t *testing.T) {
	// Test that create,delete order also counts as replace
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "aws_instance.foo",
				Actions: []string{"create", "delete"},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Errorf("expected 1 decision for create+delete, got %d", len(runner.decisions))
	}
}
