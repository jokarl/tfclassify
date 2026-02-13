package bundled

import (
	"errors"
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

// mockRunner implements sdk.Runner for testing.
type mockRunner struct {
	changes   []*sdk.ResourceChange
	decisions []*emittedDecision
	err       error
	emitErr   error
}

type emittedDecision struct {
	analyzer sdk.Analyzer
	change   *sdk.ResourceChange
	decision *sdk.Decision
}

func (m *mockRunner) GetResourceChanges(patterns []string) ([]*sdk.ResourceChange, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.changes, nil
}

func (m *mockRunner) GetResourceChange(address string) (*sdk.ResourceChange, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, c := range m.changes {
		if c.Address == address {
			return c, nil
		}
	}
	return nil, nil
}

func (m *mockRunner) EmitDecision(analyzer sdk.Analyzer, change *sdk.ResourceChange, decision *sdk.Decision) error {
	if m.emitErr != nil {
		return m.emitErr
	}
	m.decisions = append(m.decisions, &emittedDecision{
		analyzer: analyzer,
		change:   change,
		decision: decision,
	})
	return nil
}

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
		{
			name: "multiple sensitive attributes with one new",
			change: &sdk.ResourceChange{
				Before:          map[string]interface{}{"password": "old1", "api_key": "old2"},
				After:           map[string]interface{}{"password": "new1", "api_key": "new2"},
				BeforeSensitive: map[string]interface{}{"password": true},
				AfterSensitive:  map[string]interface{}{"password": true, "api_key": true},
			},
			expected: 2,
		},
		{
			name: "sensitive not a bool (false value)",
			change: &sdk.ResourceChange{
				Before:          map[string]interface{}{"password": "old"},
				After:           map[string]interface{}{"password": "new"},
				BeforeSensitive: map[string]interface{}{"password": false},
				AfterSensitive:  map[string]interface{}{"password": false},
			},
			expected: 0,
		},
		{
			name: "no duplicate when attr in both before and after sensitive",
			change: &sdk.ResourceChange{
				Before:          map[string]interface{}{"password": "old"},
				After:           map[string]interface{}{"password": "new"},
				BeforeSensitive: map[string]interface{}{"password": true},
				AfterSensitive:  map[string]interface{}{"password": true},
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

func TestDeletionAnalyzer_Analyze(t *testing.T) {
	tests := []struct {
		name           string
		changes        []*sdk.ResourceChange
		expectedCount  int
		expectedReason string
	}{
		{
			name: "emits decision for standalone delete",
			changes: []*sdk.ResourceChange{
				{Address: "aws_instance.foo", Actions: []string{"delete"}},
			},
			expectedCount:  1,
			expectedReason: "Resource aws_instance.foo is being deleted",
		},
		{
			name: "does not emit for replace",
			changes: []*sdk.ResourceChange{
				{Address: "aws_instance.foo", Actions: []string{"delete", "create"}},
			},
			expectedCount: 0,
		},
		{
			name: "does not emit for create",
			changes: []*sdk.ResourceChange{
				{Address: "aws_instance.foo", Actions: []string{"create"}},
			},
			expectedCount: 0,
		},
		{
			name: "emits for multiple deletes",
			changes: []*sdk.ResourceChange{
				{Address: "aws_instance.foo", Actions: []string{"delete"}},
				{Address: "aws_instance.bar", Actions: []string{"delete"}},
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &PluginConfig{DeletionEnabled: true}
			analyzer := NewDeletionAnalyzer(config)
			runner := &mockRunner{changes: tt.changes}

			err := analyzer.Analyze(runner)
			if err != nil {
				t.Fatalf("Analyze() returned error: %v", err)
			}

			if len(runner.decisions) != tt.expectedCount {
				t.Errorf("expected %d decisions, got %d", tt.expectedCount, len(runner.decisions))
			}

			if tt.expectedCount > 0 && tt.expectedReason != "" {
				if runner.decisions[0].decision.Reason != tt.expectedReason {
					t.Errorf("expected reason %q, got %q", tt.expectedReason, runner.decisions[0].decision.Reason)
				}
			}
		})
	}
}

func TestDeletionAnalyzer_Analyze_Error(t *testing.T) {
	config := &PluginConfig{DeletionEnabled: true}
	analyzer := NewDeletionAnalyzer(config)
	runner := &mockRunner{err: errors.New("test error")}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDeletionAnalyzer_Analyze_EmitError(t *testing.T) {
	config := &PluginConfig{DeletionEnabled: true}
	analyzer := NewDeletionAnalyzer(config)
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{Address: "aws_instance.foo", Actions: []string{"delete"}},
		},
		emitErr: errors.New("emit error"),
	}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSensitiveAnalyzer_Analyze(t *testing.T) {
	tests := []struct {
		name          string
		changes       []*sdk.ResourceChange
		expectedCount int
	}{
		{
			name: "emits decision for sensitive attribute change",
			changes: []*sdk.ResourceChange{
				{
					Address:         "aws_db_instance.main",
					Actions:         []string{"update"},
					Before:          map[string]interface{}{"password": "old"},
					After:           map[string]interface{}{"password": "new"},
					BeforeSensitive: map[string]interface{}{"password": true},
					AfterSensitive:  map[string]interface{}{"password": true},
				},
			},
			expectedCount: 1,
		},
		{
			name: "does not emit for non-sensitive change",
			changes: []*sdk.ResourceChange{
				{
					Address:         "aws_db_instance.main",
					Actions:         []string{"update"},
					Before:          map[string]interface{}{"name": "old"},
					After:           map[string]interface{}{"name": "new"},
					BeforeSensitive: nil,
					AfterSensitive:  nil,
				},
			},
			expectedCount: 0,
		},
		{
			name: "does not emit when sensitive attribute unchanged",
			changes: []*sdk.ResourceChange{
				{
					Address:         "aws_db_instance.main",
					Actions:         []string{"update"},
					Before:          map[string]interface{}{"password": "same"},
					After:           map[string]interface{}{"password": "same"},
					BeforeSensitive: map[string]interface{}{"password": true},
					AfterSensitive:  map[string]interface{}{"password": true},
				},
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &PluginConfig{SensitiveEnabled: true}
			analyzer := NewSensitiveAnalyzer(config)
			runner := &mockRunner{changes: tt.changes}

			err := analyzer.Analyze(runner)
			if err != nil {
				t.Fatalf("Analyze() returned error: %v", err)
			}

			if len(runner.decisions) != tt.expectedCount {
				t.Errorf("expected %d decisions, got %d", tt.expectedCount, len(runner.decisions))
			}
		})
	}
}

func TestSensitiveAnalyzer_Analyze_Error(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)
	runner := &mockRunner{err: errors.New("test error")}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSensitiveAnalyzer_Analyze_EmitError(t *testing.T) {
	config := &PluginConfig{SensitiveEnabled: true}
	analyzer := NewSensitiveAnalyzer(config)
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address:         "aws_db_instance.main",
				Actions:         []string{"update"},
				Before:          map[string]interface{}{"password": "old"},
				After:           map[string]interface{}{"password": "new"},
				BeforeSensitive: map[string]interface{}{"password": true},
				AfterSensitive:  map[string]interface{}{"password": true},
			},
		},
		emitErr: errors.New("emit error"),
	}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestReplaceAnalyzer_Analyze(t *testing.T) {
	tests := []struct {
		name           string
		changes        []*sdk.ResourceChange
		expectedCount  int
		expectedReason string
	}{
		{
			name: "emits decision for replace",
			changes: []*sdk.ResourceChange{
				{Address: "aws_instance.foo", Actions: []string{"delete", "create"}},
			},
			expectedCount:  1,
			expectedReason: "Resource aws_instance.foo will be replaced (destroy and recreate)",
		},
		{
			name: "does not emit for standalone delete",
			changes: []*sdk.ResourceChange{
				{Address: "aws_instance.foo", Actions: []string{"delete"}},
			},
			expectedCount: 0,
		},
		{
			name: "does not emit for create only",
			changes: []*sdk.ResourceChange{
				{Address: "aws_instance.foo", Actions: []string{"create"}},
			},
			expectedCount: 0,
		},
		{
			name: "emits for multiple replaces",
			changes: []*sdk.ResourceChange{
				{Address: "aws_instance.foo", Actions: []string{"delete", "create"}},
				{Address: "aws_instance.bar", Actions: []string{"create", "delete"}},
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &PluginConfig{ReplaceEnabled: true}
			analyzer := NewReplaceAnalyzer(config)
			runner := &mockRunner{changes: tt.changes}

			err := analyzer.Analyze(runner)
			if err != nil {
				t.Fatalf("Analyze() returned error: %v", err)
			}

			if len(runner.decisions) != tt.expectedCount {
				t.Errorf("expected %d decisions, got %d", tt.expectedCount, len(runner.decisions))
			}

			if tt.expectedCount > 0 && tt.expectedReason != "" {
				if runner.decisions[0].decision.Reason != tt.expectedReason {
					t.Errorf("expected reason %q, got %q", tt.expectedReason, runner.decisions[0].decision.Reason)
				}
			}
		})
	}
}

func TestReplaceAnalyzer_Analyze_Error(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)
	runner := &mockRunner{err: errors.New("test error")}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestReplaceAnalyzer_Analyze_EmitError(t *testing.T) {
	config := &PluginConfig{ReplaceEnabled: true}
	analyzer := NewReplaceAnalyzer(config)
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{Address: "aws_instance.foo", Actions: []string{"delete", "create"}},
		},
		emitErr: errors.New("emit error"),
	}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAnalyzer_ResourcePatterns(t *testing.T) {
	config := &PluginConfig{
		DeletionEnabled:  true,
		SensitiveEnabled: true,
		ReplaceEnabled:   true,
	}

	analyzers := []sdk.Analyzer{
		NewDeletionAnalyzer(config),
		NewSensitiveAnalyzer(config),
		NewReplaceAnalyzer(config),
	}

	for _, a := range analyzers {
		patterns := a.ResourcePatterns()
		if len(patterns) != 1 || patterns[0] != "*" {
			t.Errorf("analyzer %s: expected patterns [*], got %v", a.Name(), patterns)
		}
	}
}
