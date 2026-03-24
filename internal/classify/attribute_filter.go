package classify

import (
	"sort"
	"strings"

	"github.com/jokarl/tfclassify/internal/plan"
)

// FilterCosmeticChanges preprocesses plan changes, downgrading cosmetic-only
// updates to no-op. A change is cosmetic if ALL its changed attributes are
// covered by the ignoreAttrs prefixes. Only ["update"] actions are evaluated;
// creates, deletes, and replacements are never affected.
//
// When a resource is downgraded, its OriginalActions and IgnoredAttributes
// fields are populated for output visibility.
func FilterCosmeticChanges(changes []plan.ResourceChange, ignoreAttrs []string) {
	if len(ignoreAttrs) == 0 {
		return
	}

	for i := range changes {
		change := &changes[i]

		// Only evaluate pure update actions
		if !isUpdateOnly(change.Actions) {
			continue
		}

		// Updates always have both Before and After, but guard against edge cases
		if change.Before == nil || change.After == nil {
			continue
		}

		// Fast path: check if all changes are covered by ignore prefixes
		if !hasOnlyIgnoredChanges(change.Before, change.After, "", ignoreAttrs) {
			continue
		}

		// All changes are covered. Collect the specific paths for annotation.
		changedPaths := collectChangedPaths(change.Before, change.After, "")
		if len(changedPaths) == 0 {
			continue
		}

		change.OriginalActions = change.Actions
		change.IgnoredAttributes = changedPaths
		change.Actions = []string{"no-op"}
	}
}

// hasOnlyIgnoredChanges returns true if every changed attribute between before
// and after (at the given path prefix) is covered by the ignore prefixes.
// It short-circuits on the first uncovered change for performance.
func hasOnlyIgnoredChanges(before, after map[string]interface{}, pathPrefix string, ignoreAttrs []string) bool {
	for _, key := range mergedKeys(before, after) {
		path := key
		if pathPrefix != "" {
			path = pathPrefix + "." + key
		}

		bv, av := before[key], after[key]
		if !valueChanged(bv, av) {
			continue
		}

		// Check if this path is directly covered by a prefix
		if isPathCovered(path, ignoreAttrs) {
			continue
		}

		// Not directly covered. If both values are maps and a nested ignore
		// prefix exists under this path, recurse to check sub-attributes.
		bMap, bOk := bv.(map[string]interface{})
		aMap, aOk := av.(map[string]interface{})
		if bOk && aOk && hasNestedPrefix(path, ignoreAttrs) {
			if !hasOnlyIgnoredChanges(bMap, aMap, path, ignoreAttrs) {
				return false
			}
			continue
		}

		// Uncovered change found — not cosmetic
		return false
	}
	return true
}

// collectChangedPaths returns dot-delimited paths of all attributes that
// differ between before and after. Recurses into nested maps to report
// leaf-level paths (e.g., "tags.tf-module-l2" rather than just "tags").
func collectChangedPaths(before, after map[string]interface{}, pathPrefix string) []string {
	var paths []string

	for _, key := range mergedKeys(before, after) {
		path := key
		if pathPrefix != "" {
			path = pathPrefix + "." + key
		}

		bv, av := before[key], after[key]
		if !valueChanged(bv, av) {
			continue
		}

		// If both are maps, recurse to find specific sub-paths
		bMap, bOk := bv.(map[string]interface{})
		aMap, aOk := av.(map[string]interface{})
		if bOk && aOk {
			paths = append(paths, collectChangedPaths(bMap, aMap, path)...)
			continue
		}

		paths = append(paths, path)
	}

	return paths
}

// isPathCovered returns true if path equals a prefix or has a prefix as a
// dot-delimited ancestor. For example, prefix "tags" covers "tags" and
// "tags.env" but NOT "tags_all".
func isPathCovered(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if path == prefix {
			return true
		}
		if len(path) > len(prefix) && strings.HasPrefix(path, prefix) && path[len(prefix)] == '.' {
			return true
		}
	}
	return false
}

// hasNestedPrefix returns true if any prefix starts with path + ".",
// meaning there's a more specific ignore entry beneath this path.
func hasNestedPrefix(path string, prefixes []string) bool {
	needle := path + "."
	for _, prefix := range prefixes {
		if strings.HasPrefix(prefix, needle) {
			return true
		}
	}
	return false
}

// isUpdateOnly returns true if the actions slice is exactly ["update"].
func isUpdateOnly(actions []string) bool {
	return len(actions) == 1 && actions[0] == "update"
}

// mergedKeys returns the sorted union of keys from two maps.
// Sorting ensures deterministic output order.
func mergedKeys(m1, m2 map[string]interface{}) []string {
	seen := make(map[string]struct{}, len(m1)+len(m2))
	for k := range m1 {
		seen[k] = struct{}{}
	}
	for k := range m2 {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
