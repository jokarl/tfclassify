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

	// VersionConstraint returns a semver constraint string specifying which
	// tfclassify host versions this plugin is compatible with.
	// An empty string means the plugin works with any host version.
	// Example: ">= 0.1.0"
	VersionConstraint() string

	// ConfigSchema returns the schema for this plugin's configuration block.
	// Returns nil if the plugin accepts no configuration or doesn't validate.
	ConfigSchema() *ConfigSchemaSpec
}

// ConfigSchemaSpec describes the expected structure of a plugin's config block.
type ConfigSchemaSpec struct {
	Attributes []ConfigAttribute
}

// ConfigAttribute describes a single attribute in a plugin's config schema.
type ConfigAttribute struct {
	Name     string
	Type     string // HCL type: "string", "number", "bool", "list(string)", etc.
	Required bool
}
