// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	"fmt"
	"strings"

	"github.com/jokarl/tfclassify/sdk"
)

// NetworkExposureAnalyzer detects overly permissive network security rules.
// It inspects source_address_prefix on inbound allow rules and detects
// when they use permissive sources like "*" or "0.0.0.0/0".
type NetworkExposureAnalyzer struct {
	sdk.DefaultAnalyzer
	config *PluginConfig
}

// NewNetworkExposureAnalyzer creates a new NetworkExposureAnalyzer.
func NewNetworkExposureAnalyzer(config *PluginConfig) *NetworkExposureAnalyzer {
	return &NetworkExposureAnalyzer{config: config}
}

// Name returns the analyzer name.
func (a *NetworkExposureAnalyzer) Name() string {
	return "network-exposure"
}

// Enabled returns whether this analyzer is enabled.
func (a *NetworkExposureAnalyzer) Enabled() bool {
	return a.config.NetworkEnabled
}

// ResourcePatterns returns the patterns this analyzer is interested in.
func (a *NetworkExposureAnalyzer) ResourcePatterns() []string {
	return []string{"azurerm_network_security_rule"}
}

// Analyze inspects network security rules for overly permissive sources.
func (a *NetworkExposureAnalyzer) Analyze(runner sdk.Runner) error {
	changes, err := runner.GetResourceChanges(a.ResourcePatterns())
	if err != nil {
		return fmt.Errorf("failed to get resource changes: %w", err)
	}

	permissive := toSet(a.config.PermissiveSources)

	for _, change := range changes {
		// Inspect the "after" state (what the rule will become)
		after := change.After
		if after == nil {
			continue // Rule is being deleted
		}

		// Check if this is an inbound allow rule
		direction := strings.ToLower(stringField(after, "direction"))
		access := strings.ToLower(stringField(after, "access"))

		if direction != "inbound" || access != "allow" {
			continue // Only care about inbound allow rules
		}

		// Check source_address_prefix
		source := stringField(after, "source_address_prefix")
		if source == "" {
			// Try source_address_prefixes (array)
			if prefixes, ok := after["source_address_prefixes"].([]interface{}); ok {
				for _, p := range prefixes {
					if s, ok := p.(string); ok && permissive[s] {
						source = s
						break
					}
				}
			}
		}

		if permissive[source] {
			decision := &sdk.Decision{
				Classification: "",
				Reason:         fmt.Sprintf("inbound allow rule with overly permissive source %q", source),
				Severity:       85,
				Metadata: map[string]interface{}{
					"analyzer":    "network-exposure",
					"direction":   direction,
					"access":      access,
					"source":      source,
					"rule_name":   stringField(after, "name"),
				},
			}

			if err := runner.EmitDecision(a, change, decision); err != nil {
				return fmt.Errorf("failed to emit decision: %w", err)
			}
		}
	}

	return nil
}
