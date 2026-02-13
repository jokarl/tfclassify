// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	"math"
	"strings"
)

// ScopeLevel represents the level of an ARM scope path.
type ScopeLevel int

const (
	// ScopeLevelUnknown is returned for unrecognized or invalid scope paths.
	ScopeLevelUnknown ScopeLevel = iota
	// ScopeLevelResource represents scope at an individual resource.
	ScopeLevelResource
	// ScopeLevelResourceGroup represents scope at a resource group level.
	ScopeLevelResourceGroup
	// ScopeLevelSubscription represents scope at a subscription level.
	ScopeLevelSubscription
	// ScopeLevelManagementGroup represents scope at a management group level.
	ScopeLevelManagementGroup
)

// String returns a human-readable label for the scope level.
func (s ScopeLevel) String() string {
	switch s {
	case ScopeLevelManagementGroup:
		return "management-group"
	case ScopeLevelSubscription:
		return "subscription"
	case ScopeLevelResourceGroup:
		return "resource-group"
	case ScopeLevelResource:
		return "resource"
	default:
		return "unknown"
	}
}

// ParseScopeLevel classifies an ARM scope path into a ScopeLevel.
// The parsing is case-insensitive since ARM paths are case-insensitive in Azure.
func ParseScopeLevel(scope string) ScopeLevel {
	// Normalize: lowercase and trim trailing slash
	scope = strings.ToLower(strings.TrimSpace(scope))
	scope = strings.TrimSuffix(scope, "/")

	// Empty or whitespace-only input
	if scope == "" {
		return ScopeLevelUnknown
	}

	// Management group: contains microsoft.management/managementgroups
	if strings.Contains(scope, "microsoft.management/managementgroups") {
		return ScopeLevelManagementGroup
	}

	// Must start with /subscriptions/ for other valid scopes
	if !strings.HasPrefix(scope, "/subscriptions/") {
		return ScopeLevelUnknown
	}

	// Check for resource groups
	rgIndex := strings.Index(scope, "/resourcegroups/")
	if rgIndex == -1 {
		// No resource groups segment = subscription scope
		return ScopeLevelSubscription
	}

	// Find if there's a /providers/ segment after /resourcegroups/
	afterRG := scope[rgIndex+len("/resourcegroups/"):]
	if strings.Contains(afterRG, "/providers/") {
		return ScopeLevelResource
	}

	// Has resource group but no providers after = resource group scope
	return ScopeLevelResourceGroup
}

// ScopeMultiplier returns the severity multiplier for a given scope level.
// The multipliers are based on ADR-0006:
//   - Management group: 1.1 (broadest scope, affects multiple subscriptions)
//   - Subscription: 1.0 (baseline)
//   - Resource group: 0.8 (limited to one resource group)
//   - Resource: 0.6 (tightly scoped to a single resource)
//   - Unknown: 0.9 (conservative default)
func ScopeMultiplier(level ScopeLevel) float64 {
	switch level {
	case ScopeLevelManagementGroup:
		return 1.1
	case ScopeLevelSubscription:
		return 1.0
	case ScopeLevelResourceGroup:
		return 0.8
	case ScopeLevelResource:
		return 0.6
	case ScopeLevelUnknown:
		return 0.9
	default:
		return 0.9
	}
}

// ApplyScopeMultiplier computes the scope-weighted severity score.
// It parses the scope level, applies the multiplier, rounds to the nearest integer,
// and clamps the result to [0, 100].
func ApplyScopeMultiplier(score int, scope string) int {
	// Zero score stays zero regardless of scope
	if score == 0 {
		return 0
	}

	level := ParseScopeLevel(scope)
	multiplier := ScopeMultiplier(level)

	// Apply multiplier and round
	result := float64(score) * multiplier
	rounded := int(math.Round(result))

	// Clamp to [0, 100]
	if rounded < 0 {
		return 0
	}
	if rounded > 100 {
		return 100
	}
	return rounded
}
