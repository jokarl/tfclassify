// Package classify provides the core classification engine.
package classify

import (
	"github.com/jokarl/tfclassify/pkg/plan"
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

// RunBuiltinAnalyzers runs all registered builtin analyzers against the given
// changes and merges their decisions into the result using precedence rules.
func (c *Classifier) RunBuiltinAnalyzers(result *Result, changes []plan.ResourceChange, analyzers []BuiltinAnalyzer) {
	var allDecisions []ResourceDecision
	for _, a := range analyzers {
		decisions := a.Analyze(changes)
		allDecisions = append(allDecisions, decisions...)
	}
	if len(allDecisions) > 0 {
		c.AddPluginDecisions(result, allDecisions)
	}
}
