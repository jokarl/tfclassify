// Package sdk provides the public interfaces and types for tfclassify plugin authors.
package sdk

// Analyzer inspects resource changes and emits classification decisions.
// Plugin authors implement this interface for each type of analysis they want to perform.
type Analyzer interface {
	// Name returns the unique name of this analyzer within its plugin set.
	Name() string

	// Enabled returns whether this analyzer is currently enabled.
	Enabled() bool

	// ResourcePatterns returns glob patterns for resources this analyzer is interested in.
	// An empty slice means the analyzer wants all resources.
	ResourcePatterns() []string

	// Analyze inspects resources and emits decisions via the Runner.
	// This is called by the host after the plugin has been configured.
	Analyze(runner Runner) error
}
