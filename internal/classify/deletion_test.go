package classify

import (
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/internal/plan"
)

func TestDeletionAnalyzer_Name(t *testing.T) {
	a := &DeletionAnalyzer{}
	if a.Name() != "deletion" {
		t.Errorf("expected name 'deletion', got %q", a.Name())
	}
}

func TestDeletionAnalyzer_StandaloneDelete(t *testing.T) {
	a := &DeletionAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_resource_group.example",
			Type:    "azurerm_resource_group",
			Actions: []string{"delete"},
		},
	}

	decisions := a.Analyze(changes)

	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Address != "azurerm_resource_group.example" {
		t.Errorf("expected address 'azurerm_resource_group.example', got %q", decisions[0].Address)
	}
	if !strings.Contains(decisions[0].MatchedRules[0], "builtin: deletion") {
		t.Errorf("expected MatchedRule to contain 'builtin: deletion', got %q", decisions[0].MatchedRules[0])
	}
	if !strings.Contains(decisions[0].MatchedRules[0], "is being deleted") {
		t.Errorf("expected MatchedRule to mention deletion, got %q", decisions[0].MatchedRules[0])
	}
}

func TestDeletionAnalyzer_ReplaceNotFlagged(t *testing.T) {
	a := &DeletionAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_resource_group.example",
			Type:    "azurerm_resource_group",
			Actions: []string{"delete", "create"},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for replace, got %d", len(decisions))
	}
}

func TestDeletionAnalyzer_CreateIgnored(t *testing.T) {
	a := &DeletionAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"create"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for create, got %d", len(decisions))
	}
}

func TestDeletionAnalyzer_UpdateIgnored(t *testing.T) {
	a := &DeletionAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"update"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for update, got %d", len(decisions))
	}
}

func TestDeletionAnalyzer_EmptyActions(t *testing.T) {
	a := &DeletionAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for empty actions, got %d", len(decisions))
	}
}

func TestDeletionAnalyzer_MultipleResources(t *testing.T) {
	a := &DeletionAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"delete"}},
		{Address: "vnet.one", Type: "azurerm_virtual_network", Actions: []string{"create"}},
		{Address: "rg.two", Type: "azurerm_resource_group", Actions: []string{"update"}},
		{Address: "rg.three", Type: "azurerm_resource_group", Actions: []string{"delete"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions (two deletes), got %d", len(decisions))
	}
	if decisions[0].Address != "rg.one" {
		t.Errorf("expected first decision for 'rg.one', got %q", decisions[0].Address)
	}
	if decisions[1].Address != "rg.three" {
		t.Errorf("expected second decision for 'rg.three', got %q", decisions[1].Address)
	}
}

func TestDeletionAnalyzer_NoChanges(t *testing.T) {
	a := &DeletionAnalyzer{}
	decisions := a.Analyze([]plan.ResourceChange{})
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for no changes, got %d", len(decisions))
	}
}

func TestDeletionAnalyzer_NilChanges(t *testing.T) {
	a := &DeletionAnalyzer{}
	decisions := a.Analyze(nil)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for nil changes, got %d", len(decisions))
	}
}

func TestDeletionAnalyzer_ResourceTypePreserved(t *testing.T) {
	a := &DeletionAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "aws_instance.web", Type: "aws_instance", Actions: []string{"delete"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].ResourceType != "aws_instance" {
		t.Errorf("expected resource type 'aws_instance', got %q", decisions[0].ResourceType)
	}
	if len(decisions[0].Actions) != 1 || decisions[0].Actions[0] != "delete" {
		t.Errorf("expected actions [delete], got %v", decisions[0].Actions)
	}
}

func TestDeletionAnalyzer_EmptyClassification(t *testing.T) {
	a := &DeletionAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"delete"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Classification != "" {
		t.Errorf("expected empty classification, got %q", decisions[0].Classification)
	}
}

func TestIsStandaloneDelete(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
		want    bool
	}{
		{"delete only", []string{"delete"}, true},
		{"delete and create", []string{"delete", "create"}, false},
		{"create and delete", []string{"create", "delete"}, false},
		{"create only", []string{"create"}, false},
		{"update only", []string{"update"}, false},
		{"no-op", []string{"no-op"}, false},
		{"empty", []string{}, false},
		{"delete with update", []string{"delete", "update"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStandaloneDelete(tt.actions)
			if got != tt.want {
				t.Errorf("isStandaloneDelete(%v) = %v, want %v", tt.actions, got, tt.want)
			}
		})
	}
}
