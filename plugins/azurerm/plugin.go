// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	"github.com/jokarl/tfclassify/sdk"
)

// Version is the plugin version.
const Version = "0.1.0"

// AzurermPluginSet is the main plugin set for the azurerm deep inspection plugin.
type AzurermPluginSet struct {
	*sdk.BuiltinPluginSet
	config *PluginConfig
}

// PluginConfig holds the configuration for the azurerm plugin.
type PluginConfig struct {
	// PrivilegedRoles are roles that trigger privilege escalation detection.
	PrivilegedRoles []string

	// PermissiveSources are network sources that trigger network exposure detection.
	PermissiveSources []string

	// DestructiveKVPermissions are key vault permissions that trigger destructive access detection.
	DestructiveKVPermissions []string

	// Enabled flags for each analyzer
	PrivilegeEnabled  bool
	NetworkEnabled    bool
	KeyVaultEnabled   bool
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *PluginConfig {
	return &PluginConfig{
		PrivilegedRoles: []string{
			"Owner",
			"User Access Administrator",
			"Contributor",
		},
		PermissiveSources: []string{
			"*",
			"0.0.0.0/0",
			"Internet",
		},
		DestructiveKVPermissions: []string{
			"delete",
			"purge",
		},
		PrivilegeEnabled: true,
		NetworkEnabled:   true,
		KeyVaultEnabled:  true,
	}
}

// NewAzurermPluginSet creates a new AzurermPluginSet with default configuration.
func NewAzurermPluginSet() *AzurermPluginSet {
	return NewAzurermPluginSetWithConfig(DefaultConfig())
}

// NewAzurermPluginSetWithConfig creates a new AzurermPluginSet with the given configuration.
func NewAzurermPluginSetWithConfig(config *PluginConfig) *AzurermPluginSet {
	ps := &AzurermPluginSet{
		config: config,
	}

	ps.BuiltinPluginSet = &sdk.BuiltinPluginSet{
		Name:    "azurerm",
		Version: Version,
		Analyzers: []sdk.Analyzer{
			NewPrivilegeEscalationAnalyzer(ps.config),
			NewNetworkExposureAnalyzer(ps.config),
			NewKeyVaultAccessAnalyzer(ps.config),
		},
	}

	return ps
}
