// Package config provides HCL configuration loading for tfclassify.
package config

import (
	"fmt"
)

// Validate checks that the configuration is valid.
func Validate(cfg *Config) error {
	if err := validatePrecedence(cfg); err != nil {
		return err
	}

	if err := validateDefaults(cfg); err != nil {
		return err
	}

	if err := validateClassifications(cfg); err != nil {
		return err
	}

	if err := validateRules(cfg); err != nil {
		return err
	}

	return nil
}

// validatePrecedence checks that all precedence entries reference defined classifications.
func validatePrecedence(cfg *Config) error {
	if len(cfg.Precedence) == 0 {
		return fmt.Errorf("precedence must not be empty")
	}

	classificationNames := make(map[string]bool)
	for _, c := range cfg.Classifications {
		classificationNames[c.Name] = true
	}

	for _, name := range cfg.Precedence {
		if !classificationNames[name] {
			return fmt.Errorf("precedence references undefined classification %q", name)
		}
	}

	return nil
}

// validateDefaults checks that default values reference valid classifications.
func validateDefaults(cfg *Config) error {
	if cfg.Defaults == nil {
		return fmt.Errorf("defaults block is required")
	}

	classificationNames := make(map[string]bool)
	for _, c := range cfg.Classifications {
		classificationNames[c.Name] = true
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
