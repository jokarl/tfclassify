package main

import (
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

func TestPrivilegeEscalation_ReaderToOwner(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"role_definition_name": "Reader",
				},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
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

	decision := runner.decisions[0]
	if decision.Severity != 90 {
		t.Errorf("expected severity 90, got %d", decision.Severity)
	}

	meta := decision.Metadata
	if meta["direction"] != "escalation" {
		t.Errorf("expected direction escalation, got %v", meta["direction"])
	}
	if meta["before_role"] != "Reader" {
		t.Errorf("expected before_role Reader, got %v", meta["before_role"])
	}
	if meta["after_role"] != "Owner" {
		t.Errorf("expected after_role Owner, got %v", meta["after_role"])
	}
}

func TestPrivilegeEscalation_OwnerToReader(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"role_definition_name": "Owner",
				},
				After: map[string]interface{}{
					"role_definition_name": "Reader",
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

	decision := runner.decisions[0]
	if decision.Severity != 40 {
		t.Errorf("expected severity 40, got %d", decision.Severity)
	}

	if decision.Metadata["direction"] != "de-escalation" {
		t.Errorf("expected direction de-escalation, got %v", decision.Metadata["direction"])
	}
}

func TestPrivilegeEscalation_NoChange(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/old/scope",
				},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/new/scope",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions when role unchanged, got %d", len(runner.decisions))
	}
}

func TestPrivilegeEscalation_NonPrivilegedChange(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"role_definition_name": "Reader",
				},
				After: map[string]interface{}{
					"role_definition_name": "Custom Role", // Not in default privileged list
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Neither Reader nor "Custom Role" is privileged, so no escalation/de-escalation
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for non-privileged change, got %d", len(runner.decisions))
	}
}

func TestPrivilegeEscalation_CustomRoles(t *testing.T) {
	config := &PluginConfig{
		PrivilegedRoles:  []string{"Custom Admin"},
		PrivilegeEnabled: true,
	}
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"role_definition_name": "Reader",
				},
				After: map[string]interface{}{
					"role_definition_name": "Custom Admin",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for custom privileged role, got %d", len(runner.decisions))
	}

	// "Owner" should not trigger with custom config
	runner2 := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test2",
				Type:    "azurerm_role_assignment",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"role_definition_name": "Reader",
				},
				After: map[string]interface{}{
					"role_definition_name": "Owner", // Not in custom list
				},
			},
		},
	}

	err = analyzer.Analyze(runner2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner2.decisions) != 0 {
		t.Errorf("expected 0 decisions for Owner when not in custom list, got %d", len(runner2.decisions))
	}
}

func TestPrivilegeEscalation_NewAssignment(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				Before:  nil, // No before state
				After: map[string]interface{}{
					"role_definition_name": "Owner",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for new privileged assignment, got %d", len(runner.decisions))
	}

	if runner.decisions[0].Severity != 90 {
		t.Errorf("expected severity 90, got %d", runner.decisions[0].Severity)
	}
}
