// Package config provides HCL configuration loading for tfclassify.
package config

import (
	"fmt"
	"io"
)

// Validate checks that the configuration is valid.
func Validate(cfg *Config) error {
	// Build classification names set once for all validators
	classificationNames := make(map[string]bool, len(cfg.Classifications))
	for _, c := range cfg.Classifications {
		classificationNames[c.Name] = true
	}

	if err := validatePrecedence(cfg, classificationNames); err != nil {
		return err
	}

	if err := validateDefaults(cfg, classificationNames); err != nil {
		return err
	}

	if err := validateClassifications(cfg); err != nil {
		return err
	}

	if err := validateRules(cfg); err != nil {
		return err
	}

	if err := validatePluginReferences(cfg); err != nil {
		return err
	}

	return nil
}

// validatePrecedence checks that all precedence entries reference defined classifications.
func validatePrecedence(cfg *Config, classificationNames map[string]bool) error {
	if len(cfg.Precedence) == 0 {
		return fmt.Errorf("precedence must not be empty")
	}

	for _, name := range cfg.Precedence {
		if !classificationNames[name] {
			return fmt.Errorf("precedence references undefined classification %q", name)
		}
	}

	return nil
}

// validateDefaults checks that default values reference valid classifications.
func validateDefaults(cfg *Config, classificationNames map[string]bool) error {
	if cfg.Defaults == nil {
		return fmt.Errorf("defaults block is required")
	}

	if cfg.Defaults.Unclassified != "" && !classificationNames[cfg.Defaults.Unclassified] {
		return fmt.Errorf("defaults.unclassified references undefined classification %q", cfg.Defaults.Unclassified)
	}

	if cfg.Defaults.NoChanges != "" && !classificationNames[cfg.Defaults.NoChanges] {
		return fmt.Errorf("defaults.no_changes references undefined classification %q", cfg.Defaults.NoChanges)
	}

	return nil
}

// validateClassifications checks for duplicate classification names.
func validateClassifications(cfg *Config) error {
	seen := make(map[string]bool)
	for _, c := range cfg.Classifications {
		if seen[c.Name] {
			return fmt.Errorf("duplicate classification name %q", c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}

// validateRules checks that each rule has at least resource or not_resource defined.
func validateRules(cfg *Config) error {
	for _, classification := range cfg.Classifications {
		for i, rule := range classification.Rules {
			if len(rule.Resource) == 0 && len(rule.NotResource) == 0 {
				return fmt.Errorf("classification %q rule %d: rule must specify resource or not_resource",
					classification.Name, i+1)
			}
		}
	}
	return nil
}

// validatePluginReferences checks that plugin blocks in classifications reference enabled plugins.
func validatePluginReferences(cfg *Config) error {
	// Build a map of enabled plugins
	enabledPlugins := make(map[string]bool)
	for _, p := range cfg.Plugins {
		if p.Enabled {
			enabledPlugins[p.Name] = true
		}
	}

	// Check each classification's plugin references
	for _, classification := range cfg.Classifications {
		for pluginName := range classification.PluginAnalyzerConfigs {
			if !enabledPlugins[pluginName] {
				return fmt.Errorf("classification %q references plugin %q which is not enabled",
					classification.Name, pluginName)
			}
		}
	}
	return nil
}

// WarnRedundantNotResource emits a warning to w when a not_resource rule
// contains only patterns that are already present in higher-precedence
// resource rules. In such cases, using resource = ["*"] is simpler and
// less error-prone because the precedence-ordered evaluation already
// ensures higher-priority classifications match first.
//
// This function is intended to be called with verbose mode enabled.
func WarnRedundantNotResource(cfg *Config, w io.Writer) {
	// Build a map of classifications by name for quick lookup
	classificationByName := make(map[string]*ClassificationConfig)
	for i := range cfg.Classifications {
		classificationByName[cfg.Classifications[i].Name] = &cfg.Classifications[i]
	}

	// Collect resource patterns from each classification in precedence order
	higherPatterns := make(map[string]bool)

	for _, classificationName := range cfg.Precedence {
		classification, ok := classificationByName[classificationName]
		if !ok {
			continue
		}

		for ruleIdx, rule := range classification.Rules {
			// Check if this not_resource list is fully covered by higher-precedence patterns
			if len(rule.NotResource) > 0 && allPatternsKnown(rule.NotResource, higherPatterns) {
				fmt.Fprintf(w, "Warning: classification %q rule %d uses not_resource with patterns "+
					"already covered by higher-precedence rules. Consider using resource = [\"*\"] instead.\n",
					classificationName, ruleIdx+1)
			}

			// Accumulate resource patterns for lower-precedence checks
			for _, pattern := range rule.Resource {
				higherPatterns[pattern] = true
			}
		}
	}
}

// allPatternsKnown returns true if every pattern in patterns exists in known.
func allPatternsKnown(patterns []string, known map[string]bool) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		if !known[p] {
			return false
		}
	}
	return true
}
