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

// ClassificationAwareAnalyzer is an optional interface that analyzers can implement
// to receive classification context. Analyzers that implement this interface receive
// the classification name and JSON-encoded per-analyzer configuration, allowing them
// to apply different thresholds or behaviors per classification level.
type ClassificationAwareAnalyzer interface {
	Analyzer

	// AnalyzeWithClassification inspects resources with classification context.
	// classification is the name of the classification block (e.g., "critical")
	// analyzerConfig is JSON-encoded per-analyzer configuration from the classification block
	AnalyzeWithClassification(runner Runner, classification string, analyzerConfig []byte) error
}
