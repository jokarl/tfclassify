package classify

import (
	"strings"
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
				// No rules — no-op resources short-circuit to defaults.no_changes;
				// the workaround rule from before CR-0036 is no longer needed.
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
			MatchedRules:   []string{"plugin: sensitive change detected"},
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
			MatchedRules:   []string{"plugin: some detection"},
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

func TestExplainClassify_AllRulesEvaluated(t *testing.T) {
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

	result := classifier.ExplainClassify(changes)

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	// newTestConfig defines rules for "critical" and "standard" only;
	// "auto" carries no rules post-CR-0036 (no-op resources short-circuit).
	res := result.Resources[0]
	if len(res.Trace) != 2 {
		t.Fatalf("expected 2 trace entries (one per rule), got %d", len(res.Trace))
	}

	classifications := make(map[string]bool)
	for _, entry := range res.Trace {
		classifications[entry.Classification] = true
	}
	for _, name := range []string{"critical", "standard"} {
		if !classifications[name] {
			t.Errorf("expected trace entry for classification %q", name)
		}
	}
}

func TestExplainClassify_SkipReasons(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_virtual_network.main",
			Type:    "azurerm_virtual_network",
			Actions: []string{"create"},
		},
	}

	result := classifier.ExplainClassify(changes)
	res := result.Resources[0]

	// Critical rule should skip with resource mismatch (vnet doesn't match *_role_*)
	if res.Trace[0].Result != TraceSkip {
		t.Errorf("expected critical rule to skip, got %s", res.Trace[0].Result)
	}
	if res.Trace[0].Reason != "resource mismatch" {
		t.Errorf("expected 'resource mismatch' reason, got %q", res.Trace[0].Reason)
	}
}

func TestExplainClassify_MatchesClassify(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "azurerm_role_assignment.example", Type: "azurerm_role_assignment", Actions: []string{"delete"}},
		{Address: "azurerm_virtual_network.main", Type: "azurerm_virtual_network", Actions: []string{"update"}},
	}

	classifyResult := classifier.Classify(changes)
	explainResult := classifier.ExplainClassify(changes)
	classifier.FinalizeExplanation(explainResult)

	for i, decision := range classifyResult.ResourceDecisions {
		if explainResult.Resources[i].FinalClassification != decision.Classification {
			t.Errorf("resource %s: explain says %q, classify says %q",
				decision.Address,
				explainResult.Resources[i].FinalClassification,
				decision.Classification)
		}
	}
}

func TestExplainClassify_NoChanges(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	result := classifier.ExplainClassify([]plan.ResourceChange{})

	if !result.NoChanges {
		t.Error("expected NoChanges to be true")
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestExplainClassify_DefaultFallback(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:  "critical",
				Rules: []config.RuleConfig{{Resource: []string{"critical_*"}}},
			},
		},
		Precedence: []string{"critical"},
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
		{Address: "azurerm_virtual_network.main", Type: "azurerm_virtual_network", Actions: []string{"update"}},
	}

	result := classifier.ExplainClassify(changes)
	classifier.FinalizeExplanation(result)
	res := result.Resources[0]

	// All rules should skip
	for _, entry := range res.Trace {
		if entry.Result != TraceSkip {
			t.Errorf("expected all entries to skip for unmatched resource, got %s for %s", entry.Result, entry.Rule)
		}
	}

	if res.FinalClassification != "standard" {
		t.Errorf("expected default 'standard', got %q", res.FinalClassification)
	}
	if res.FinalSource != "default" {
		t.Errorf("expected source 'default', got %q", res.FinalSource)
	}
	if res.WinnerReason != "no rule matched" {
		t.Errorf("expected winner reason 'no rule matched', got %q", res.WinnerReason)
	}
}

func TestExplainClassify_BuiltinAnalyzerTrace(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "azurerm_resource_group.delete", Type: "azurerm_resource_group", Actions: []string{"delete"}},
	}

	result := classifier.ExplainClassify(changes)
	builtinDecisions := classifier.AddExplainBuiltinAnalyzers(result, planResultFromChanges(changes), []BuiltinAnalyzer{
		&DeletionAnalyzer{},
	}, nil)

	if len(builtinDecisions) != 1 {
		t.Fatalf("expected 1 builtin decision, got %d", len(builtinDecisions))
	}

	// Find the deletion trace entry
	res := result.Resources[0]
	found := false
	for _, entry := range res.Trace {
		if entry.Source == "builtin: deletion" {
			found = true
			if entry.Result != TraceMatch {
				t.Errorf("expected deletion entry to be MATCH, got %s", entry.Result)
			}
			break
		}
	}
	if !found {
		t.Error("expected deletion analyzer trace entry")
	}
}

func TestExplainClassify_PluginDecisionTrace(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{Address: "azurerm_role_assignment.example", Type: "azurerm_role_assignment", Actions: []string{"create"}},
	}

	result := classifier.ExplainClassify(changes)

	// Simulate plugin decisions
	pluginDecisions := []ResourceDecision{
		{
			Address:        "azurerm_role_assignment.example",
			Classification: "critical",
			MatchedRules:   []string{"azurerm/privilege-escalation"},
		},
	}

	classifier.AddExplainPluginDecisions(result, pluginDecisions)
	classifier.FinalizeExplanation(result)

	res := result.Resources[0]

	// Should have plugin entry in trace
	found := false
	for _, entry := range res.Trace {
		if entry.Source == "plugin: azurerm/privilege-escalation" {
			found = true
			if entry.Result != TraceMatch {
				t.Errorf("expected plugin entry to be MATCH, got %s", entry.Result)
			}
			break
		}
	}
	if !found {
		t.Error("expected plugin trace entry")
	}

	// Final should be critical (from plugin)
	if res.FinalClassification != "critical" {
		t.Errorf("expected final 'critical', got %q", res.FinalClassification)
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
			MatchedRules:   []string{"plugin: some detection"},
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

func TestClassify_ModuleScopedRules(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}, Actions: []string{"delete"}, Module: []string{"module.production", "module.production.**"}},
				},
			},
			{
				Name:        "standard",
				Description: "Standard",
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

	tests := []struct {
		name          string
		moduleAddress string
		actions       []string
		wantClass     string
	}{
		{
			name:          "delete in production module → critical",
			moduleAddress: "module.production",
			actions:       []string{"delete"},
			wantClass:     "critical",
		},
		{
			name:          "delete in production submodule → critical",
			moduleAddress: "module.production.module.network",
			actions:       []string{"delete"},
			wantClass:     "critical",
		},
		{
			name:          "delete in staging module → standard (no module match)",
			moduleAddress: "module.staging",
			actions:       []string{"delete"},
			wantClass:     "standard",
		},
		{
			name:          "delete in root module → standard (no module match)",
			moduleAddress: "",
			actions:       []string{"delete"},
			wantClass:     "standard",
		},
		{
			name:          "create in production module → standard (no action match)",
			moduleAddress: "module.production",
			actions:       []string{"create"},
			wantClass:     "standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := []plan.ResourceChange{
				{
					Address:       "azurerm_resource_group.test",
					Type:          "azurerm_resource_group",
					Actions:       tt.actions,
					ModuleAddress: tt.moduleAddress,
				},
			}

			result := classifier.Classify(changes)

			if result.ResourceDecisions[0].Classification != tt.wantClass {
				t.Errorf("expected classification %q, got %q",
					tt.wantClass, result.ResourceDecisions[0].Classification)
			}
		})
	}
}

func TestClassify_NotModuleRules(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}, Actions: []string{"delete"}, NotModule: []string{"module.staging", "module.staging.**"}},
				},
			},
			{
				Name:        "standard",
				Description: "Standard",
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

	tests := []struct {
		name          string
		moduleAddress string
		wantClass     string
	}{
		{"delete in production → critical", "module.production", "critical"},
		{"delete in root → critical", "", "critical"},
		{"delete in staging → standard (excluded)", "module.staging", "standard"},
		{"delete in staging submodule → standard (excluded)", "module.staging.module.db", "standard"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := []plan.ResourceChange{
				{
					Address:       "azurerm_resource_group.test",
					Type:          "azurerm_resource_group",
					Actions:       []string{"delete"},
					ModuleAddress: tt.moduleAddress,
				},
			}

			result := classifier.Classify(changes)

			if result.ResourceDecisions[0].Classification != tt.wantClass {
				t.Errorf("expected classification %q, got %q",
					tt.wantClass, result.ResourceDecisions[0].Classification)
			}
		})
	}
}

func TestExplainClassify_ModuleMismatchReason(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}, Module: []string{"module.production"}},
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

	changes := []plan.ResourceChange{
		{
			Address:       "azurerm_resource_group.test",
			Type:          "azurerm_resource_group",
			Actions:       []string{"delete"},
			ModuleAddress: "module.staging",
		},
	}

	result := classifier.ExplainClassify(changes)
	res := result.Resources[0]

	// The critical rule should show "module mismatch"
	if res.Trace[0].Reason != "module mismatch" {
		t.Errorf("expected 'module mismatch' reason for critical rule, got %q", res.Trace[0].Reason)
	}
}

func TestClassify_AllNoOpReportsNoChanges(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	// All resources are no-op (simulates post-FilterCosmeticChanges state)
	changes := []plan.ResourceChange{
		{
			Address:         "azurerm_search_service.this",
			Type:            "azurerm_search_service",
			Actions:         []string{"no-op"},
			OriginalActions: []string{"update"},
		},
		{
			Address: "azurerm_resource_group.rg",
			Type:    "azurerm_resource_group",
			Actions: []string{"no-op"},
		},
	}

	result := classifier.Classify(changes)

	if !result.NoChanges {
		t.Error("expected NoChanges to be true when all resources are no-op")
	}
	if result.Overall != "auto" {
		t.Errorf("expected overall 'auto' (no_changes default), got '%s'", result.Overall)
	}
	if result.OverallExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.OverallExitCode)
	}
	// Resource decisions should still be populated for visibility
	if len(result.ResourceDecisions) != 2 {
		t.Fatalf("expected 2 resource decisions, got %d", len(result.ResourceDecisions))
	}
}

func TestClassify_MixedNoOpAndRealNotNoChanges(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_search_service.this",
			Type:    "azurerm_search_service",
			Actions: []string{"no-op"},
		},
		{
			Address: "azurerm_resource_group.rg",
			Type:    "azurerm_resource_group",
			Actions: []string{"update"},
		},
	}

	result := classifier.Classify(changes)

	if result.NoChanges {
		t.Error("expected NoChanges to be false when some resources have real changes")
	}
}

// CR-0036: a no-op resource whose type would match a higher-precedence rule
// must NOT be classified by that rule. Short-circuit assigns defaults.no_changes
// and records a synthetic rule that describes the ignore_attributes downgrade.
func TestClassifyResource_NoOpShortCircuit(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "major",
				Description: "Core services",
				Rules: []config.RuleConfig{
					{Resource: []string{"azurerm_key_vault_key"}, Description: "Encryption keys"},
				},
			},
			{
				Name:        "minor",
				Description: "Plumbing",
			},
		},
		Precedence: []string{"major", "minor"},
		Defaults: &config.DefaultsConfig{
			Unclassified: "major",
			NoChanges:    "minor",
		},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	change := plan.ResourceChange{
		Address:           "azurerm_key_vault_key.cmk",
		Type:              "azurerm_key_vault_key",
		Actions:           []string{"no-op"},
		OriginalActions:   []string{"update"},
		IgnoredAttributes: []string{"tags.tf-module-l2"},
	}

	result := classifier.Classify([]plan.ResourceChange{change})

	if len(result.ResourceDecisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(result.ResourceDecisions))
	}
	d := result.ResourceDecisions[0]
	if d.Classification != "minor" {
		t.Errorf("expected Classification = defaults.no_changes (minor), got %q", d.Classification)
	}
	if len(d.MatchedRules) != 1 {
		t.Fatalf("expected 1 matched rule, got %d: %v", len(d.MatchedRules), d.MatchedRules)
	}
	if !strings.Contains(d.MatchedRules[0], "ignore_attributes") {
		t.Errorf("expected synthetic rule to reference ignore_attributes, got %q", d.MatchedRules[0])
	}
	if !strings.Contains(d.MatchedRules[0], "tags.tf-module-l2") {
		t.Errorf("expected synthetic rule to list the ignored attribute path, got %q", d.MatchedRules[0])
	}
	if strings.Contains(d.MatchedRules[0], "Encryption keys") {
		t.Errorf("synthetic rule must not reference the major rule that would have matched the type, got %q", d.MatchedRules[0])
	}
}

// CR-0036: a Terraform-native no-op (no OriginalActions) gets the
// "no-op (no change)" synthetic description.
func TestClassifyResource_NativeNoOp(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{Name: "major", Rules: []config.RuleConfig{{Resource: []string{"*"}}}},
			{Name: "minor"},
		},
		Precedence: []string{"major", "minor"},
		Defaults:   &config.DefaultsConfig{Unclassified: "major", NoChanges: "minor"},
	}
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	change := plan.ResourceChange{
		Address: "azurerm_resource_group.rg",
		Type:    "azurerm_resource_group",
		Actions: []string{"no-op"},
	}

	result := classifier.Classify([]plan.ResourceChange{change})
	d := result.ResourceDecisions[0]

	if d.Classification != "minor" {
		t.Errorf("expected Classification = defaults.no_changes (minor), got %q", d.Classification)
	}
	if len(d.MatchedRules) != 1 || d.MatchedRules[0] != "no-op (no change)" {
		t.Errorf("expected [\"no-op (no change)\"], got %v", d.MatchedRules)
	}
}

// CR-0036: explain must emit exactly one synthetic trace entry for a no-op
// resource — the whole point is to NOT evaluate rules.
func TestExplainClassify_NoOpSingleTraceEntry(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{Name: "major", Rules: []config.RuleConfig{{Resource: []string{"azurerm_key_vault_key"}}}},
			{Name: "minor", Rules: []config.RuleConfig{{Resource: []string{"*"}}}},
		},
		Precedence: []string{"major", "minor"},
		Defaults:   &config.DefaultsConfig{Unclassified: "major", NoChanges: "minor"},
	}
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	change := plan.ResourceChange{
		Address:           "azurerm_key_vault_key.cmk",
		Type:              "azurerm_key_vault_key",
		Actions:           []string{"no-op"},
		OriginalActions:   []string{"update"},
		IgnoredAttributes: []string{"tags.tf-module-l2"},
	}

	result := classifier.ExplainClassify([]plan.ResourceChange{change})
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	exp := result.Resources[0]
	if len(exp.Trace) != 1 {
		t.Fatalf("expected 1 trace entry (short-circuit), got %d: %v", len(exp.Trace), exp.Trace)
	}
	entry := exp.Trace[0]
	if entry.Result != TraceMatch {
		t.Errorf("expected TraceMatch, got %q", entry.Result)
	}
	if entry.Classification != "minor" {
		t.Errorf("expected trace classification = minor, got %q", entry.Classification)
	}
	if !strings.Contains(entry.Rule, "ignore_attributes") {
		t.Errorf("expected synthetic rule to reference ignore_attributes, got %q", entry.Rule)
	}
	if exp.FinalClassification != "minor" {
		t.Errorf("expected FinalClassification = minor, got %q", exp.FinalClassification)
	}
}

// A no-op decision (post FilterCosmeticChanges) must not elevate Overall above
// the real changes in the plan. Otherwise the text output says "major" while
// all major-classified resources are hidden as no-op, forcing a rule author to
// discover the `not_actions = ["no-op"]` workaround.
func TestClassify_NoOpDoesNotElevateOverall(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name:        "major",
				Description: "Core services",
				Rules: []config.RuleConfig{
					{Resource: []string{"azurerm_key_vault_key"}, Description: "Encryption keys"},
				},
			},
			{
				Name:        "minor",
				Description: "Plumbing",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}, Actions: []string{"read"}, Description: "Data reads"},
				},
			},
		},
		Precedence: []string{"major", "minor"},
		Defaults: &config.DefaultsConfig{
			Unclassified: "major",
			NoChanges:    "minor",
		},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	// A key_vault_key tag-only update that FilterCosmeticChanges downgraded to no-op,
	// alongside a single data-source read.
	changes := []plan.ResourceChange{
		{
			Address:           "azurerm_key_vault_key.cmk",
			Type:              "azurerm_key_vault_key",
			Actions:           []string{"no-op"},
			OriginalActions:   []string{"update"},
			IgnoredAttributes: []string{"tags.tf-module-l2"},
		},
		{
			Address: "data.azapi_resource_action.account_keys[0]",
			Type:    "azapi_resource_action",
			Actions: []string{"read"},
		},
	}

	result := classifier.Classify(changes)

	if result.Overall != "minor" {
		t.Errorf("expected Overall 'minor' (real change is the data read) — a no-op-downgraded resource should not elevate Overall; got %q", result.Overall)
	}
}

func TestExplainClassify_AllNoOpReportsNoChanges(t *testing.T) {
	cfg := newTestConfig()
	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_search_service.this",
			Type:    "azurerm_search_service",
			Actions: []string{"no-op"},
		},
	}

	result := classifier.ExplainClassify(changes)

	if !result.NoChanges {
		t.Error("expected NoChanges to be true when all resources are no-op")
	}
	// Resources should still be in the trace for visibility
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource in trace, got %d", len(result.Resources))
	}
}

func TestClassify_NotActions(t *testing.T) {
	cfg := &config.Config{
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
					{Resource: []string{"*"}, NotActions: []string{"no-op"}},
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

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	tests := []struct {
		name           string
		resourceType   string
		actions        []string
		wantClass      string
	}{
		{
			name:         "create matches not_actions=[no-op]",
			resourceType: "azurerm_resource_group",
			actions:      []string{"create"},
			wantClass:    "standard",
		},
		{
			name:         "update matches not_actions=[no-op]",
			resourceType: "azurerm_resource_group",
			actions:      []string{"update"},
			wantClass:    "standard",
		},
		{
			name:         "no-op excluded by not_actions, falls through to auto",
			resourceType: "azurerm_resource_group",
			actions:      []string{"no-op"},
			wantClass:    "auto",
		},
		{
			name:         "delete on role still matched by higher-precedence critical rule",
			resourceType: "azurerm_role_assignment",
			actions:      []string{"delete"},
			wantClass:    "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := []plan.ResourceChange{
				{
					Address: tt.resourceType + ".test",
					Type:    tt.resourceType,
					Actions: tt.actions,
				},
			}

			result := classifier.Classify(changes)

			if result.ResourceDecisions[0].Classification != tt.wantClass {
				t.Errorf("expected classification %q, got %q",
					tt.wantClass, result.ResourceDecisions[0].Classification)
			}
		})
	}
}
