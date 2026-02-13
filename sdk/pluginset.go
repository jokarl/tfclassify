// Package sdk provides the public interfaces and types for tfclassify plugin authors.
package sdk

// PluginSet defines a collection of analyzers provided by a plugin.
// Each plugin binary implements a single PluginSet containing one or more Analyzers.
type PluginSet interface {
	// PluginSetName returns the unique name of this plugin set.
	PluginSetName() string

	// PluginSetVersion returns the version of this plugin set.
	PluginSetVersion() string

	// AnalyzerNames returns the names of all analyzers in this plugin set.
	AnalyzerNames() []string
}
