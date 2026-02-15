package main

import (
	"errors"
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

func TestKeyVaultAccess_PurgePermission(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewKeyVaultAccessAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_key_vault_access_policy.test",
				Type:    "azurerm_key_vault_access_policy",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"secret_permissions": []interface{}{"get", "list", "purge"},
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Severity != 80 {
		t.Errorf("expected severity 80, got %d", decision.Severity)
	}

	permissions := decision.Metadata["permissions"].([]string)
	if len(permissions) != 1 || permissions[0] != "purge" {
		t.Errorf("expected permissions [purge], got %v", permissions)
	}
}

func TestKeyVaultAccess_DeletePermission(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewKeyVaultAccessAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_key_vault_access_policy.test",
				Type:    "azurerm_key_vault_access_policy",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"key_permissions": []interface{}{"get", "delete", "list"},
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for delete permission, got %d", len(runner.decisions))
	}
}

func TestKeyVaultAccess_ReadOnly(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewKeyVaultAccessAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_key_vault_access_policy.test",
				Type:    "azurerm_key_vault_access_policy",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"secret_permissions": []interface{}{"get", "list"},
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for read-only access, got %d", len(runner.decisions))
	}
}

func TestKeyVaultAccess_MultiplePermissionFields(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewKeyVaultAccessAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_key_vault_access_policy.test",
				Type:    "azurerm_key_vault_access_policy",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"secret_permissions":      []interface{}{"get", "purge"},
					"key_permissions":         []interface{}{"get", "delete"},
					"certificate_permissions": []interface{}{"get", "list"},
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should emit decisions for both secret_permissions and key_permissions
	if len(runner.decisions) != 2 {
		t.Fatalf("expected 2 decisions for multiple permission fields, got %d", len(runner.decisions))
	}
}

func TestKeyVaultAccess_Deletion(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewKeyVaultAccessAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_key_vault_access_policy.test",
				Type:    "azurerm_key_vault_access_policy",
				Actions: []string{"delete"},
				Before: map[string]interface{}{
					"secret_permissions": []interface{}{"get", "purge"},
				},
				After: nil, // Being deleted
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Deletions are fine - we only care about what's being created/updated
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for deletion, got %d", len(runner.decisions))
	}
}

func TestKeyVaultAccess_CustomPermissions(t *testing.T) {
	config := &PluginConfig{
		DestructiveKVPermissions: []string{"backup", "restore"},
		KeyVaultEnabled:          true,
	}
	analyzer := NewKeyVaultAccessAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_key_vault_access_policy.test",
				Type:    "azurerm_key_vault_access_policy",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"secret_permissions": []interface{}{"get", "backup"}, // backup is in custom list
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for custom permission, got %d", len(runner.decisions))
	}

	// Verify purge doesn't trigger with custom config
	runner2 := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_key_vault_access_policy.test2",
				Type:    "azurerm_key_vault_access_policy",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"secret_permissions": []interface{}{"get", "purge"}, // purge not in custom list
				},
			},
		},
	}

	err = analyzer.Analyze(runner2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner2.decisions) != 0 {
		t.Errorf("expected 0 decisions when purge not in custom list, got %d", len(runner2.decisions))
	}
}

func TestKeyVaultAccess_GetResourceChangesError(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewKeyVaultAccessAnalyzer(config)
	runner := &mockRunner{err: errors.New("test error")}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestKeyVaultAccess_EmitDecisionError(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewKeyVaultAccessAnalyzer(config)
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_key_vault_access_policy.test",
				Type:    "azurerm_key_vault_access_policy",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"secret_permissions": []interface{}{"get", "purge"},
				},
			},
		},
		emitErr: errors.New("emit error"),
	}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestKeyVaultAccess_Name(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewKeyVaultAccessAnalyzer(config)

	if analyzer.Name() != "keyvault-access" {
		t.Errorf("expected name 'keyvault-access', got %q", analyzer.Name())
	}
}

func TestKeyVaultAccess_Enabled(t *testing.T) {
	enabledConfig := &PluginConfig{KeyVaultEnabled: true}
	disabledConfig := &PluginConfig{KeyVaultEnabled: false}

	enabledAnalyzer := NewKeyVaultAccessAnalyzer(enabledConfig)
	disabledAnalyzer := NewKeyVaultAccessAnalyzer(disabledConfig)

	if !enabledAnalyzer.Enabled() {
		t.Error("expected analyzer to be enabled when KeyVaultEnabled is true")
	}
	if disabledAnalyzer.Enabled() {
		t.Error("expected analyzer to be disabled when KeyVaultEnabled is false")
	}
}
