// Package config provides HCL configuration loading for tfclassify.
package config

import (
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
}

// RuleConfig represents a classification rule.
type RuleConfig struct {
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
