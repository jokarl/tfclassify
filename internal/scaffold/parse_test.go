package scaffold

import (
	"strings"
	"testing"
)

func TestParseStateList(t *testing.T) {
	input := `azurerm_resource_group.example
azurerm_storage_account.main
azurerm_role_assignment.reader
module.network.azurerm_virtual_network.main
module.network.azurerm_subnet.default
`
	r, err := ParseStateList(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantTypes := []string{
		"azurerm_resource_group",
		"azurerm_role_assignment",
		"azurerm_storage_account",
		"azurerm_subnet",
		"azurerm_virtual_network",
	}
	if len(r.ResourceTypes) != len(wantTypes) {
		t.Fatalf("got %d types, want %d: %v", len(r.ResourceTypes), len(wantTypes), r.ResourceTypes)
	}
	for i, got := range r.ResourceTypes {
		if got != wantTypes[i] {
			t.Errorf("type[%d] = %q, want %q", i, got, wantTypes[i])
		}
	}

	wantModules := []string{"module.network"}
	if len(r.Modules) != len(wantModules) {
		t.Fatalf("got %d modules, want %d: %v", len(r.Modules), len(wantModules), r.Modules)
	}
	for i, got := range r.Modules {
		if got != wantModules[i] {
			t.Errorf("module[%d] = %q, want %q", i, got, wantModules[i])
		}
	}
}

func TestParseStateList_SkipsDataSources(t *testing.T) {
	input := `azurerm_resource_group.main
data.azurerm_client_config.current
data.azurerm_subscription.primary
azurerm_storage_account.main
`
	r, err := ParseStateList(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.ResourceTypes) != 2 {
		t.Fatalf("got %d types, want 2: %v", len(r.ResourceTypes), r.ResourceTypes)
	}
}

func TestParseStateList_StripsInstanceIndices(t *testing.T) {
	input := `azurerm_subnet.main[0]
azurerm_subnet.main[1]
azurerm_subnet.main["web"]
azurerm_role_assignment.readers["user1"]
`
	r, err := ParseStateList(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTypes := []string{"azurerm_role_assignment", "azurerm_subnet"}
	if len(r.ResourceTypes) != len(wantTypes) {
		t.Fatalf("got %d types, want %d: %v", len(r.ResourceTypes), len(wantTypes), r.ResourceTypes)
	}
	for i, got := range r.ResourceTypes {
		if got != wantTypes[i] {
			t.Errorf("type[%d] = %q, want %q", i, got, wantTypes[i])
		}
	}
}

func TestParseStateList_NestedModules(t *testing.T) {
	input := `module.prod.module.network.azurerm_virtual_network.main
module.prod.module.network.azurerm_subnet.default
module.prod.azurerm_resource_group.main
`
	r, err := ParseStateList(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantModules := []string{
		"module.prod",
		"module.prod.module.network",
	}
	if len(r.Modules) != len(wantModules) {
		t.Fatalf("got %d modules, want %d: %v", len(r.Modules), len(wantModules), r.Modules)
	}
	for i, got := range r.Modules {
		if got != wantModules[i] {
			t.Errorf("module[%d] = %q, want %q", i, got, wantModules[i])
		}
	}
}

func TestParseStateList_Deduplication(t *testing.T) {
	input := `azurerm_storage_account.one
azurerm_storage_account.two
azurerm_storage_account.three
`
	r, err := ParseStateList(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.ResourceTypes) != 1 {
		t.Fatalf("got %d types, want 1 (should deduplicate): %v", len(r.ResourceTypes), r.ResourceTypes)
	}
	if r.ResourceTypes[0] != "azurerm_storage_account" {
		t.Errorf("got %q, want %q", r.ResourceTypes[0], "azurerm_storage_account")
	}
}

func TestParseStateList_EmptyInput(t *testing.T) {
	_, err := ParseStateList(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseStateList_OnlyDataSources(t *testing.T) {
	input := `data.azurerm_client_config.current
data.azurerm_subscription.primary
`
	_, err := ParseStateList(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error when only data sources present")
	}
}

func TestParseStateList_BlankLines(t *testing.T) {
	input := `
azurerm_resource_group.main

azurerm_storage_account.main

`
	r, err := ParseStateList(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.ResourceTypes) != 2 {
		t.Fatalf("got %d types, want 2: %v", len(r.ResourceTypes), r.ResourceTypes)
	}
}
