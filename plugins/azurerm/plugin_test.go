package main

import (
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

// mockRunner implements sdk.Runner for testing.
type mockRunner struct {
	changes   []*sdk.ResourceChange
	decisions []*sdk.Decision
	err       error
	emitErr   error
}

func (r *mockRunner) GetResourceChanges(patterns []string) ([]*sdk.ResourceChange, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(patterns) == 0 {
		return r.changes, nil
	}

	// Filter by patterns (simple implementation for tests)
	var filtered []*sdk.ResourceChange
	for _, c := range r.changes {
		for _, p := range patterns {
			if p == "*" || c.Type == p {
				filtered = append(filtered, c)
				break
			}
		}
	}
	return filtered, nil
}

func (r *mockRunner) GetResourceChange(address string) (*sdk.ResourceChange, error) {
	if r.err != nil {
		return nil, r.err
	}
	for _, c := range r.changes {
		if c.Address == address {
			return c, nil
		}
	}
	return nil, nil
}

func (r *mockRunner) EmitDecision(analyzer sdk.Analyzer, change *sdk.ResourceChange, decision *sdk.Decision) error {
	if r.emitErr != nil {
		return r.emitErr
	}
	r.decisions = append(r.decisions, decision)
	return nil
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.PrivilegeEnabled {
		t.Error("expected PrivilegeEnabled to be true by default")
	}
	if config.RoleDatabase == nil {
		t.Error("expected RoleDatabase to be set by default")
	}
	if !config.CrossReferenceCustomRoles {
		t.Error("expected CrossReferenceCustomRoles to be true by default")
	}
}

func TestNewAzurermPluginSet(t *testing.T) {
	ps := NewAzurermPluginSet()

	if ps.PluginSetName() != "azurerm" {
		t.Errorf("expected plugin name 'azurerm', got %q", ps.PluginSetName())
	}

	if ps.PluginSetVersion() != Version {
		t.Errorf("expected version %q, got %q", Version, ps.PluginSetVersion())
	}

	names := ps.AnalyzerNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 analyzer, got %d", len(names))
	}

	if names[0] != "privilege-escalation" {
		t.Errorf("expected analyzer name 'privilege-escalation', got %q", names[0])
	}
}

func TestNewAzurermPluginSetWithConfig(t *testing.T) {
	config := &PluginConfig{
		PrivilegeEnabled: false,
	}

	ps := NewAzurermPluginSetWithConfig(config)

	// Check that the privilege analyzer respects the enabled flag
	analyzer := ps.GetAnalyzer("privilege-escalation")
	if analyzer == nil {
		t.Fatal("expected to find privilege-escalation analyzer")
	}

	if analyzer.Enabled() {
		t.Error("expected privilege-escalation to be disabled")
	}
}

func TestAnalyzerResourcePatterns(t *testing.T) {
	config := DefaultConfig()
	ps := NewAzurermPluginSetWithConfig(config)

	analyzer := ps.GetAnalyzer("privilege-escalation")
	if analyzer == nil {
		t.Fatal("analyzer 'privilege-escalation' not found")
	}

	patterns := analyzer.ResourcePatterns()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0] != "azurerm_role_assignment" {
		t.Errorf("expected pattern 'azurerm_role_assignment', got %q", patterns[0])
	}
}
