// Package sdk provides the public interfaces and types for tfclassify plugin authors.
package sdk

// Runner provides access to plan data and emits decisions.
// This interface is implemented by the host and called by plugins during analysis.
type Runner interface {
	// GetResourceChanges returns resource changes matching the given glob patterns.
	// If patterns is empty, returns all resource changes.
	GetResourceChanges(patterns []string) ([]*ResourceChange, error)

	// GetResourceChange returns a specific resource change by its address.
	GetResourceChange(address string) (*ResourceChange, error)

	// EmitDecision records a classification decision for a resource change.
	EmitDecision(analyzer Analyzer, change *ResourceChange, decision *Decision) error
}
