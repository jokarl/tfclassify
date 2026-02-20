package classify

import (
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

func intPtr(n int) *int { return &n }

func TestBlastRadiusAnalyzer_Name(t *testing.T) {
	a := NewBlastRadiusAnalyzer(nil)
	if a.Name() != "blast_radius" {
		t.Errorf("expected name 'blast_radius', got %q", a.Name())
	}
}

func TestBlastRadius_DeletionThresholdExceeded(t *testing.T) {
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{Name: "critical", BlastRadius: &config.BlastRadiusConfig{MaxDeletions: intPtr(5)}},
	})

	changes := make([]plan.ResourceChange, 8)
	for i := range changes {
		changes[i] = plan.ResourceChange{
			Address: "azurerm_resource_group.rg" + string(rune('0'+i)),
			Type:    "azurerm_resource_group",
			Actions: []string{"delete"},
		}
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 8 {
		t.Fatalf("expected 8 decisions (one per resource), got %d", len(decisions))
	}

	for _, d := range decisions {
		if d.Classification != "critical" {
			t.Errorf("expected classification 'critical', got %q", d.Classification)
		}
		if len(d.MatchedRules) != 1 {
			t.Fatalf("expected 1 matched rule, got %d", len(d.MatchedRules))
		}
		if !strings.Contains(d.MatchedRules[0], "8 deletions exceeded max_deletions threshold of 5") {
			t.Errorf("unexpected reason: %s", d.MatchedRules[0])
		}
	}
}

func TestBlastRadius_DeletionThresholdNotExceeded(t *testing.T) {
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{Name: "critical", BlastRadius: &config.BlastRadiusConfig{MaxDeletions: intPtr(5)}},
	})

	changes := make([]plan.ResourceChange, 3)
	for i := range changes {
		changes[i] = plan.ResourceChange{
			Address: "rg." + string(rune('0'+i)),
			Type:    "azurerm_resource_group",
			Actions: []string{"delete"},
		}
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions, got %d", len(decisions))
	}
}

func TestBlastRadius_ReplacementThreshold(t *testing.T) {
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{Name: "critical", BlastRadius: &config.BlastRadiusConfig{MaxReplacements: intPtr(10)}},
	})

	changes := make([]plan.ResourceChange, 11)
	for i := range changes {
		changes[i] = plan.ResourceChange{
			Address: "res." + string(rune('a'+i)),
			Type:    "azurerm_virtual_machine",
			Actions: []string{"delete", "create"},
		}
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 11 {
		t.Fatalf("expected 11 decisions, got %d", len(decisions))
	}
	if !strings.Contains(decisions[0].MatchedRules[0], "11 replacements exceeded max_replacements threshold of 10") {
		t.Errorf("unexpected reason: %s", decisions[0].MatchedRules[0])
	}
}

func TestBlastRadius_TotalChangesThreshold(t *testing.T) {
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{Name: "critical", BlastRadius: &config.BlastRadiusConfig{MaxChanges: intPtr(50)}},
	})

	changes := make([]plan.ResourceChange, 51)
	for i := range changes {
		changes[i] = plan.ResourceChange{
			Address: "res." + string(rune('a'+i%26)) + string(rune('a'+i/26)),
			Type:    "azurerm_virtual_machine",
			Actions: []string{"create"},
		}
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 51 {
		t.Fatalf("expected 51 decisions, got %d", len(decisions))
	}
	if !strings.Contains(decisions[0].MatchedRules[0], "51 total changes exceeded max_changes threshold of 50") {
		t.Errorf("unexpected reason: %s", decisions[0].MatchedRules[0])
	}
}

func TestBlastRadius_MultipleThresholdsExceeded(t *testing.T) {
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{Name: "critical", BlastRadius: &config.BlastRadiusConfig{
			MaxDeletions: intPtr(2),
			MaxChanges:   intPtr(3),
		}},
	})

	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.two", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.three", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "vm.one", Type: "azurerm_virtual_machine", Actions: []string{"create"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 4 {
		t.Fatalf("expected 4 decisions, got %d", len(decisions))
	}

	// Should have 2 reasons: deletions and total changes
	if len(decisions[0].MatchedRules) != 2 {
		t.Fatalf("expected 2 matched rules, got %d: %v", len(decisions[0].MatchedRules), decisions[0].MatchedRules)
	}

	foundDeletions := false
	foundChanges := false
	for _, r := range decisions[0].MatchedRules {
		if strings.Contains(r, "max_deletions") {
			foundDeletions = true
		}
		if strings.Contains(r, "max_changes") {
			foundChanges = true
		}
	}
	if !foundDeletions || !foundChanges {
		t.Errorf("expected both deletion and change reasons, got: %v", decisions[0].MatchedRules)
	}
}

func TestBlastRadius_OmittedFieldsIgnored(t *testing.T) {
	// Only max_deletions configured; 100 creates should not trigger
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{Name: "critical", BlastRadius: &config.BlastRadiusConfig{MaxDeletions: intPtr(5)}},
	})

	changes := make([]plan.ResourceChange, 100)
	for i := range changes {
		changes[i] = plan.ResourceChange{
			Address: "res." + string(rune('a'+i%26)),
			Type:    "azurerm_virtual_machine",
			Actions: []string{"create"},
		}
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions (creates don't count toward deletions), got %d", len(decisions))
	}
}

func TestBlastRadius_NoConfig(t *testing.T) {
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{Name: "critical"},
	})

	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"delete"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions with no blast_radius config, got %d", len(decisions))
	}
}

func TestBlastRadius_NoOpExcluded(t *testing.T) {
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{Name: "critical", BlastRadius: &config.BlastRadiusConfig{MaxChanges: intPtr(50)}},
	})

	changes := make([]plan.ResourceChange, 60)
	for i := range changes {
		changes[i] = plan.ResourceChange{
			Address: "res." + string(rune('a'+i%26)),
			Type:    "azurerm_virtual_machine",
			Actions: []string{"no-op"},
		}
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions (no-ops don't count), got %d", len(decisions))
	}
}

func TestBlastRadius_EmitsForAllResources(t *testing.T) {
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{Name: "critical", BlastRadius: &config.BlastRadiusConfig{MaxDeletions: intPtr(2)}},
	})

	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.two", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "rg.three", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "vm.one", Type: "azurerm_virtual_machine", Actions: []string{"create"}},
		{Address: "vm.two", Type: "azurerm_virtual_machine", Actions: []string{"update"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 5 {
		t.Fatalf("expected 5 decisions (one per resource), got %d", len(decisions))
	}

	addresses := make(map[string]bool)
	for _, d := range decisions {
		addresses[d.Address] = true
	}
	for _, c := range changes {
		if !addresses[c.Address] {
			t.Errorf("expected decision for %s", c.Address)
		}
	}
}

func TestBlastRadius_NoChanges(t *testing.T) {
	a := NewBlastRadiusAnalyzer([]config.ClassificationConfig{
		{Name: "critical", BlastRadius: &config.BlastRadiusConfig{MaxChanges: intPtr(1)}},
	})

	decisions := a.Analyze([]plan.ResourceChange{})
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for empty changes, got %d", len(decisions))
	}
}
