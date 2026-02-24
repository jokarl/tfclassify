// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	"github.com/jokarl/tfclassify/sdk"
)

// Version is the plugin version.
var Version = "dev"

// AzurermPluginSet is the main plugin set for the azurerm deep inspection plugin.
type AzurermPluginSet struct {
	*sdk.BuiltinPluginSet
	config *PluginConfig
}

// PluginConfig holds the configuration for the azurerm plugin.
type PluginConfig struct {
	// Enabled flags for each analyzer
	PrivilegeEnabled bool

	// RoleDatabase is the built-in Azure role database for role permission lookup.
	// If nil, DefaultRoleDatabase() is used.
	RoleDatabase *RoleDatabase

	// CrossReferenceCustomRoles enables lookup of azurerm_role_definition resources in the plan.
	// Default: true
	CrossReferenceCustomRoles bool
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *PluginConfig {
	return &PluginConfig{
		PrivilegeEnabled:          true,
		RoleDatabase:              DefaultRoleDatabase(),
		CrossReferenceCustomRoles: true,
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
		},
	}

	return ps
}
