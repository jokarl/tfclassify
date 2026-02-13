// Package main provides the bundled Terraform plugin for tfclassify.
package main

import (
	"fmt"

	"github.com/jokarl/tfclassify/sdk"
)

// ReplaceAnalyzer identifies resources being replaced (destroy + create)
// which may cause downtime.
type ReplaceAnalyzer struct {
	sdk.DefaultAnalyzer
	config *PluginConfig
}

// NewReplaceAnalyzer creates a new ReplaceAnalyzer.
func NewReplaceAnalyzer(config *PluginConfig) *ReplaceAnalyzer {
	return &ReplaceAnalyzer{config: config}
}

// Name returns the analyzer name.
func (a *ReplaceAnalyzer) Name() string {
	return "replace"
}

// Enabled returns whether this analyzer is enabled.
func (a *ReplaceAnalyzer) Enabled() bool {
	return a.config.ReplaceEnabled
}

// ResourcePatterns returns the patterns this analyzer is interested in.
// Returns ["*"] to match all resources.
func (a *ReplaceAnalyzer) ResourcePatterns() []string {
	return []string{"*"}
}

// Analyze inspects resources for replacements and emits decisions.
func (a *ReplaceAnalyzer) Analyze(runner sdk.Runner) error {
	changes, err := runner.GetResourceChanges(a.ResourcePatterns())
	if err != nil {
		return fmt.Errorf("failed to get resource changes: %w", err)
	}

	for _, change := range changes {
		if isReplace(change.Actions) {
			decision := &sdk.Decision{
				Classification: "", // Let host determine based on severity
				Reason:         fmt.Sprintf("Resource %s will be replaced (destroy and recreate)", change.Address),
				Severity:       75,
				Metadata: map[string]interface{}{
					"analyzer": "replace",
					"action":   "replace",
				},
			}

			if err := runner.EmitDecision(a, change, decision); err != nil {
				return fmt.Errorf("failed to emit decision: %w", err)
			}
		}
	}

	return nil
}

// isReplace returns true if the actions indicate a resource replacement.
func isReplace(actions []string) bool {
	hasDelete := false
	hasCreate := false

	for _, action := range actions {
		if action == "delete" {
			hasDelete = true
		}
		if action == "create" {
			hasCreate = true
		}
	}

	// Replace = both delete and create
	return hasDelete && hasCreate
}
