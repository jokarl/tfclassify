// Package classify provides the core classification engine.
package classify

// Result contains the classification results for a plan.
type Result struct {
	Overall           string             // highest-precedence classification
	OverallExitCode   int                // exit code corresponding to overall classification
	ResourceDecisions []ResourceDecision // per-resource decisions
	NoChanges         bool               // true if plan has no resource changes
}

// ResourceDecision represents the classification decision for a single resource.
type ResourceDecision struct {
	Address        string   // resource address
	ResourceType   string   // resource type
	Actions        []string // actions being performed
	Classification string   // the classification assigned
	MatchedRule    string   // description of which rule matched
}
