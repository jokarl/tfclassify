package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_WithExplicitPath(t *testing.T) {
	cfg, err := Load("testdata/valid.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Classifications) != 3 {
		t.Errorf("expected 3 classifications, got %d", len(cfg.Classifications))
	}

	if cfg.Classifications[0].Name != "critical" {
		t.Errorf("expected first classification 'critical', got %q", cfg.Classifications[0].Name)
	}
}

func TestLoad_WithDiscovery(t *testing.T) {
	// Create a temp directory with a .tfclassify.hcl file
	tmpDir, err := os.MkdirTemp("", "tfclassify-load-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configContent := `
classification "standard" {
  description = "Standard"
  rule {
    resource = ["*"]
  }
}

precedence = ["standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
`
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory so Discover finds the config
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Classifications) != 1 {
		t.Errorf("expected 1 classification, got %d", len(cfg.Classifications))
	}
	if cfg.Classifications[0].Name != "standard" {
		t.Errorf("expected 'standard', got %q", cfg.Classifications[0].Name)
	}
}

func TestLoad_DiscoveryFails(t *testing.T) {
	// Create a temp directory with NO config file
	tmpDir, err := os.MkdirTemp("", "tfclassify-noconfig-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	_, err = Load("")
	if err == nil {
		t.Fatal("expected error when no config file found")
	}
	if !strings.Contains(err.Error(), "no configuration file found") {
		t.Errorf("expected 'no configuration file found' error, got: %v", err)
	}
}

func TestLoad_ExplicitPathNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.hcl")
	if err == nil {
		t.Fatal("expected error for nonexistent explicit path")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("expected 'config file not found' error, got: %v", err)
	}
}

func TestLoadFile_UnreadableFile(t *testing.T) {
	// Create a temp directory with an unreadable file
	tmpDir, err := os.MkdirTemp("", "tfclassify-unreadable-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "unreadable.hcl")
	if err := os.WriteFile(configPath, []byte("content"), 0000); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = LoadFile(configPath)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
	// Error could be a read error or a parse error depending on OS/user permissions
	errStr := err.Error()
	if !strings.Contains(errStr, "failed to read config file") && !strings.Contains(errStr, "configuration error") {
		t.Errorf("expected read or parse error, got: %v", err)
	}
}

func TestLoadFile_FileNotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/file.hcl")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("expected 'failed to read config file' error, got: %v", err)
	}
}

func TestParse_EmptyBytes(t *testing.T) {
	// Empty bytes should fail validation (no precedence, etc.)
	_, err := Parse([]byte(""), "empty.hcl")
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}

func TestDiscover_HomeDirectory(t *testing.T) {
	// Create a temp directory to use as HOME with a config file
	tmpHome, err := os.MkdirTemp("", "tfclassify-home-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Create another temp dir to use as CWD (without config)
	tmpCwd, err := os.MkdirTemp("", "tfclassify-cwd-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpCwd)

	// Put config in HOME
	configPath := filepath.Join(tmpHome, ConfigFileName)
	if err := os.WriteFile(configPath, []byte("# test"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to CWD without config
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpCwd); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Set HOME to our temp home
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	path, err := Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != configPath {
		t.Errorf("expected path %q, got %q", configPath, path)
	}
}

func TestLoad_ClassificationScopedPluginConfig(t *testing.T) {
	cfg, err := Load("testdata/classification_plugin_config.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Classifications) != 3 {
		t.Fatalf("expected 3 classifications, got %d", len(cfg.Classifications))
	}

	// Test critical classification has plugin config
	critical := cfg.Classifications[0]
	if critical.Name != "critical" {
		t.Errorf("expected first classification 'critical', got %q", critical.Name)
	}

	if critical.PluginAnalyzerConfigs == nil {
		t.Fatal("expected PluginAnalyzerConfigs to be set for 'critical'")
	}

	azurermCfg, ok := critical.PluginAnalyzerConfigs["azurerm"]
	if !ok {
		t.Fatal("expected 'azurerm' plugin config in 'critical' classification")
	}

	// Test privilege_escalation config
	if azurermCfg.PrivilegeEscalation == nil {
		t.Fatal("expected PrivilegeEscalation config")
	}
	if len(azurermCfg.PrivilegeEscalation.Actions) != 1 {
		t.Errorf("expected 1 Actions pattern, got %d", len(azurermCfg.PrivilegeEscalation.Actions))
	} else if azurermCfg.PrivilegeEscalation.Actions[0] != "Microsoft.Authorization/*" {
		t.Errorf("expected Actions[0] 'Microsoft.Authorization/*', got %q", azurermCfg.PrivilegeEscalation.Actions[0])
	}
	if len(azurermCfg.PrivilegeEscalation.Exclude) != 2 {
		t.Errorf("expected 2 Exclude roles, got %d", len(azurermCfg.PrivilegeEscalation.Exclude))
	}
	if azurermCfg.PrivilegeEscalation.Exclude[0] != "AcrPush" {
		t.Errorf("expected first Exclude role 'AcrPush', got %q", azurermCfg.PrivilegeEscalation.Exclude[0])
	}

	// Test network_exposure config
	if azurermCfg.NetworkExposure == nil {
		t.Fatal("expected NetworkExposure config")
	}
	if len(azurermCfg.NetworkExposure.PermissiveSources) != 3 {
		t.Errorf("expected 3 PermissiveSources, got %d", len(azurermCfg.NetworkExposure.PermissiveSources))
	}

	// Test keyvault_access config (empty block)
	if azurermCfg.KeyVaultAccess == nil {
		t.Fatal("expected KeyVaultAccess config (even if empty)")
	}

	// Test high classification has partial plugin config
	high := cfg.Classifications[1]
	if high.Name != "high" {
		t.Errorf("expected second classification 'high', got %q", high.Name)
	}

	azurermHighCfg, ok := high.PluginAnalyzerConfigs["azurerm"]
	if !ok {
		t.Fatal("expected 'azurerm' plugin config in 'high' classification")
	}

	if azurermHighCfg.PrivilegeEscalation == nil {
		t.Fatal("expected PrivilegeEscalation config in 'high'")
	}
	if len(azurermHighCfg.PrivilegeEscalation.Roles) != 2 {
		t.Errorf("expected 2 Roles, got %d", len(azurermHighCfg.PrivilegeEscalation.Roles))
	}

	// Test standard classification has no plugin config
	standard := cfg.Classifications[2]
	if standard.Name != "standard" {
		t.Errorf("expected third classification 'standard', got %q", standard.Name)
	}

	if len(standard.PluginAnalyzerConfigs) != 0 {
		t.Errorf("expected no plugin configs for 'standard', got %d", len(standard.PluginAnalyzerConfigs))
	}
}

func TestLoad_PatternBasedDetection(t *testing.T) {
	cfg, err := Load("testdata/pattern_based_detection.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Classifications) != 2 {
		t.Fatalf("expected 2 classifications, got %d", len(cfg.Classifications))
	}

	// Test critical classification has pattern-based detection config
	critical := cfg.Classifications[0]
	if critical.Name != "critical" {
		t.Errorf("expected first classification 'critical', got %q", critical.Name)
	}

	if critical.PluginAnalyzerConfigs == nil {
		t.Fatal("expected PluginAnalyzerConfigs to be set for 'critical'")
	}

	azurermCfg, ok := critical.PluginAnalyzerConfigs["azurerm"]
	if !ok {
		t.Fatal("expected 'azurerm' plugin config in 'critical' classification")
	}

	// Test privilege_escalation config with pattern-based detection
	if azurermCfg.PrivilegeEscalation == nil {
		t.Fatal("expected PrivilegeEscalation config")
	}

	// Test CR-0028: actions field (control-plane patterns)
	if len(azurermCfg.PrivilegeEscalation.Actions) != 2 {
		t.Errorf("expected 2 Actions patterns, got %d", len(azurermCfg.PrivilegeEscalation.Actions))
	}
	if azurermCfg.PrivilegeEscalation.Actions[0] != "*" {
		t.Errorf("expected first Actions pattern '*', got %q", azurermCfg.PrivilegeEscalation.Actions[0])
	}
	if azurermCfg.PrivilegeEscalation.Actions[1] != "Microsoft.Authorization/*" {
		t.Errorf("expected second Actions pattern 'Microsoft.Authorization/*', got %q", azurermCfg.PrivilegeEscalation.Actions[1])
	}

	// Test CR-0027: data_actions field (data-plane patterns)
	if len(azurermCfg.PrivilegeEscalation.DataActions) != 2 {
		t.Errorf("expected 2 DataActions patterns, got %d", len(azurermCfg.PrivilegeEscalation.DataActions))
	}
	if azurermCfg.PrivilegeEscalation.DataActions[0] != "*/read" {
		t.Errorf("expected first DataActions pattern '*/read', got %q", azurermCfg.PrivilegeEscalation.DataActions[0])
	}
	if azurermCfg.PrivilegeEscalation.DataActions[1] != "*/write" {
		t.Errorf("expected second DataActions pattern '*/write', got %q", azurermCfg.PrivilegeEscalation.DataActions[1])
	}

	// Test CR-0028: scopes field
	if len(azurermCfg.PrivilegeEscalation.Scopes) != 2 {
		t.Errorf("expected 2 Scopes, got %d", len(azurermCfg.PrivilegeEscalation.Scopes))
	}
	if azurermCfg.PrivilegeEscalation.Scopes[0] != "subscription" {
		t.Errorf("expected first Scope 'subscription', got %q", azurermCfg.PrivilegeEscalation.Scopes[0])
	}
	if azurermCfg.PrivilegeEscalation.Scopes[1] != "management_group" {
		t.Errorf("expected second Scope 'management_group', got %q", azurermCfg.PrivilegeEscalation.Scopes[1])
	}

	// Test CR-0028: flag_unknown_roles field
	if azurermCfg.PrivilegeEscalation.FlagUnknownRoles == nil {
		t.Fatal("expected FlagUnknownRoles to be set")
	}
	if *azurermCfg.PrivilegeEscalation.FlagUnknownRoles != false {
		t.Errorf("expected FlagUnknownRoles to be false, got %v", *azurermCfg.PrivilegeEscalation.FlagUnknownRoles)
	}
}

func TestLoad_ClassificationScopedPluginConfig_UnknownPlugin(t *testing.T) {
	configContent := `
plugin "azurerm" {
  enabled = true
  source  = "github.com/example/plugin"
  version = "0.1.0"
}

classification "critical" {
  description = "Critical"
  rule {
    resource = ["*"]
  }

  # Reference to plugin that doesn't exist
  nonexistent {
    some_analyzer {}
  }
}

precedence = ["critical"]

defaults {
  unclassified = "critical"
  no_changes   = "critical"
}
`
	_, err := Parse([]byte(configContent), "test.hcl")
	if err == nil {
		t.Fatal("expected error for plugin reference to non-existent plugin")
	}
	if !strings.Contains(err.Error(), "nonexistent") || !strings.Contains(err.Error(), "not enabled") {
		t.Errorf("expected error about unknown plugin reference, got: %v", err)
	}
}

func TestLoad_ClassificationScopedPluginConfig_DisabledPlugin(t *testing.T) {
	configContent := `
plugin "azurerm" {
  enabled = false
  source  = "github.com/example/plugin"
  version = "0.1.0"
}

classification "critical" {
  description = "Critical"
  rule {
    resource = ["*"]
  }

  # Reference to disabled plugin
  azurerm {
    privilege_escalation {}
  }
}

precedence = ["critical"]

defaults {
  unclassified = "critical"
  no_changes   = "critical"
}
`
	_, err := Parse([]byte(configContent), "test.hcl")
	if err == nil {
		t.Fatal("expected error for plugin reference to disabled plugin")
	}
	if !strings.Contains(err.Error(), "azurerm") || !strings.Contains(err.Error(), "not enabled") {
		t.Errorf("expected error about disabled plugin reference, got: %v", err)
	}
}

func TestLoad_ScoreThresholdRejected(t *testing.T) {
	configContent := `
plugin "azurerm" {
  enabled = true
  source  = "github.com/example/plugin"
  version = "0.1.0"
}

classification "critical" {
  description = "Critical"
  rule {
    resource = ["*"]
  }

  azurerm {
    privilege_escalation {
      score_threshold = 80
    }
  }
}

precedence = ["critical"]

defaults {
  unclassified = "critical"
  no_changes   = "critical"
}
`
	_, err := Parse([]byte(configContent), "test.hcl")
	if err == nil {
		t.Fatal("expected error for score_threshold (removed in CR-0028)")
	}
	if !strings.Contains(err.Error(), "score_threshold") || !strings.Contains(err.Error(), "no longer supported") {
		t.Errorf("expected 'score_threshold is no longer supported' error, got: %v", err)
	}
}
