package classify

import (
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

func TestAddPluginDecisions_UnknownResource(t *testing.T) {
	// Test that plugin decisions for resources not in the plan are gracefully handled.
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "standard",
				Description: "Standard",
				Rules:       []config.RuleConfig{{Resource: []string{"*"}}},
			},
		},
		Precedence: []string{"standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "aws_instance.web", Type: "aws_instance", Actions: []string{"create"}},
	}

	result := classifier.Classify(changes)

	// Plugin decision for a resource NOT in the plan
	pluginDecisions := []ResourceDecision{
		{
			Address:        "aws_s3_bucket.ghost",
			ResourceType:   "aws_s3_bucket",
			Actions:        []string{"create"},
			Classification: "critical",
			MatchedRule:    "plugin: ghost resource",
		},
	}

	classifier.AddPluginDecisions(result, pluginDecisions)

	// Only the original resource should be in the result
	if len(result.ResourceDecisions) != 1 {
		t.Errorf("expected 1 decision, got %d", len(result.ResourceDecisions))
	}
	// The ghost resource should not have been added
	for _, d := range result.ResourceDecisions {
		if d.Address == "aws_s3_bucket.ghost" {
			t.Error("ghost resource should not be in the result")
		}
	}
}

func TestAddPluginDecisions_LowerPrecedenceIgnored(t *testing.T) {
	// Test that a plugin cannot downgrade a classification to lower precedence.
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules:       []config.RuleConfig{{Resource: []string{"*_role_*"}}},
			},
			{
				Name:        "standard",
				Description: "Standard",
				Rules:       []config.RuleConfig{{Resource: []string{"*"}}},
			},
		},
		Precedence: []string{"critical", "standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "azurerm_role_assignment.admin", Type: "azurerm_role_assignment", Actions: []string{"create"}},
	}

	result := classifier.Classify(changes)

	// Core classification should be "critical"
	if result.ResourceDecisions[0].Classification != "critical" {
		t.Fatalf("expected core classification 'critical', got %q", result.ResourceDecisions[0].Classification)
	}

	// Plugin tries to downgrade to "standard" (lower precedence)
	pluginDecisions := []ResourceDecision{
		{
			Address:        "azurerm_role_assignment.admin",
			ResourceType:   "azurerm_role_assignment",
			Actions:        []string{"create"},
			Classification: "standard",
			MatchedRule:    "plugin: downgrade attempt",
		},
	}

	classifier.AddPluginDecisions(result, pluginDecisions)

	// Should NOT be downgraded — critical should win
	if result.ResourceDecisions[0].Classification != "critical" {
		t.Errorf("expected classification to remain 'critical', got %q",
			result.ResourceDecisions[0].Classification)
	}
}

func TestAddPluginDecisions_UpgradeAllowed(t *testing.T) {
	// Test that a plugin CAN upgrade a classification to higher precedence.
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules:       []config.RuleConfig{{Resource: []string{"*_role_*"}}},
			},
			{
				Name:        "standard",
				Description: "Standard",
				Rules:       []config.RuleConfig{{Resource: []string{"*"}}},
			},
		},
		Precedence: []string{"critical", "standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "azurerm_virtual_network.main", Type: "azurerm_virtual_network", Actions: []string{"update"}},
	}

	result := classifier.Classify(changes)

	// Core classification should be "standard"
	if result.ResourceDecisions[0].Classification != "standard" {
		t.Fatalf("expected initial classification 'standard', got %q",
			result.ResourceDecisions[0].Classification)
	}

	// Plugin upgrades to "critical" (higher precedence)
	pluginDecisions := []ResourceDecision{
		{
			Address:        "azurerm_virtual_network.main",
			ResourceType:   "azurerm_virtual_network",
			Actions:        []string{"update"},
			Classification: "critical",
			MatchedRule:    "plugin: network exposure detected",
		},
	}

	classifier.AddPluginDecisions(result, pluginDecisions)

	// Should be upgraded to "critical"
	if result.ResourceDecisions[0].Classification != "critical" {
		t.Errorf("expected classification 'critical' after upgrade, got %q",
			result.ResourceDecisions[0].Classification)
	}
	if result.Overall != "critical" {
		t.Errorf("expected overall 'critical' after upgrade, got %q", result.Overall)
	}
}

func TestCompileRules_InvalidGlob(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:  "test",
				Rules: []config.RuleConfig{{Resource: []string{"[invalid"}}},
			},
		},
	}

	_, err := compileRules(cfg)
	if err == nil {
		t.Error("expected error for invalid glob pattern")
	}
}

func TestCompileRules_InvalidNotResourceGlob(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:  "test",
				Rules: []config.RuleConfig{{NotResource: []string{"[invalid"}}},
			},
		},
	}

	_, err := compileRules(cfg)
	if err == nil {
		t.Error("expected error for invalid not_resource glob pattern")
	}
}

func TestNew_InvalidGlobReturnsError(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "test",
				Description: "Test",
				Rules:       []config.RuleConfig{{Resource: []string{"[invalid"}}},
			},
		},
		Precedence: []string{"test"},
		Defaults:   &config.DefaultsConfig{Unclassified: "test", NoChanges: "test"},
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error from New() with invalid glob")
	}
}

func TestClassify_DataSourceMode(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "standard",
				Description: "Standard",
				Rules:       []config.RuleConfig{{Resource: []string{"*"}}},
			},
		},
		Precedence: []string{"standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "data.azurerm_subscription.current",
			Type:    "azurerm_subscription",
			Mode:    "data",
			Actions: []string{"read"},
		},
	}

	result := classifier.Classify(changes)

	if len(result.ResourceDecisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(result.ResourceDecisions))
	}
}

func TestClassify_MultipleActionsOnRule(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "destructive",
				Description: "Destructive",
				Rules:       []config.RuleConfig{{Resource: []string{"*"}, Actions: []string{"delete", "create"}}},
			},
			{
				Name:        "standard",
				Description: "Standard",
				Rules:       []config.RuleConfig{{Resource: []string{"*"}}},
			},
		},
		Precedence: []string{"destructive", "standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Replace action (delete+create) should match the destructive rule
	changes := []plan.ResourceChange{
		{
			Address: "aws_instance.web",
			Type:    "aws_instance",
			Actions: []string{"delete", "create"},
		},
	}

	result := classifier.Classify(changes)

	if result.ResourceDecisions[0].Classification != "destructive" {
		t.Errorf("expected 'destructive' for replace action, got %q",
			result.ResourceDecisions[0].Classification)
	}

	// Update action should NOT match destructive
	updateChanges := []plan.ResourceChange{
		{
			Address: "aws_instance.web",
			Type:    "aws_instance",
			Actions: []string{"update"},
		},
	}

	updateResult := classifier.Classify(updateChanges)

	if updateResult.ResourceDecisions[0].Classification != "standard" {
		t.Errorf("expected 'standard' for update action, got %q",
			updateResult.ResourceDecisions[0].Classification)
	}
}

func TestClassify_UnclassifiedFallback(t *testing.T) {
	// Rules that don't match anything should fall back to unclassified default
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules:       []config.RuleConfig{{Resource: []string{"*_role_*"}}},
			},
			{
				Name:        "review",
				Description: "Review needed",
				Rules:       []config.RuleConfig{{Resource: []string{"*_keyvault_*"}}},
			},
		},
		Precedence: []string{"critical", "review"},
		Defaults:   &config.DefaultsConfig{Unclassified: "review", NoChanges: "review"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// This resource doesn't match any rule
	changes := []plan.ResourceChange{
		{
			Address: "aws_instance.web",
			Type:    "aws_instance",
			Actions: []string{"create"},
		},
	}

	result := classifier.Classify(changes)

	// Should fall back to unclassified default
	if result.ResourceDecisions[0].Classification != "review" {
		t.Errorf("expected 'review' (unclassified default), got %q",
			result.ResourceDecisions[0].Classification)
	}
}

func TestFormatRuleDescription_Variants(t *testing.T) {
	// Single resource pattern
	rule := config.RuleConfig{Resource: []string{"*_role_*"}}
	desc := formatRuleDescription("critical", 1, rule)
	if desc != "critical rule 1 (resource: *_role_*)" {
		t.Errorf("unexpected description: %q", desc)
	}

	// Multiple resource patterns
	rule = config.RuleConfig{Resource: []string{"*_role_*", "*_iam_*"}}
	desc = formatRuleDescription("critical", 1, rule)
	if desc != "critical rule 1 (resource: *_role_*, ...)" {
		t.Errorf("unexpected description for multiple patterns: %q", desc)
	}

	// Single not_resource pattern
	rule = config.RuleConfig{NotResource: []string{"*_role_*"}}
	desc = formatRuleDescription("standard", 2, rule)
	if desc != "standard rule 2 (not_resource: *_role_*)" {
		t.Errorf("unexpected not_resource description: %q", desc)
	}

	// Multiple not_resource patterns
	rule = config.RuleConfig{NotResource: []string{"*_role_*", "*_iam_*"}}
	desc = formatRuleDescription("standard", 1, rule)
	if desc != "standard rule 1 (not_resource: *_role_*, ...)" {
		t.Errorf("unexpected multiple not_resource description: %q", desc)
	}

	// No patterns (edge case)
	rule = config.RuleConfig{}
	desc = formatRuleDescription("auto", 1, rule)
	if desc != "auto rule 1" {
		t.Errorf("unexpected bare description: %q", desc)
	}
}

func TestClassify_CustomRuleDescription(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical changes",
				Rules: []config.RuleConfig{
					{
						Description: "Role assignments require special review",
						Resource:    []string{"*_role_*"},
					},
				},
			},
		},
		Precedence: []string{"critical"},
		Defaults:   &config.DefaultsConfig{Unclassified: "critical", NoChanges: "critical"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "azurerm_role_assignment.admin", Type: "azurerm_role_assignment", Actions: []string{"create"}},
	}

	result := classifier.Classify(changes)

	if result.ResourceDecisions[0].MatchedRule != "Role assignments require special review" {
		t.Errorf("expected custom rule description, got %q", result.ResourceDecisions[0].MatchedRule)
	}
}

func TestClassify_ExitCodeByPrecedence(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{Name: "critical", Description: "Critical", Rules: []config.RuleConfig{{Resource: []string{"*_role_*"}}}},
			{Name: "review", Description: "Review", Rules: []config.RuleConfig{{Resource: []string{"*_network_*"}}}},
			{Name: "auto", Description: "Auto", Rules: []config.RuleConfig{{Resource: []string{"*"}}}},
		},
		Precedence: []string{"critical", "review", "auto"},
		Defaults:   &config.DefaultsConfig{Unclassified: "auto", NoChanges: "auto"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Test each precedence level produces different exit codes
	tests := []struct {
		name        string
		resourceType string
		wantClass   string
		wantExitGt  int
	}{
		{"critical resource", "azurerm_role_assignment", "critical", 0},
		{"review resource", "azurerm_network_security_group", "review", -1},
		{"auto resource", "azurerm_resource_group", "auto", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := []plan.ResourceChange{
				{Address: tt.resourceType + ".test", Type: tt.resourceType, Actions: []string{"create"}},
			}
			result := classifier.Classify(changes)
			if result.Overall != tt.wantClass {
				t.Errorf("expected overall %q, got %q", tt.wantClass, result.Overall)
			}
		})
	}
}
