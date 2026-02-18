// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	"strings"
)

// actionMatchesPattern checks if an action matches a pattern using Azure RBAC matching rules.
// Azure RBAC uses glob-like matching:
//   - "*" matches everything
//   - "Microsoft.Compute/*" matches all actions under that provider
//   - "*/read" matches any read action
//   - "Microsoft.Authorization/*/Write" matches any authorization write action
//
// Matching is case-insensitive.
func actionMatchesPattern(action, pattern string) bool {
	action = strings.ToLower(action)
	pattern = strings.ToLower(pattern)

	// Exact match
	if action == pattern {
		return true
	}

	// Global wildcard
	if pattern == "*" {
		return true
	}

	// Suffix wildcard (Microsoft.Compute/*)
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(action, prefix+"/")
	}

	// Prefix wildcard (*/read)
	if strings.HasPrefix(pattern, "*/") {
		suffix := strings.TrimPrefix(pattern, "*/")
		return strings.HasSuffix(action, "/"+suffix) || strings.HasSuffix(action, suffix)
	}

	// Middle wildcard (Microsoft.Authorization/*/Write)
	// Split on "*" and check prefix/suffix matching
	if strings.Contains(pattern, "*") {
		parts := strings.SplitN(pattern, "*", 2)
		if len(parts) == 2 {
			return strings.HasPrefix(action, parts[0]) && strings.HasSuffix(action, parts[1])
		}
	}

	return false
}

// matchesAny checks if an action matches any of the patterns.
func matchesAny(action string, patterns []string) bool {
	for _, p := range patterns {
		if actionMatchesPattern(action, p) {
			return true
		}
	}
	return false
}

// computeEffectiveActions subtracts notActions from actions using pattern matching.
// When actions contain wildcards (e.g., "*", "Microsoft.Storage/*"), they are
// expanded to concrete actions via the action registry before subtraction.
// This enables proper NotActions handling for wildcard roles like Contributor
// (Actions: ["*"], NotActions: ["Microsoft.Authorization/*"]).
func computeEffectiveActions(actions, notActions []string) []string {
	return computeEffectiveActionsWithRegistry(actions, notActions, false, nil)
}

// computeEffectiveActionsWithRegistry subtracts notActions from actions using
// pattern matching, with optional wildcard expansion via the action registry.
// If registry is nil, DefaultActionRegistry() is used.
//
// Wildcard patterns in actions (e.g., "*", "Microsoft.Storage/*") are always
// expanded to concrete actions so that downstream pattern matching works
// correctly against the effective action list.
func computeEffectiveActionsWithRegistry(actions, notActions []string, dataPlane bool, registry *ActionRegistry) []string {
	if registry == nil {
		registry = DefaultActionRegistry()
	}

	// Expand any wildcard patterns in actions to concrete actions
	expanded := registry.ExpandActions(actions, dataPlane)

	if len(notActions) == 0 {
		return expanded
	}

	effective := make([]string, 0, len(expanded))
	for _, action := range expanded {
		if !matchesAny(action, notActions) {
			effective = append(effective, action)
		}
	}
	return effective
}
