// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	"strings"
)

// PermissionScore represents the computed risk score for a role's permissions.
type PermissionScore struct {
	// Total is the final score in range [0, 100].
	Total int
	// HasWildcard is true when effective actions include unrestricted "*".
	HasWildcard bool
	// HasAuthWrite is true when effective actions include Microsoft.Authorization write.
	HasAuthWrite bool
	// Factors contains human-readable reasons for the score.
	Factors []string
}

// ScorePermissions computes a risk score from a role definition's permission set.
// It analyzes effective actions (actions minus notActions), classifies them into
// risk tiers, and returns a structured score.
//
// The scoring tiers are:
//   - 95: Unrestricted wildcard (*) without auth exclusion (Owner pattern)
//   - 85: Microsoft.Authorization/* control (User Access Administrator pattern)
//   - 75: Targeted escalation (Microsoft.Authorization/roleAssignments/write only)
//   - 70: Wildcard with auth excluded (Contributor pattern)
//   - 50-65: Provider wildcards (Microsoft.X/*)
//   - 30: Limited write access
//   - 15: Read-only access
//   - 0: No permissions
func ScorePermissions(role *RoleDefinition) PermissionScore {
	if role == nil || len(role.Permissions) == 0 {
		return PermissionScore{
			Total:   0,
			Factors: []string{"no permissions defined"},
		}
	}

	// Find the highest scoring permission block
	var best PermissionScore
	for _, perm := range role.Permissions {
		score := scorePermissionBlock(perm)
		if score.Total > best.Total {
			best = score
		}
	}

	return best
}

// scorePermissionBlock scores a single permission block.
func scorePermissionBlock(perm Permission) PermissionScore {
	if len(perm.Actions) == 0 && len(perm.DataActions) == 0 {
		return PermissionScore{
			Total:   0,
			Factors: []string{"no actions defined"},
		}
	}

	// Check for wildcard patterns
	hasWildcard := containsPattern(perm.Actions, "*")
	authExcluded := coversAuthorizationWrite(perm.NotActions)
	hasAuthAction := hasAuthorizationAccess(perm.Actions, perm.NotActions)
	hasTargetedAuthWrite := hasTargetedRoleAssignmentWrite(perm.Actions, perm.NotActions)
	isReadOnly := isReadOnlyPermissions(perm.Actions, perm.NotActions)

	// Tier 1: Unrestricted wildcard (Owner pattern) - 95
	if hasWildcard && !authExcluded {
		return PermissionScore{
			Total:        95,
			HasWildcard:  true,
			HasAuthWrite: true,
			Factors:      []string{"unrestricted wildcard (*) without authorization exclusion"},
		}
	}

	// Tier 2: Microsoft.Authorization/* control (UAA pattern) - 85
	if hasAuthAction && !hasWildcard {
		return PermissionScore{
			Total:        85,
			HasWildcard:  false,
			HasAuthWrite: true,
			Factors:      []string{"Microsoft.Authorization control access"},
		}
	}

	// Tier 3: Targeted escalation (roleAssignments/write only) - 75
	if hasTargetedAuthWrite && !hasAuthAction {
		return PermissionScore{
			Total:        75,
			HasWildcard:  false,
			HasAuthWrite: true,
			Factors:      []string{"targeted role assignment write access"},
		}
	}

	// Tier 4: Wildcard with auth excluded (Contributor pattern) - 70
	if hasWildcard && authExcluded {
		return PermissionScore{
			Total:        70,
			HasWildcard:  false, // Effectively not unrestricted due to auth exclusion
			HasAuthWrite: false,
			Factors:      []string{"wildcard (*) with Microsoft.Authorization excluded"},
		}
	}

	// Tier 5: Provider wildcards - 50-65
	providerWildcards := countProviderWildcards(perm.Actions)
	if providerWildcards > 0 {
		// Score based on count: 50 + min(providerWildcards * 5, 15)
		bonus := providerWildcards * 5
		if bonus > 15 {
			bonus = 15
		}
		return PermissionScore{
			Total:        50 + bonus,
			HasWildcard:  false,
			HasAuthWrite: false,
			Factors:      []string{"provider-level wildcard access"},
		}
	}

	// Tier 6: Read-only - 15
	if isReadOnly {
		return PermissionScore{
			Total:        15,
			HasWildcard:  false,
			HasAuthWrite: false,
			Factors:      []string{"read-only access"},
		}
	}

	// Tier 7: Limited write - 30
	return PermissionScore{
		Total:        30,
		HasWildcard:  false,
		HasAuthWrite: false,
		Factors:      []string{"limited write access"},
	}
}

// actionMatchesPattern checks if an action matches a pattern using Azure RBAC matching rules.
// Azure RBAC uses glob-like matching:
//   - "*" matches everything
//   - "Microsoft.Compute/*" matches all actions under that provider
//   - "*/read" matches any read action
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

// containsPattern checks if patterns contain a specific pattern (case-insensitive).
func containsPattern(patterns []string, pattern string) bool {
	pattern = strings.ToLower(pattern)
	for _, p := range patterns {
		if strings.ToLower(p) == pattern {
			return true
		}
	}
	return false
}

// computeEffectiveActions subtracts notActions from actions using pattern matching.
// For explicit action lists, it filters out actions that match notActions patterns.
// Note: When actions contains "*", the result is still ["*"] - the effective
// calculation is handled at scoring time via the authExcluded check.
func computeEffectiveActions(actions, notActions []string) []string {
	if len(notActions) == 0 {
		return actions
	}

	// If actions is just ["*"], return as-is - special handling at scoring time
	if len(actions) == 1 && strings.ToLower(actions[0]) == "*" {
		return actions
	}

	var effective []string
	for _, action := range actions {
		if !matchesAny(action, notActions) {
			effective = append(effective, action)
		}
	}
	return effective
}

// coversAuthorizationWrite checks if notActions cover Microsoft.Authorization write operations.
func coversAuthorizationWrite(notActions []string) bool {
	for _, na := range notActions {
		lower := strings.ToLower(na)
		// Check for patterns that would exclude auth writes
		if lower == "microsoft.authorization/*" ||
			lower == "microsoft.authorization/*/write" ||
			lower == "microsoft.authorization/*/delete" {
			return true
		}
		// Also check for combined write+delete exclusion
		if strings.HasPrefix(lower, "microsoft.authorization/") &&
			(strings.Contains(lower, "write") || strings.Contains(lower, "delete")) {
			// Check if it's a broad exclusion
			if strings.HasSuffix(lower, "/*") || strings.HasSuffix(lower, "/write") {
				return true
			}
		}
	}
	return false
}

// hasAuthorizationAccess checks if actions include Microsoft.Authorization/* control
// without being excluded by notActions.
func hasAuthorizationAccess(actions, notActions []string) bool {
	for _, action := range actions {
		lower := strings.ToLower(action)
		if lower == "microsoft.authorization/*" {
			// Check if not excluded
			if !matchesAny(action, notActions) {
				return true
			}
		}
	}
	return false
}

// hasTargetedRoleAssignmentWrite checks for specific roleAssignments/write without
// broader authorization access.
func hasTargetedRoleAssignmentWrite(actions, notActions []string) bool {
	for _, action := range actions {
		lower := strings.ToLower(action)
		if strings.Contains(lower, "microsoft.authorization/roleassignments/write") ||
			strings.Contains(lower, "microsoft.authorization/roleassignments/*") {
			if !matchesAny(action, notActions) {
				return true
			}
		}
	}
	return false
}

// isReadOnlyPermissions checks if all actions are read-only.
func isReadOnlyPermissions(actions, notActions []string) bool {
	if len(actions) == 0 {
		return false
	}

	for _, action := range actions {
		lower := strings.ToLower(action)
		// Check if it's a read pattern
		if lower == "*/read" || strings.HasSuffix(lower, "/read") {
			continue
		}
		// Provider/*/read pattern
		if strings.HasSuffix(lower, "/*") {
			// This could include writes, so not read-only
			return false
		}
		// If we get here, it's not a read-only pattern
		return false
	}
	return true
}

// countProviderWildcards counts how many provider-level wildcards are in actions.
// Provider wildcards are patterns like "Microsoft.Compute/*".
func countProviderWildcards(actions []string) int {
	count := 0
	for _, action := range actions {
		lower := strings.ToLower(action)
		// Must end with /* and contain a provider prefix
		if strings.HasSuffix(lower, "/*") && lower != "*" && lower != "/*" {
			// Should be something like "microsoft.compute/*"
			if strings.Contains(lower, ".") {
				count++
			}
		}
	}
	return count
}
