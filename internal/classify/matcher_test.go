package classify

import (
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
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
		actions: map[string]struct{}{"delete": {}},
	}

	if !rule.matchesActions([]string{"delete"}) {
		t.Error("expected delete to match rule with actions=[delete]")
	}
}

func TestMatchesActions_NoMatch(t *testing.T) {
	rule := compiledRule{
		actions: map[string]struct{}{"delete": {}},
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
		actions: map[string]struct{}{"delete": {}, "create": {}},
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

func TestMatchesActions_NotActions(t *testing.T) {
	rule := compiledRule{
		notActions: map[string]struct{}{"no-op": {}},
	}

	if !rule.matchesActions([]string{"create"}) {
		t.Error("expected create to match rule with not_actions=[no-op]")
	}
	if !rule.matchesActions([]string{"update"}) {
		t.Error("expected update to match rule with not_actions=[no-op]")
	}
	if !rule.matchesActions([]string{"delete"}) {
		t.Error("expected delete to match rule with not_actions=[no-op]")
	}
}

func TestMatchesActions_NotActionsExcluded(t *testing.T) {
	rule := compiledRule{
		notActions: map[string]struct{}{"no-op": {}},
	}

	if rule.matchesActions([]string{"no-op"}) {
		t.Error("expected no-op NOT to match rule with not_actions=[no-op]")
	}
}

func TestMatchesActions_NotActionsMultiple(t *testing.T) {
	rule := compiledRule{
		notActions: map[string]struct{}{"read": {}, "no-op": {}},
	}

	if rule.matchesActions([]string{"read"}) {
		t.Error("expected read NOT to match rule with not_actions=[read, no-op]")
	}
	if rule.matchesActions([]string{"no-op"}) {
		t.Error("expected no-op NOT to match rule with not_actions=[read, no-op]")
	}
	if !rule.matchesActions([]string{"create"}) {
		t.Error("expected create to match rule with not_actions=[read, no-op]")
	}
	if !rule.matchesActions([]string{"delete"}) {
		t.Error("expected delete to match rule with not_actions=[read, no-op]")
	}
}

func TestMatchesActions_NeitherActionsNorNotActions(t *testing.T) {
	rule := compiledRule{
		actions:    nil,
		notActions: nil,
	}

	for _, action := range []string{"create", "update", "delete", "read", "no-op"} {
		if !rule.matchesActions([]string{action}) {
			t.Errorf("expected %s to match rule with neither actions nor not_actions", action)
		}
	}
}

func TestMatchesModule_NoModuleSpecified(t *testing.T) {
	// Rule with no module/not_module should match everything
	rule := compiledRule{}
	if !rule.matchesModule("") {
		t.Error("expected empty module address to match rule with no module constraint")
	}
	if !rule.matchesModule("module.production") {
		t.Error("expected module.production to match rule with no module constraint")
	}
}

func TestMatchesModule_MatchExact(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}, Module: []string{"module.production"}},
				},
			},
		},
	}

	matchers, err := compileRules(cfg)
	if err != nil {
		t.Fatalf("failed to compile rules: %v", err)
	}

	rules := matchers["critical"]
	if !rules[0].matchesModule("module.production") {
		t.Error("expected module.production to match exact module pattern")
	}
	if rules[0].matchesModule("module.staging") {
		t.Error("expected module.staging NOT to match module.production pattern")
	}
	if rules[0].matchesModule("") {
		t.Error("expected root module NOT to match module.production pattern")
	}
}

func TestMatchesModule_WildcardSingleLevel(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}, Module: []string{"module.*"}},
				},
			},
		},
	}

	matchers, err := compileRules(cfg)
	if err != nil {
		t.Fatalf("failed to compile rules: %v", err)
	}

	rules := matchers["critical"]
	if !rules[0].matchesModule("module.production") {
		t.Error("expected module.production to match module.* pattern")
	}
	if !rules[0].matchesModule("module.staging") {
		t.Error("expected module.staging to match module.* pattern")
	}
	// With dot separator, * should NOT match nested modules
	if rules[0].matchesModule("module.production.module.network") {
		t.Error("expected module.production.module.network NOT to match module.* (single level)")
	}
}

func TestMatchesModule_WildcardMultiLevel(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "critical",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}, Module: []string{"module.production.**"}},
				},
			},
		},
	}

	matchers, err := compileRules(cfg)
	if err != nil {
		t.Fatalf("failed to compile rules: %v", err)
	}

	rules := matchers["critical"]
	if !rules[0].matchesModule("module.production.module.network") {
		t.Error("expected module.production.module.network to match module.production.** pattern")
	}
	if !rules[0].matchesModule("module.production.module.network.module.subnet") {
		t.Error("expected deeply nested module to match module.production.** pattern")
	}
	if rules[0].matchesModule("module.staging.module.network") {
		t.Error("expected module.staging.module.network NOT to match module.production.** pattern")
	}
}

func TestMatchesModule_NotModule(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "standard",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}, NotModule: []string{"module.production", "module.production.**"}},
				},
			},
		},
	}

	matchers, err := compileRules(cfg)
	if err != nil {
		t.Fatalf("failed to compile rules: %v", err)
	}

	rules := matchers["standard"]
	if rules[0].matchesModule("module.production") {
		t.Error("expected module.production NOT to match not_module pattern")
	}
	if rules[0].matchesModule("module.production.module.network") {
		t.Error("expected module.production.module.network NOT to match not_module pattern")
	}
	if !rules[0].matchesModule("module.staging") {
		t.Error("expected module.staging to match not_module (excluded production)")
	}
	if !rules[0].matchesModule("") {
		t.Error("expected root module to match not_module (excluded production)")
	}
}

func TestMatchesModule_RootModuleEmptyString(t *testing.T) {
	cfg := &config.Config{
		Classifications: []config.ClassificationConfig{
			{
				Name: "critical",
				Rules: []config.RuleConfig{
					// Empty string matches root module resources
					{Resource: []string{"*"}, Module: []string{""}},
				},
			},
		},
	}

	matchers, err := compileRules(cfg)
	if err != nil {
		t.Fatalf("failed to compile rules: %v", err)
	}

	rules := matchers["critical"]
	if !rules[0].matchesModule("") {
		t.Error("expected root module (empty string) to match empty module pattern")
	}
	if rules[0].matchesModule("module.production") {
		t.Error("expected module.production NOT to match empty module pattern")
	}
}

func TestMatchesResource_NoGlobsSpecified(t *testing.T) {
	// A rule with neither resource nor not_resource globs should not match anything.
	// This is an edge case that shouldn't occur in valid configs (validation should catch it),
	// but the behavior should be well-defined.
	rule := compiledRule{
		resourceGlobs:    nil,
		notResourceGlobs: nil,
	}

	if rule.matchesResource("azurerm_virtual_network") {
		t.Error("expected a rule with no globs to NOT match any resource")
	}

	if rule.matchesResource("azurerm_role_assignment") {
		t.Error("expected a rule with no globs to NOT match any resource")
	}
}
