package classify

import (
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

func newTestConfig() *config.Config {
	return &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical changes",
				Rules: []config.RuleConfig{
					{Resource: []string{"*_role_*"}, Actions: []string{"delete"}},
				},
			},
			{
				Name:        "standard",
				Description: "Standard changes",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}},
				},
			},
			{
				Name:        "auto",
				Description: "Auto-approved",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}, Actions: []string{"no-op"}},
				},
			},
		},
		Precedence: []string{"critical", "standard", "auto"},
		Defaults: &config.DefaultsConfig{
			Unclassified: "standard",
			NoChanges:    "auto",
		},
	}
}

func TestClassify_SingleResource(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_role_assignment.example",
			Type:    "azurerm_role_assignment",
			Actions: []string{"delete"},
		},
	}

	result := classifier.Classify(changes)

	if result.Overall != "critical" {
		t.Errorf("expected overall classification 'critical', got '%s'", result.Overall)
	}

	if len(result.ResourceDecisions) != 1 {
		t.Fatalf("expected 1 resource decision, got %d", len(result.ResourceDecisions))
	}

	if result.ResourceDecisions[0].Classification != "critical" {
		t.Errorf("expected resource classification 'critical', got '%s'",
			result.ResourceDecisions[0].Classification)
	}
}

func TestClassify_PrecedenceWins(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*_role_*"}},
				},
			},
			{
				Name: "standard",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}},
				},
			},
		},
		Precedence: []string{"critical", "standard"},
		Defaults: &config.DefaultsConfig{
			Unclassified: "standard",
			NoChanges:    "standard",
		},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	// This resource matches both critical (*_role_*) and standard (*)
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_role_assignment.example",
			Type:    "azurerm_role_assignment",
			Actions: []string{"create"},
		},
	}

	result := classifier.Classify(changes)

	// Critical should win because it's first in precedence
	if result.ResourceDecisions[0].Classification != "critical" {
		t.Errorf("expected 'critical' to win due to precedence, got '%s'",
			result.ResourceDecisions[0].Classification)
	}
}

func TestClassify_OverallIsHighest(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_virtual_network.main",
			Type:    "azurerm_virtual_network",
			Actions: []string{"update"},
		},
		{
			Address: "azurerm_role_assignment.example",
			Type:    "azurerm_role_assignment",
			Actions: []string{"delete"},
		},
		{
			Address: "azurerm_resource_group.rg",
			Type:    "azurerm_resource_group",
			Actions: []string{"create"},
		},
	}

	result := classifier.Classify(changes)

	// Overall should be critical (highest precedence among the resources)
	if result.Overall != "critical" {
		t.Errorf("expected overall 'critical', got '%s'", result.Overall)
	}
}

func TestClassify_Unclassified(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*_role_*"}},
				},
			},
		},
		Precedence: []string{"critical"},
		Defaults: &config.DefaultsConfig{
			Unclassified: "standard", // Note: "standard" is not a defined classification, but it's the default
			NoChanges:    "auto",
		},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_virtual_network.main",
			Type:    "azurerm_virtual_network",
			Actions: []string{"update"},
		},
	}

	result := classifier.Classify(changes)

	// Should get the default unclassified value
	if result.ResourceDecisions[0].Classification != "standard" {
		t.Errorf("expected unclassified resource to get 'standard', got '%s'",
			result.ResourceDecisions[0].Classification)
	}
}

func TestClassify_UnclassifiedWithDescription(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*_role_*"}},
				},
			},
			{
				Name:        "standard",
				Description: "Standard changes requiring review",
				Rules:       []config.RuleConfig{}, // No rules - used as fallback
			},
		},
		Precedence: []string{"critical", "standard"},
		Defaults: &config.DefaultsConfig{
			Unclassified: "standard",
			NoChanges:    "auto",
		},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_virtual_network.main",
			Type:    "azurerm_virtual_network",
			Actions: []string{"update"},
		},
	}

	result := classifier.Classify(changes)

	// Should get the default unclassified value with description
	if result.ResourceDecisions[0].Classification != "standard" {
		t.Errorf("expected unclassified resource to get 'standard', got '%s'",
			result.ResourceDecisions[0].Classification)
	}

	if result.ResourceDecisions[0].ClassificationDescription != "Standard changes requiring review" {
		t.Errorf("expected unclassified resource to have description, got '%s'",
			result.ResourceDecisions[0].ClassificationDescription)
	}
}

func TestClassify_NoChanges(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	result := classifier.Classify([]plan.ResourceChange{})

	if !result.NoChanges {
		t.Error("expected NoChanges to be true")
	}

	if result.Overall != "auto" {
		t.Errorf("expected overall 'auto' for no changes, got '%s'", result.Overall)
	}

	if result.OverallExitCode != 0 {
		t.Errorf("expected exit code 0 for no changes, got %d", result.OverallExitCode)
	}
}

func TestClassify_ExitCodes(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:  "critical",
				Rules: []config.RuleConfig{{Resource: []string{"critical_*"}}},
			},
			{
				Name:  "review",
				Rules: []config.RuleConfig{{Resource: []string{"review_*"}}},
			},
			{
				Name:  "standard",
				Rules: []config.RuleConfig{{Resource: []string{"standard_*"}}},
			},
			{
				Name:  "auto",
				Rules: []config.RuleConfig{{Resource: []string{"auto_*"}}},
			},
		},
		Precedence: []string{"critical", "review", "standard", "auto"},
		Defaults: &config.DefaultsConfig{
			Unclassified: "standard",
			NoChanges:    "auto",
		},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	tests := []struct {
		resourceType   string
		expectedCode   int
		classification string
	}{
		{"critical_resource", 3, "critical"}, // highest precedence = highest exit code
		{"review_resource", 2, "review"},
		{"standard_resource", 1, "standard"},
		{"auto_resource", 0, "auto"}, // lowest precedence = lowest exit code
	}

	for _, tt := range tests {
		changes := []plan.ResourceChange{
			{Address: "test.resource", Type: tt.resourceType, Actions: []string{"create"}},
		}

		result := classifier.Classify(changes)

		if result.Overall != tt.classification {
			t.Errorf("type %s: expected classification '%s', got '%s'",
				tt.resourceType, tt.classification, result.Overall)
		}

		if result.OverallExitCode != tt.expectedCode {
			t.Errorf("type %s (%s): expected exit code %d, got %d",
				tt.resourceType, tt.classification, tt.expectedCode, result.OverallExitCode)
		}
	}
}

func TestClassify_WithPluginDecisions(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_virtual_network.main",
			Type:    "azurerm_virtual_network",
			Actions: []string{"update"},
		},
	}

	result := classifier.Classify(changes)

	// Core classification should be "standard"
	if result.Overall != "standard" {
		t.Errorf("expected initial overall 'standard', got '%s'", result.Overall)
	}

	// Now add plugin decisions that upgrade to "critical"
	pluginDecisions := []ResourceDecision{
		{
			Address:        "azurerm_virtual_network.main",
			ResourceType:   "azurerm_virtual_network",
			Actions:        []string{"update"},
			Classification: "critical",
			MatchedRule:    "plugin: sensitive change detected",
		},
	}

	classifier.AddPluginDecisions(result, pluginDecisions)

	// Should be upgraded to "critical"
	if result.Overall != "critical" {
		t.Errorf("expected overall 'critical' after plugin decision, got '%s'", result.Overall)
	}

	if result.ResourceDecisions[0].Classification != "critical" {
		t.Errorf("expected resource to be 'critical' after plugin decision, got '%s'",
			result.ResourceDecisions[0].Classification)
	}
}

func TestClassify_PluginDecisionsWithEmptyClassification(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_virtual_network.main",
			Type:    "azurerm_virtual_network",
			Actions: []string{"update"},
		},
	}

	result := classifier.Classify(changes)

	// Core classification should be "standard"
	if result.Overall != "standard" {
		t.Errorf("expected initial overall 'standard', got '%s'", result.Overall)
	}

	// Add plugin decisions with empty classification (should be ignored)
	pluginDecisions := []ResourceDecision{
		{
			Address:        "azurerm_virtual_network.main",
			ResourceType:   "azurerm_virtual_network",
			Actions:        []string{"update"},
			Classification: "", // Empty - should be ignored
			MatchedRule:    "plugin: some detection",
		},
	}

	classifier.AddPluginDecisions(result, pluginDecisions)

	// Should remain "standard" because empty classification is ignored
	if result.Overall != "standard" {
		t.Errorf("expected overall to remain 'standard' when empty classification is ignored, got '%s'", result.Overall)
	}

	if result.ResourceDecisions[0].Classification != "standard" {
		t.Errorf("expected resource to remain 'standard' when empty classification is ignored, got '%s'",
			result.ResourceDecisions[0].Classification)
	}
}

func TestClassify_PluginDecisionsWithUnknownClassification(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_virtual_network.main",
			Type:    "azurerm_virtual_network",
			Actions: []string{"update"},
		},
	}

	result := classifier.Classify(changes)

	// Core classification should be "standard"
	if result.Overall != "standard" {
		t.Errorf("expected initial overall 'standard', got '%s'", result.Overall)
	}

	// Add plugin decisions with unknown classification (should be ignored)
	pluginDecisions := []ResourceDecision{
		{
			Address:        "azurerm_virtual_network.main",
			ResourceType:   "azurerm_virtual_network",
			Actions:        []string{"update"},
			Classification: "unknown_classification", // Not in precedence list
			MatchedRule:    "plugin: some detection",
		},
	}

	classifier.AddPluginDecisions(result, pluginDecisions)

	// Should remain "standard" because unknown classification is ignored
	if result.Overall != "standard" {
		t.Errorf("expected overall to remain 'standard' when unknown classification is ignored, got '%s'", result.Overall)
	}

	if result.ResourceDecisions[0].Classification != "standard" {
		t.Errorf("expected resource to remain 'standard' when unknown classification is ignored, got '%s'",
			result.ResourceDecisions[0].Classification)
	}
}
