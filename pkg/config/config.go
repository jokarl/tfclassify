// Package config provides HCL configuration loading for tfclassify.
package config

import (
	"encoding/json"

	"github.com/hashicorp/hcl/v2"
)

// Config represents the full tfclassify configuration.
type Config struct {
	Plugins         []PluginConfig         `hcl:"plugin,block"`
	Classifications []ClassificationConfig `hcl:"classification,block"`
	Precedence      []string               `hcl:"precedence"`
	Defaults        *DefaultsConfig        `hcl:"defaults,block"`
}

// PluginConfig represents a plugin declaration in the configuration.
// Plugin binary metadata is kept top-level (source, version, enabled).
// Runtime analyzer configuration is now per-classification (see ClassificationConfig.PluginAnalyzerConfigs).
type PluginConfig struct {
	Name    string            `hcl:"name,label"`
	Enabled bool              `hcl:"enabled"`
	Source  string            `hcl:"source,optional"`
	Version string            `hcl:"version,optional"`
	Config  *PluginBodyConfig `hcl:"config,block"`
}

// PluginBodyConfig wraps the raw HCL body for plugin-specific configuration.
// The Remain field captures all attributes in the config block.
type PluginBodyConfig struct {
	Remain hcl.Body `hcl:",remain"`
}

// ClassificationConfig represents a classification definition.
type ClassificationConfig struct {
	Name        string       `hcl:"name,label"`
	Description string       `hcl:"description"`
	Rules       []RuleConfig `hcl:"rule,block"`

	// PluginAnalyzerConfigs holds per-analyzer configuration for each plugin.
	// Key is the plugin name (e.g., "azurerm").
	// This is populated during parsing when plugin-named blocks are found.
	PluginAnalyzerConfigs map[string]*PluginAnalyzerConfig

	// Remain captures plugin-named blocks (e.g., azurerm {}) inside the classification.
	Remain hcl.Body `hcl:",remain"`
}

// PluginAnalyzerConfig holds per-analyzer sub-block configurations for a plugin.
// Each analyzer can have its own thresholds, filters, etc.
type PluginAnalyzerConfig struct {
	// PrivilegeEscalation holds configuration for the privilege-escalation analyzer.
	PrivilegeEscalation *PrivilegeEscalationConfig `hcl:"privilege_escalation,block"`

	// NetworkExposure holds configuration for the network-exposure analyzer.
	NetworkExposure *NetworkExposureConfig `hcl:"network_exposure,block"`

	// KeyVaultAccess holds configuration for the keyvault-access analyzer.
	KeyVaultAccess *KeyVaultAccessConfig `hcl:"keyvault_access,block"`
}

// PrivilegeEscalationConfig holds configuration for the privilege_escalation analyzer.
type PrivilegeEscalationConfig struct {
	// ScoreThreshold is the minimum score required to trigger this classification.
	// Default: 0 (any score triggers)
	ScoreThreshold int `hcl:"score_threshold,optional" json:"score_threshold,omitempty"`

	// Roles limits triggering to specific role names (case-insensitive).
	// Empty means any role can trigger.
	Roles []string `hcl:"roles,optional" json:"roles,omitempty"`

	// Exclude is a list of role names to skip (case-insensitive).
	Exclude []string `hcl:"exclude,optional" json:"exclude,omitempty"`
}

// NetworkExposureConfig holds configuration for the network_exposure analyzer.
type NetworkExposureConfig struct {
	// PermissiveSources overrides the default permissive source detection.
	// Default: ["*", "0.0.0.0/0", "Internet"]
	PermissiveSources []string `hcl:"permissive_sources,optional" json:"permissive_sources,omitempty"`
}

// KeyVaultAccessConfig holds configuration for the keyvault_access analyzer.
type KeyVaultAccessConfig struct {
	// DestructivePermissions overrides the default destructive permission list.
	// Default: ["delete", "purge"]
	DestructivePermissions []string `hcl:"destructive_permissions,optional" json:"destructive_permissions,omitempty"`
}

// ToJSON serializes the analyzer config for gRPC transport.
func (c *PluginAnalyzerConfig) ToJSON() ([]byte, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

// RuleConfig represents a classification rule.
type RuleConfig struct {
	Description string   `hcl:"description,optional"`
	Resource    []string `hcl:"resource,optional"`
	NotResource []string `hcl:"not_resource,optional"`
	Actions     []string `hcl:"actions,optional"`
}

// DefaultsConfig contains default configuration values.
type DefaultsConfig struct {
	Unclassified  string `hcl:"unclassified"`
	NoChanges     string `hcl:"no_changes"`
	PluginTimeout string `hcl:"plugin_timeout,optional"`
}
