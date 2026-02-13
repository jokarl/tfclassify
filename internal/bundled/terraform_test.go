package bundled

import (
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

func TestNewTerraformPluginSet(t *testing.T) {
	ps := NewTerraformPluginSet()

	if ps.PluginSetName() != "terraform" {
		t.Errorf("expected name 'terraform', got %q", ps.PluginSetName())
	}

	if ps.PluginSetVersion() != Version {
		t.Errorf("expected version %q, got %q", Version, ps.PluginSetVersion())
	}

	names := ps.AnalyzerNames()
	expected := []string{"deletion", "sensitive", "replace"}
	if len(names) != len(expected) {
		t.Errorf("expected %d analyzers, got %d", len(expected), len(names))
	}

	for _, name := range expected {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected analyzer %q not found", name)
		}
	}
}

func TestDeletionAnalyzer_Name(t *testing.T) {
	config := &PluginConfig{DeletionEnabled: true}
	analyzer := NewDeletionAnalyzer(config)

	if analyzer.Name() != "deletion" {
		t.Errorf("expected name 'deletion', got %q", analyzer.Name())
	}
}

func TestDeletionAnalyzer_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &PluginConfig{DeletionEnabled: tt.enabled}
			analyzer := NewDeletionAnalyzer(config)
			if analyzer.Enabled() != tt.expected {
				t.Errorf("expected Enabled() = %v, got %v", tt.expected, analyzer.Enabled())
			}
		})
	}
}

func TestIsStandaloneDelete(t *testing.T) {
	tests := []struct {
		name     string
		actions  []string
		expected bool
	}{
		{"standalone delete", []string{"delete"}, true},
		{"replace (delete+create)", []string{"delete", "create"}, false},
		{"create only", []string{"create"}, false},
		{"update only", []string{"update"}, false},
		{"no-op", []string{"no-op"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isStandaloneDelete(tt.actions)
			if result != tt.expected {
				t.Errorf("isStandaloneDelete(%v) = %v, expected %v", tt.actions, result, tt.expected)
			}
		})
	}
}

func TestSensitiveAnalyzer_Name(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)

	if analyzer.Name() != "sensitive" {
		t.Errorf("expected name 'sensitive', got %q", analyzer.Name())
	}
}

func TestSensitiveAnalyzer_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &PluginConfig{SensitiveEnabled: tt.enabled}
			analyzer := NewSensitiveAnalyzer(config)
			if analyzer.Enabled() != tt.expected {
				t.Errorf("expected Enabled() = %v, got %v", tt.expected, analyzer.Enabled())
			}
		})
	}
}

func TestFindSensitiveChanges(t *testing.T) {
	tests := []struct {
		name     string
		change   *sdk.ResourceChange
		expected int
	}{
		{
			name: "sensitive attribute changed",
			change: &sdk.ResourceChange{
				Before:          map[string]interface{}{"password": "old"},
				After:           map[string]interface{}{"password": "new"},
				BeforeSensitive: map[string]interface{}{"password": true},
				AfterSensitive:  map[string]interface{}{"password": true},
			},
			expected: 1,
		},
		{
			name: "no sensitive attributes",
			change: &sdk.ResourceChange{
				Before:          map[string]interface{}{"name": "old"},
				After:           map[string]interface{}{"name": "new"},
				BeforeSensitive: nil,
				AfterSensitive:  nil,
			},
			expected: 0,
		},
		{
			name: "sensitive unchanged",
			change: &sdk.ResourceChange{
				Before:          map[string]interface{}{"password": "same"},
				After:           map[string]interface{}{"password": "same"},
				BeforeSensitive: map[string]interface{}{"password": true},
				AfterSensitive:  map[string]interface{}{"password": true},
			},
			expected: 0,
		},
		{
			name: "newly sensitive attribute",
			change: &sdk.ResourceChange{
				Before:          map[string]interface{}{"token": "old"},
				After:           map[string]interface{}{"token": "new"},
				BeforeSensitive: nil,
				AfterSensitive:  map[string]interface{}{"token": true},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSensitiveChanges(tt.change)
			if len(result) != tt.expected {
				t.Errorf("findSensitiveChanges() returned %d attrs, expected %d", len(result), tt.expected)
			}
		})
	}
}

func TestReplaceAnalyzer_Name(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)

	if analyzer.Name() != "replace" {
		t.Errorf("expected name 'replace', got %q", analyzer.Name())
	}
}

func TestReplaceAnalyzer_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &PluginConfig{ReplaceEnabled: tt.enabled}
			analyzer := NewReplaceAnalyzer(config)
			if analyzer.Enabled() != tt.expected {
				t.Errorf("expected Enabled() = %v, got %v", tt.expected, analyzer.Enabled())
			}
		})
	}
}

func TestIsReplace(t *testing.T) {
	tests := []struct {
		name     string
		actions  []string
		expected bool
	}{
		{"replace (delete+create)", []string{"delete", "create"}, true},
		{"replace (create+delete)", []string{"create", "delete"}, true},
		{"standalone delete", []string{"delete"}, false},
		{"create only", []string{"create"}, false},
		{"update only", []string{"update"}, false},
		{"no-op", []string{"no-op"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReplace(tt.actions)
			if result != tt.expected {
				t.Errorf("isReplace(%v) = %v, expected %v", tt.actions, result, tt.expected)
			}
		})
	}
}

func TestAsBoolMap(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"nil input", nil, true},
		{"valid map", map[string]interface{}{"key": true}, false},
		{"invalid type", "string", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := asBoolMap(tt.input)
			isNil := result == nil
			if isNil != tt.expected {
				t.Errorf("asBoolMap() returned nil=%v, expected nil=%v", isNil, tt.expected)
			}
		})
	}
}

func TestHasAttributeChanged(t *testing.T) {
	tests := []struct {
		name     string
		attr     string
		before   map[string]interface{}
		after    map[string]interface{}
		expected bool
	}{
		{
			name:     "attribute changed",
			attr:     "name",
			before:   map[string]interface{}{"name": "old"},
			after:    map[string]interface{}{"name": "new"},
			expected: true,
		},
		{
			name:     "attribute unchanged",
			attr:     "name",
			before:   map[string]interface{}{"name": "same"},
			after:    map[string]interface{}{"name": "same"},
			expected: false,
		},
		{
			name:     "attribute added",
			attr:     "name",
			before:   map[string]interface{}{},
			after:    map[string]interface{}{"name": "new"},
			expected: true,
		},
		{
			name:     "attribute removed",
			attr:     "name",
			before:   map[string]interface{}{"name": "old"},
			after:    map[string]interface{}{},
			expected: true,
		},
		{
			name:     "both nil",
			attr:     "name",
			before:   nil,
			after:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAttributeChanged(tt.attr, tt.before, tt.after)
			if result != tt.expected {
				t.Errorf("hasAttributeChanged() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
