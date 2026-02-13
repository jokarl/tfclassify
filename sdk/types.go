// Package sdk provides the public interfaces and types for tfclassify plugin authors.
package sdk

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
	BeforeSensitive interface{}
	AfterSensitive  interface{}
}

// Decision represents a classification decision from a plugin analyzer.
type Decision struct {
	// Classification is the classification level to assign.
	// If empty, the host will use severity to determine classification.
	Classification string

	// Reason explains why this decision was made.
	Reason string

	// Severity is a fine-grained ordering value within a classification (0-100).
	// Higher values indicate more severe changes.
	Severity int

	// Metadata contains additional context about the decision.
	Metadata map[string]interface{}
}
