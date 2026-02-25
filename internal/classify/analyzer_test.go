package classify

import (
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

func defaultAnalyzers() []BuiltinAnalyzer {
	return []BuiltinAnalyzer{
		&DeletionAnalyzer{},
		&ReplaceAnalyzer{},
		&SensitiveAnalyzer{},
	}
}

// planResultFromChanges creates a minimal ParseResult wrapping changes.
func planResultFromChanges(changes []plan.ResourceChange) *plan.ParseResult {
	return &plan.ParseResult{Changes: changes}
}

func TestRunBuiltinAnalyzers_Integration(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{Name: "critical", Description: "Critical", Rules: []config.RuleConfig{{Resource: []string{"*_role_*"}}}},
			{Name: "standard", Description: "Standard", Rules: []config.RuleConfig{{Resource: []string{"*"}}}},
		},
		Precedence: []string{"critical", "standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "rg.deleted", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.replaced", Type: "azurerm_resource_group", Actions: []string{"delete", "create"}},
		{
			Address:         "db.sensitive",
			Type:            "aws_db_instance",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"password": "old"},
			After:           map[string]interface{}{"password": "new"},
			BeforeSensitive: map[string]interface{}{"password": true},
			AfterSensitive:  map[string]interface{}{"password": true},
		},
		{Address: "vnet.normal", Type: "azurerm_virtual_network", Actions: []string{"update"}},
	}

	result := classifier.Classify(changes)

	// All should be "standard" from core rules (no *_role_* match)
	for _, d := range result.ResourceDecisions {
		if d.Classification != "standard" {
			t.Errorf("expected initial classification 'standard' for %s, got %q", d.Address, d.Classification)
		}
	}

	// Run builtin analyzers
	classifier.RunBuiltinAnalyzers(result, planResultFromChanges(changes), defaultAnalyzers(), nil)

	// Analyzers emit empty classification => maps to "standard" (unclassified default)
	// So results remain standard (no upgrade)
	if result.Overall != "standard" {
		t.Errorf("expected overall 'standard', got %q", result.Overall)
	}
}

func TestRunBuiltinAnalyzers_MergePrecedence(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{Name: "critical", Description: "Critical", Rules: []config.RuleConfig{{Resource: []string{"*_role_*"}}}},
			{Name: "standard", Description: "Standard", Rules: []config.RuleConfig{{Resource: []string{"*"}}}},
		},
		Precedence: []string{"critical", "standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "critical", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "rg.deleted", Type: "azurerm_resource_group", Actions: []string{"delete"}},
	}

	result := classifier.Classify(changes)

	if result.ResourceDecisions[0].Classification != "standard" {
		t.Fatalf("expected initial classification 'standard', got %q", result.ResourceDecisions[0].Classification)
	}

	// Builtin analyzers emit empty classification => maps to defaults.unclassified = "critical"
	classifier.RunBuiltinAnalyzers(result, planResultFromChanges(changes), defaultAnalyzers(), nil)

	if result.ResourceDecisions[0].Classification != "critical" {
		t.Errorf("expected 'critical' after builtin analyzer merge, got %q", result.ResourceDecisions[0].Classification)
	}
	if result.Overall != "critical" {
		t.Errorf("expected overall 'critical', got %q", result.Overall)
	}
}

func TestRunBuiltinAnalyzers_NoChanges(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{Name: "standard", Description: "Standard", Rules: []config.RuleConfig{{Resource: []string{"*"}}}},
		},
		Precedence: []string{"standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	result := classifier.Classify([]plan.ResourceChange{})

	classifier.RunBuiltinAnalyzers(result, planResultFromChanges(nil), defaultAnalyzers(), nil)

	if !result.NoChanges {
		t.Error("expected NoChanges to remain true")
	}
}

func TestRunBuiltinAnalyzers_NoTriggers(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{Name: "standard", Description: "Standard", Rules: []config.RuleConfig{{Resource: []string{"*"}}}},
		},
		Precedence: []string{"standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "vnet.one", Type: "azurerm_virtual_network", Actions: []string{"update"}},
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"create"}},
	}

	result := classifier.Classify(changes)
	initialDecisions := make([]ResourceDecision, len(result.ResourceDecisions))
	copy(initialDecisions, result.ResourceDecisions)

	classifier.RunBuiltinAnalyzers(result, planResultFromChanges(changes), defaultAnalyzers(), nil)

	for i, d := range result.ResourceDecisions {
		if d.Classification != initialDecisions[i].Classification {
			t.Errorf("expected classification unchanged for %s, was %q now %q",
				d.Address, initialDecisions[i].Classification, d.Classification)
		}
	}
}

func TestRunBuiltinAnalyzers_NilAnalyzers(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{Name: "standard", Description: "Standard", Rules: []config.RuleConfig{{Resource: []string{"*"}}}},
		},
		Precedence: []string{"standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"delete"}},
	}

	result := classifier.Classify(changes)

	classifier.RunBuiltinAnalyzers(result, planResultFromChanges(changes), nil, nil)

	if result.ResourceDecisions[0].Classification != "standard" {
		t.Errorf("expected classification unchanged, got %q", result.ResourceDecisions[0].Classification)
	}
}

func TestRunBuiltinAnalyzers_AllThreeAnalyzersTrigger(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{Name: "critical", Description: "Critical", Rules: []config.RuleConfig{{Resource: []string{"never_match_*"}}}},
			{Name: "standard", Description: "Standard", Rules: []config.RuleConfig{{Resource: []string{"*"}}}},
		},
		Precedence: []string{"critical", "standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "rg.deleted", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.replaced", Type: "azurerm_resource_group", Actions: []string{"delete", "create"}},
		{
			Address:         "db.sensitive",
			Type:            "aws_db_instance",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"password": "old"},
			After:           map[string]interface{}{"password": "new"},
			BeforeSensitive: map[string]interface{}{"password": true},
			AfterSensitive:  map[string]interface{}{"password": true},
		},
		{Address: "vnet.normal", Type: "azurerm_virtual_network", Actions: []string{"update"}},
	}

	result := classifier.Classify(changes)
	classifier.RunBuiltinAnalyzers(result, planResultFromChanges(changes), defaultAnalyzers(), nil)

	if len(result.ResourceDecisions) != 4 {
		t.Fatalf("expected 4 decisions, got %d", len(result.ResourceDecisions))
	}
}
