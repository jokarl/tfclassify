// Package main provides the bundled Terraform plugin for tfclassify.
package main

import (
	"fmt"
	"strings"

	"github.com/jokarl/tfclassify/sdk"
)

// SensitiveAnalyzer detects changes to attributes marked as sensitive by Terraform.
type SensitiveAnalyzer struct {
	sdk.DefaultAnalyzer
	config *PluginConfig
}

// NewSensitiveAnalyzer creates a new SensitiveAnalyzer.
func NewSensitiveAnalyzer(config *PluginConfig) *SensitiveAnalyzer {
	return &SensitiveAnalyzer{config: config}
}

// Name returns the analyzer name.
func (a *SensitiveAnalyzer) Name() string {
	return "sensitive"
}

// Enabled returns whether this analyzer is enabled.
func (a *SensitiveAnalyzer) Enabled() bool {
	return a.config.SensitiveEnabled
}

// ResourcePatterns returns the patterns this analyzer is interested in.
// Returns ["*"] to match all resources.
func (a *SensitiveAnalyzer) ResourcePatterns() []string {
	return []string{"*"}
}

// Analyze inspects resources for sensitive attribute changes and emits decisions.
func (a *SensitiveAnalyzer) Analyze(runner sdk.Runner) error {
	changes, err := runner.GetResourceChanges(a.ResourcePatterns())
	if err != nil {
		return fmt.Errorf("failed to get resource changes: %w", err)
	}

	for _, change := range changes {
		sensitiveAttrs := findSensitiveChanges(change)
		if len(sensitiveAttrs) > 0 {
			// Create reason without exposing actual values
			reason := fmt.Sprintf("Resource %s has changes to sensitive attributes: %s",
				change.Address, strings.Join(sensitiveAttrs, ", "))

			decision := &sdk.Decision{
				Classification: "", // Let host determine based on severity
				Reason:         reason,
				Severity:       70,
				Metadata: map[string]interface{}{
					"analyzer":             "sensitive",
					"sensitive_attrs":      sensitiveAttrs,
					"sensitive_attr_count": len(sensitiveAttrs),
				},
			}

			if err := runner.EmitDecision(a, change, decision); err != nil {
				return fmt.Errorf("failed to emit decision: %w", err)
			}
		}
	}

	return nil
}

// findSensitiveChanges walks the sensitive markers and returns attribute names
// that have sensitive values that changed.
func findSensitiveChanges(change *sdk.ResourceChange) []string {
	var sensitiveAttrs []string

	// Check BeforeSensitive
	beforeSens := asBoolMap(change.BeforeSensitive)
	afterSens := asBoolMap(change.AfterSensitive)

	// Find all sensitive attributes from before
	for attr := range beforeSens {
		if beforeSens[attr] == true {
			// Check if the value actually changed
			if hasAttributeChanged(attr, change.Before, change.After) {
				sensitiveAttrs = append(sensitiveAttrs, attr)
			}
		}
	}

	// Find sensitive attributes that are newly sensitive in after
	for attr := range afterSens {
		if afterSens[attr] == true && beforeSens[attr] != true {
			if hasAttributeChanged(attr, change.Before, change.After) {
				// Avoid duplicates
				found := false
				for _, existing := range sensitiveAttrs {
					if existing == attr {
						found = true
						break
					}
				}
				if !found {
					sensitiveAttrs = append(sensitiveAttrs, attr)
				}
			}
		}
	}

	return sensitiveAttrs
}

// asBoolMap converts an interface{} to map[string]interface{} for walking.
func asBoolMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

// hasAttributeChanged checks if an attribute value changed between before and after.
func hasAttributeChanged(attr string, before, after map[string]interface{}) bool {
	if before == nil && after == nil {
		return false
	}

	beforeVal, beforeOk := before[attr]
	afterVal, afterOk := after[attr]

	// Attribute added
	if !beforeOk && afterOk {
		return true
	}

	// Attribute removed
	if beforeOk && !afterOk {
		return true
	}

	// Attribute changed (simple comparison, not deep)
	// For sensitive values, we only report that it changed, not what changed
	return fmt.Sprintf("%v", beforeVal) != fmt.Sprintf("%v", afterVal)
}
