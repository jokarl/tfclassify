package config

import (
	"strings"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	cfg, err := LoadFile("testdata/valid.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check plugins
	if len(cfg.Plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(cfg.Plugins))
	}

	// Check terraform plugin
	if cfg.Plugins[0].Name != "terraform" {
		t.Errorf("expected first plugin name to be 'terraform', got %s", cfg.Plugins[0].Name)
	}
	if !cfg.Plugins[0].Enabled {
		t.Error("expected terraform plugin to be enabled")
	}

	// Check azurerm plugin
	if cfg.Plugins[1].Name != "azurerm" {
		t.Errorf("expected second plugin name to be 'azurerm', got %s", cfg.Plugins[1].Name)
	}
	if cfg.Plugins[1].Source != "github.com/jokarl/tfclassify-plugin-azurerm" {
		t.Errorf("unexpected source: %s", cfg.Plugins[1].Source)
	}
	if cfg.Plugins[1].Version != "0.1.0" {
		t.Errorf("unexpected version: %s", cfg.Plugins[1].Version)
	}

	// Check classifications
	if len(cfg.Classifications) != 3 {
		t.Errorf("expected 3 classifications, got %d", len(cfg.Classifications))
	}

	// Check critical classification
	critical := cfg.Classifications[0]
	if critical.Name != "critical" {
		t.Errorf("expected first classification to be 'critical', got %s", critical.Name)
	}
	if critical.Description != "Requires security team approval" {
		t.Errorf("unexpected description: %s", critical.Description)
	}
	if len(critical.Rules) != 2 {
		t.Errorf("expected 2 rules for critical, got %d", len(critical.Rules))
	}

	// Check precedence
	if len(cfg.Precedence) != 3 {
		t.Errorf("expected 3 precedence entries, got %d", len(cfg.Precedence))
	}
	expected := []string{"critical", "standard", "auto"}
	for i, e := range expected {
		if cfg.Precedence[i] != e {
			t.Errorf("precedence[%d]: expected %s, got %s", i, e, cfg.Precedence[i])
		}
	}

	// Check defaults
	if cfg.Defaults == nil {
		t.Fatal("expected defaults to be non-nil")
	}
	if cfg.Defaults.Unclassified != "standard" {
		t.Errorf("expected unclassified to be 'standard', got %s", cfg.Defaults.Unclassified)
	}
	if cfg.Defaults.NoChanges != "auto" {
		t.Errorf("expected no_changes to be 'auto', got %s", cfg.Defaults.NoChanges)
	}
	if cfg.Defaults.PluginTimeout != "30s" {
		t.Errorf("expected plugin_timeout to be '30s', got %s", cfg.Defaults.PluginTimeout)
	}
}

func TestLoad_MinimalConfig(t *testing.T) {
	cfg, err := LoadFile("testdata/minimal.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(cfg.Plugins))
	}

	if len(cfg.Classifications) != 1 {
		t.Errorf("expected 1 classification, got %d", len(cfg.Classifications))
	}

	if cfg.Defaults == nil {
		t.Error("expected defaults to be non-nil")
	}
}

func TestLoad_MultiplePlugins(t *testing.T) {
	cfg, err := LoadFile("testdata/multiple_plugins.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(cfg.Plugins))
	}

	names := []string{"terraform", "azurerm", "aws"}
	for i, name := range names {
		if cfg.Plugins[i].Name != name {
			t.Errorf("plugin %d: expected name %s, got %s", i, name, cfg.Plugins[i].Name)
		}
	}

	// Check aws is disabled
	if cfg.Plugins[2].Enabled {
		t.Error("expected aws plugin to be disabled")
	}
}

func TestLoad_MultipleRules(t *testing.T) {
	cfg, err := LoadFile("testdata/multiple_rules.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	critical := cfg.Classifications[0]
	if len(critical.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(critical.Rules))
	}

	// Check first rule
	if len(critical.Rules[0].Resource) != 1 || critical.Rules[0].Resource[0] != "*_role_*" {
		t.Errorf("unexpected first rule resource: %v", critical.Rules[0].Resource)
	}

	// Check second rule
	if len(critical.Rules[1].Resource) != 1 || critical.Rules[1].Resource[0] != "*_key_vault*" {
		t.Errorf("unexpected second rule resource: %v", critical.Rules[1].Resource)
	}
}

func TestLoad_PluginConfigBody(t *testing.T) {
	cfg, err := LoadFile("testdata/plugin_with_config.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(cfg.Plugins))
	}

	plugin := cfg.Plugins[0]
	if plugin.Config == nil {
		t.Error("expected plugin Config to be non-nil")
	}
}

func TestLoad_InvalidHCL(t *testing.T) {
	_, err := LoadFile("testdata/invalid_hcl.hcl")
	if err == nil {
		t.Fatal("expected error for invalid HCL, got nil")
	}

	// Should contain filename and line number
	errStr := err.Error()
	if !strings.Contains(errStr, "invalid_hcl.hcl") {
		t.Errorf("expected error to contain filename, got: %v", err)
	}
}
