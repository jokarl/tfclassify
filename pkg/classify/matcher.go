// Package classify provides the core classification engine.
package classify

import (
	"fmt"

	"github.com/gobwas/glob"
	"github.com/jokarl/tfclassify/pkg/config"
)

// compiledRule is a pre-compiled classification rule.
type compiledRule struct {
	classification   string
	resourceGlobs    []glob.Glob
	notResourceGlobs []glob.Glob
	actions          []string
	ruleDescription  string
}

// compileRules compiles all rules from a config into compiled rules.
func compileRules(cfg *config.Config) (map[string][]compiledRule, error) {
	result := make(map[string][]compiledRule)

	for _, classification := range cfg.Classifications {
		rules := make([]compiledRule, 0, len(classification.Rules))

		for i, rule := range classification.Rules {
			compiled := compiledRule{
				classification:  classification.Name,
				actions:         rule.Actions,
				ruleDescription: formatRuleDescription(classification.Name, i+1, rule),
			}

			// Compile resource globs
			for _, pattern := range rule.Resource {
				g, err := glob.Compile(pattern)
				if err != nil {
					return nil, err
				}
				compiled.resourceGlobs = append(compiled.resourceGlobs, g)
			}

			// Compile not_resource globs
			for _, pattern := range rule.NotResource {
				g, err := glob.Compile(pattern)
				if err != nil {
					return nil, err
				}
				compiled.notResourceGlobs = append(compiled.notResourceGlobs, g)
			}

			rules = append(rules, compiled)
		}

		result[classification.Name] = rules
	}

	return result, nil
}

// matchesResource returns true if the resource type matches this rule's patterns.
func (r *compiledRule) matchesResource(resourceType string) bool {
	// If resource globs are specified, check if type matches any of them
	if len(r.resourceGlobs) > 0 {
		for _, g := range r.resourceGlobs {
			if g.Match(resourceType) {
				return true
			}
		}
		return false
	}

	// If not_resource globs are specified, check if type does NOT match any of them
	if len(r.notResourceGlobs) > 0 {
		for _, g := range r.notResourceGlobs {
			if g.Match(resourceType) {
				return false
			}
		}
		return true
	}

	return false
}

// matchesActions returns true if any of the change actions match this rule's actions.
// If the rule has no actions specified, it matches any actions.
func (r *compiledRule) matchesActions(changeActions []string) bool {
	if len(r.actions) == 0 {
		return true
	}

	for _, ruleAction := range r.actions {
		for _, changeAction := range changeActions {
			if ruleAction == changeAction {
				return true
			}
		}
	}

	return false
}

// formatRuleDescription creates a human-readable description of a rule.
func formatRuleDescription(classification string, ruleIndex int, rule config.RuleConfig) string {
	desc := fmt.Sprintf("%s rule %d", classification, ruleIndex)

	if len(rule.Resource) > 0 {
		desc += " (resource: " + rule.Resource[0]
		if len(rule.Resource) > 1 {
			desc += ", ..."
		}
		desc += ")"
	} else if len(rule.NotResource) > 0 {
		desc += " (not_resource: " + rule.NotResource[0]
		if len(rule.NotResource) > 1 {
			desc += ", ..."
		}
		desc += ")"
	}

	return desc
}
