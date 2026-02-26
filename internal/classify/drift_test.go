package classify

import (
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

func TestDriftAnalyzer_NoDriftClassification(t *testing.T) {
	a := &DriftAnalyzer{DriftClassification: ""}
	result := &plan.ParseResult{
		Changes:      []plan.ResourceChange{{Address: "a.b", Type: "a", Actions: []string{"update"}}},
		DriftChanges: []plan.ResourceChange{{Address: "a.b", Type: "a", Actions: []string{"update"}}},
	}
	decisions := a.AnalyzePlan(result)
	if len(decisions) != 0 {
		t.Errorf("expected no decisions when drift_classification is empty, got %d", len(decisions))
	}
}

func TestDriftAnalyzer_NoDriftChanges(t *testing.T) {
	a := &DriftAnalyzer{DriftClassification: "standard"}
	result := &plan.ParseResult{
		Changes:      []plan.ResourceChange{{Address: "a.b", Type: "a", Actions: []string{"update"}}},
		DriftChanges: nil,
	}
	decisions := a.AnalyzePlan(result)
	if len(decisions) != 0 {
		t.Errorf("expected no decisions when no drift, got %d", len(decisions))
	}
}

func TestDriftAnalyzer_DriftOnly(t *testing.T) {
	a := &DriftAnalyzer{DriftClassification: "standard"}
	result := &plan.ParseResult{
		Changes: []plan.ResourceChange{
			{Address: "azurerm_resource_group.main", Type: "azurerm_resource_group", Actions: []string{"update"}},
		},
		DriftChanges: []plan.ResourceChange{
			{Address: "azurerm_resource_group.main", Type: "azurerm_resource_group", Actions: []string{"update"}},
		},
	}

	decisions := a.AnalyzePlan(result)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Classification != "standard" {
		t.Errorf("expected classification 'standard', got %q", decisions[0].Classification)
	}
	if decisions[0].Address != "azurerm_resource_group.main" {
		t.Errorf("expected address azurerm_resource_group.main, got %q", decisions[0].Address)
	}
}

func TestDriftAnalyzer_MixedDriftAndIntent(t *testing.T) {
	a := &DriftAnalyzer{DriftClassification: "standard"}
	result := &plan.ParseResult{
		Changes: []plan.ResourceChange{
			{Address: "azurerm_resource_group.main", Type: "azurerm_resource_group", Actions: []string{"update"}},
			{Address: "azurerm_virtual_network.main", Type: "azurerm_virtual_network", Actions: []string{"create"}},
		},
		DriftChanges: []plan.ResourceChange{
			{Address: "azurerm_resource_group.main", Type: "azurerm_resource_group", Actions: []string{"update"}},
		},
	}

	decisions := a.AnalyzePlan(result)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision (only drift resource), got %d", len(decisions))
	}
	if decisions[0].Address != "azurerm_resource_group.main" {
		t.Errorf("expected drift decision for azurerm_resource_group.main, got %q", decisions[0].Address)
	}
}

func TestDriftAnalyzer_Name(t *testing.T) {
	a := &DriftAnalyzer{}
	if a.Name() != "drift" {
		t.Errorf("expected name 'drift', got %q", a.Name())
	}
}

func TestDriftAddresses_NilPlanResult(t *testing.T) {
	addrs := DriftAddresses(nil)
	if addrs != nil {
		t.Errorf("expected nil for nil plan result, got %v", addrs)
	}
}

func TestDriftAddresses_NoDrift(t *testing.T) {
	addrs := DriftAddresses(&plan.ParseResult{})
	if addrs != nil {
		t.Errorf("expected nil for no drift, got %v", addrs)
	}
}

func TestDriftAddresses_WithDrift(t *testing.T) {
	result := &plan.ParseResult{
		DriftChanges: []plan.ResourceChange{
			{Address: "a.b"},
			{Address: "c.d"},
		},
	}
	addrs := DriftAddresses(result)
	if len(addrs) != 2 {
		t.Fatalf("expected 2 drift addresses, got %d", len(addrs))
	}
	if _, ok := addrs["a.b"]; !ok {
		t.Error("expected a.b in drift addresses")
	}
	if _, ok := addrs["c.d"]; !ok {
		t.Error("expected c.d in drift addresses")
	}
}

func TestBlastRadius_ExcludeDrift(t *testing.T) {
	maxDel := 1
	excludeTrue := true
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{
			Name: "critical",
			BlastRadius: &config.BlastRadiusConfig{
				MaxDeletions: &maxDel,
				ExcludeDrift: &excludeTrue,
			},
		},
	})

	// 3 deletions, but 2 are drift → only 1 non-drift deletion → should NOT trigger (1 is not > 1)
	a.SetDriftAddresses(map[string]struct{}{
		"rg.drift1": {},
		"rg.drift2": {},
	})

	changes := []plan.ResourceChange{
		{Address: "rg.drift1", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.drift2", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.intentional", Type: "azurerm_resource_group", Actions: []string{"delete"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected no decisions when drift excluded brings count below threshold, got %d", len(decisions))
	}
}

func TestBlastRadius_ExcludeDrift_StillTriggered(t *testing.T) {
	maxDel := 1
	excludeTrue := true
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{
			Name: "critical",
			BlastRadius: &config.BlastRadiusConfig{
				MaxDeletions: &maxDel,
				ExcludeDrift: &excludeTrue,
			},
		},
	})

	// 3 deletions, 1 is drift → 2 non-drift deletions → should trigger (2 > 1)
	a.SetDriftAddresses(map[string]struct{}{
		"rg.drift1": {},
	})

	changes := []plan.ResourceChange{
		{Address: "rg.drift1", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.intentional1", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.intentional2", Type: "azurerm_resource_group", Actions: []string{"delete"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) == 0 {
		t.Error("expected decisions when non-drift count exceeds threshold")
	}
}

func TestBlastRadius_ExcludeDriftFalse(t *testing.T) {
	maxDel := 1
	excludeFalse := false
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{
			Name: "critical",
			BlastRadius: &config.BlastRadiusConfig{
				MaxDeletions: &maxDel,
				ExcludeDrift: &excludeFalse,
			},
		},
	})

	a.SetDriftAddresses(map[string]struct{}{
		"rg.drift1": {},
		"rg.drift2": {},
	})

	changes := []plan.ResourceChange{
		{Address: "rg.drift1", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.drift2", Type: "azurerm_resource_group", Actions: []string{"delete"}},
	}

	// exclude_drift = false, so drift resources ARE counted → 2 > 1 → trigger
	decisions := a.Analyze(changes)
	if len(decisions) == 0 {
		t.Error("expected decisions when exclude_drift is false")
	}
}

func TestRunBuiltinAnalyzers_WithDriftAnalyzer(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{Name: "critical", Description: "Critical", Rules: []config.RuleConfig{{Resource: []string{"never_*"}}}},
			{Name: "standard", Description: "Standard", Rules: []config.RuleConfig{{Resource: []string{"*"}}}},
		},
		Precedence: []string{"critical", "standard"},
		Defaults:   &config.DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	classifier, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	planResult := &plan.ParseResult{
		Changes: []plan.ResourceChange{
			{Address: "azurerm_resource_group.main", Type: "azurerm_resource_group", Actions: []string{"update"}},
		},
		DriftChanges: []plan.ResourceChange{
			{Address: "azurerm_resource_group.main", Type: "azurerm_resource_group", Actions: []string{"update"}},
		},
	}

	result := classifier.Classify(planResult.Changes)

	driftAnalyzer := &DriftAnalyzer{DriftClassification: "critical"}
	classifier.RunBuiltinAnalyzers(result, planResult, nil, []PlanAwareAnalyzer{driftAnalyzer})

	if result.ResourceDecisions[0].Classification != "critical" {
		t.Errorf("expected drift to upgrade to 'critical', got %q", result.ResourceDecisions[0].Classification)
	}
}
