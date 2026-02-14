// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	"fmt"

	"github.com/jokarl/tfclassify/sdk"
)

// KeyVaultAccessAnalyzer detects when key vault access policies grant destructive permissions.
// It inspects permission fields like secret_permissions, key_permissions, etc. for
// destructive operations like "delete" or "purge".
type KeyVaultAccessAnalyzer struct {
	sdk.DefaultAnalyzer
	config *PluginConfig
}

// NewKeyVaultAccessAnalyzer creates a new KeyVaultAccessAnalyzer.
func NewKeyVaultAccessAnalyzer(config *PluginConfig) *KeyVaultAccessAnalyzer {
	return &KeyVaultAccessAnalyzer{config: config}
}

// Name returns the analyzer name.
func (a *KeyVaultAccessAnalyzer) Name() string {
	return "key-vault-access"
}

// Enabled returns whether this analyzer is enabled.
func (a *KeyVaultAccessAnalyzer) Enabled() bool {
	return a.config.KeyVaultEnabled
}

// ResourcePatterns returns the patterns this analyzer is interested in.
func (a *KeyVaultAccessAnalyzer) ResourcePatterns() []string {
	return []string{"azurerm_key_vault_access_policy"}
}

// Analyze inspects key vault access policies for destructive permissions.
func (a *KeyVaultAccessAnalyzer) Analyze(runner sdk.Runner) error {
	changes, err := runner.GetResourceChanges(a.ResourcePatterns())
	if err != nil {
		return fmt.Errorf("failed to get resource changes: %w", err)
	}

	destructive := toSet(a.config.DestructiveKVPermissions)

	// Permission fields to check
	permissionFields := []string{
		"secret_permissions",
		"key_permissions",
		"certificate_permissions",
		"storage_permissions",
	}

	for _, change := range changes {
		after := change.After
		if after == nil {
			continue // Policy is being deleted
		}

		// Check each permission field for destructive permissions
		for _, field := range permissionFields {
			if permissions, ok := after[field].([]interface{}); ok {
				foundDestructive := []string{}
				for _, p := range permissions {
					if perm, ok := p.(string); ok && destructive[perm] {
						foundDestructive = append(foundDestructive, perm)
					}
				}

				if len(foundDestructive) > 0 {
					decision := &sdk.Decision{
						Classification: "",
						Reason:         fmt.Sprintf("key vault access policy grants destructive permissions: %v on %s", foundDestructive, field),
						Severity:       80,
						Metadata: map[string]interface{}{
							"analyzer":         "key-vault-access",
							"permission_field": field,
							"permissions":      foundDestructive,
						},
					}

					if err := runner.EmitDecision(a, change, decision); err != nil {
						return fmt.Errorf("failed to emit decision: %w", err)
					}
				}
			}
		}
	}

	return nil
}
