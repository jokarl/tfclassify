package main

import (
	"strings"
	"testing"
)

func TestActionRegistry_EmbeddedData(t *testing.T) {
	// AC-15: Action registry embedded data is valid
	reg := DefaultActionRegistry()

	if reg.ActionCount() < 14000 {
		t.Errorf("expected at least 14,000 control-plane actions, got %d", reg.ActionCount())
	}
	if reg.DataActionCount() < 2500 {
		t.Errorf("expected at least 2,500 data-plane actions, got %d", reg.DataActionCount())
	}
	if reg.ProviderCount() < 200 {
		t.Errorf("expected at least 200 providers, got %d", reg.ProviderCount())
	}
}

func TestActionRegistry_ExpandPattern_GlobalWildcard(t *testing.T) {
	reg := DefaultActionRegistry()

	result := reg.ExpandPattern("*", false)
	if len(result) < 14000 {
		t.Errorf("expanding * should return all actions, got %d", len(result))
	}

	resultData := reg.ExpandPattern("*", true)
	if len(resultData) < 2500 {
		t.Errorf("expanding * (data-plane) should return all data actions, got %d", len(resultData))
	}
}

func TestActionRegistry_ExpandPattern_ProviderWildcard(t *testing.T) {
	reg := DefaultActionRegistry()

	result := reg.ExpandPattern("Microsoft.Storage/*", false)
	if len(result) == 0 {
		t.Fatal("expanding Microsoft.Storage/* should return storage actions")
	}

	// All results should be under Microsoft.Storage
	for _, action := range result {
		if !strings.HasPrefix(strings.ToLower(action), "microsoft.storage/") {
			t.Errorf("action %q is not under Microsoft.Storage/", action)
		}
	}

	// Should not include actions from other providers
	for _, action := range result {
		if strings.HasPrefix(strings.ToLower(action), "microsoft.compute/") {
			t.Errorf("Microsoft.Storage/* expanded should not include compute actions: %q", action)
		}
	}
}

func TestActionRegistry_ExpandPattern_SuffixWildcard(t *testing.T) {
	reg := DefaultActionRegistry()

	result := reg.ExpandPattern("*/read", false)
	if len(result) == 0 {
		t.Fatal("expanding */read should return read actions")
	}

	// All results should end with /read
	for _, action := range result {
		lower := strings.ToLower(action)
		if !strings.HasSuffix(lower, "/read") {
			t.Errorf("action %q does not end with /read", action)
		}
	}
}

func TestActionRegistry_ExpandPattern_CaseInsensitive(t *testing.T) {
	reg := DefaultActionRegistry()

	// Should match case-insensitively
	lower := reg.ExpandPattern("microsoft.storage/*", false)
	upper := reg.ExpandPattern("MICROSOFT.STORAGE/*", false)
	mixed := reg.ExpandPattern("Microsoft.Storage/*", false)

	if len(lower) != len(upper) || len(lower) != len(mixed) {
		t.Errorf("case-insensitive lookup failed: lower=%d, upper=%d, mixed=%d",
			len(lower), len(upper), len(mixed))
	}
}

func TestActionRegistry_ExpandPattern_ExactMatch(t *testing.T) {
	reg := DefaultActionRegistry()

	result := reg.ExpandPattern("Microsoft.Storage/storageAccounts/read", false)
	if len(result) != 1 {
		t.Errorf("exact match should return 1 result, got %d", len(result))
	}
	if result[0] != "Microsoft.Storage/storageAccounts/read" {
		t.Errorf("exact match should return the action as-is, got %q", result[0])
	}
}

func TestActionRegistry_ExpandPattern_UnknownProvider(t *testing.T) {
	reg := DefaultActionRegistry()

	result := reg.ExpandPattern("Microsoft.NonExistent/*", false)
	// Should return the pattern as-is when provider not found
	if len(result) != 1 || result[0] != "Microsoft.NonExistent/*" {
		t.Errorf("unknown provider should return pattern as-is, got %v", result)
	}
}

func TestActionRegistry_ExpandActions_Dedup(t *testing.T) {
	reg := DefaultActionRegistry()

	// Microsoft.Storage/* and Microsoft.Storage/storageAccounts/read should overlap
	patterns := []string{"Microsoft.Storage/*", "Microsoft.Storage/storageAccounts/read"}
	result := reg.ExpandActions(patterns, false)

	// Check for duplicates
	seen := make(map[string]bool)
	for _, action := range result {
		if seen[action] {
			t.Errorf("duplicate action in result: %q", action)
		}
		seen[action] = true
	}
}

func TestActionRegistry_ExpandActions_DataPlane(t *testing.T) {
	reg := DefaultActionRegistry()

	result := reg.ExpandActions([]string{"Microsoft.Storage/*"}, true)
	if len(result) == 0 {
		t.Fatal("expanding Microsoft.Storage/* (data-plane) should return data actions")
	}

	// All results should be under Microsoft.Storage
	for _, action := range result {
		if !strings.HasPrefix(strings.ToLower(action), "microsoft.storage/") {
			t.Errorf("data action %q is not under Microsoft.Storage/", action)
		}
	}
}

func TestActionRegistry_NilSafe(t *testing.T) {
	var reg *ActionRegistry

	if reg.ActionCount() != 0 {
		t.Error("nil registry should return 0 action count")
	}
	if reg.DataActionCount() != 0 {
		t.Error("nil registry should return 0 data action count")
	}
	if reg.ProviderCount() != 0 {
		t.Error("nil registry should return 0 provider count")
	}

	// Should not panic
	result := reg.ExpandPattern("*", false)
	if len(result) != 1 || result[0] != "*" {
		t.Errorf("nil registry ExpandPattern should return pattern as-is, got %v", result)
	}

	result = reg.ExpandActions([]string{"*"}, false)
	if len(result) != 1 || result[0] != "*" {
		t.Errorf("nil registry ExpandActions should return patterns as-is, got %v", result)
	}
}
