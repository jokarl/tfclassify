// Package main provides the bundled Terraform plugin for tfclassify.
package main

import (
	"fmt"

	"github.com/jokarl/tfclassify/sdk"
)

// DeletionAnalyzer classifies resource deletions, distinguishing standalone
// deletes from delete-and-recreate (replace) operations.
type DeletionAnalyzer struct {
	sdk.DefaultAnalyzer
	config *PluginConfig
}

// NewDeletionAnalyzer creates a new DeletionAnalyzer.
func NewDeletionAnalyzer(config *PluginConfig) *DeletionAnalyzer {
	return &DeletionAnalyzer{config: config}
}

// Name returns the analyzer name.
func (a *DeletionAnalyzer) Name() string {
	return "deletion"
}

// Enabled returns whether this analyzer is enabled.
func (a *DeletionAnalyzer) Enabled() bool {
	return a.config.DeletionEnabled
}

// ResourcePatterns returns the patterns this analyzer is interested in.
// Returns ["*"] to match all resources.
func (a *DeletionAnalyzer) ResourcePatterns() []string {
	return []string{"*"}
}

// Analyze inspects resources for standalone deletions and emits decisions.
func (a *DeletionAnalyzer) Analyze(runner sdk.Runner) error {
	changes, err := runner.GetResourceChanges(a.ResourcePatterns())
	if err != nil {
		return fmt.Errorf("failed to get resource changes: %w", err)
	}

	for _, change := range changes {
		if isStandaloneDelete(change.Actions) {
			decision := &sdk.Decision{
				Classification: "", // Let host determine based on severity
				Reason:         fmt.Sprintf("Resource %s is being deleted", change.Address),
				Severity:       80,
				Metadata: map[string]interface{}{
					"analyzer": "deletion",
					"action":   "delete",
				},
			}

			if err := runner.EmitDecision(a, change, decision); err != nil {
				return fmt.Errorf("failed to emit decision: %w", err)
			}
		}
	}

	return nil
}

// isStandaloneDelete returns true if the actions indicate a standalone delete
// (not part of a replace operation).
func isStandaloneDelete(actions []string) bool {
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

	// Standalone delete = delete without create
	return hasDelete && !hasCreate
}
