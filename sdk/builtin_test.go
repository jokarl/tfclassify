package sdk

import (
	"testing"
)

// testAnalyzer is a simple analyzer for testing.
type testAnalyzer struct {
	DefaultAnalyzer
	name string
}

func (a *testAnalyzer) Name() string {
	return a.name
}

func (a *testAnalyzer) Analyze(runner Runner) error {
	return nil
}

func TestBuiltinPluginSet_Name(t *testing.T) {
	ps := &BuiltinPluginSet{
		Name: "test",
	}

	if ps.PluginSetName() != "test" {
		t.Errorf("expected name 'test', got '%s'", ps.PluginSetName())
	}
}

func TestBuiltinPluginSet_Version(t *testing.T) {
	ps := &BuiltinPluginSet{
		Version: "1.0.0",
	}

	if ps.PluginSetVersion() != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", ps.PluginSetVersion())
	}
}

func TestBuiltinPluginSet_AnalyzerNames(t *testing.T) {
	ps := &BuiltinPluginSet{
		Analyzers: []Analyzer{
			&testAnalyzer{name: "analyzer-a"},
			&testAnalyzer{name: "analyzer-b"},
		},
	}

	names := ps.AnalyzerNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}

	if names[0] != "analyzer-a" {
		t.Errorf("expected first name 'analyzer-a', got '%s'", names[0])
	}

	if names[1] != "analyzer-b" {
		t.Errorf("expected second name 'analyzer-b', got '%s'", names[1])
	}
}

func TestBuiltinPluginSet_GetAnalyzer(t *testing.T) {
	a1 := &testAnalyzer{name: "analyzer-a"}
	a2 := &testAnalyzer{name: "analyzer-b"}

	ps := &BuiltinPluginSet{
		Analyzers: []Analyzer{a1, a2},
	}

	found := ps.GetAnalyzer("analyzer-a")
	if found != a1 {
		t.Error("expected to find analyzer-a")
	}

	found = ps.GetAnalyzer("nonexistent")
	if found != nil {
		t.Error("expected nil for nonexistent analyzer")
	}
}

func TestDefaultAnalyzer_Enabled(t *testing.T) {
	da := DefaultAnalyzer{}

	if !da.Enabled() {
		t.Error("expected DefaultAnalyzer to be enabled by default")
	}
}

func TestDefaultAnalyzer_ResourcePatterns(t *testing.T) {
	da := DefaultAnalyzer{}

	patterns := da.ResourcePatterns()
	if patterns != nil {
		t.Errorf("expected nil patterns, got %v", patterns)
	}
}

func TestBuiltinPluginSet_VersionConstraint_Empty(t *testing.T) {
	ps := &BuiltinPluginSet{
		Name:    "test",
		Version: "1.0.0",
	}

	constraint := ps.VersionConstraint()
	if constraint != "" {
		t.Errorf("expected empty version constraint by default, got %q", constraint)
	}
}

func TestBuiltinPluginSet_VersionConstraint_Set(t *testing.T) {
	ps := &BuiltinPluginSet{
		Name:                  "test",
		Version:               "1.0.0",
		HostVersionConstraint: ">= 0.1.0",
	}

	constraint := ps.VersionConstraint()
	if constraint != ">= 0.1.0" {
		t.Errorf("expected version constraint '>= 0.1.0', got %q", constraint)
	}
}

func TestBuiltinPluginSet_ConfigSchema_Nil(t *testing.T) {
	ps := &BuiltinPluginSet{
		Name:    "test",
		Version: "1.0.0",
	}

	schema := ps.ConfigSchema()
	if schema != nil {
		t.Errorf("expected nil config schema by default, got %v", schema)
	}
}

func TestBuiltinPluginSet_ConfigSchema_Set(t *testing.T) {
	ps := &BuiltinPluginSet{
		Name:    "test",
		Version: "1.0.0",
		Schema: &ConfigSchemaSpec{
			Attributes: []ConfigAttribute{
				{Name: "privileged_roles", Type: "list(string)", Required: false},
			},
		},
	}

	schema := ps.ConfigSchema()
	if schema == nil {
		t.Fatal("expected non-nil config schema")
	}

	if len(schema.Attributes) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(schema.Attributes))
	}

	if schema.Attributes[0].Name != "privileged_roles" {
		t.Errorf("expected attribute name 'privileged_roles', got %q", schema.Attributes[0].Name)
	}
}
