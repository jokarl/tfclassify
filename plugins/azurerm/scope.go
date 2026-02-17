// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
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

