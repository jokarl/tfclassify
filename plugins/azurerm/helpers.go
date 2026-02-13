// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import "github.com/jokarl/tfclassify/sdk"

// Compile-time interface assertions to verify our analyzers implement the sdk.Analyzer interface.
var (
	_ sdk.Analyzer = (*PrivilegeEscalationAnalyzer)(nil)
	_ sdk.Analyzer = (*NetworkExposureAnalyzer)(nil)
	_ sdk.Analyzer = (*KeyVaultAccessAnalyzer)(nil)
)

// toSet converts a slice to a set (map with bool values).
func toSet(slice []string) map[string]bool {
	set := make(map[string]bool, len(slice))
	for _, s := range slice {
		set[s] = true
	}
	return set
}

// stringField extracts a string field from a map, returning empty string if not found.
func stringField(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
