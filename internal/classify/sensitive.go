package classify

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jokarl/tfclassify/internal/plan"
)

// SensitiveAnalyzer detects changes to attributes marked as sensitive by Terraform.
type SensitiveAnalyzer struct{}

// Name returns the analyzer name.
func (a *SensitiveAnalyzer) Name() string {
	return "sensitive"
}

// Analyze inspects resources for sensitive attribute changes.
func (a *SensitiveAnalyzer) Analyze(changes []plan.ResourceChange) []ResourceDecision {
	var decisions []ResourceDecision

	for _, change := range changes {
		sensitiveAttrs := findSensitiveChanges(change)
		if len(sensitiveAttrs) > 0 {
			reason := fmt.Sprintf("builtin: sensitive - Resource %s has changes to sensitive attributes: %s",
				change.Address, strings.Join(sensitiveAttrs, ", "))

			decisions = append(decisions, ResourceDecision{
				Address:      change.Address,
				ResourceType: change.Type,
				Actions:      change.Actions,
				MatchedRules: []string{reason},
			})
		}
	}

	return decisions
}

// findSensitiveChanges walks the sensitive markers recursively and returns
// dot-delimited attribute paths that have sensitive values that changed.
// Terraform's BeforeSensitive/AfterSensitive mirrors the structure of
// Before/After: true at a leaf means sensitive, nested maps/slices indicate
// nested sensitive attributes.
func findSensitiveChanges(change plan.ResourceChange) []string {
	seen := make(map[string]struct{})
	var sensitiveAttrs []string

	// Walk before-sensitive markers
	collectSensitivePaths(change.BeforeSensitive, change.Before, change.After, "", seen, &sensitiveAttrs)

	// Walk after-sensitive markers (picks up newly-sensitive attributes)
	collectSensitivePaths(change.AfterSensitive, change.Before, change.After, "", seen, &sensitiveAttrs)

	return sensitiveAttrs
}

// collectSensitivePaths recursively walks a sensitive marker tree and appends
// changed sensitive attribute paths to the result slice.
func collectSensitivePaths(
	sensMarker interface{},
	before, after interface{},
	prefix string,
	seen map[string]struct{},
	result *[]string,
) {
	if sensMarker == nil {
		return
	}

	switch marker := sensMarker.(type) {
	case bool:
		// Leaf: this attribute is sensitive if marker is true
		if !marker {
			return
		}
		path := prefix
		if path == "" {
			return // shouldn't happen — top-level true without a key
		}
		if _, exists := seen[path]; exists {
			return
		}
		if valueChanged(before, after) {
			seen[path] = struct{}{}
			*result = append(*result, path)
		}

	case map[string]interface{}:
		// Nested object: recurse into each key
		beforeMap, _ := before.(map[string]interface{})
		afterMap, _ := after.(map[string]interface{})
		for key, childMarker := range marker {
			childPath := key
			if prefix != "" {
				childPath = prefix + "." + key
			}
			var childBefore, childAfter interface{}
			if beforeMap != nil {
				childBefore = beforeMap[key]
			}
			if afterMap != nil {
				childAfter = afterMap[key]
			}
			collectSensitivePaths(childMarker, childBefore, childAfter, childPath, seen, result)
		}

	case []interface{}:
		// List: recurse into each indexed element
		beforeSlice, _ := before.([]interface{})
		afterSlice, _ := after.([]interface{})
		for i, childMarker := range marker {
			childPath := fmt.Sprintf("%d", i)
			if prefix != "" {
				childPath = prefix + "." + childPath
			}
			var childBefore, childAfter interface{}
			if i < len(beforeSlice) {
				childBefore = beforeSlice[i]
			}
			if i < len(afterSlice) {
				childAfter = afterSlice[i]
			}
			collectSensitivePaths(childMarker, childBefore, childAfter, childPath, seen, result)
		}
	}
}

// valueChanged checks if two arbitrary values differ.
// Uses type-specific comparison for JSON types to avoid reflect.DeepEqual overhead.
func valueChanged(before, after interface{}) bool {
	if before == nil && after == nil {
		return false
	}
	if before == nil || after == nil {
		return true
	}

	switch b := before.(type) {
	case string:
		if a, ok := after.(string); ok {
			return b != a
		}
	case float64:
		if a, ok := after.(float64); ok {
			return b != a
		}
	case bool:
		if a, ok := after.(bool); ok {
			return b != a
		}
	}

	// Fall back to reflect.DeepEqual for maps, slices, and mismatched types
	return !reflect.DeepEqual(before, after)
}
