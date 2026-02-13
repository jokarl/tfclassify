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

// toStringSlice converts an interface{} to []string for parsing permission arrays from plan JSON.
// Handles nil, non-slice, and mixed-type inputs gracefully.
func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}

	slice, ok := v.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
