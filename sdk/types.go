// Package sdk provides the public interfaces and types for tfclassify plugin authors.
package sdk

import "fmt"

// ResourceChange represents a single resource change from a Terraform plan.
// This is the SDK version of the type that plugins receive.
type ResourceChange struct {
	Address         string
	Type            string
	ProviderName    string
	Mode            string // "managed" or "data"
	Actions         []string
	Before          map[string]interface{}
	After           map[string]interface{}
	BeforeSensitive map[string]interface{}
	AfterSensitive  map[string]interface{}
}

// Decision represents a classification decision from a plugin analyzer.
type Decision struct {
	// Classification is the classification level to assign.
	// If empty, the host will use severity to determine classification.
	Classification string

	// Reason explains why this decision was made.
	Reason string

	// Severity is a fine-grained ordering value within a classification.
	// Valid range is 0-100 inclusive, where 0 means lowest severity and 100
	// means highest severity. Values outside this range will be rejected by
	// Validate. Plugin authors should use this to express relative importance
	// within a single classification level.
	Severity int

	// Metadata contains additional context about the decision.
	Metadata map[string]interface{}
}

// Validate checks that the Decision fields are within acceptable bounds.
// Returns an error if Severity is outside the 0-100 range.
func (d *Decision) Validate() error {
	if d.Severity < 0 || d.Severity > 100 {
		return fmt.Errorf("severity must be between 0 and 100, got %d", d.Severity)
	}
	return nil
}
