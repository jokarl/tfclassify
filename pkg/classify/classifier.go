// Package classify provides the core classification engine.
package classify

import (
	"github.com/jokarl/tfclassify/pkg/config"
	"github.com/jokarl/tfclassify/pkg/plan"
)

// Classifier applies config rules to plan changes.
type Classifier struct {
	config         *config.Config
	matchers       map[string][]compiledRule // classification name -> compiled rules
	precedenceMap  map[string]int            // classification name -> precedence index (lower is higher precedence)
	descriptionMap map[string]string         // classification name -> description
}

// New creates a new Classifier from a config.
func New(cfg *config.Config) (*Classifier, error) {
	matchers, err := compileRules(cfg)
	if err != nil {
		return nil, err
	}

	// Build precedence map and description map
	precedenceMap := make(map[string]int)
	descriptionMap := make(map[string]string)
	for i, name := range cfg.Precedence {
		precedenceMap[name] = i
	}
	for _, classification := range cfg.Classifications {
		descriptionMap[classification.Name] = classification.Description
	}

	return &Classifier{
		config:         cfg,
		matchers:       matchers,
		precedenceMap:  precedenceMap,
		descriptionMap: descriptionMap,
	}, nil
}

// Classify applies rules from config to the given resource changes.
func (c *Classifier) Classify(changes []plan.ResourceChange) *Result {
	result := &Result{
		ResourceDecisions: make([]ResourceDecision, 0, len(changes)),
	}

	// Handle no changes case
	if len(changes) == 0 {
		result.NoChanges = true
		result.Overall = c.config.Defaults.NoChanges
		result.OverallDescription = c.descriptionMap[result.Overall]
		result.OverallExitCode = c.getExitCode(result.Overall)
		return result
	}

	// Classify each resource
	highestPrecedence := -1
	for _, change := range changes {
		decision := c.classifyResource(change)
		result.ResourceDecisions = append(result.ResourceDecisions, decision)

		// Track highest precedence classification
		precedence := c.precedenceMap[decision.Classification]
		if highestPrecedence == -1 || precedence < highestPrecedence {
			highestPrecedence = precedence
			result.Overall = decision.Classification
			result.OverallDescription = c.descriptionMap[result.Overall]
		}
	}

	result.OverallExitCode = c.getExitCode(result.Overall)
	return result
}

// classifyResource determines the classification for a single resource change.
func (c *Classifier) classifyResource(change plan.ResourceChange) ResourceDecision {
	decision := ResourceDecision{
		Address:      change.Address,
		ResourceType: change.Type,
		Actions:      change.Actions,
	}

	// Try each classification in precedence order
	for _, classificationName := range c.config.Precedence {
		rules := c.matchers[classificationName]

		for _, rule := range rules {
			if rule.matchesResource(change.Type) && rule.matchesActions(change.Actions) {
				decision.Classification = classificationName
				decision.ClassificationDescription = rule.classificationDescription
				decision.MatchedRule = rule.ruleDescription
				return decision
			}
		}
	}

	// No rule matched, use default
	decision.Classification = c.config.Defaults.Unclassified
	decision.MatchedRule = "default (no rule matched)"
	return decision
}

// getExitCode returns the exit code for a classification.
// Exit codes are derived from the classification's position in the precedence list.
// The highest precedence (first in list) gets the highest exit code.
func (c *Classifier) getExitCode(classification string) int {
	// Special case: no changes returns 0
	if classification == c.config.Defaults.NoChanges {
		return 0
	}

	precedence, ok := c.precedenceMap[classification]
	if !ok {
		return 0
	}

	// Invert: first in precedence (index 0) gets highest code
	// Exit codes: 0 = lowest precedence, N-1 = highest precedence
	// But we want: highest precedence = highest code
	maxPrecedence := len(c.config.Precedence) - 1
	return maxPrecedence - precedence
}

// AddPluginDecisions integrates plugin decisions with the existing core decisions.
// Plugin decisions are merged with the existing resource decisions, and the overall
// classification is recalculated based on precedence.
func (c *Classifier) AddPluginDecisions(result *Result, pluginDecisions []ResourceDecision) {
	// Create a map of existing decisions by address
	decisionMap := make(map[string]*ResourceDecision)
	for i := range result.ResourceDecisions {
		decisionMap[result.ResourceDecisions[i].Address] = &result.ResourceDecisions[i]
	}

	// Process plugin decisions
	for _, pluginDecision := range pluginDecisions {
		// Skip decisions with empty classification - these are invalid
		if pluginDecision.Classification == "" {
			continue
		}

		// Skip decisions with unknown classifications not in precedence map
		if _, known := c.precedenceMap[pluginDecision.Classification]; !known {
			continue
		}

		existing, ok := decisionMap[pluginDecision.Address]
		if !ok {
			// New resource from plugin (shouldn't happen in normal flow)
			result.ResourceDecisions = append(result.ResourceDecisions, pluginDecision)
			continue
		}

		// Compare precedence and keep higher precedence classification
		existingPrecedence, existingKnown := c.precedenceMap[existing.Classification]
		pluginPrecedence := c.precedenceMap[pluginDecision.Classification]

		// Only replace if plugin classification has higher precedence
		// (lower index = higher precedence)
		if !existingKnown || pluginPrecedence < existingPrecedence {
			existing.Classification = pluginDecision.Classification
			existing.ClassificationDescription = c.descriptionMap[pluginDecision.Classification]
			existing.MatchedRule = pluginDecision.MatchedRule
		}
	}

	// Recalculate overall
	if len(result.ResourceDecisions) > 0 {
		highestPrecedence := -1
		for _, decision := range result.ResourceDecisions {
			precedence, known := c.precedenceMap[decision.Classification]
			if !known {
				continue
			}
			if highestPrecedence == -1 || precedence < highestPrecedence {
				highestPrecedence = precedence
				result.Overall = decision.Classification
				result.OverallDescription = c.descriptionMap[decision.Classification]
			}
		}
		result.OverallExitCode = c.getExitCode(result.Overall)
	}
}
