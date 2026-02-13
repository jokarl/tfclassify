package main

import (
	"testing"
)

func TestPluginSetName(t *testing.T) {
	ps := NewTerraformPluginSet()

	if ps.PluginSetName() != "terraform" {
		t.Errorf("expected name 'terraform', got '%s'", ps.PluginSetName())
	}
}

func TestPluginSetVersion(t *testing.T) {
	ps := NewTerraformPluginSet()

	if ps.PluginSetVersion() != Version {
		t.Errorf("expected version '%s', got '%s'", Version, ps.PluginSetVersion())
	}
}

func TestAnalyzerNames(t *testing.T) {
	ps := NewTerraformPluginSet()

	names := ps.AnalyzerNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 analyzers, got %d", len(names))
	}

	expected := map[string]bool{
		"deletion":  false,
		"sensitive": false,
		"replace":   false,
	}

	for _, name := range names {
		if _, ok := expected[name]; !ok {
			t.Errorf("unexpected analyzer name: %s", name)
		}
		expected[name] = true
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing analyzer: %s", name)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	ps := NewTerraformPluginSet()

	if !ps.config.DeletionEnabled {
		t.Error("expected DeletionEnabled to be true by default")
	}
	if !ps.config.SensitiveEnabled {
		t.Error("expected SensitiveEnabled to be true by default")
	}
	if !ps.config.ReplaceEnabled {
		t.Error("expected ReplaceEnabled to be true by default")
	}
}
