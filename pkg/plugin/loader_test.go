package plugin

import (
	"testing"

	"github.com/gobwas/glob"
	"github.com/jokarl/tfclassify/pkg/config"
	"github.com/jokarl/tfclassify/pkg/plan"
	"github.com/jokarl/tfclassify/sdk"
)

func TestRunner_GetResourceChanges_PatternMatch(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "azurerm_role_assignment.example", Type: "azurerm_role_assignment"},
		{Address: "azurerm_virtual_network.main", Type: "azurerm_virtual_network"},
		{Address: "azurerm_role_definition.custom", Type: "azurerm_role_definition"},
	}

	runner := NewRunner(host)

	changes, err := runner.GetResourceChanges([]string{"*_role_*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}

	// Verify only role-related resources are returned
	for _, change := range changes {
		if change.Type != "azurerm_role_assignment" && change.Type != "azurerm_role_definition" {
			t.Errorf("unexpected resource type: %s", change.Type)
		}
	}
}

func TestRunner_GetResourceChanges_NoMatch(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "azurerm_virtual_network.main", Type: "azurerm_virtual_network"},
	}

	runner := NewRunner(host)

	changes, err := runner.GetResourceChanges([]string{"*_nonexistent_*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}
}

func TestRunner_GetResourceChanges_AllResources(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "azurerm_role_assignment.example", Type: "azurerm_role_assignment"},
		{Address: "azurerm_virtual_network.main", Type: "azurerm_virtual_network"},
	}

	runner := NewRunner(host)

	// Empty patterns means all resources
	changes, err := runner.GetResourceChanges([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(changes) != 2 {
		t.Errorf("expected 2 changes, got %d", len(changes))
	}
}

func TestRunner_GetResourceChange_ByAddress(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "azurerm_virtual_network.main", Type: "azurerm_virtual_network"},
	}

	runner := NewRunner(host)

	change, err := runner.GetResourceChange("azurerm_virtual_network.main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if change.Address != "azurerm_virtual_network.main" {
		t.Errorf("unexpected address: %s", change.Address)
	}
}

func TestRunner_GetResourceChange_NotFound(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{}

	runner := NewRunner(host)

	_, err := runner.GetResourceChange("nonexistent.resource")
	if err == nil {
		t.Fatal("expected error for nonexistent resource")
	}
}

// mockAnalyzer is a test analyzer implementation
type mockAnalyzer struct {
	sdk.DefaultAnalyzer
	name string
}

func (m *mockAnalyzer) Name() string {
	return m.name
}

func (m *mockAnalyzer) Analyze(runner sdk.Runner) error {
	return nil
}

func TestRunner_EmitDecision(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	runner := NewRunner(host)

	analyzer := &mockAnalyzer{name: "test-analyzer"}
	change := &sdk.ResourceChange{
		Address: "test.resource",
		Type:    "test_resource",
		Actions: []string{"create"},
	}
	decision := &sdk.Decision{
		Classification: "critical",
		Reason:         "test reason",
		Severity:       80,
	}

	err := runner.EmitDecision(analyzer, change, decision)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(host.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(host.decisions))
	}

	d := host.decisions[0]
	if d.Classification != "critical" {
		t.Errorf("expected classification 'critical', got '%s'", d.Classification)
	}

	if d.Address != "test.resource" {
		t.Errorf("expected address 'test.resource', got '%s'", d.Address)
	}
}

func TestHost_NewHost(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{
			PluginTimeout: "10s",
		},
	}

	host := NewHost(cfg)

	if host.cfg != cfg {
		t.Error("expected config to be set")
	}

	if host.clients == nil {
		t.Error("expected clients map to be initialized")
	}
}

func TestRunner_GetResourceChanges_InvalidPattern(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "test.resource", Type: "test_resource"},
	}

	runner := NewRunner(host)

	// Invalid glob pattern (unmatched bracket)
	_, err := runner.GetResourceChanges([]string{"[invalid"})
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestMatchesAny(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		patterns     []string
		expected     bool
	}{
		{
			name:         "matches wildcard",
			resourceType: "azurerm_role_assignment",
			patterns:     []string{"*_role_*"},
			expected:     true,
		},
		{
			name:         "no match",
			resourceType: "azurerm_virtual_network",
			patterns:     []string{"*_role_*"},
			expected:     false,
		},
		{
			name:         "matches exact",
			resourceType: "aws_instance",
			patterns:     []string{"aws_instance"},
			expected:     true,
		},
		{
			name:         "matches one of multiple patterns",
			resourceType: "aws_instance",
			patterns:     []string{"*_bucket", "aws_*"},
			expected:     true,
		},
		{
			name:         "empty patterns returns false",
			resourceType: "anything",
			patterns:     []string{},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var globs []glob.Glob
			for _, p := range tt.patterns {
				g, err := glob.Compile(p)
				if err != nil {
					t.Fatalf("failed to compile pattern %q: %v", p, err)
				}
				globs = append(globs, g)
			}

			result := matchesAny(tt.resourceType, globs)
			if result != tt.expected {
				t.Errorf("matchesAny(%q, %v) = %v, want %v", tt.resourceType, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestToSDKResourceChange(t *testing.T) {
	planChange := &plan.ResourceChange{
		Address:         "aws_instance.main",
		Type:            "aws_instance",
		ProviderName:    "registry.terraform.io/hashicorp/aws",
		Mode:            "managed",
		Actions:         []string{"create"},
		Before:          nil,
		After:           map[string]interface{}{"ami": "ami-12345"},
		BeforeSensitive: nil,
		AfterSensitive:  map[string]interface{}{"password": true},
	}

	sdkChange := toSDKResourceChange(planChange)

	if sdkChange.Address != planChange.Address {
		t.Errorf("Address: got %q, want %q", sdkChange.Address, planChange.Address)
	}
	if sdkChange.Type != planChange.Type {
		t.Errorf("Type: got %q, want %q", sdkChange.Type, planChange.Type)
	}
	if sdkChange.ProviderName != planChange.ProviderName {
		t.Errorf("ProviderName: got %q, want %q", sdkChange.ProviderName, planChange.ProviderName)
	}
	if sdkChange.Mode != planChange.Mode {
		t.Errorf("Mode: got %q, want %q", sdkChange.Mode, planChange.Mode)
	}
	if len(sdkChange.Actions) != len(planChange.Actions) {
		t.Errorf("Actions length: got %d, want %d", len(sdkChange.Actions), len(planChange.Actions))
	}
	if sdkChange.After == nil {
		t.Error("After should not be nil")
	}
	if sdkChange.AfterSensitive == nil {
		t.Error("AfterSensitive should not be nil")
	}
}

func TestHost_RunAnalysis(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{
			PluginTimeout: "5s",
		},
	}

	host := NewHost(cfg)
	host.plugins = make(map[string]*DiscoveredPlugin)

	changes := []plan.ResourceChange{
		{Address: "aws_instance.main", Type: "aws_instance", Actions: []string{"create"}},
	}

	decisions, err := host.RunAnalysis(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With no plugins, expect no decisions
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions with no plugins, got %d", len(decisions))
	}
}

func TestHost_RunAnalysis_DefaultTimeout(t *testing.T) {
	cfg := &config.Config{
		Defaults: nil, // nil defaults should use default timeout
	}

	host := NewHost(cfg)
	host.plugins = make(map[string]*DiscoveredPlugin)

	changes := []plan.ResourceChange{}

	decisions, err := host.RunAnalysis(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions, got %d", len(decisions))
	}
}

func TestHost_RunAnalysis_InvalidTimeout(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{
			PluginTimeout: "invalid",
		},
	}

	host := NewHost(cfg)
	host.plugins = make(map[string]*DiscoveredPlugin)

	changes := []plan.ResourceChange{}

	// Should not error - invalid timeout should fall back to default
	decisions, err := host.RunAnalysis(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions, got %d", len(decisions))
	}
}

func TestHost_Shutdown(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)

	// Shutdown with no clients should not panic
	host.Shutdown()

	// Verify clients map is still valid
	if host.clients == nil {
		t.Error("clients map should not be nil after shutdown")
	}
}
