// Package classify provides the core classification engine.
package classify

import (
	"github.com/jokarl/tfclassify/internal/plan"
)

// BuiltinAnalyzer inspects plan changes and produces additional classification decisions.
// Builtin analyzers run in-process after core rule classification and before external plugins.
type BuiltinAnalyzer interface {
	// Name returns the analyzer's identifier.
	Name() string
	// Analyze inspects the given resource changes and returns decisions for any
	// changes that match the analyzer's criteria.
	Analyze(changes []plan.ResourceChange) []ResourceDecision
}

// PlanAwareAnalyzer is like BuiltinAnalyzer but receives the full ParseResult
// for analyzers that need plan-level context (drift, topology).
type PlanAwareAnalyzer interface {
	// Name returns the analyzer's identifier.
	Name() string
	// AnalyzePlan inspects the full parse result and returns decisions.
	AnalyzePlan(result *plan.ParseResult) []ResourceDecision
}

// RunBuiltinAnalyzers runs all registered builtin analyzers and plan-aware analyzers
// against the given plan result and merges their decisions into the result using precedence rules.
func (c *Classifier) RunBuiltinAnalyzers(result *Result, planResult *plan.ParseResult, analyzers []BuiltinAnalyzer, planAnalyzers []PlanAwareAnalyzer) {
	var allDecisions []ResourceDecision
	for _, a := range analyzers {
		decisions := a.Analyze(planResult.Changes)
		allDecisions = append(allDecisions, decisions...)
	}
	for _, a := range planAnalyzers {
		decisions := a.AnalyzePlan(planResult)
		allDecisions = append(allDecisions, decisions...)
	}
	if len(allDecisions) > 0 {
		c.AddPluginDecisions(result, allDecisions)
	}
}
