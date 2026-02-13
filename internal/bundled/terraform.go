// Package bundled provides the bundled Terraform plugin implementation.
// This package allows the host binary to run as the bundled plugin when
// invoked with --act-as-bundled-plugin.
package bundled

import (
	"fmt"
	"strings"

	"github.com/jokarl/tfclassify/sdk"
	sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
)

// Version is the bundled terraform plugin version.
const Version = "0.1.0"

// ServeTerraform starts the bundled terraform plugin gRPC server.
// This is called when tfclassify is invoked with --act-as-bundled-plugin.
func ServeTerraform() {
	sdkplugin.Serve(&sdkplugin.ServeOpts{
		PluginSet: NewTerraformPluginSet(),
	})
}

// TerraformPluginSet is the main plugin set for the bundled terraform plugin.
type TerraformPluginSet struct {
	*sdk.BuiltinPluginSet
	config *PluginConfig
}

// PluginConfig holds the configuration for the terraform plugin.
type PluginConfig struct {
	DeletionEnabled  bool
	SensitiveEnabled bool
	ReplaceEnabled   bool
}

// NewTerraformPluginSet creates a new TerraformPluginSet with default configuration.
func NewTerraformPluginSet() *TerraformPluginSet {
	ps := &TerraformPluginSet{
		config: &PluginConfig{
			DeletionEnabled:  true,
			SensitiveEnabled: true,
			ReplaceEnabled:   true,
		},
	}

	ps.BuiltinPluginSet = &sdk.BuiltinPluginSet{
		Name:    "terraform",
		Version: Version,
		Analyzers: []sdk.Analyzer{
			NewDeletionAnalyzer(ps.config),
			NewSensitiveAnalyzer(ps.config),
			NewReplaceAnalyzer(ps.config),
		},
	}

	return ps
}

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
				Classification: "",
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

// isStandaloneDelete returns true if the actions indicate a standalone delete.
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

	return hasDelete && !hasCreate
}

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
			reason := fmt.Sprintf("Resource %s has changes to sensitive attributes: %s",
				change.Address, strings.Join(sensitiveAttrs, ", "))

			decision := &sdk.Decision{
				Classification: "",
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

	beforeSens := asBoolMap(change.BeforeSensitive)
	afterSens := asBoolMap(change.AfterSensitive)

	for attr := range beforeSens {
		if beforeSens[attr] == true {
			if hasAttributeChanged(attr, change.Before, change.After) {
				sensitiveAttrs = append(sensitiveAttrs, attr)
			}
		}
	}

	for attr := range afterSens {
		if afterSens[attr] == true && beforeSens[attr] != true {
			if hasAttributeChanged(attr, change.Before, change.After) {
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

	if !beforeOk && afterOk {
		return true
	}

	if beforeOk && !afterOk {
		return true
	}

	return fmt.Sprintf("%v", beforeVal) != fmt.Sprintf("%v", afterVal)
}

// ReplaceAnalyzer identifies resources being replaced (destroy + create).
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
				Classification: "",
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

	return hasDelete && hasCreate
}
