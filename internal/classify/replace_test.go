package classify

import (
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/internal/plan"
)

func TestReplaceAnalyzer_Name(t *testing.T) {
	a := &ReplaceAnalyzer{}
	if a.Name() != "replace" {
		t.Errorf("expected name 'replace', got %q", a.Name())
	}
}

func TestReplaceAnalyzer_DetectsReplace(t *testing.T) {
	a := &ReplaceAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_resource_group.example",
			Type:    "azurerm_resource_group",
			Actions: []string{"delete", "create"},
		},
	}

	decisions := a.Analyze(changes)

	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if !strings.Contains(decisions[0].MatchedRules[0], "builtin: replace") {
		t.Errorf("expected MatchedRule to contain 'builtin: replace', got %q", decisions[0].MatchedRules[0])
	}
	if !strings.Contains(decisions[0].MatchedRules[0], "will be replaced") {
		t.Errorf("expected MatchedRule to mention replacement, got %q", decisions[0].MatchedRules[0])
	}
}

func TestReplaceAnalyzer_CreateDeleteOrder(t *testing.T) {
	a := &ReplaceAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "aws_instance.foo", Type: "aws_instance", Actions: []string{"create", "delete"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Errorf("expected 1 decision for create+delete, got %d", len(decisions))
	}
}

func TestReplaceAnalyzer_DeleteOnly(t *testing.T) {
	a := &ReplaceAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"delete"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for delete-only, got %d", len(decisions))
	}
}

func TestReplaceAnalyzer_CreateOnly(t *testing.T) {
	a := &ReplaceAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"create"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for create-only, got %d", len(decisions))
	}
}

func TestReplaceAnalyzer_EmptyActions(t *testing.T) {
	a := &ReplaceAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for empty actions, got %d", len(decisions))
	}
}

func TestReplaceAnalyzer_NoChanges(t *testing.T) {
	a := &ReplaceAnalyzer{}
	decisions := a.Analyze([]plan.ResourceChange{})
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for no changes, got %d", len(decisions))
	}
}

func TestReplaceAnalyzer_NilChanges(t *testing.T) {
	a := &ReplaceAnalyzer{}
	decisions := a.Analyze(nil)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for nil changes, got %d", len(decisions))
	}
}

func TestReplaceAnalyzer_MultipleResources(t *testing.T) {
	a := &ReplaceAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "aws.one", Type: "aws_instance", Actions: []string{"delete", "create"}},
		{Address: "aws.two", Type: "aws_instance", Actions: []string{"update"}},
		{Address: "aws.three", Type: "aws_instance", Actions: []string{"create", "delete"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions (two replaces), got %d", len(decisions))
	}
	if decisions[0].Address != "aws.one" {
		t.Errorf("expected first decision for 'aws.one', got %q", decisions[0].Address)
	}
	if decisions[1].Address != "aws.three" {
		t.Errorf("expected second decision for 'aws.three', got %q", decisions[1].Address)
	}
}

func TestReplaceAnalyzer_EmptyClassification(t *testing.T) {
	a := &ReplaceAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "rg.one", Type: "azurerm_resource_group", Actions: []string{"delete", "create"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Classification != "" {
		t.Errorf("expected empty classification, got %q", decisions[0].Classification)
	}
}

func TestReplaceAnalyzer_ResourceTypePreserved(t *testing.T) {
	a := &ReplaceAnalyzer{}
	changes := []plan.ResourceChange{
		{Address: "aws_instance.web", Type: "aws_instance", Actions: []string{"delete", "create"}},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].ResourceType != "aws_instance" {
		t.Errorf("expected resource type 'aws_instance', got %q", decisions[0].ResourceType)
	}
}

func TestIsReplace(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
		want    bool
	}{
		{"delete+create", []string{"delete", "create"}, true},
		{"create+delete", []string{"create", "delete"}, true},
		{"delete only", []string{"delete"}, false},
		{"create only", []string{"create"}, false},
		{"update only", []string{"update"}, false},
		{"no-op", []string{"no-op"}, false},
		{"empty", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isReplace(tt.actions)
			if got != tt.want {
				t.Errorf("isReplace(%v) = %v, want %v", tt.actions, got, tt.want)
			}
		})
	}
}
