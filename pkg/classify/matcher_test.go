package classify

import (
	"testing"

	"github.com/jokarl/tfclassify/pkg/config"
)

func TestMatchesResource_SimpleGlob(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*_role_*"}},
				},
			},
		},
	}

	matchers, err := compileRules(cfg)
	if err != nil {
		t.Fatalf("failed to compile rules: %v", err)
	}

	rules := matchers["critical"]
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	if !rules[0].matchesResource("azurerm_role_assignment") {
		t.Error("expected azurerm_role_assignment to match *_role_*")
	}
}

func TestMatchesResource_NoMatch(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*_role_*"}},
				},
			},
		},
	}

	matchers, err := compileRules(cfg)
	if err != nil {
		t.Fatalf("failed to compile rules: %v", err)
	}

	rules := matchers["critical"]
	if rules[0].matchesResource("azurerm_virtual_network") {
		t.Error("expected azurerm_virtual_network NOT to match *_role_*")
	}
}

func TestMatchesResource_NotResource(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "standard",
				Rules: []config.RuleConfig{
					{NotResource: []string{"*_role_*"}},
				},
			},
		},
	}

	matchers, err := compileRules(cfg)
	if err != nil {
		t.Fatalf("failed to compile rules: %v", err)
	}

	rules := matchers["standard"]

	// Should NOT match resources containing _role_
	if rules[0].matchesResource("azurerm_role_assignment") {
		t.Error("expected azurerm_role_assignment NOT to match not_resource: *_role_*")
	}

	// Should match resources NOT containing _role_
	if !rules[0].matchesResource("azurerm_virtual_network") {
		t.Error("expected azurerm_virtual_network to match not_resource: *_role_*")
	}
}

func TestMatchesActions_Match(t *testing.T) {
	rule := compiledRule{
		actions: []string{"delete"},
	}

	if !rule.matchesActions([]string{"delete"}) {
		t.Error("expected delete to match rule with actions=[delete]")
	}
}

func TestMatchesActions_NoMatch(t *testing.T) {
	rule := compiledRule{
		actions: []string{"delete"},
	}

	if rule.matchesActions([]string{"update"}) {
		t.Error("expected update NOT to match rule with actions=[delete]")
	}
}

func TestMatchesActions_NoActionsInRule(t *testing.T) {
	rule := compiledRule{
		actions: nil,
	}

	if !rule.matchesActions([]string{"update"}) {
		t.Error("expected any action to match rule with no actions specified")
	}

	if !rule.matchesActions([]string{"delete"}) {
		t.Error("expected any action to match rule with no actions specified")
	}
}

func TestMatchesActions_MultipleActions(t *testing.T) {
	rule := compiledRule{
		actions: []string{"delete", "create"},
	}

	// Match if any of the change actions match any of the rule actions
	if !rule.matchesActions([]string{"delete"}) {
		t.Error("expected delete to match rule with actions=[delete,create]")
	}

	if !rule.matchesActions([]string{"create"}) {
		t.Error("expected create to match rule with actions=[delete,create]")
	}

	if rule.matchesActions([]string{"update"}) {
		t.Error("expected update NOT to match rule with actions=[delete,create]")
	}
}
