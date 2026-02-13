// Package sdk provides the public interfaces and types for tfclassify plugin authors.
package sdk

// BuiltinPluginSet provides a default PluginSet implementation.
// Plugin authors can embed this struct and configure it to create a PluginSet.
type BuiltinPluginSet struct {
	Name      string
	Version   string
	Analyzers []Analyzer
}

// PluginSetName returns the name of this plugin set.
func (s *BuiltinPluginSet) PluginSetName() string {
	return s.Name
}

// PluginSetVersion returns the version of this plugin set.
func (s *BuiltinPluginSet) PluginSetVersion() string {
	return s.Version
}

// AnalyzerNames returns the names of all analyzers in this plugin set.
func (s *BuiltinPluginSet) AnalyzerNames() []string {
	names := make([]string, len(s.Analyzers))
	for i, a := range s.Analyzers {
		names[i] = a.Name()
	}
	return names
}

// GetAnalyzer returns an analyzer by name, or nil if not found.
func (s *BuiltinPluginSet) GetAnalyzer(name string) Analyzer {
	for _, a := range s.Analyzers {
		if a.Name() == name {
			return a
		}
	}
	return nil
}

// DefaultAnalyzer provides default implementations for optional Analyzer methods.
// Plugin authors can embed this struct in their analyzer implementations.
type DefaultAnalyzer struct{}

// Enabled returns true by default. Override this method to conditionally disable an analyzer.
func (d DefaultAnalyzer) Enabled() bool {
	return true
}

// ResourcePatterns returns nil by default, meaning the analyzer wants all resources.
// Override this method to specify specific resource patterns.
func (d DefaultAnalyzer) ResourcePatterns() []string {
	return nil
}
