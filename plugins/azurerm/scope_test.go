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

