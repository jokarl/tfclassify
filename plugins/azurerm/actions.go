// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ActionRegistry provides fast lookup and expansion of Azure RBAC actions.
// It loads a provider-keyed catalog of all known Azure operations from
// embedded JSON data, enabling proper wildcard expansion in
// computeEffectiveActions.
type ActionRegistry struct {
	actions        map[string][]string // lowercase provider → sorted action names
	dataActions    map[string][]string // lowercase provider → sorted data action names
	allActions     []string            // cached flat sorted slice (control-plane)
	allDataActions []string            // cached flat sorted slice (data-plane)
}

// actionRegistryData is the raw embedded JSON for the action registry.
//
//go:embed actiondata/actions.json
var actionRegistryData []byte

var (
	defaultRegistry     *ActionRegistry
	defaultRegistryOnce sync.Once
	defaultRegistryErr  error
)

// DefaultActionRegistry returns a singleton ActionRegistry loaded from the
// embedded JSON. It is safe for concurrent access from multiple goroutines.
// Panics if the embedded data is malformed.
func DefaultActionRegistry() *ActionRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry, defaultRegistryErr = NewActionRegistryFromJSON(actionRegistryData)
		if defaultRegistryErr != nil {
			panic(fmt.Sprintf("failed to initialize default action registry: %v", defaultRegistryErr))
		}
	})
	return defaultRegistry
}

// actionRegistryJSON is the JSON schema for the embedded action data.
type actionRegistryJSON struct {
	Actions     map[string][]string `json:"actions"`
	DataActions map[string][]string `json:"dataActions"`
}

// NewActionRegistryFromJSON creates an ActionRegistry from JSON data.
func NewActionRegistryFromJSON(data []byte) (*ActionRegistry, error) {
	var raw actionRegistryJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse action registry: %w", err)
	}

	reg := &ActionRegistry{
		actions:     make(map[string][]string, len(raw.Actions)),
		dataActions: make(map[string][]string, len(raw.DataActions)),
	}

	// Normalize keys to lowercase and build flat caches
	for provider, actions := range raw.Actions {
		key := strings.ToLower(provider)
		reg.actions[key] = actions
		reg.allActions = append(reg.allActions, actions...)
	}
	for provider, actions := range raw.DataActions {
		key := strings.ToLower(provider)
		reg.dataActions[key] = actions
		reg.allDataActions = append(reg.allDataActions, actions...)
	}

	sort.Strings(reg.allActions)
	sort.Strings(reg.allDataActions)

	return reg, nil
}

// ExpandPattern expands a single wildcard pattern to concrete actions.
// If dataPlane is true, expands against data-plane actions; otherwise control-plane.
//
// Pattern strategies:
//   - "*" → return all actions (cached flat slice)
//   - "Microsoft.Storage/*" → map lookup by provider key (O(1))
//   - "*/read" → linear scan of flat slice, filter by suffix (O(N))
//   - Exact match → return as-is (no expansion needed)
func (r *ActionRegistry) ExpandPattern(pattern string, dataPlane bool) []string {
	if r == nil {
		return []string{pattern}
	}

	lower := strings.ToLower(pattern)

	// Global wildcard
	if lower == "*" {
		if dataPlane {
			return r.allDataActions
		}
		return r.allActions
	}

	source := r.actions
	allSource := r.allActions
	if dataPlane {
		source = r.dataActions
		allSource = r.allDataActions
	}

	// Provider wildcard (Microsoft.Storage/*)
	if strings.HasSuffix(lower, "/*") {
		prefix := strings.TrimSuffix(lower, "/*")
		if actions, ok := source[prefix]; ok {
			return actions
		}
		// If provider not found, return the pattern as-is
		return []string{pattern}
	}

	// Suffix wildcard (*/read, */write, */delete, */action)
	if strings.HasPrefix(lower, "*/") {
		suffix := strings.TrimPrefix(lower, "*/")
		var matched []string
		for _, action := range allSource {
			actionLower := strings.ToLower(action)
			if strings.HasSuffix(actionLower, "/"+suffix) || strings.HasSuffix(actionLower, suffix) {
				matched = append(matched, action)
			}
		}
		if len(matched) > 0 {
			return matched
		}
		return nil
	}

	// Exact action — return as-is
	return []string{pattern}
}

// ExpandActions expands and deduplicates a list of patterns to concrete actions.
func (r *ActionRegistry) ExpandActions(patterns []string, dataPlane bool) []string {
	if r == nil || len(patterns) == 0 {
		return patterns
	}

	seen := make(map[string]bool)
	var result []string

	for _, pattern := range patterns {
		expanded := r.ExpandPattern(pattern, dataPlane)
		for _, action := range expanded {
			if !seen[action] {
				seen[action] = true
				result = append(result, action)
			}
		}
	}

	return result
}

// ActionCount returns the number of control-plane actions in the registry.
func (r *ActionRegistry) ActionCount() int {
	if r == nil {
		return 0
	}
	return len(r.allActions)
}

// DataActionCount returns the number of data-plane actions in the registry.
func (r *ActionRegistry) DataActionCount() int {
	if r == nil {
		return 0
	}
	return len(r.allDataActions)
}

// ProviderCount returns the number of unique providers across both planes.
func (r *ActionRegistry) ProviderCount() int {
	if r == nil {
		return 0
	}
	providers := make(map[string]bool)
	for k := range r.actions {
		providers[k] = true
	}
	for k := range r.dataActions {
		providers[k] = true
	}
	return len(providers)
}
