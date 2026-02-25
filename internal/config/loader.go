// Package config provides HCL configuration loading for tfclassify.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Load discovers and loads the configuration file.
// If explicitPath is provided, it uses that path directly.
// Otherwise, it searches for config files in standard locations.
func Load(explicitPath string) (*Config, error) {
	path, err := Discover(explicitPath)
	if err != nil {
		return nil, err
	}

	return LoadFile(path)
}

// LoadFile loads configuration from a specific file path.
func LoadFile(path string) (*Config, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	return Parse(src, path)
}

// Parse parses HCL configuration from bytes.
func Parse(src []byte, filename string) (*Config, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(src, filename)
	if diags.HasErrors() {
		return nil, formatDiagnostics(diags, filename)
	}

	var cfg Config
	decodeDiags := gohcl.DecodeBody(file.Body, nil, &cfg)
	if decodeDiags.HasErrors() {
		return nil, formatDiagnostics(decodeDiags, filename)
	}

	// Parse plugin-named blocks inside classification blocks
	if err := parseClassificationPluginBlocks(&cfg, file.Body); err != nil {
		return nil, err
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// parseClassificationPluginBlocks parses plugin-named blocks (e.g., azurerm {})
// inside classification blocks and populates PluginAnalyzerConfigs.
func parseClassificationPluginBlocks(cfg *Config, body hcl.Body) error {
	// Get the list of enabled plugin names
	enabledPlugins := make(map[string]bool)
	for _, p := range cfg.Plugins {
		enabledPlugins[p.Name] = p.Enabled
	}

	// Get the hcl.Body as hclsyntax.Body to iterate blocks
	syntaxBody, ok := body.(*hclsyntax.Body)
	if !ok {
		// If we can't access the syntax body directly, skip plugin block parsing
		return nil
	}

	// Build a map of classification blocks by name
	classificationBlocks := make(map[string]*hclsyntax.Block)
	for _, block := range syntaxBody.Blocks {
		if block.Type == "classification" && len(block.Labels) > 0 {
			classificationBlocks[block.Labels[0]] = block
		}
	}

	// For each classification in the config, check for plugin-named sub-blocks
	for i := range cfg.Classifications {
		classification := &cfg.Classifications[i]
		block, ok := classificationBlocks[classification.Name]
		if !ok {
			continue
		}

		// Initialize the map
		classification.PluginAnalyzerConfigs = make(map[string]*PluginAnalyzerConfig)

		// Iterate through sub-blocks in the classification block
		for _, subBlock := range block.Body.Blocks {
			// Skip rule blocks - they're already handled
			if subBlock.Type == "rule" {
				continue
			}

			// Parse blast_radius blocks
			if subBlock.Type == "blast_radius" {
				brConfig := &BlastRadiusConfig{}
				if err := parseBlastRadiusConfig(subBlock, brConfig); err != nil {
					return fmt.Errorf("classification %q: %w", classification.Name, err)
				}
				classification.BlastRadius = brConfig
				continue
			}

			// This is a plugin-named block (e.g., "azurerm")
			pluginName := subBlock.Type

			// Record the reference even if plugin isn't enabled, so validation catches it
			if _, exists := enabledPlugins[pluginName]; !exists {
				// Add empty config to record the reference for validation
				classification.PluginAnalyzerConfigs[pluginName] = &PluginAnalyzerConfig{}
				continue
			}

			// Parse the analyzer sub-blocks within the plugin block
			analyzerConfig := &PluginAnalyzerConfig{}

			for _, analyzerBlock := range subBlock.Body.Blocks {
				switch analyzerBlock.Type {
				case "privilege_escalation":
					config := &PrivilegeEscalationConfig{}
					if err := parsePrivilegeEscalationConfig(analyzerBlock, config); err != nil {
						return fmt.Errorf("classification %q, %s: %w", classification.Name, pluginName, err)
					}
					analyzerConfig.PrivilegeEscalation = config
				default:
					return fmt.Errorf("classification %q, %s: unknown analyzer %q", classification.Name, pluginName, analyzerBlock.Type)
				}
			}

			classification.PluginAnalyzerConfigs[pluginName] = analyzerConfig
		}
	}

	return nil
}

// parsePrivilegeEscalationConfig parses attributes from a privilege_escalation block.
func parsePrivilegeEscalationConfig(block *hclsyntax.Block, config *PrivilegeEscalationConfig) error {
	for name, attr := range block.Body.Attributes {
		switch name {
		case "score_threshold":
			return fmt.Errorf("privilege_escalation: score_threshold is no longer supported; use actions/data_actions for pattern-based detection (see CR-0028)")
		case "roles":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return fmt.Errorf("privilege_escalation.roles: %v", diags.Error())
			}
			config.Roles = toStringSlice(val)
		case "exclude":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return fmt.Errorf("privilege_escalation.exclude: %v", diags.Error())
			}
			config.Exclude = toStringSlice(val)
		case "actions":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return fmt.Errorf("privilege_escalation.actions: %v", diags.Error())
			}
			config.Actions = toStringSlice(val)
		case "data_actions":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return fmt.Errorf("privilege_escalation.data_actions: %v", diags.Error())
			}
			config.DataActions = toStringSlice(val)
		case "scopes":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return fmt.Errorf("privilege_escalation.scopes: %v", diags.Error())
			}
			config.Scopes = toStringSlice(val)
		case "flag_unknown_roles":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return fmt.Errorf("privilege_escalation.flag_unknown_roles: %v", diags.Error())
			}
			b := val.True()
			config.FlagUnknownRoles = &b
		case "merge_principal_roles":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return fmt.Errorf("privilege_escalation.merge_principal_roles: %v", diags.Error())
			}
			b := val.True()
			config.MergePrincipalRoles = &b
		default:
			return fmt.Errorf("privilege_escalation: unknown attribute %q", name)
		}
	}
	return nil
}


// parseBlastRadiusConfig parses attributes from a blast_radius block.
func parseBlastRadiusConfig(block *hclsyntax.Block, config *BlastRadiusConfig) error {
	for name, attr := range block.Body.Attributes {
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return fmt.Errorf("blast_radius.%s: %v", name, diags.Error())
		}

		switch name {
		case "max_deletions":
			n, _ := val.AsBigFloat().Int64()
			v := int(n)
			config.MaxDeletions = &v
		case "max_replacements":
			n, _ := val.AsBigFloat().Int64()
			v := int(n)
			config.MaxReplacements = &v
		case "max_changes":
			n, _ := val.AsBigFloat().Int64()
			v := int(n)
			config.MaxChanges = &v
		default:
			return fmt.Errorf("blast_radius: unknown attribute %q", name)
		}
	}
	return nil
}

// toStringSlice converts a cty.Value list to []string.
func toStringSlice(val cty.Value) []string {
	if val.IsNull() || !val.CanIterateElements() {
		return nil
	}
	result := make([]string, 0, val.LengthInt())
	for it := val.ElementIterator(); it.Next(); {
		_, v := it.Element()
		if v.Type() == cty.String {
			result = append(result, v.AsString())
		}
	}
	return result
}

// formatDiagnostics converts HCL diagnostics into a readable error.
func formatDiagnostics(diags hcl.Diagnostics, filename string) error {
	var sb strings.Builder
	for _, diag := range diags {
		if diag.Severity == hcl.DiagError {
			if diag.Subject != nil {
				fmt.Fprintf(&sb, "%s:%d:%d: %s: %s\n",
					filename,
					diag.Subject.Start.Line,
					diag.Subject.Start.Column,
					diag.Summary,
					diag.Detail)
			} else {
				fmt.Fprintf(&sb, "%s: %s: %s\n", filename, diag.Summary, diag.Detail)
			}
		}
	}
	return fmt.Errorf("configuration error:\n%s", sb.String())
}
