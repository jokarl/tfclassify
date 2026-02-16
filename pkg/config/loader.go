// Package config provides HCL configuration loading for tfclassify.
package config

import (
	"fmt"
	"os"

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
	if err := parseClassificationPluginBlocks(&cfg, file.Body, filename); err != nil {
		return nil, err
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// parseClassificationPluginBlocks parses plugin-named blocks (e.g., azurerm {})
// inside classification blocks and populates PluginAnalyzerConfigs.
func parseClassificationPluginBlocks(cfg *Config, body hcl.Body, filename string) error {
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
				case "network_exposure":
					config := &NetworkExposureConfig{}
					if err := parseNetworkExposureConfig(analyzerBlock, config); err != nil {
						return fmt.Errorf("classification %q, %s: %w", classification.Name, pluginName, err)
					}
					analyzerConfig.NetworkExposure = config
				case "keyvault_access":
					config := &KeyVaultAccessConfig{}
					if err := parseKeyVaultAccessConfig(analyzerBlock, config); err != nil {
						return fmt.Errorf("classification %q, %s: %w", classification.Name, pluginName, err)
					}
					analyzerConfig.KeyVaultAccess = config
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
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return fmt.Errorf("privilege_escalation.score_threshold: %v", diags.Error())
			}
			num, _ := val.AsBigFloat().Int64()
			config.ScoreThreshold = int(num)
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
		default:
			return fmt.Errorf("privilege_escalation: unknown attribute %q", name)
		}
	}
	return nil
}

// parseNetworkExposureConfig parses attributes from a network_exposure block.
func parseNetworkExposureConfig(block *hclsyntax.Block, config *NetworkExposureConfig) error {
	for name, attr := range block.Body.Attributes {
		switch name {
		case "permissive_sources":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return fmt.Errorf("network_exposure.permissive_sources: %v", diags.Error())
			}
			config.PermissiveSources = toStringSlice(val)
		default:
			return fmt.Errorf("network_exposure: unknown attribute %q", name)
		}
	}
	return nil
}

// parseKeyVaultAccessConfig parses attributes from a keyvault_access block.
func parseKeyVaultAccessConfig(block *hclsyntax.Block, config *KeyVaultAccessConfig) error {
	for name, attr := range block.Body.Attributes {
		switch name {
		case "destructive_permissions":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return fmt.Errorf("keyvault_access.destructive_permissions: %v", diags.Error())
			}
			config.DestructivePermissions = toStringSlice(val)
		default:
			return fmt.Errorf("keyvault_access: unknown attribute %q", name)
		}
	}
	return nil
}

// toStringSlice converts a cty.Value list to []string.
func toStringSlice(val cty.Value) []string {
	if val.IsNull() || !val.CanIterateElements() {
		return nil
	}
	var result []string
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
	var errMsg string
	for _, diag := range diags {
		if diag.Severity == hcl.DiagError {
			if diag.Subject != nil {
				errMsg += fmt.Sprintf("%s:%d:%d: %s: %s\n",
					filename,
					diag.Subject.Start.Line,
					diag.Subject.Start.Column,
					diag.Summary,
					diag.Detail)
			} else {
				errMsg += fmt.Sprintf("%s: %s: %s\n", filename, diag.Summary, diag.Detail)
			}
		}
	}
	return fmt.Errorf("configuration error:\n%s", errMsg)
}
