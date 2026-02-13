// Package main provides the bundled Terraform plugin for tfclassify.
package main

import (
	"github.com/jokarl/tfclassify/sdk"
)

// Version is the plugin version.
const Version = "0.1.0"

// TerraformPluginSet is the main plugin set for the bundled terraform plugin.
type TerraformPluginSet struct {
	*sdk.BuiltinPluginSet
	config *PluginConfig
}

// PluginConfig holds the configuration for the terraform plugin.
type PluginConfig struct {
	DeletionEnabled  bool
	SensitiveEnabled bool
	ReplaceEnabled   bool
}

// NewTerraformPluginSet creates a new TerraformPluginSet with default configuration.
func NewTerraformPluginSet() *TerraformPluginSet {
	ps := &TerraformPluginSet{
		config: &PluginConfig{
			DeletionEnabled:  true,
			SensitiveEnabled: true,
			ReplaceEnabled:   true,
		},
	}

	ps.BuiltinPluginSet = &sdk.BuiltinPluginSet{
		Name:    "terraform",
		Version: Version,
		Analyzers: []sdk.Analyzer{
			NewDeletionAnalyzer(ps.config),
			NewSensitiveAnalyzer(ps.config),
			NewReplaceAnalyzer(ps.config),
		},
	}

	return ps
}
