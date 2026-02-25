package main

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

// === Pattern-based detection tests (CR-0028) ===

func TestPrivilege_PatternDetection_ControlPlane(t *testing.T) {
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
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Classification != "critical" {
		t.Errorf("expected classification 'critical', got %q", decision.Classification)
	}
	if decision.Metadata["trigger"] != "control-plane" {
		t.Errorf("expected trigger control-plane, got %v", decision.Metadata["trigger"])
	}
	if decision.Metadata["role_source"] != "builtin" {
		t.Errorf("expected role_source builtin, got %v", decision.Metadata["role_source"])
	}
}

func TestPrivilege_PatternDetection_DataPlane(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Create a custom role with data actions via plan cross-reference
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.data",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Storage Data Reader",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":      []interface{}{},
							"not_actions":  []interface{}{},
							"data_actions": []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read"},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Storage Data Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			DataActions: []string{"Microsoft.Storage/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["trigger"] != "data-plane" {
		t.Errorf("expected trigger data-plane, got %v", decision.Metadata["trigger"])
	}
}

func TestPrivilege_PatternDetection_BothPlanes(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Create a custom role with both control-plane and data-plane actions
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.both",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Full Access",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":      []interface{}{"Microsoft.Authorization/roleAssignments/write"},
							"not_actions":  []interface{}{},
							"data_actions": []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read"},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Full Access",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions:     []string{"Microsoft.Authorization/*"},
			DataActions: []string{"Microsoft.Storage/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["trigger"] != "both" {
		t.Errorf("expected trigger both, got %v", decision.Metadata["trigger"])
	}
}

func TestPrivilege_PatternDetection_NoMatch(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	// Reader has only */read actions - should not match Microsoft.Authorization/* write patterns
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/roleAssignments/write"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (Reader doesn't have auth write), got %d", len(runner.decisions))
	}
}

func TestPrivilege_PatternDetection_ContributorNotActionsSubtraction(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Contributor",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	// Contributor has Actions: ["*"], NotActions: ["Microsoft.Authorization/*/Delete", "Microsoft.Authorization/*/Write", "Microsoft.Authorization/elevateAccess/Action"]
	// After expansion: Contributor retains Microsoft.Authorization/*/read actions
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/roleAssignments/write"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contributor's NotActions block write, so this should NOT trigger
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (Contributor NotActions blocks write), got %d", len(runner.decisions))
	}
}

// === No patterns = no decisions (backward compat) ===

func TestPrivilege_NoPatternsConfigured_NoDecision(t *testing.T) {
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
	}

	// Analyze() with no config: no actions/data_actions = no pattern detection
	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without action patterns, no pattern-based decisions are emitted
	// Unknown role flagging doesn't apply because Owner resolves from builtin
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (no patterns configured), got %d", len(runner.decisions))
	}
}

// === Unknown role handling (CR-0028) ===

func TestPrivilege_UnknownRole_FlagEnabled(t *testing.T) {
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

	// flag_unknown_roles defaults to true (nil = true)
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision (unknown role flagged), got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["trigger"] != "unknown-role" {
		t.Errorf("expected trigger unknown-role, got %v", decision.Metadata["trigger"])
	}
	if decision.Metadata["role_name"] != "Totally Unknown Role" {
		t.Errorf("expected role_name 'Totally Unknown Role', got %v", decision.Metadata["role_name"])
	}
	// Should have resolution_attempts
	attempts, ok := decision.Metadata["resolution_attempts"].([]string)
	if !ok {
		t.Errorf("expected resolution_attempts to be []string, got %T", decision.Metadata["resolution_attempts"])
	} else if len(attempts) == 0 {
		t.Error("expected non-empty resolution_attempts")
	}
}

func TestPrivilege_UnknownRole_FlagDisabled(t *testing.T) {
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

	flagFalse := false
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions:          []string{"Microsoft.Authorization/*"},
			FlagUnknownRoles: &flagFalse,
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (flag_unknown_roles=false), got %d", len(runner.decisions))
	}
}

// === Scope filter tests (CR-0028) ===

func TestPrivilege_ScopeFilter(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	tests := []struct {
		name           string
		scope          string
		scopeFilter    []string
		expectDecision bool
	}{
		{
			name:           "subscription in filter - should trigger",
			scope:          "/subscriptions/00000000-0000-0000-0000-000000000000",
			scopeFilter:    []string{"subscription"},
			expectDecision: true,
		},
		{
			name:           "resource_group not in filter - should not trigger",
			scope:          "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg",
			scopeFilter:    []string{"subscription", "management_group"},
			expectDecision: false,
		},
		{
			name:           "management_group in filter - should trigger",
			scope:          "/providers/Microsoft.Management/managementGroups/my-mg",
			scopeFilter:    []string{"management_group"},
			expectDecision: true,
		},
		{
			name:           "empty filter - should trigger (matches any)",
			scope:          "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg",
			scopeFilter:    []string{},
			expectDecision: true,
		},
		{
			name:           "resource in filter - should trigger",
			scope:          "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm",
			scopeFilter:    []string{"resource"},
			expectDecision: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{
				changes: []*sdk.ResourceChange{
					{
						Address: "azurerm_role_assignment.test",
						Type:    "azurerm_role_assignment",
						Actions: []string{"create"},
						After: map[string]interface{}{
							"role_definition_name": "Owner",
							"scope":                tt.scope,
						},
					},
				},
			}

			analyzerConfig := &PluginAnalyzerConfig{
				PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
					Actions: []string{"*"},
					Scopes:  tt.scopeFilter,
				},
			}
			configJSON, _ := json.Marshal(analyzerConfig)

			err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectDecision {
				if len(runner.decisions) == 0 {
					t.Fatalf("expected a decision, got none")
				}
			} else {
				if len(runner.decisions) != 0 {
					t.Errorf("expected 0 decisions, got %d", len(runner.decisions))
				}
			}
		})
	}
}

// === Role exclusion tests ===

func TestPrivilege_AnalyzeWithClassification_RoleExclusion(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	tests := []struct {
		name           string
		role           string
		exclude        []string
		expectDecision bool
	}{
		{
			name:           "Owner not excluded - should trigger",
			role:           "Owner",
			exclude:        []string{"AcrPush", "Reader"},
			expectDecision: true,
		},
		{
			name:           "Owner excluded - should not trigger",
			role:           "Owner",
			exclude:        []string{"Owner", "Contributor"},
			expectDecision: false,
		},
		{
			name:           "Owner excluded case-insensitive - should not trigger",
			role:           "Owner",
			exclude:        []string{"OWNER"},
			expectDecision: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{
				changes: []*sdk.ResourceChange{
					{
						Address: "azurerm_role_assignment.test",
						Type:    "azurerm_role_assignment",
						Actions: []string{"create"},
						After: map[string]interface{}{
							"role_definition_name": tt.role,
							"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
						},
					},
				},
			}

			analyzerConfig := &PluginAnalyzerConfig{
				PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
					Actions: []string{"*"},
					Exclude: tt.exclude,
				},
			}
			configJSON, _ := json.Marshal(analyzerConfig)

			err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectDecision {
				if len(runner.decisions) == 0 {
					t.Fatalf("expected a decision, got none")
				}
			} else {
				if len(runner.decisions) != 0 {
					t.Errorf("expected 0 decisions, got %d", len(runner.decisions))
				}
			}
		})
	}
}

// === Roles filter tests ===

func TestPrivilege_AnalyzeWithClassification_RolesFilter(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	tests := []struct {
		name           string
		role           string
		roles          []string
		expectDecision bool
	}{
		{
			name:           "Owner in roles filter - should trigger",
			role:           "Owner",
			roles:          []string{"Owner", "User Access Administrator"},
			expectDecision: true,
		},
		{
			name:           "Contributor not in roles filter - should not trigger",
			role:           "Contributor",
			roles:          []string{"Owner", "User Access Administrator"},
			expectDecision: false,
		},
		{
			name:           "Owner case-insensitive in roles filter - should trigger",
			role:           "Owner",
			roles:          []string{"OWNER"},
			expectDecision: true,
		},
		{
			name:           "Empty roles filter - should trigger (no filter)",
			role:           "Owner",
			roles:          []string{},
			expectDecision: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{
				changes: []*sdk.ResourceChange{
					{
						Address: "azurerm_role_assignment.test",
						Type:    "azurerm_role_assignment",
						Actions: []string{"create"},
						After: map[string]interface{}{
							"role_definition_name": tt.role,
							"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
						},
					},
				},
			}

			analyzerConfig := &PluginAnalyzerConfig{
				PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
					Actions: []string{"*"},
					Roles:   tt.roles,
				},
			}
			configJSON, _ := json.Marshal(analyzerConfig)

			err := analyzer.AnalyzeWithClassification(runner, "standard", configJSON)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectDecision {
				if len(runner.decisions) == 0 {
					t.Fatalf("expected a decision, got none")
				}
				if runner.decisions[0].Classification != "standard" {
					t.Errorf("expected classification 'standard', got %q", runner.decisions[0].Classification)
				}
			} else {
				if len(runner.decisions) != 0 {
					t.Errorf("expected 0 decisions, got %d", len(runner.decisions))
				}
			}
		})
	}
}

// === Custom role cross-reference tests ===

func TestCustomRoleCrossReference(t *testing.T) {
	config := DefaultConfig()
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
							"actions":          []interface{}{"Microsoft.Authorization/roleAssignments/write"},
							"not_actions":      []interface{}{},
							"data_actions":     []interface{}{},
							"not_data_actions": []interface{}{},
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
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
}

func TestCustomRoleCrossReference_Disabled(t *testing.T) {
	config := &PluginConfig{
		PrivilegeEnabled:          true,
		RoleDatabase:              DefaultRoleDatabase(),
		CrossReferenceCustomRoles: false,
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	// With cross-reference disabled, should fall through to unknown
	decision := runner.decisions[0]
	if decision.Metadata["trigger"] != "unknown-role" {
		t.Errorf("expected trigger unknown-role (cross-ref disabled), got %v", decision.Metadata["trigger"])
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
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
}

func TestCustomRoleCrossReference_RoleDefinitionError(t *testing.T) {
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still work and flag as unknown role
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["trigger"] != "unknown-role" {
		t.Errorf("expected trigger unknown-role, got %v", runner.decisions[0].Metadata["trigger"])
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should work - empty name is skipped, assignment flagged as unknown
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
}

func TestCustomRoleCrossReference_NoPermissions(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.noperms",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "No Perms Role",
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should work - role with no permissions is skipped, assignment uses unknown
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["trigger"] != "unknown-role" {
		t.Errorf("expected trigger unknown-role, got %v", runner.decisions[0].Metadata["trigger"])
	}
}

func TestCustomRoleCrossReference_InvalidPermissionsType(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle gracefully
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["trigger"] != "unknown-role" {
		t.Errorf("expected trigger unknown-role, got %v", runner.decisions[0].Metadata["trigger"])
	}
}

func TestCustomRoleCrossReference_InvalidPermissionBlockType(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.badblock",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Bad Block Role",
					"permissions": []interface{}{
						"not-a-map",
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle gracefully
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["trigger"] != "unknown-role" {
		t.Errorf("expected trigger unknown-role, got %v", runner.decisions[0].Metadata["trigger"])
	}
}

func TestCustomRoleCrossReference_UsesBeforeStateForDelete(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["role_source"] != "plan-custom-role" {
		t.Errorf("expected role_source plan-custom-role, got %v", runner.decisions[0].Metadata["role_source"])
	}
}

// === Role resolution tests ===

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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["role_source"] != "builtin" {
		t.Errorf("expected role_source builtin, got %v", decision.Metadata["role_source"])
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
					"role_definition_id": "8e3af657-a8ff-443c-a75c-2fe8c4bcb635", // Owner
					"scope":              "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	if runner.decisions[0].Metadata["role_source"] != "builtin" {
		t.Errorf("expected role_source builtin, got %v", runner.decisions[0].Metadata["role_source"])
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
					"role_definition_id": "/providers/Microsoft.Authorization/roleDefinitions/00000000-0000-0000-0000-000000000000",
					"scope":              "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	if runner.decisions[0].Metadata["trigger"] != "unknown-role" {
		t.Errorf("expected trigger unknown-role, got %v", runner.decisions[0].Metadata["trigger"])
	}
}

func TestResolveRole_IDWithNameOverride(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_id":   "8e3af657-a8ff-443c-a75c-2fe8c4bcb635", // Owner
					"role_definition_name": "My Custom Owner Alias",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	// Name lookup fails, but ID lookup succeeds
	if runner.decisions[0].Metadata["role_source"] != "builtin" {
		t.Errorf("expected role_source builtin, got %v", runner.decisions[0].Metadata["role_source"])
	}
	// Should use the provided name
	if runner.decisions[0].Metadata["after_role"] != "My Custom Owner Alias" {
		t.Errorf("expected after_role 'My Custom Owner Alias', got %v", runner.decisions[0].Metadata["after_role"])
	}
}

func TestResolveRole_NoRoleIdentifiers(t *testing.T) {
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
					"scope": "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No decision: empty role name is treated as unknown with empty name, which is skipped
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (no role specified), got %d", len(runner.decisions))
	}
}

func TestScopeFromBeforeState(t *testing.T) {
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
					// No scope in After - should fall back to Before
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	if runner.decisions[0].Metadata["scope"] != "/subscriptions/00000000-0000-0000-0000-000000000000" {
		t.Errorf("expected scope from before state, got %v", runner.decisions[0].Metadata["scope"])
	}
}

// === Error handling tests ===

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

	runner := &mockRunnerWithEmitError{
		mockRunner: mockRunner{
			changes: []*sdk.ResourceChange{
				{
					Address: "azurerm_role_assignment.test",
					Type:    "azurerm_role_assignment",
					Actions: []string{"create"},
					After: map[string]interface{}{
						"role_definition_name": "Unknown Role",
						"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					},
				},
			},
		},
	}

	// Need actions configured so the unknown role flagging emits a decision
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
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

// === Analyzer name/enabled tests ===

func TestPrivilegeEscalation_Name(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	if analyzer.Name() != "privilege-escalation" {
		t.Errorf("expected name 'privilege-escalation', got %q", analyzer.Name())
	}
}

// === Nil role database test ===

func TestAnalyze_NilRoleDatabase(t *testing.T) {
	config := &PluginConfig{
		PrivilegeEnabled:          true,
		RoleDatabase:              nil,
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

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
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
}

// === Classification tests ===

func TestPrivilege_AnalyzeWithClassification_EmitsClassification(t *testing.T) {
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
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	if runner.decisions[0].Classification != "critical" {
		t.Errorf("expected Classification 'critical', got %q", runner.decisions[0].Classification)
	}
}

// === Combined filter tests ===

func TestPrivilege_AnalyzeWithClassification_CombinedFilters(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			// Owner - passes all filters
			{
				Address: "azurerm_role_assignment.owner",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
			// User Access Administrator - excluded
			{
				Address: "azurerm_role_assignment.uaa",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "User Access Administrator",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
			// Contributor - not in roles filter
			{
				Address: "azurerm_role_assignment.contributor",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Contributor",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Roles:   []string{"Owner", "User Access Administrator"},
			Exclude: []string{"User Access Administrator"},
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only Owner should pass: it's in roles, not excluded, and matches actions
	// UAA is excluded, Contributor is not in roles filter
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	if runner.decisions[0].Metadata["after_role"] != "Owner" {
		t.Errorf("expected Owner, got %v", runner.decisions[0].Metadata["after_role"])
	}
}

// === matchesScopeFilter unit tests ===

func TestMatchesScopeFilter(t *testing.T) {
	tests := []struct {
		name   string
		scope  string
		filter []string
		want   bool
	}{
		{
			name:   "empty filter matches any",
			scope:  "/subscriptions/00000000-0000-0000-0000-000000000000",
			filter: nil,
			want:   true,
		},
		{
			name:   "subscription matches subscription filter",
			scope:  "/subscriptions/00000000-0000-0000-0000-000000000000",
			filter: []string{"subscription"},
			want:   true,
		},
		{
			name:   "subscription does not match resource_group filter",
			scope:  "/subscriptions/00000000-0000-0000-0000-000000000000",
			filter: []string{"resource_group"},
			want:   false,
		},
		{
			name:   "management_group matches case-insensitive",
			scope:  "/providers/Microsoft.Management/managementGroups/mg",
			filter: []string{"MANAGEMENT_GROUP"},
			want:   true,
		},
		{
			name:   "unknown scope does not match any filter",
			scope:  "invalid",
			filter: []string{"subscription", "resource_group"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesScopeFilter(tt.scope, tt.filter)
			if got != tt.want {
				t.Errorf("matchesScopeFilter(%q, %v) = %v, want %v", tt.scope, tt.filter, got, tt.want)
			}
		})
	}
}

// === flagUnknownRolesEnabled unit tests ===

func TestFlagUnknownRolesEnabled(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name string
		cfg  *PrivilegeEscalationAnalyzerConfig
		want bool
	}{
		{"nil config", nil, true},
		{"nil pointer", &PrivilegeEscalationAnalyzerConfig{FlagUnknownRoles: nil}, true},
		{"explicit true", &PrivilegeEscalationAnalyzerConfig{FlagUnknownRoles: &trueVal}, true},
		{"explicit false", &PrivilegeEscalationAnalyzerConfig{FlagUnknownRoles: &falseVal}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flagUnknownRolesEnabled(tt.cfg)
			if got != tt.want {
				t.Errorf("flagUnknownRolesEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// === CR-0027 AC-2: NotDataActions subtraction prevents false positives ===

func TestPrivilege_DataActions_NotDataActionsSubtraction(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Custom role with data actions partially blocked by notDataActions
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.partial_data",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Partial Data Access",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":          []interface{}{},
							"not_actions":      []interface{}{},
							"data_actions":     []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/*"},
							"not_data_actions": []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read"},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Partial Data Access",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	// data_actions = ["*/read"] - should NOT match because notDataActions blocks reads
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			DataActions: []string{"*/read"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// NotDataActions blocks reads, so */read should NOT trigger
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (NotDataActions blocks reads), got %d", len(runner.decisions))
	}
}

// === CR-0027 AC-3: Write-only roles do not match read patterns ===

func TestPrivilege_DataActions_WriteOnlyNotMatchingRead(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Custom role with write-only data actions (no reads at all)
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.write_only",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Write Only Data",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":          []interface{}{},
							"not_actions":      []interface{}{},
							"data_actions":     []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/write", "Microsoft.Storage/storageAccounts/blobServices/containers/blobs/delete"},
							"not_data_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Write Only Data",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	// data_actions = ["*/read"] - should NOT match write-only role
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			DataActions: []string{"*/read"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (write-only role doesn't match */read), got %d", len(runner.decisions))
	}

	// But should match */write
	runner.decisions = nil
	analyzerConfig2 := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			DataActions: []string{"*/write"},
		},
	}
	configJSON2, _ := json.Marshal(analyzerConfig2)

	err = analyzer.AnalyzeWithClassification(runner, "critical", configJSON2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision (write-only matches */write), got %d", len(runner.decisions))
	}
	if runner.decisions[0].Metadata["trigger"] != "data-plane" {
		t.Errorf("expected trigger data-plane, got %v", runner.decisions[0].Metadata["trigger"])
	}
}

// === CR-0027 AC-4: Empty effective data actions match nothing ===

func TestPrivilege_DataActions_EmptyEffective(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Custom role with all data actions neutralized by notDataActions
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.neutralized",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Fully Neutralized Data",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":          []interface{}{},
							"not_actions":      []interface{}{},
							"data_actions":     []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read"},
							"not_data_actions": []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read"},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Fully Neutralized Data",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	// All data actions are neutralized - should match nothing even with wildcard pattern
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			DataActions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (all data actions neutralized by notDataActions), got %d", len(runner.decisions))
	}
}

// === CR-0027 AC-10: Different classifications match different data-plane patterns ===

func TestPrivilege_DataActions_PerClassification(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Role with both read and write data actions
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.readwrite",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Read Write Data",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":          []interface{}{},
							"not_actions":      []interface{}{},
							"data_actions":     []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read", "Microsoft.Storage/storageAccounts/blobServices/containers/blobs/write"},
							"not_data_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Read Write Data",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	// Critical classification: matches reads
	analyzerConfigCritical := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			DataActions: []string{"*/read"},
		},
	}
	configJSONCritical, _ := json.Marshal(analyzerConfigCritical)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSONCritical)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for critical (matches */read), got %d", len(runner.decisions))
	}
	if runner.decisions[0].Classification != "critical" {
		t.Errorf("expected classification 'critical', got %q", runner.decisions[0].Classification)
	}

	// Standard classification: matches writes
	runner.decisions = nil
	analyzerConfigStandard := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			DataActions: []string{"*/write"},
		},
	}
	configJSONStandard, _ := json.Marshal(analyzerConfigStandard)

	err = analyzer.AnalyzeWithClassification(runner, "standard", configJSONStandard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for standard (matches */write), got %d", len(runner.decisions))
	}
	if runner.decisions[0].Classification != "standard" {
		t.Errorf("expected classification 'standard', got %q", runner.decisions[0].Classification)
	}
}

// === CR-0028: Scopes filter applies to data-plane triggers too ===

func TestPrivilege_ScopeFilter_AppliesToDataPlane(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Custom role with data actions
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.data",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Data Reader",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":      []interface{}{},
							"not_actions":  []interface{}{},
							"data_actions": []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read"},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.test",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Data Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-test",
				},
			},
		},
	}

	// scopes = ["subscription"] should filter out resource_group scope assignments
	// even for data-plane triggers
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			DataActions: []string{"*/read"},
			Scopes:      []string{"subscription"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (resource_group scope filtered out), got %d", len(runner.decisions))
	}
}

// === Custom role cross-reference by ID (CR-0028) ===

func TestCustomRoleCrossReference_ByRoleDefinitionID(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Simulates a custom role definition with role_definition_resource_id
	// and a role assignment that references it by role_definition_id (not name)
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.auth_writer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Auth Writer",
					"role_definition_resource_id": "/subscriptions/00000000-0000-0000-0000-000000000000/providers/Microsoft.Authorization/roleDefinitions/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":          []interface{}{"Microsoft.Authorization/roleAssignments/write", "Microsoft.Authorization/roleAssignments/delete"},
							"not_actions":      []interface{}{},
							"data_actions":     []interface{}{},
							"not_data_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.custom",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					// Only role_definition_id, no role_definition_name — mirrors real Terraform behavior
					"role_definition_id": "/subscriptions/00000000-0000-0000-0000-000000000000/providers/Microsoft.Authorization/roleDefinitions/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
					"scope":              "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-test",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision (custom role matched by ID), got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Classification != "critical" {
		t.Errorf("expected classification 'critical', got %q", decision.Classification)
	}
	if decision.Metadata["role_source"] != "plan-custom-role" {
		t.Errorf("expected role_source plan-custom-role, got %v", decision.Metadata["role_source"])
	}
	if decision.Metadata["trigger"] != "control-plane" {
		t.Errorf("expected trigger control-plane, got %v", decision.Metadata["trigger"])
	}
	if decision.Metadata["after_role"] != "Custom Auth Writer" {
		t.Errorf("expected after_role 'Custom Auth Writer', got %v", decision.Metadata["after_role"])
	}
}

func TestCustomRoleCrossReference_UnknownComputedID(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Simulates a create plan where role_definition_id is unknown (computed reference).
	// Terraform puts the key in the map with a nil value for unknown computed attributes.
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.auth_writer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Auth Writer",
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
			{
				Address: "azurerm_role_assignment.custom",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					// role_definition_id is present but nil (unknown computed reference)
					"role_definition_id": nil,
					"scope":              "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-test",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should flag as unknown role because role_definition_id is present but unknown
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision (unknown computed role flagged), got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Classification != "critical" {
		t.Errorf("expected classification 'critical', got %q", decision.Classification)
	}
	if decision.Metadata["trigger"] != "unknown-role" {
		t.Errorf("expected trigger unknown-role, got %v", decision.Metadata["trigger"])
	}
}

func TestCustomRoleCrossReference_UnresolvedWithMatchingCustomRole(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Simulates a create plan where BOTH role_definition_id and role_definition_name
	// are absent from After (they're in after_unknown, not in the After map).
	// This is the real behavior seen in CI: Terraform omits unknown computed fields.
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.auth_writer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					// name is also unknown because it uses random_id
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":          []interface{}{"Microsoft.Authorization/roleAssignments/write", "Microsoft.Authorization/roleAssignments/delete"},
							"not_actions":      []interface{}{},
							"data_actions":     []interface{}{},
							"not_data_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.custom",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					// Only scope is known; role_definition_id and role_definition_name
					// are NOT in After (they're in after_unknown)
					"scope": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-test",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect the custom role definition in the plan and infer the match
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision (inferred custom role match), got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Classification != "critical" {
		t.Errorf("expected classification 'critical', got %q", decision.Classification)
	}
	if decision.Metadata["trigger"] != "unresolved-custom-role" {
		t.Errorf("expected trigger unresolved-custom-role, got %v", decision.Metadata["trigger"])
	}
	if decision.Metadata["role_source"] != "plan-custom-role-inferred" {
		t.Errorf("expected role_source plan-custom-role-inferred, got %v", decision.Metadata["role_source"])
	}
}

func TestCustomRoleCrossReference_UnresolvedNoMatchingCustomRole(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	// Custom role with innocuous permissions that don't match the pattern
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.reader",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Reader",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Resources/subscriptions/read"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.custom",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"scope": "/subscriptions/00000000-0000-0000-0000-000000000000",
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Custom role doesn't match Authorization patterns, so no decision
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (custom role is read-only), got %d", len(runner.decisions))
	}
}

// === CR-0028 AC-9: No severity score on decisions ===

func TestPrivilege_NoSeverityOnDecisions(t *testing.T) {
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
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"*"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if _, hasSeverity := decision.Metadata["severity"]; hasSeverity {
		t.Error("decision should NOT contain a severity field")
	}
	if _, hasScore := decision.Metadata["score"]; hasScore {
		t.Error("decision should NOT contain a score field")
	}
}

// === Combined role aggregation tests (merge_principal_roles) ===

func TestPrivilege_CombinedRoles_MergedEffectivePermissions(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)
	mergeTrue := true

	principalID := "aaaa-bbbb-cccc-dddd"

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.reader",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
			{
				Address: "azurerm_role_definition.auth_writer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Auth Writer",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Authorization/roleAssignments/write"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.auth_writer",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Custom Auth Writer",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
		},
	}

	// With merge_principal_roles, per-role emission is deferred. The combined pass
	// evaluates the union of Reader + Custom Auth Writer effective permissions
	// against the same patterns. Auth Writer has roleAssignments/write → matches.
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions:             []string{"Microsoft.Authorization/roleAssignments/write"},
			MergePrincipalRoles: &mergeTrue,
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 combined decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Classification != "critical" {
		t.Errorf("expected classification 'critical', got %q", decision.Classification)
	}
	if decision.Metadata["trigger"] != "combined-roles" {
		t.Errorf("expected trigger 'combined-roles', got %v", decision.Metadata["trigger"])
	}
	if decision.Metadata["principal_id"] != principalID {
		t.Errorf("expected principal_id %q, got %v", principalID, decision.Metadata["principal_id"])
	}
	// Verify effective_actions is present (the full effective permission set)
	if _, ok := decision.Metadata["effective_actions"]; !ok {
		t.Error("expected effective_actions in metadata")
	}
}

func TestPrivilege_CombinedRoles_NeitherTriggersIndividually(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)
	mergeTrue := true

	principalID := "aaaa-bbbb-cccc-dddd"

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.compute_reader",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Compute Reader",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Compute/virtualMachines/read"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_definition.auth_writer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Auth Writer",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Authorization/roleAssignments/write"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.compute_reader",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Compute Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
			{
				Address: "azurerm_role_assignment.auth_writer",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Custom Auth Writer",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
		},
	}

	// Pattern: Microsoft.Authorization/roleAssignments/delete — neither role has delete
	// But merge_principal_roles evaluates the union against the SAME pattern.
	// The union contains roleAssignments/write but NOT delete, so combined also doesn't match.
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions:             []string{"Microsoft.Authorization/roleAssignments/delete"},
			MergePrincipalRoles: &mergeTrue,
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Neither per-role nor combined matches (neither role has delete)
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (no role has delete), got %d", len(runner.decisions))
	}
}

func TestPrivilege_CombinedRoles_BuiltinRoles_SingleDecisionPerPrincipal(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)
	mergeTrue := true

	principalID := "aaaa-bbbb-cccc-dddd"

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.owner",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Owner",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
			{
				Address: "azurerm_role_assignment.reader",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
		},
	}

	// With merge_principal_roles, both roles are evaluated as a union per principal.
	// Owner has roleAssignments/write → union matches the pattern.
	// Result: one combined-roles decision for the principal.
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions:             []string{"Microsoft.Authorization/roleAssignments/write"},
			MergePrincipalRoles: &mergeTrue,
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Single combined decision per principal (not per role)
	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 combined decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["trigger"] != "combined-roles" {
		t.Errorf("expected trigger 'combined-roles', got %v", decision.Metadata["trigger"])
	}

	// Verify combined_roles lists both roles
	combinedRoles, ok := decision.Metadata["combined_roles"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected combined_roles in metadata, got %T", decision.Metadata["combined_roles"])
	}
	if len(combinedRoles) != 2 {
		t.Errorf("expected 2 combined_roles entries, got %d", len(combinedRoles))
	}
}

func TestPrivilege_CombinedRoles_DifferentPrincipals_NoTrigger(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)
	mergeTrue := true

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.auth_writer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Auth Writer",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Authorization/roleAssignments/write"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.reader",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         "principal-1",
				},
			},
			{
				Address: "azurerm_role_assignment.auth_writer",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Custom Auth Writer",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         "principal-2",
				},
			},
		},
	}

	// Different principals — each has only 1 role, so merge has no candidates
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions:             []string{"Microsoft.Authorization/roleAssignments/delete"},
			MergePrincipalRoles: &mergeTrue,
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (different principals, 1 role each), got %d", len(runner.decisions))
	}
}

func TestPrivilege_CombinedRoles_UnknownPrincipalID_Skipped(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)
	mergeTrue := true

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.auth_writer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Auth Writer",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Authorization/roleAssignments/write"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.reader",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					// No principal_id — unknown/computed
				},
			},
			{
				Address: "azurerm_role_assignment.auth_writer",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Custom Auth Writer",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					// No principal_id — unknown/computed
				},
			},
		},
	}

	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions:             []string{"Microsoft.Authorization/roleAssignments/delete"},
			MergePrincipalRoles: &mergeTrue,
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (unknown principal_id), got %d", len(runner.decisions))
	}
}

func TestPrivilege_CombinedRoles_ThreeRolesEffectivePermissions(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)
	mergeTrue := true

	principalID := "aaaa-bbbb-cccc-dddd"

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.role_a",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Role A",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Compute/virtualMachines/read"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_definition.role_b",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Role B",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Network/virtualNetworks/read"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_definition.role_c",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Role C",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Authorization/roleAssignments/write"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.a",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Role A",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
			{
				Address: "azurerm_role_assignment.b",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Role B",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
			{
				Address: "azurerm_role_assignment.c",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Role C",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
		},
	}

	// With merge_principal_roles, all three roles are evaluated as a union.
	// Role C has roleAssignments/write → union matches. Single combined decision.
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions:             []string{"Microsoft.Authorization/roleAssignments/write"},
			MergePrincipalRoles: &mergeTrue,
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 combined decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["trigger"] != "combined-roles" {
		t.Errorf("expected trigger 'combined-roles', got %v", decision.Metadata["trigger"])
	}

	// Verify all three roles listed in combined_roles
	combinedRoles, ok := decision.Metadata["combined_roles"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected combined_roles in metadata, got %T", decision.Metadata["combined_roles"])
	}
	if len(combinedRoles) != 3 {
		t.Errorf("expected 3 combined_roles entries, got %d", len(combinedRoles))
	}

	// Verify effective_actions includes actions from all three roles
	effectiveActions, ok := decision.Metadata["effective_actions"]
	if !ok {
		t.Fatal("expected effective_actions in metadata")
	}
	actionsList, ok := effectiveActions.([]string)
	if !ok {
		t.Fatalf("expected effective_actions to be []string, got %T", effectiveActions)
	}
	// Should contain actions from all three roles
	if len(actionsList) < 3 {
		t.Errorf("expected at least 3 effective_actions (one per role), got %d: %v", len(actionsList), actionsList)
	}
}

func TestPrivilege_CombinedRoles_MergeDisabled_NoEffect(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	principalID := "aaaa-bbbb-cccc-dddd"

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_assignment.reader",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
			{
				Address: "azurerm_role_assignment.contributor",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Contributor",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
		},
	}

	// merge_principal_roles not set (nil = false) — combined pass skipped
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions: []string{"Microsoft.Authorization/roleAssignments/delete"},
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions (merge disabled, no per-role match), got %d", len(runner.decisions))
	}
}

func TestPrivilege_CombinedRoles_CustomRolesFromPlan(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)
	mergeTrue := true

	principalID := "aaaa-bbbb-cccc-dddd"

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.reader_custom",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Reader",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Resources/subscriptions/read"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_definition.auth_writer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Custom Auth Writer",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":     []interface{}{"Microsoft.Authorization/roleAssignments/write"},
							"not_actions": []interface{}{},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.reader",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Custom Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
			{
				Address: "azurerm_role_assignment.auth_writer",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Custom Auth Writer",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
		},
	}

	// With merge_principal_roles, per-role emission is deferred. The combined pass
	// evaluates both custom roles' union of effective permissions.
	// Custom Auth Writer has roleAssignments/write → union matches.
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			Actions:             []string{"Microsoft.Authorization/roleAssignments/write"},
			MergePrincipalRoles: &mergeTrue,
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 combined decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["trigger"] != "combined-roles" {
		t.Errorf("expected trigger 'combined-roles', got %v", decision.Metadata["trigger"])
	}

	// Verify combined_roles includes both custom roles
	combinedRoles, ok := decision.Metadata["combined_roles"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected combined_roles in metadata, got %T", decision.Metadata["combined_roles"])
	}
	if len(combinedRoles) != 2 {
		t.Errorf("expected 2 combined_roles entries, got %d", len(combinedRoles))
	}
}

func TestPrivilege_CombinedRoles_DataActions(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewPrivilegeEscalationAnalyzer(config)

	principalID := "aaaa-bbbb-cccc-dddd"

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_role_definition.data_reader",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Data Reader",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":      []interface{}{},
							"not_actions":  []interface{}{},
							"data_actions": []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read"},
						},
					},
				},
			},
			{
				Address: "azurerm_role_definition.data_writer",
				Type:    "azurerm_role_definition",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name": "Data Writer",
					"permissions": []interface{}{
						map[string]interface{}{
							"actions":      []interface{}{},
							"not_actions":  []interface{}{},
							"data_actions": []interface{}{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/write"},
						},
					},
				},
			},
			{
				Address: "azurerm_role_assignment.data_reader",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Data Reader",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
			{
				Address: "azurerm_role_assignment.data_writer",
				Type:    "azurerm_role_assignment",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"role_definition_name": "Data Writer",
					"scope":                "/subscriptions/00000000-0000-0000-0000-000000000000",
					"principal_id":         principalID,
				},
			},
		},
	}

	// Neither data role matches individually, but combined they cover blob/*
	mergeTrue := true
	analyzerConfig := &PluginAnalyzerConfig{
		PrivilegeEscalation: &PrivilegeEscalationAnalyzerConfig{
			DataActions:         []string{"Microsoft.Storage/storageAccounts/blobServices/containers/blobs/*"},
			MergePrincipalRoles: &mergeTrue,
		},
	}
	configJSON, _ := json.Marshal(analyzerConfig)

	err := analyzer.AnalyzeWithClassification(runner, "critical", configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 combined decision (data plane), got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Metadata["trigger"] != "combined-roles" {
		t.Errorf("expected trigger 'combined-roles', got %v", decision.Metadata["trigger"])
	}

	// Should have matched_data_actions
	if _, ok := decision.Metadata["matched_data_actions"]; !ok {
		t.Error("expected matched_data_actions in metadata")
	}
}
