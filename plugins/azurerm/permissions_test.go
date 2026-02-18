package main

import (
	"strings"
	"testing"
)

func TestActionMatchesPattern_Wildcard(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
		want    bool
	}{
		{"Microsoft.Compute/virtualMachines/read", "*", true},
		{"anything", "*", true},
		{"", "*", true},
	}

	for _, tc := range tests {
		got := actionMatchesPattern(tc.action, tc.pattern)
		if got != tc.want {
			t.Errorf("actionMatchesPattern(%q, %q) = %v, want %v", tc.action, tc.pattern, got, tc.want)
		}
	}
}

func TestActionMatchesPattern_NamespaceWildcard(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
		want    bool
	}{
		{"Microsoft.Compute/virtualMachines/read", "Microsoft.Compute/*", true},
		{"Microsoft.Compute/disks/write", "Microsoft.Compute/*", true},
		{"Microsoft.Storage/storageAccounts/read", "Microsoft.Compute/*", false},
	}

	for _, tc := range tests {
		got := actionMatchesPattern(tc.action, tc.pattern)
		if got != tc.want {
			t.Errorf("actionMatchesPattern(%q, %q) = %v, want %v", tc.action, tc.pattern, got, tc.want)
		}
	}
}

func TestActionMatchesPattern_NamespaceNoMatch(t *testing.T) {
	got := actionMatchesPattern("Microsoft.Storage/storageAccounts/read", "Microsoft.Compute/*")
	if got {
		t.Error("expected no match for different namespace")
	}
}

func TestActionMatchesPattern_SuffixWildcard(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
		want    bool
	}{
		{"Microsoft.Compute/virtualMachines/read", "*/read", true},
		{"Microsoft.Storage/storageAccounts/read", "*/read", true},
		{"Microsoft.Compute/virtualMachines/write", "*/read", false},
	}

	for _, tc := range tests {
		got := actionMatchesPattern(tc.action, tc.pattern)
		if got != tc.want {
			t.Errorf("actionMatchesPattern(%q, %q) = %v, want %v", tc.action, tc.pattern, got, tc.want)
		}
	}
}

func TestActionMatchesPattern_ExactMatch(t *testing.T) {
	action := "Microsoft.Compute/virtualMachines/read"
	got := actionMatchesPattern(action, action)
	if !got {
		t.Errorf("expected exact match for %q", action)
	}
}

func TestActionMatchesPattern_CaseInsensitive(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
	}{
		{"MICROSOFT.COMPUTE/virtualMachines/read", "microsoft.compute/*"},
		{"microsoft.compute/virtualmachines/READ", "Microsoft.Compute/*"},
		{"MICROSOFT.AUTHORIZATION/ROLEASSIGNMENTS/WRITE", "microsoft.authorization/*"},
	}

	for _, tc := range tests {
		got := actionMatchesPattern(tc.action, tc.pattern)
		if !got {
			t.Errorf("actionMatchesPattern(%q, %q) = false, want true (case insensitive)", tc.action, tc.pattern)
		}
	}
}

func TestActionMatchesPattern_NoMatch(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
	}{
		{"Microsoft.Compute/virtualMachines/read", "Microsoft.Storage/*"},
		{"Microsoft.Compute/virtualMachines/write", "*/read"},
		{"Microsoft.Compute/virtualMachines", "Microsoft.Network/virtualNetworks"},
	}

	for _, tc := range tests {
		got := actionMatchesPattern(tc.action, tc.pattern)
		if got {
			t.Errorf("actionMatchesPattern(%q, %q) = true, want false", tc.action, tc.pattern)
		}
	}
}

func TestComputeEffective_NoExclusions(t *testing.T) {
	actions := []string{"*"}
	result := computeEffectiveActions(actions, nil)

	// With wildcard expansion, "*" expands to all known actions
	registry := DefaultActionRegistry()
	expectedCount := registry.ActionCount()
	if len(result) != expectedCount {
		t.Errorf("computeEffectiveActions with no exclusions: got %d actions, want %d", len(result), expectedCount)
	}
}

func TestComputeEffective_SubtractsMatching(t *testing.T) {
	actions := []string{
		"Microsoft.Authorization/roleAssignments/write",
		"Microsoft.Compute/virtualMachines/read",
	}
	notActions := []string{"Microsoft.Authorization/*"}

	result := computeEffectiveActions(actions, notActions)

	if len(result) != 1 {
		t.Fatalf("expected 1 effective action, got %d: %v", len(result), result)
	}
	if result[0] != "Microsoft.Compute/virtualMachines/read" {
		t.Errorf("expected Compute action, got %q", result[0])
	}
}

func TestMatchesAny(t *testing.T) {
	patterns := []string{"Microsoft.Compute/*", "Microsoft.Storage/*"}

	tests := []struct {
		action string
		want   bool
	}{
		{"Microsoft.Compute/virtualMachines/read", true},
		{"Microsoft.Storage/storageAccounts/read", true},
		{"Microsoft.Network/virtualNetworks/read", false},
	}

	for _, tc := range tests {
		got := matchesAny(tc.action, patterns)
		if got != tc.want {
			t.Errorf("matchesAny(%q, patterns) = %v, want %v", tc.action, got, tc.want)
		}
	}
}

func TestComputeEffectiveActions_WildcardExpansion(t *testing.T) {
	// CR-0028: Wildcards are expanded via the action registry before subtraction.
	// Actions: ["*"] with NotActions: ["Microsoft.Authorization/*"] should expand
	// the wildcard to all concrete actions, then subtract Microsoft.Authorization/*.
	actions := []string{"*"}
	notActions := []string{"Microsoft.Authorization/*"}

	effective := computeEffectiveActions(actions, notActions)

	// Should have many actions (all Azure actions minus Microsoft.Authorization/*)
	if len(effective) < 100 {
		t.Errorf("expected many effective actions after wildcard expansion, got %d", len(effective))
	}

	// Should NOT contain any Microsoft.Authorization/ actions
	for _, action := range effective {
		if strings.HasPrefix(strings.ToLower(action), "microsoft.authorization/") {
			t.Errorf("effective actions should not contain Microsoft.Authorization/ after subtraction, found: %q", action)
			break
		}
	}
}

func TestComputeEffectiveActions_EmptyNotActions(t *testing.T) {
	actions := []string{"Microsoft.Compute/virtualMachines/read"}
	notActions := []string{}

	effective := computeEffectiveActions(actions, notActions)

	if len(effective) != 1 || effective[0] != "Microsoft.Compute/virtualMachines/read" {
		t.Errorf("expected original actions, got %v", effective)
	}
}

func TestComputeEffectiveActions_FilteredOut(t *testing.T) {
	actions := []string{
		"Microsoft.Compute/virtualMachines/read",
		"Microsoft.Authorization/roleAssignments/write",
	}
	notActions := []string{"Microsoft.Authorization/*"}

	effective := computeEffectiveActions(actions, notActions)

	if len(effective) != 1 {
		t.Fatalf("expected 1 action, got %d", len(effective))
	}
	if effective[0] != "Microsoft.Compute/virtualMachines/read" {
		t.Errorf("expected Compute action, got %v", effective[0])
	}
}

func TestComputeEffectiveActions_ProviderWildcard(t *testing.T) {
	// Provider wildcard like "Microsoft.Compute/*" should expand to all Compute actions
	actions := []string{"Microsoft.Compute/*"}
	notActions := []string{"*/delete"}

	effective := computeEffectiveActions(actions, notActions)

	// Should have Compute actions minus any /delete actions
	for _, action := range effective {
		if strings.HasSuffix(strings.ToLower(action), "/delete") {
			t.Errorf("should not contain /delete actions after subtraction, found: %q", action)
		}
		if !strings.HasPrefix(strings.ToLower(action), "microsoft.compute/") {
			t.Errorf("all actions should be under Microsoft.Compute/, found: %q", action)
		}
	}
}

func TestComputeEffectiveActions_DataPlane(t *testing.T) {
	// Test data-plane action expansion
	actions := []string{"Microsoft.Storage/*"}
	notActions := []string{}

	effective := computeEffectiveActionsWithRegistry(actions, notActions, true, nil)

	// Should expand to Storage data-plane actions
	if len(effective) == 0 {
		t.Error("expected some data-plane actions for Microsoft.Storage/*")
	}

	for _, action := range effective {
		if !strings.HasPrefix(strings.ToLower(action), "microsoft.storage/") {
			t.Errorf("all data actions should be under Microsoft.Storage/, found: %q", action)
		}
	}
}
