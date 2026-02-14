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

	// Check privileged roles
	if len(config.PrivilegedRoles) != 3 {
		t.Errorf("expected 3 default privileged roles, got %d", len(config.PrivilegedRoles))
	}

	expectedRoles := map[string]bool{
		"Owner":                     true,
		"User Access Administrator": true,
		"Contributor":               true,
	}
	for _, role := range config.PrivilegedRoles {
		if !expectedRoles[role] {
			t.Errorf("unexpected privileged role: %s", role)
		}
	}

	// Check permissive sources
	if len(config.PermissiveSources) != 3 {
		t.Errorf("expected 3 default permissive sources, got %d", len(config.PermissiveSources))
	}

	// Check destructive KV permissions
	if len(config.DestructiveKVPermissions) != 2 {
		t.Errorf("expected 2 default destructive KV permissions, got %d", len(config.DestructiveKVPermissions))
	}

	// Check enabled flags
	if !config.PrivilegeEnabled {
		t.Error("expected PrivilegeEnabled to be true by default")
	}
	if !config.NetworkEnabled {
		t.Error("expected NetworkEnabled to be true by default")
	}
	if !config.KeyVaultEnabled {
		t.Error("expected KeyVaultEnabled to be true by default")
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
	if len(names) != 3 {
		t.Fatalf("expected 3 analyzers, got %d", len(names))
	}

	expectedNames := map[string]bool{
		"privilege-escalation": true,
		"network-exposure":     true,
		"key-vault-access":     true,
	}
	for _, name := range names {
		if !expectedNames[name] {
			t.Errorf("unexpected analyzer name: %s", name)
		}
	}
}

func TestNewAzurermPluginSetWithConfig(t *testing.T) {
	config := &PluginConfig{
		PrivilegedRoles:  []string{"Custom"},
		PrivilegeEnabled: false,
		NetworkEnabled:   true,
		KeyVaultEnabled:  true,
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

	tests := []struct {
		name     string
		expected []string
	}{
		{"privilege-escalation", []string{"azurerm_role_assignment"}},
		{"network-exposure", []string{"azurerm_network_security_rule"}},
		{"key-vault-access", []string{"azurerm_key_vault_access_policy"}},
	}

	for _, tt := range tests {
		analyzer := ps.GetAnalyzer(tt.name)
		if analyzer == nil {
			t.Errorf("analyzer %q not found", tt.name)
			continue
		}

		patterns := analyzer.ResourcePatterns()
		if len(patterns) != len(tt.expected) {
			t.Errorf("analyzer %q: expected %d patterns, got %d", tt.name, len(tt.expected), len(patterns))
			continue
		}

		for i, p := range patterns {
			if p != tt.expected[i] {
				t.Errorf("analyzer %q: expected pattern %q, got %q", tt.name, tt.expected[i], p)
			}
		}
	}
}
