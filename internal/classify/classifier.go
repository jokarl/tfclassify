// Package classify provides the core classification engine.
package classify

import (
	"fmt"

	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
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
				decision.MatchedRules = []string{rule.ruleDescription}
				return decision
			}
		}
	}

	// No rule matched, use default
	decision.Classification = c.config.Defaults.Unclassified
	decision.ClassificationDescription = c.descriptionMap[decision.Classification]
	decision.MatchedRules = []string{"default (no rule matched)"}
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

// ExplainClassify evaluates all rules for each resource without short-circuiting,
// recording each evaluation step. This produces a full trace of the classification
// pipeline for debugging. The final classification matches what Classify() returns.
func (c *Classifier) ExplainClassify(changes []plan.ResourceChange) *ExplainResult {
	result := &ExplainResult{
		Resources: make([]ResourceExplanation, 0, len(changes)),
	}

	if len(changes) == 0 {
		result.NoChanges = true
		return result
	}

	for _, change := range changes {
		explanation := c.explainResource(change)
		result.Resources = append(result.Resources, explanation)
	}

	return result
}

// explainResource traces every rule evaluation for a single resource.
func (c *Classifier) explainResource(change plan.ResourceChange) ResourceExplanation {
	explanation := ResourceExplanation{
		Address:      change.Address,
		ResourceType: change.Type,
		Actions:      change.Actions,
	}

	// Track the best match (same logic as classifyResource, but evaluate all)
	bestClassification := ""
	bestPrecedence := -1
	bestSource := ""
	bestRule := ""

	for _, classificationName := range c.config.Precedence {
		rules := c.matchers[classificationName]
		precedence := c.precedenceMap[classificationName]

		for _, rule := range rules {
			entry := TraceEntry{
				Classification: classificationName,
				Source:         "core-rule",
				Rule:           rule.ruleDescription,
			}

			resourceMatch := rule.matchesResource(change.Type)
			actionMatch := rule.matchesActions(change.Actions)

			if resourceMatch && actionMatch {
				entry.Result = TraceMatch
				if len(rule.actions) == 0 {
					entry.Reason = "catch-all"
				}
				// Track best match
				if bestPrecedence == -1 || precedence < bestPrecedence {
					bestClassification = classificationName
					bestPrecedence = precedence
					bestSource = "core-rule"
					bestRule = rule.ruleDescription
				}
			} else {
				entry.Result = TraceSkip
				if !resourceMatch {
					entry.Reason = "resource mismatch"
				} else {
					entry.Reason = formatActionMismatch(change.Actions, rule.actions)
				}
			}

			explanation.Trace = append(explanation.Trace, entry)
		}
	}

	// Set final classification
	if bestClassification != "" {
		explanation.FinalClassification = bestClassification
		explanation.FinalSource = bestSource
		_ = bestRule // used for winner reason
	} else {
		explanation.FinalClassification = c.config.Defaults.Unclassified
		explanation.FinalSource = "default"
	}

	return explanation
}

// formatActionMismatch describes why an action-constrained rule didn't match.
func formatActionMismatch(changeActions []string, ruleActions map[string]struct{}) string {
	if len(ruleActions) == 0 {
		return "action mismatch"
	}
	ruleList := make([]string, 0, len(ruleActions))
	for a := range ruleActions {
		ruleList = append(ruleList, a)
	}
	return fmt.Sprintf("action mismatch: %v not in %v", changeActions, ruleList)
}

// AddExplainBuiltinAnalyzers runs builtin analyzers and adds their results to the
// explanation trace. Returns decisions for precedence merging.
func (c *Classifier) AddExplainBuiltinAnalyzers(result *ExplainResult, changes []plan.ResourceChange, analyzers []BuiltinAnalyzer) []ResourceDecision {
	// Build a lookup of explanations by address
	explanationMap := make(map[string]*ResourceExplanation)
	for i := range result.Resources {
		explanationMap[result.Resources[i].Address] = &result.Resources[i]
	}

	var allDecisions []ResourceDecision
	for _, analyzer := range analyzers {
		decisions := analyzer.Analyze(changes)
		for _, decision := range decisions {
			allDecisions = append(allDecisions, decision)

			// Resolve empty classification to default (same as AddPluginDecisions)
			classification := decision.Classification
			if classification == "" {
				classification = c.config.Defaults.Unclassified
			}

			reason := ""
			if len(decision.MatchedRules) > 0 {
				reason = decision.MatchedRules[0]
			}

			entry := TraceEntry{
				Classification: classification,
				Source:         "builtin: " + analyzer.Name(),
				Rule:           analyzer.Name(),
				Result:         TraceMatch,
				Reason:         reason,
			}

			if exp, ok := explanationMap[decision.Address]; ok {
				exp.Trace = append(exp.Trace, entry)
			}
		}
	}

	return allDecisions
}

// AddExplainPluginDecisions adds plugin decisions to the explanation trace
// and merges them by precedence.
func (c *Classifier) AddExplainPluginDecisions(result *ExplainResult, pluginDecisions []ResourceDecision) {
	explanationMap := make(map[string]*ResourceExplanation)
	for i := range result.Resources {
		explanationMap[result.Resources[i].Address] = &result.Resources[i]
	}

	for _, decision := range pluginDecisions {
		classification := decision.Classification
		if classification == "" {
			classification = c.config.Defaults.Unclassified
		}
		if _, known := c.precedenceMap[classification]; !known {
			continue
		}

		pluginRule := ""
		if len(decision.MatchedRules) > 0 {
			pluginRule = decision.MatchedRules[0]
		}

		entry := TraceEntry{
			Classification: classification,
			Source:         "plugin: " + pluginRule,
			Rule:           pluginRule,
			Result:         TraceMatch,
		}

		if exp, ok := explanationMap[decision.Address]; ok {
			exp.Trace = append(exp.Trace, entry)
		}
	}
}

// FinalizeExplanation sets the winner reason and final source for each resource
// by re-evaluating precedence across all trace matches (core, builtin, plugin).
func (c *Classifier) FinalizeExplanation(result *ExplainResult) {
	for i := range result.Resources {
		exp := &result.Resources[i]

		bestPrecedence := -1
		bestClassification := ""
		bestSource := ""

		for _, entry := range exp.Trace {
			if entry.Result != TraceMatch {
				continue
			}

			precedence, known := c.precedenceMap[entry.Classification]
			if !known {
				continue
			}

			if bestPrecedence == -1 || precedence < bestPrecedence {
				bestPrecedence = precedence
				bestClassification = entry.Classification
				bestSource = entry.Source
			}
		}

		if bestClassification != "" {
			exp.FinalClassification = bestClassification
			exp.FinalSource = bestSource

			// Build winner reason
			matchCount := 0
			for _, entry := range exp.Trace {
				if entry.Result == TraceMatch {
					matchCount++
				}
			}
			if matchCount == 1 {
				exp.WinnerReason = "only match"
			} else {
				exp.WinnerReason = fmt.Sprintf("precedence rank %d", bestPrecedence)
			}
		} else {
			exp.FinalClassification = c.config.Defaults.Unclassified
			exp.FinalSource = "default"
			exp.WinnerReason = "no rule matched"
		}
	}
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
		classification := pluginDecision.Classification

		// Handle empty classification - use defaults.unclassified as fallback
		// (Future: map based on severity thresholds from config)
		if classification == "" {
			classification = c.config.Defaults.Unclassified
		}

		// Skip decisions with unknown classifications not in precedence map
		if _, known := c.precedenceMap[classification]; !known {
			continue
		}

		// Update the decision with resolved classification
		pluginDecision.Classification = classification

		existing, ok := decisionMap[pluginDecision.Address]
		if !ok {
			// New resource from plugin (shouldn't happen in normal flow)
			result.ResourceDecisions = append(result.ResourceDecisions, pluginDecision)
			continue
		}

		// Compare precedence and keep higher precedence classification
		existingPrecedence, existingKnown := c.precedenceMap[existing.Classification]
		pluginPrecedence := c.precedenceMap[pluginDecision.Classification]

		if !existingKnown || pluginPrecedence < existingPrecedence {
			// Higher precedence: replace classification and rules entirely
			existing.Classification = pluginDecision.Classification
			existing.ClassificationDescription = c.descriptionMap[pluginDecision.Classification]
			existing.MatchedRules = pluginDecision.MatchedRules
		} else if pluginPrecedence == existingPrecedence {
			// Same classification level: append rules for multi-reason visibility
			existing.MatchedRules = append(existing.MatchedRules, pluginDecision.MatchedRules...)
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
