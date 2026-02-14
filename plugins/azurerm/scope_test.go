package main

import (
	"testing"
)

func TestParseScopeLevel_ManagementGroup(t *testing.T) {
	tests := []struct {
		name  string
		scope string
	}{
		{
			name:  "standard management group path",
			scope: "/providers/Microsoft.Management/managementGroups/my-mg",
		},
		{
			name:  "management group with subscription",
			scope: "/providers/Microsoft.Management/managementGroups/my-mg/subscriptions/00000000-0000-0000-0000-000000000000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseScopeLevel(tc.scope)
			if got != ScopeLevelManagementGroup {
				t.Errorf("ParseScopeLevel(%q) = %v, want %v", tc.scope, got, ScopeLevelManagementGroup)
			}
		})
	}
}

func TestParseScopeLevel_Subscription(t *testing.T) {
	tests := []struct {
		name  string
		scope string
	}{
		{
			name:  "subscription only",
			scope: "/subscriptions/00000000-0000-0000-0000-000000000000",
		},
		{
			name:  "subscription with trailing slash",
			scope: "/subscriptions/00000000-0000-0000-0000-000000000000/",
		},
		{
			name:  "subscription with providers but no resourcegroups",
			scope: "/subscriptions/00000000-0000-0000-0000-000000000000/providers/Microsoft.Resources/deployments/my-deployment",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseScopeLevel(tc.scope)
			if got != ScopeLevelSubscription {
				t.Errorf("ParseScopeLevel(%q) = %v, want %v", tc.scope, got, ScopeLevelSubscription)
			}
		})
	}
}

func TestParseScopeLevel_ResourceGroup(t *testing.T) {
	tests := []struct {
		name  string
		scope string
	}{
		{
			name:  "resource group only",
			scope: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-rg",
		},
		{
			name:  "resource group with trailing slash",
			scope: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-rg/",
		},
		{
			name:  "resource group lowercase",
			scope: "/subscriptions/abc/resourcegroups/my-rg",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseScopeLevel(tc.scope)
			if got != ScopeLevelResourceGroup {
				t.Errorf("ParseScopeLevel(%q) = %v, want %v", tc.scope, got, ScopeLevelResourceGroup)
			}
		})
	}
}

func TestParseScopeLevel_Resource(t *testing.T) {
	tests := []struct {
		name  string
		scope string
	}{
		{
			name:  "virtual machine",
			scope: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/my-vm",
		},
		{
			name:  "storage account",
			scope: "/subscriptions/abc-123/resourceGroups/rg1/providers/Microsoft.Storage/storageAccounts/mystorageaccount",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseScopeLevel(tc.scope)
			if got != ScopeLevelResource {
				t.Errorf("ParseScopeLevel(%q) = %v, want %v", tc.scope, got, ScopeLevelResource)
			}
		})
	}
}

func TestParseScopeLevel_NestedResource(t *testing.T) {
	tests := []struct {
		name  string
		scope string
	}{
		{
			name:  "subnet (nested under vnet)",
			scope: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet1",
		},
		{
			name:  "key vault secret",
			scope: "/subscriptions/abc/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/myvault/secrets/mysecret",
		},
		{
			name:  "sql database (nested under server)",
			scope: "/subscriptions/abc/resourceGroups/rg/providers/Microsoft.Sql/servers/myserver/databases/mydb",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseScopeLevel(tc.scope)
			if got != ScopeLevelResource {
				t.Errorf("ParseScopeLevel(%q) = %v, want %v", tc.scope, got, ScopeLevelResource)
			}
		})
	}
}

func TestParseScopeLevel_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		expected ScopeLevel
	}{
		{
			name:     "uppercase SUBSCRIPTIONS",
			scope:    "/SUBSCRIPTIONS/abc/RESOURCEGROUPS/rg",
			expected: ScopeLevelResourceGroup,
		},
		{
			name:     "mixed case ResourceGroups",
			scope:    "/subscriptions/abc/ResourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm",
			expected: ScopeLevelResource,
		},
		{
			name:     "uppercase management group",
			scope:    "/providers/MICROSOFT.MANAGEMENT/MANAGEMENTGROUPS/mg",
			expected: ScopeLevelManagementGroup,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseScopeLevel(tc.scope)
			if got != tc.expected {
				t.Errorf("ParseScopeLevel(%q) = %v, want %v", tc.scope, got, tc.expected)
			}
		})
	}
}

func TestParseScopeLevel_EmptyString(t *testing.T) {
	got := ParseScopeLevel("")
	if got != ScopeLevelUnknown {
		t.Errorf("ParseScopeLevel(\"\") = %v, want %v", got, ScopeLevelUnknown)
	}
}

func TestParseScopeLevel_Whitespace(t *testing.T) {
	tests := []string{
		"   ",
		"\t",
		"\n",
		"  \t  ",
	}

	for _, scope := range tests {
		got := ParseScopeLevel(scope)
		if got != ScopeLevelUnknown {
			t.Errorf("ParseScopeLevel(%q) = %v, want %v", scope, got, ScopeLevelUnknown)
		}
	}
}

func TestParseScopeLevel_Malformed(t *testing.T) {
	tests := []struct {
		name  string
		scope string
	}{
		{
			name:  "random string",
			scope: "not-a-valid-path",
		},
		{
			name:  "just a slash",
			scope: "/",
		},
		{
			name:  "missing leading slash",
			scope: "subscriptions/abc",
		},
		{
			name:  "partial path",
			scope: "/subscriptions",
		},
		{
			name:  "random URL",
			scope: "https://example.com/path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseScopeLevel(tc.scope)
			if got != ScopeLevelUnknown {
				t.Errorf("ParseScopeLevel(%q) = %v, want %v", tc.scope, got, ScopeLevelUnknown)
			}
		})
	}
}

func TestScopeMultiplier_AllLevels(t *testing.T) {
	tests := []struct {
		level    ScopeLevel
		expected float64
	}{
		{ScopeLevelManagementGroup, 1.1},
		{ScopeLevelSubscription, 1.0},
		{ScopeLevelResourceGroup, 0.8},
		{ScopeLevelResource, 0.6},
		{ScopeLevelUnknown, 0.9},
	}

	for _, tc := range tests {
		t.Run(tc.level.String(), func(t *testing.T) {
			got := ScopeMultiplier(tc.level)
			if got != tc.expected {
				t.Errorf("ScopeMultiplier(%v) = %v, want %v", tc.level, got, tc.expected)
			}
		})
	}
}

func TestApplyScopeMultiplier_Baseline(t *testing.T) {
	// Subscription scope = 1.0x multiplier, no change
	scope := "/subscriptions/00000000-0000-0000-0000-000000000000"
	got := ApplyScopeMultiplier(90, scope)
	if got != 90 {
		t.Errorf("ApplyScopeMultiplier(90, subscription) = %d, want 90", got)
	}
}

func TestApplyScopeMultiplier_MgIncrease(t *testing.T) {
	// Management group = 1.1x multiplier: 90 * 1.1 = 99
	scope := "/providers/Microsoft.Management/managementGroups/my-mg"
	got := ApplyScopeMultiplier(90, scope)
	if got != 99 {
		t.Errorf("ApplyScopeMultiplier(90, mgmt-group) = %d, want 99", got)
	}
}

func TestApplyScopeMultiplier_RgReduction(t *testing.T) {
	// Resource group = 0.8x multiplier: 90 * 0.8 = 72
	scope := "/subscriptions/abc/resourceGroups/rg"
	got := ApplyScopeMultiplier(90, scope)
	if got != 72 {
		t.Errorf("ApplyScopeMultiplier(90, rg) = %d, want 72", got)
	}
}

func TestApplyScopeMultiplier_ResourceReduction(t *testing.T) {
	// Resource = 0.6x multiplier: 90 * 0.6 = 54
	scope := "/subscriptions/abc/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm"
	got := ApplyScopeMultiplier(90, scope)
	if got != 54 {
		t.Errorf("ApplyScopeMultiplier(90, resource) = %d, want 54", got)
	}
}

func TestApplyScopeMultiplier_ClampAt100(t *testing.T) {
	// 95 * 1.1 = 104.5, should clamp to 100
	scope := "/providers/Microsoft.Management/managementGroups/my-mg"
	got := ApplyScopeMultiplier(95, scope)
	if got != 100 {
		t.Errorf("ApplyScopeMultiplier(95, mgmt-group) = %d, want 100 (clamped)", got)
	}
}

func TestApplyScopeMultiplier_ClampAtZero(t *testing.T) {
	// Score 0 should always return 0 regardless of scope
	scopes := []string{
		"/providers/Microsoft.Management/managementGroups/my-mg",
		"/subscriptions/abc",
		"/subscriptions/abc/resourceGroups/rg",
		"/subscriptions/abc/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm",
		"invalid-scope",
	}

	for _, scope := range scopes {
		got := ApplyScopeMultiplier(0, scope)
		if got != 0 {
			t.Errorf("ApplyScopeMultiplier(0, %q) = %d, want 0", scope, got)
		}
	}
}

func TestApplyScopeMultiplier_UnknownScope(t *testing.T) {
	// Unknown scope = 0.9x multiplier: 90 * 0.9 = 81
	got := ApplyScopeMultiplier(90, "invalid-scope")
	if got != 81 {
		t.Errorf("ApplyScopeMultiplier(90, invalid) = %d, want 81", got)
	}
}

func TestApplyScopeMultiplier_Rounding(t *testing.T) {
	// Test rounding behavior
	tests := []struct {
		score    int
		scope    string
		expected int
	}{
		// 85 * 0.8 = 68 (exact)
		{85, "/subscriptions/abc/resourceGroups/rg", 68},
		// 87 * 0.8 = 69.6, rounds to 70
		{87, "/subscriptions/abc/resourceGroups/rg", 70},
		// 88 * 0.8 = 70.4, rounds to 70
		{88, "/subscriptions/abc/resourceGroups/rg", 70},
	}

	for _, tc := range tests {
		got := ApplyScopeMultiplier(tc.score, tc.scope)
		if got != tc.expected {
			t.Errorf("ApplyScopeMultiplier(%d, rg) = %d, want %d", tc.score, got, tc.expected)
		}
	}
}

func TestScopeLevel_String(t *testing.T) {
	tests := []struct {
		level    ScopeLevel
		expected string
	}{
		{ScopeLevelManagementGroup, "management-group"},
		{ScopeLevelSubscription, "subscription"},
		{ScopeLevelResourceGroup, "resource-group"},
		{ScopeLevelResource, "resource"},
		{ScopeLevelUnknown, "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			got := tc.level.String()
			if got != tc.expected {
				t.Errorf("ScopeLevel(%d).String() = %q, want %q", tc.level, got, tc.expected)
			}
		})
	}
}

func TestScopeLevel_String_InvalidValue(t *testing.T) {
	// Test that invalid ScopeLevel values default to "unknown"
	invalid := ScopeLevel(99)
	got := invalid.String()
	if got != "unknown" {
		t.Errorf("ScopeLevel(99).String() = %q, want %q", got, "unknown")
	}
}

func TestScopeMultiplier_InvalidValue(t *testing.T) {
	// Test that invalid ScopeLevel values return the default 0.9 multiplier
	invalid := ScopeLevel(99)
	got := ScopeMultiplier(invalid)
	if got != 0.9 {
		t.Errorf("ScopeMultiplier(ScopeLevel(99)) = %v, want 0.9", got)
	}
}
