package main

import (
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
