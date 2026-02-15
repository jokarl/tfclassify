package main

import (
	"errors"
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
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
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
	// Owner scores 95 at subscription scope (1.0x)
	if decision.Severity != 95 {
		t.Errorf("expected severity 95 (Owner at subscription), got %d", decision.Severity)
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
	if meta["role_source"] != "builtin" {
		t.Errorf("expected role_source builtin, got %v", meta["role_source"])
	}
	if meta["scope_level"] != "subscription" {
		t.Errorf("expected scope_level subscription, got %v", meta["scope_level"])
	}
}

func TestPrivilegeEscalation_OwnerToReader_NoDecision(t *testing.T) {
	// De-escalation is no longer detected (CR-0024)
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
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// De-escalation is no longer detected (CR-0024)
	if len(runner.decisions) != 0 {
		t.Fatalf("expected 0 decisions for de-escalation (no longer detected), got %d", len(runner.decisions))
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
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/old-rg",
				},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/new-rg",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Same role, same scope level = same score = no decision
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
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: map[string]interface{}{
					"role_definition_name": "Custom Role", // Unknown role
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With the rewrite, "Custom Role" is unknown (score 50), Reader is 15
	// This IS an escalation from 15 -> 50
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision (unknown role scores higher than Reader), got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["role_source"] != "unknown" {
		t.Errorf("expected role_source unknown, got %v", decision.Metadata["role_source"])
	}
	if decision.Severity != 50 {
		t.Errorf("expected severity 50 (unknown role), got %d", decision.Severity)
	}
}

func TestPrivilegeEscalation_ConfigFallback(t *testing.T) {
	config := &PluginConfig{
		PrivilegedRoles:           []string{"Custom Admin"},
		PrivilegeEnabled:          true,
		RoleDatabase:              DefaultRoleDatabase(),
		UnknownPrivilegedSeverity: 80,
		UnknownRoleSeverity:       50,
		CrossReferenceCustomRoles: true,
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
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: map[string]interface{}{
					"role_definition_name": "Custom Admin",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for config fallback role, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["role_source"] != "config-fallback" {
		t.Errorf("expected role_source config-fallback, got %v", decision.Metadata["role_source"])
	}
	if decision.Severity != 80 {
		t.Errorf("expected severity 80 (config fallback), got %d", decision.Severity)
	}
}

func TestPrivilegeEscalation_BuiltinRoleResolution(t *testing.T) {
	// Test that Owner is still detected with custom config because it's in the DB
	config := &PluginConfig{
		PrivilegedRoles:           []string{"Some Other Role"},
		PrivilegeEnabled:          true,
		RoleDatabase:              DefaultRoleDatabase(),
		UnknownPrivilegedSeverity: 80,
		UnknownRoleSeverity:       50,
		CrossReferenceCustomRoles: true,
	}
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test2",
				Type:    "azurerm_role_assignment",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Owner is resolved from DB, not from config
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for Owner (from DB), got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["role_source"] != "builtin" {
		t.Errorf("expected role_source builtin, got %v", decision.Metadata["role_source"])
	}
	if decision.Severity != 95 {
		t.Errorf("expected severity 95 (Owner from DB), got %d", decision.Severity)
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
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
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

	decision := runner.decisions[0]
	// Owner at subscription scope = 95
	if decision.Severity != 95 {
		t.Errorf("expected severity 95 (Owner at subscription), got %d", decision.Severity)
	}
	// before_score should be 0 for new assignments
	if decision.Metadata["before_score"] != 0 {
		t.Errorf("expected before_score 0, got %v", decision.Metadata["before_score"])
	}
}

func TestPrivilegeEscalation_GetResourceChangesError(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)
	runner := &mockRunner{err: errors.New("test error")}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPrivilegeEscalation_EmitDecisionError(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
		emitErr: errors.New("emit error"),
	}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPrivilegeEscalation_Name(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	if analyzer.Name() != "privilege-escalation" {
		t.Errorf("expected name 'privilege-escalation', got %q", analyzer.Name())
	}
}

func TestPrivilegeEscalation_RoleRemoval_NoDecision(t *testing.T) {
	// De-escalation (role removal) is no longer detected (CR-0024)
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"delete"},
				Before: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: nil,
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// De-escalation is no longer detected (CR-0024)
	if len(runner.decisions) != 0 {
		t.Fatalf("expected 0 decisions for role removal (de-escalation no longer detected), got %d", len(runner.decisions))
	}
}

func TestGraduatedSeverity_OwnerVsContributor(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Test Owner escalation
	ownerRunner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.owner",
				Type:    "azurerm_role_assignment",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(ownerRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test Contributor escalation
	contribRunner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.contributor",
				Type:    "azurerm_role_assignment",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: map[string]interface{}{
					"role_definition_name": "Contributor",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err = analyzer.Analyze(contribRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ownerSeverity := ownerRunner.decisions[0].Severity
	contribSeverity := contribRunner.decisions[0].Severity

	// Owner should score higher than Contributor
	if ownerSeverity <= contribSeverity {
		t.Errorf("Owner severity (%d) should be > Contributor severity (%d)", ownerSeverity, contribSeverity)
	}

	// Verify expected values
	if ownerSeverity != 95 {
		t.Errorf("Owner severity = %d, want 95", ownerSeverity)
	}
	if contribSeverity != 70 {
		t.Errorf("Contributor severity = %d, want 70", contribSeverity)
	}
}

func TestScopeWeighting_SubVsRG(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Test at subscription scope
	subRunner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.sub",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Contributor",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(subRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test at resource group scope
	rgRunner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.rg",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Contributor",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-rg",
				},
			},
		},
	}

	err = analyzer.Analyze(rgRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	subSeverity := subRunner.decisions[0].Severity
	rgSeverity := rgRunner.decisions[0].Severity

	// Subscription should score higher than resource group
	if subSeverity <= rgSeverity {
		t.Errorf("Subscription severity (%d) should be > RG severity (%d)", subSeverity, rgSeverity)
	}

	// Contributor base is 70, subscription 1.0x = 70, RG 0.8x = 56
	if subSeverity != 70 {
		t.Errorf("Subscription severity = %d, want 70", subSeverity)
	}
	if rgSeverity != 56 {
		t.Errorf("RG severity = %d, want 56", rgSeverity)
	}
}

func TestScopeWeighting_ManagementGroup(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.mg",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/providers/Microsoft.Management/managementGroups/my-mg",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Owner at management group: 95 * 1.1 = 104.5 -> clamped to 100
	if runner.decisions[0].Severity != 100 {
		t.Errorf("Owner at mgmt group severity = %d, want 100 (clamped)", runner.decisions[0].Severity)
	}
}

func TestScopeWeighting_Resource(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.resource",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Owner at resource: 95 * 0.6 = 57
	if runner.decisions[0].Severity != 57 {
		t.Errorf("Owner at resource severity = %d, want 57", runner.decisions[0].Severity)
	}
}

func TestCustomRoleCrossReference(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Mock runner with both role definition and role assignment
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			// Custom role definition in the plan
			{
				Address: "azurerm_role_definition.deployer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Deployer",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":          []interface{}{"Microsoft.Authorization/roleAssignments/write"},
							"not_actions":      []interface{}{},
							"data_actions":     []interface{}{},
							"not_data_actions": []interface{}{},
						},
					},
				},
			},
			// Role assignment using the custom role
			{
				Address: "azurerm_role_assignment.deployer",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Custom Deployer",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
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
	if decision.Metadata["role_source"] != "plan-custom-role" {
		t.Errorf("expected role_source plan-custom-role, got %v", decision.Metadata["role_source"])
	}
	// Custom role with roleAssignments/write scores ~75
	if decision.Severity != 75 {
		t.Errorf("Custom Deployer severity = %d, want 75", decision.Severity)
	}
}

func TestCustomRoleCrossReference_Disabled(t *testing.T) {
	config := &PluginConfig{
		PrivilegedRoles:           []string{},
		PrivilegeEnabled:          true,
		RoleDatabase:              DefaultRoleDatabase(),
		UnknownPrivilegedSeverity: 80,
		UnknownRoleSeverity:       50,
		CrossReferenceCustomRoles: false, // Disabled
	}
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.deployer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Deployer",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions": []interface{}{"Microsoft.Authorization/roleAssignments/write"},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.deployer",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Custom Deployer",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
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

	// With cross-reference disabled, should fall through to unknown
	decision := runner.decisions[0]
	if decision.Metadata["role_source"] != "unknown" {
		t.Errorf("expected role_source unknown (cross-ref disabled), got %v", decision.Metadata["role_source"])
	}
	if decision.Severity != 50 {
		t.Errorf("expected severity 50 (unknown), got %d", decision.Severity)
	}
}

func TestCustomRoleCrossReference_WildcardActions(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.super",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Super Admin",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":          []interface{}{"*"},
							"not_actions":      []interface{}{},
							"data_actions":     []interface{}{},
							"not_data_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.super",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Super Admin",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
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
	// Custom role with wildcard actions scores 95 (like Owner)
	if decision.Severity != 95 {
		t.Errorf("Custom wildcard role severity = %d, want 95", decision.Severity)
	}
}

func TestUnknownRole_NotInDB(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Totally Unknown Role",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
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
	if decision.Severity != 50 {
		t.Errorf("Unknown role severity = %d, want 50", decision.Severity)
	}
	if decision.Metadata["role_source"] != "unknown" {
		t.Errorf("expected role_source unknown, got %v", decision.Metadata["role_source"])
	}
}

func TestUnknownRole_ConfiguredSeverity(t *testing.T) {
	config := &PluginConfig{
		PrivilegedRoles:           []string{},
		PrivilegeEnabled:          true,
		RoleDatabase:              DefaultRoleDatabase(),
		UnknownPrivilegedSeverity: 80,
		UnknownRoleSeverity:       60, // Custom severity
		CrossReferenceCustomRoles: true,
	}
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Totally Unknown Role",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runner.decisions[0].Severity != 60 {
		t.Errorf("Unknown role severity = %d, want 60", runner.decisions[0].Severity)
	}
}

func TestRichMetadata(t *testing.T) {
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
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	meta := runner.decisions[0].Metadata

	// Check all required metadata fields
	requiredFields := []string{
		"before_score", "after_score", "scope", "scope_level",
		"score_factors", "role_source", "analyzer", "direction",
		"before_role", "after_role",
	}

	for _, field := range requiredFields {
		if meta[field] == nil {
			t.Errorf("missing required metadata field: %s", field)
		}
	}
}

func TestRoleResolvedByID(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_id": "/providers/Microsoft.Authorization/roleDefinitions/8e3af657-a8ff-443c-a75c-2fe8c4bcb635", // Owner GUID
					"scope":              "/subscriptions/00000000-0000-0000-0000-000000000000",
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
	// Should resolve Owner by ID
	if decision.Metadata["role_source"] != "builtin" {
		t.Errorf("expected role_source builtin, got %v", decision.Metadata["role_source"])
	}
	if decision.Severity != 95 {
		t.Errorf("Owner (by ID) severity = %d, want 95", decision.Severity)
	}
}

func TestDefaultConfig_NewFields(t *testing.T) {
	config := DefaultConfig()

	if config.RoleDatabase == nil {
		t.Error("RoleDatabase should not be nil")
	}
	if config.UnknownPrivilegedSeverity != 80 {
		t.Errorf("UnknownPrivilegedSeverity = %d, want 80", config.UnknownPrivilegedSeverity)
	}
	if config.UnknownRoleSeverity != 50 {
		t.Errorf("UnknownRoleSeverity = %d, want 50", config.UnknownRoleSeverity)
	}
	if !config.CrossReferenceCustomRoles {
		t.Error("CrossReferenceCustomRoles should be true by default")
	}
}

func TestCustomRoleCrossReference_RoleDefinitionError(t *testing.T) {
	// Mock runner that returns error for role_definition queries
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunnerWithRoleDefError{
		mockRunner: mockRunner{
			changes: []*sdk.ResourceChange{
				{
					Address: "azurerm_role_assignment.test",
					Type:    "azurerm_role_assignment",
					Actions: []string{"create"},
					After: map[string]interface{}{
						"role_definition_name": "Custom Role",
						"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					},
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still work and fall back to unknown role
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["role_source"] != "unknown" {
		t.Errorf("expected role_source unknown (role def error), got %v", runner.decisions[0].Metadata["role_source"])
	}
}

// mockRunnerWithRoleDefError returns error when querying role_definition resources
type mockRunnerWithRoleDefError struct {
	mockRunner
}

func (r *mockRunnerWithRoleDefError) GetResourceChanges(patterns []string) ([]*sdk.ResourceChange, error) {
	for _, p := range patterns {
		if p == "azurerm_role_definition" {
			return nil, errors.New("mock error for role_definition")
		}
	}
	return r.mockRunner.GetResourceChanges(patterns)
}

func TestCustomRoleCrossReference_NoAfterOrBeforeState(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			// Role definition with nil states
			{
				Address: "azurerm_role_definition.orphan",
				Type:    "azurerm_role_definition",
				Actions: []string{"delete"},
				Before:  nil,
				After:   nil,
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Orphan Role",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still work - orphan role def is skipped, assignment uses unknown
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
}

func TestCustomRoleCrossReference_EmptyName(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			// Role definition with empty name
			{
				Address: "azurerm_role_definition.empty",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions": []interface{}{"*"},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Some Role",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should work - empty name is skipped
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
}

func TestCustomRoleCrossReference_NoPermissions(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			// Role definition with no permissions field
			{
				Address: "azurerm_role_definition.noperms",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "No Perms Role",
					// Missing permissions field
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "No Perms Role",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should work - role with no permissions is skipped, assignment uses unknown
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["role_source"] != "unknown" {
		t.Errorf("expected role_source unknown, got %v", runner.decisions[0].Metadata["role_source"])
	}
}

func TestCustomRoleCrossReference_InvalidPermissionsType(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			// Role definition with permissions as string (invalid type)
			{
				Address: "azurerm_role_definition.badtype",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name":        "Bad Type Role",
					"permissions": "not-a-list",
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Bad Type Role",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should work - invalid type is handled gracefully
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["role_source"] != "unknown" {
		t.Errorf("expected role_source unknown, got %v", runner.decisions[0].Metadata["role_source"])
	}
}

func TestCustomRoleCrossReference_InvalidPermissionBlockType(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			// Role definition with permission block as string (invalid)
			{
				Address: "azurerm_role_definition.badblock",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Bad Block Role",
					"permissions": []interface{}{
						"not-a-map", // Invalid - should be map[string]interface{}
					},
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Bad Block Role",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should work - invalid block type is handled gracefully
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["role_source"] != "unknown" {
		t.Errorf("expected role_source unknown, got %v", runner.decisions[0].Metadata["role_source"])
	}
}

func TestCustomRoleCrossReference_UsesBeforeStateForDelete(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			// Role definition being deleted - should use Before state
			{
				Address: "azurerm_role_definition.deleting",
				Type:    "azurerm_role_definition",
				Actions: []string{"delete"},
				Before: map[string]interface{}{
					"name": "Deleting Role",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions": []interface{}{"Microsoft.Authorization/roleAssignments/write"},
						},
					},
				},
				After: nil,
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Deleting Role",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use the Before state for the deleted role definition
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	// Should have found the custom role from Before state
	if runner.decisions[0].Metadata["role_source"] != "plan-custom-role" {
		t.Errorf("expected role_source plan-custom-role, got %v", runner.decisions[0].Metadata["role_source"])
	}
}

func TestResolveRole_OnlyRoleDefinitionID(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					// Only ID, no name - should use ID for lookup and display
					"role_definition_id": "8e3af657-a8ff-443c-a75c-2fe8c4bcb635", // Owner
					"scope":              "/subscriptions/00000000-0000-0000-0000-000000000000",
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

	// Should resolve by ID
	if runner.decisions[0].Metadata["role_source"] != "builtin" {
		t.Errorf("expected role_source builtin, got %v", runner.decisions[0].Metadata["role_source"])
	}
	if runner.decisions[0].Severity != 95 {
		t.Errorf("expected severity 95 (Owner), got %d", runner.decisions[0].Severity)
	}
}

func TestResolveRole_UnknownIDOnly(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					// Unknown ID, no name
					"role_definition_id": "/providers/Microsoft.Authorization/roleDefinitions/00000000-0000-0000-0000-000000000000",
					"scope":              "/subscriptions/00000000-0000-0000-0000-000000000000",
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

	// Should fall back to unknown, using ID as the name
	if runner.decisions[0].Metadata["role_source"] != "unknown" {
		t.Errorf("expected role_source unknown, got %v", runner.decisions[0].Metadata["role_source"])
	}
	if runner.decisions[0].Metadata["after_role"] != "/providers/Microsoft.Authorization/roleDefinitions/00000000-0000-0000-0000-000000000000" {
		t.Errorf("expected after_role to be the ID, got %v", runner.decisions[0].Metadata["after_role"])
	}
}

func TestScopeFromBeforeState(t *testing.T) {
	// Test that scope is taken from Before when After has no scope
	// This tests escalation where scope comes from Before
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Test an escalation scenario where After is new role at the same scope
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"update"},
				Before: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					// No scope in After - should fall back to Before
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

	// Should use scope from Before state
	if runner.decisions[0].Metadata["scope"] != "/subscriptions/00000000-0000-0000-0000-000000000000" {
		t.Errorf("expected scope from before state, got %v", runner.decisions[0].Metadata["scope"])
	}
}

func TestResolveRole_IDWithNameOverride(t *testing.T) {
	// Test the case where we resolve by ID but also have a roleName provided,
	// which should prefer the roleName for display
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					// Both ID and name - but name doesn't match a known role
					// ID does match Owner, but we should use the provided name for display
					"role_definition_id":   "8e3af657-a8ff-443c-a75c-2fe8c4bcb635", // Owner
					"role_definition_name": "My Custom Owner Alias",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
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

	// The name lookup fails, but ID lookup succeeds - the name in assignment
	// should be preserved for display
	if runner.decisions[0].Metadata["role_source"] != "builtin" {
		t.Errorf("expected role_source builtin, got %v", runner.decisions[0].Metadata["role_source"])
	}
	// Should use the provided name (not "Owner" from the database)
	if runner.decisions[0].Metadata["after_role"] != "My Custom Owner Alias" {
		t.Errorf("expected after_role 'My Custom Owner Alias', got %v", runner.decisions[0].Metadata["after_role"])
	}
	// Should still have Owner's score (95)
	if runner.decisions[0].Severity != 95 {
		t.Errorf("expected severity 95 (Owner), got %d", runner.decisions[0].Severity)
	}
}

func TestResolveRole_NoRoleIdentifiers(t *testing.T) {
	// Test when neither roleName nor roleID is specified
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.before",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				Before:  nil,
				After: map[string]interface{}{
					// Scope only, no role - edge case
					"scope": "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No decision should be emitted because score is 0 for no role
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (no role specified), got %d", len(runner.decisions))
	}
}

func TestAnalyze_NilRoleDatabase(t *testing.T) {
	// Test when config has nil RoleDatabase (should use DefaultRoleDatabase)
	config := &PluginConfig{
		PrivilegeEnabled:          true,
		RoleDatabase:              nil, // Explicitly nil
		UnknownPrivilegedSeverity: 80,
		UnknownRoleSeverity:       50,
		CrossReferenceCustomRoles: true,
	}
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still work - falls back to DefaultRoleDatabase
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["role_source"] != "builtin" {
		t.Errorf("expected role_source builtin, got %v", runner.decisions[0].Metadata["role_source"])
	}
	if runner.decisions[0].Severity != 95 {
		t.Errorf("expected severity 95 (Owner), got %d", runner.decisions[0].Severity)
	}
}

func TestAnalyze_EmitDecisionError(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunnerWithEmitError{
		mockRunner: mockRunner{
			changes: []*sdk.ResourceChange{
				{
					Address: "azurerm_role_assignment.test",
					Type:    "azurerm_role_assignment",
					Actions: []string{"create"},
					After: map[string]interface{}{
						"role_definition_name": "Owner",
						"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					},
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error from EmitDecision, got nil")
	}
	if !errors.Is(err, errMockEmit) {
		t.Errorf("expected errMockEmit, got %v", err)
	}
}

var errMockEmit = errors.New("mock emit decision error")

type mockRunnerWithEmitError struct {
	mockRunner
}

func (r *mockRunnerWithEmitError) EmitDecision(analyzer sdk.Analyzer, change *sdk.ResourceChange, decision *sdk.Decision) error {
	return errMockEmit
}

func TestAnalyze_NoDeescalation(t *testing.T) {
	// De-escalation is no longer detected (CR-0024)
	// This test verifies that no decision is emitted for de-escalation
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
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
				After: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// De-escalation is no longer detected (CR-0024)
	if len(runner.decisions) != 0 {
		t.Fatalf("expected 0 decisions for de-escalation (no longer detected), got %d", len(runner.decisions))
	}
}
