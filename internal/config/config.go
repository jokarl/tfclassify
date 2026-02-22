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
	Evidence        *EvidenceConfig        `hcl:"evidence,block"`
}

// EvidenceConfig holds configuration for the evidence artifact output.
type EvidenceConfig struct {
	IncludeTrace     bool   `hcl:"include_trace,optional"`
	IncludeResources *bool  `hcl:"include_resources,optional"`
	SigningKey       string `hcl:"signing_key,optional"`
}

// ShouldIncludeResources returns whether resources should be included.
// Default is true when not explicitly set.
func (e *EvidenceConfig) ShouldIncludeResources() bool {
	if e == nil {
		return true
	}
	if e.IncludeResources == nil {
		return true
	}
	return *e.IncludeResources
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
	SARIFLevel  string       `hcl:"sarif_level,optional"`
	Rules       []RuleConfig `hcl:"rule,block"`

	// BlastRadius holds the optional blast radius thresholds for this classification.
	// Populated during parsing from blast_radius {} blocks.
	BlastRadius *BlastRadiusConfig

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
	// Roles limits triggering to specific role names (case-insensitive).
	// Empty means any role can trigger.
	Roles []string `hcl:"roles,optional" json:"roles,omitempty"`

	// Exclude is a list of role names to skip (case-insensitive).
	Exclude []string `hcl:"exclude,optional" json:"exclude,omitempty"`

	// Actions is a list of Azure RBAC control-plane action patterns to match.
	// When configured, roles with effective actions matching ANY pattern trigger.
	// Uses Azure RBAC pattern matching: "*", "Microsoft.Authorization/*", "*/write".
	// If omitted or empty, control-plane actions are not evaluated.
	// CR-0028: Pattern-Based Control-Plane Detection
	Actions []string `hcl:"actions,optional" json:"actions,omitempty"`

	// DataActions is a list of Azure RBAC data-plane action patterns to match.
	// When configured, roles with effective data actions matching ANY pattern trigger.
	// Uses Azure RBAC pattern matching: "*", "Microsoft.Storage/*", "*/read".
	// If omitted or empty, data-plane actions are not evaluated.
	// CR-0027: Data-Plane Action Detection
	DataActions []string `hcl:"data_actions,optional" json:"data_actions,omitempty"`

	// Scopes limits triggering to specific ARM scope levels.
	// Values: "management_group", "subscription", "resource_group", "resource".
	// If omitted or empty, matches any scope level.
	// CR-0028: Pattern-Based Control-Plane Detection
	Scopes []string `hcl:"scopes,optional" json:"scopes,omitempty"`

	// FlagUnknownRoles controls whether roles whose permissions cannot be resolved
	// emit a decision with diagnostic metadata. Default: true.
	// CR-0028: Pattern-Based Control-Plane Detection
	FlagUnknownRoles *bool `hcl:"flag_unknown_roles,optional" json:"flag_unknown_roles,omitempty"`
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

// BlastRadiusConfig holds thresholds for the blast radius analyzer.
// Each field is a pointer so that omitted fields are distinguishable from zero.
type BlastRadiusConfig struct {
	// MaxDeletions triggers when standalone deletions exceed this count.
	MaxDeletions *int `json:"max_deletions,omitempty"`
	// MaxReplacements triggers when replacements (delete+create) exceed this count.
	MaxReplacements *int `json:"max_replacements,omitempty"`
	// MaxChanges triggers when total non-no-op changes exceed this count.
	MaxChanges *int `json:"max_changes,omitempty"`
}

// RuleConfig represents a classification rule.
type RuleConfig struct {
	Description string   `hcl:"description,optional"`
	Resource    []string `hcl:"resource,optional"`
	NotResource []string `hcl:"not_resource,optional"`
	Actions     []string `hcl:"actions,optional"`
	NotActions  []string `hcl:"not_actions,optional"`
}

// DefaultsConfig contains default configuration values.
type DefaultsConfig struct {
	Unclassified  string `hcl:"unclassified"`
	NoChanges     string `hcl:"no_changes"`
	PluginTimeout string `hcl:"plugin_timeout,optional"`
}
