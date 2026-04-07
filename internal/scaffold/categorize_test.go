package scaffold

import (
	"testing"
)

func TestCategorize_Azure(t *testing.T) {
	parsed := &ParseResult{
		ResourceTypes: []string{
			"azurerm_key_vault",
			"azurerm_linux_virtual_machine",
			"azurerm_network_security_group",
			"azurerm_resource_group",
			"azurerm_role_assignment",
			"azurerm_role_definition",
			"azurerm_storage_account",
			"azurerm_subnet",
			"azurerm_virtual_network",
		},
	}

	result := Categorize(parsed)

	// Should have critical categories
	if len(result.Critical) == 0 {
		t.Fatal("expected critical categories")
	}

	// Check IAM is critical
	foundIAM := false
	for _, c := range result.Critical {
		if c.Name == "IAM" {
			foundIAM = true
			if len(c.Types) != 2 {
				t.Errorf("IAM: got %d types, want 2: %v", len(c.Types), c.Types)
			}
		}
	}
	if !foundIAM {
		t.Error("IAM category not found in critical")
	}

	// Check Security is critical (key_vault exact match)
	foundSecurity := false
	for _, c := range result.Critical {
		if c.Name == "Security" {
			foundSecurity = true
			if len(c.Types) != 1 || c.Types[0] != "azurerm_key_vault" {
				t.Errorf("Security: got %v, want [azurerm_key_vault]", c.Types)
			}
		}
	}
	if !foundSecurity {
		t.Error("Security category not found in critical")
	}

	// Check Network ACLs is critical
	foundNACL := false
	for _, c := range result.Critical {
		if c.Name == "Network ACLs" {
			foundNACL = true
		}
	}
	if !foundNACL {
		t.Error("Network ACLs category not found in critical")
	}
}

func TestCategorize_Azure_KeyVaultExactMatch(t *testing.T) {
	// key_vault (exact) should NOT match key_vault_secret
	parsed := &ParseResult{
		ResourceTypes: []string{
			"azurerm_key_vault",
			"azurerm_key_vault_secret",
			"azurerm_key_vault_key",
		},
	}

	result := Categorize(parsed)

	for _, c := range result.Critical {
		if c.Name == "Security" {
			if len(c.Types) != 1 {
				t.Errorf("Security should only match azurerm_key_vault (exact), got: %v", c.Types)
			}
		}
	}

	// key_vault_secret and key_vault_key should be uncategorized
	if len(result.Uncategorized) != 2 {
		t.Errorf("expected 2 uncategorized (key_vault_secret, key_vault_key), got %d: %v",
			len(result.Uncategorized), result.Uncategorized)
	}
}

func TestCategorize_AWS(t *testing.T) {
	parsed := &ParseResult{
		ResourceTypes: []string{
			"aws_iam_role",
			"aws_iam_policy",
			"aws_instance",
			"aws_s3_bucket",
			"aws_vpc",
		},
	}

	result := Categorize(parsed)

	if len(result.Critical) == 0 {
		t.Fatal("expected critical categories for AWS")
	}

	foundIAM := false
	for _, c := range result.Critical {
		if c.Name == "IAM" {
			foundIAM = true
			if len(c.Types) != 2 {
				t.Errorf("IAM: got %d types, want 2: %v", len(c.Types), c.Types)
			}
		}
	}
	if !foundIAM {
		t.Error("IAM category not found in critical for AWS")
	}
}

func TestCategorize_UnknownProvider(t *testing.T) {
	parsed := &ParseResult{
		ResourceTypes: []string{
			"digitalocean_droplet",
			"digitalocean_volume",
			"digitalocean_firewall",
		},
	}

	result := Categorize(parsed)

	if len(result.Critical) != 0 {
		t.Errorf("expected no critical categories for unknown provider, got %d", len(result.Critical))
	}
	if len(result.Uncategorized) != 3 {
		t.Errorf("expected 3 uncategorized, got %d: %v", len(result.Uncategorized), result.Uncategorized)
	}
}

func TestCategorize_MixedProviders(t *testing.T) {
	// When mixed, the dominant provider wins
	parsed := &ParseResult{
		ResourceTypes: []string{
			"azurerm_resource_group",
			"azurerm_storage_account",
			"azurerm_role_assignment",
			"random_password",
		},
	}

	result := Categorize(parsed)

	// azurerm is dominant, so its knowledge base applies
	if len(result.Critical) == 0 {
		t.Fatal("expected critical categories")
	}

	// random_password should be uncategorized
	found := false
	for _, u := range result.Uncategorized {
		if u == "random_password" {
			found = true
		}
	}
	if !found {
		t.Error("random_password should be uncategorized")
	}
}

func TestCategorize_PreservesModules(t *testing.T) {
	parsed := &ParseResult{
		ResourceTypes: []string{"azurerm_resource_group"},
		Modules:       []string{"module.network", "module.production"},
	}

	result := Categorize(parsed)

	if len(result.Modules) != 2 {
		t.Errorf("expected 2 modules, got %d: %v", len(result.Modules), result.Modules)
	}
}

func TestGeneratePatterns(t *testing.T) {
	types := []string{
		"azurerm_role_assignment",
		"azurerm_role_definition",
	}
	patterns := generatePatterns(types, "azurerm")

	want := []string{"*_role_assignment", "*_role_definition"}
	if len(patterns) != len(want) {
		t.Fatalf("got %d patterns, want %d: %v", len(patterns), len(want), patterns)
	}
	for i, got := range patterns {
		if got != want[i] {
			t.Errorf("pattern[%d] = %q, want %q", i, got, want[i])
		}
	}
}
