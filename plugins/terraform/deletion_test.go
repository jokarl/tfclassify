package main

import (
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

type mockRunner struct {
	changes   []*sdk.ResourceChange
	decisions []*mockDecision
}

type mockDecision struct {
	analyzer sdk.Analyzer
	change   *sdk.ResourceChange
	decision *sdk.Decision
}

func (r *mockRunner) GetResourceChanges(patterns []string) ([]*sdk.ResourceChange, error) {
	return r.changes, nil
}

func (r *mockRunner) GetResourceChange(address string) (*sdk.ResourceChange, error) {
	for _, c := range r.changes {
		if c.Address == address {
			return c, nil
		}
	}
	return nil, nil
}

func (r *mockRunner) EmitDecision(analyzer sdk.Analyzer, change *sdk.ResourceChange, decision *sdk.Decision) error {
	r.decisions = append(r.decisions, &mockDecision{
		analyzer: analyzer,
		change:   change,
		decision: decision,
	})
	return nil
}

func TestDeletionAnalyzer_StandaloneDelete(t *testing.T) {
	config := &PluginConfig{DeletionEnabled: true}
	analyzer := NewDeletionAnalyzer(config)

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

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0].decision
	if decision.Severity != 80 {
		t.Errorf("expected severity 80, got %d", decision.Severity)
	}
}

func TestDeletionAnalyzer_ReplaceNotFlagged(t *testing.T) {
	config := &PluginConfig{DeletionEnabled: true}
	analyzer := NewDeletionAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_resource_group.example",
				Type:    "azurerm_resource_group",
				Actions: []string{"delete", "create"}, // This is a replace, not standalone delete
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT emit a decision for a replace operation
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for replace, got %d", len(runner.decisions))
	}
}

func TestDeletionAnalyzer_CreateIgnored(t *testing.T) {
	config := &PluginConfig{DeletionEnabled: true}
	analyzer := NewDeletionAnalyzer(config)

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
		t.Errorf("expected 0 decisions for create, got %d", len(runner.decisions))
	}
}

func TestDeletionAnalyzer_UpdateIgnored(t *testing.T) {
	config := &PluginConfig{DeletionEnabled: true}
	analyzer := NewDeletionAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_resource_group.example",
				Type:    "azurerm_resource_group",
				Actions: []string{"update"},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for update, got %d", len(runner.decisions))
	}
}

func TestDeletionAnalyzer_Disabled(t *testing.T) {
	config := &PluginConfig{DeletionEnabled: false}
	analyzer := NewDeletionAnalyzer(config)

	if analyzer.Enabled() {
		t.Error("expected analyzer to be disabled")
	}
}
