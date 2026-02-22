package plugin

import (
	"testing"

	"github.com/gobwas/glob"
	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
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

func TestHost_DiscoverAndStart_NoPlugins(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{}, // no plugins
	}

	host := NewHost(cfg)
	err := host.DiscoverAndStart("/usr/bin/tfclassify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(host.plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(host.plugins))
	}
}

func TestHost_DiscoverAndStart_SourcelessPluginNotFound(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "nonexistent", Enabled: true}, // no source, binary not found
		},
	}

	host := NewHost(cfg)
	err := host.DiscoverAndStart("/usr/bin/tfclassify")
	if err == nil {
		t.Fatal("expected error for enabled plugin with no binary")
	}

	// Cleanup
	host.Shutdown()
}

func TestHost_DiscoverAndStart_DisabledPlugins(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "terraform", Enabled: false}, // disabled
		},
	}

	host := NewHost(cfg)
	err := host.DiscoverAndStart("/usr/bin/tfclassify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(host.plugins) != 0 {
		t.Errorf("expected 0 plugins (disabled skipped), got %d", len(host.plugins))
	}
}

func TestHost_RunAnalysis_WithTimeout(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{
			PluginTimeout: "100ms",
		},
	}

	host := NewHost(cfg)
	host.plugins = make(map[string]*DiscoveredPlugin)

	changes := []plan.ResourceChange{
		{Address: "test.resource", Type: "test_type", Actions: []string{"create"}},
	}

	decisions, err := host.RunAnalysis(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With no plugins, expect no decisions but no error
	if decisions == nil {
		t.Error("expected non-nil decisions slice")
	}
}

func TestRunner_GetResourceChanges_MultiplePatterns(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "aws_instance.web", Type: "aws_instance"},
		{Address: "aws_s3_bucket.data", Type: "aws_s3_bucket"},
		{Address: "azurerm_role_assignment.admin", Type: "azurerm_role_assignment"},
		{Address: "google_compute_instance.server", Type: "google_compute_instance"},
	}

	runner := NewRunner(host)

	// Multiple patterns should return matches for all
	changes, err := runner.GetResourceChanges([]string{"aws_*", "google_*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(changes) != 3 {
		t.Errorf("expected 3 changes (2 aws + 1 google), got %d", len(changes))
	}
}

func TestRunner_EmitDecision_MetadataOnly(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	runner := NewRunner(host)

	analyzer := &mockAnalyzer{name: "sensitive-analyzer"}
	change := &sdk.ResourceChange{
		Address: "test.resource",
		Type:    "test_resource",
		Actions: []string{"update"},
	}
	// Empty classification = metadata-only decision
	decision := &sdk.Decision{
		Classification: "", // empty means metadata-only
		Reason:         "sensitive attribute changed",
		Severity:       70,
		Metadata:       map[string]interface{}{"attribute": "password"},
	}

	err := runner.EmitDecision(analyzer, change, decision)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(host.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(host.decisions))
	}

	d := host.decisions[0]
	if d.Classification != "" {
		t.Errorf("expected empty classification for metadata-only, got %q", d.Classification)
	}
}

func TestRunner_GetResourceChanges_WildcardPattern(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "aws_instance.web", Type: "aws_instance"},
		{Address: "azure_vm.server", Type: "azure_vm"},
	}

	runner := NewRunner(host)

	// Wildcard should match all
	changes, err := runner.GetResourceChanges([]string{"*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(changes) != 2 {
		t.Errorf("expected 2 changes, got %d", len(changes))
	}
}

func TestToSDKResourceChange_AllFields(t *testing.T) {
	planChange := &plan.ResourceChange{
		Address:      "module.app.aws_instance.main",
		Type:         "aws_instance",
		ProviderName: "registry.terraform.io/hashicorp/aws",
		Mode:         "managed",
		Actions:      []string{"delete", "create"},
		Before: map[string]interface{}{
			"ami":           "ami-old",
			"instance_type": "t2.micro",
		},
		After: map[string]interface{}{
			"ami":           "ami-new",
			"instance_type": "t3.small",
		},
		BeforeSensitive: map[string]interface{}{"password": true},
		AfterSensitive:  map[string]interface{}{"api_key": true},
	}

	sdkChange := toSDKResourceChange(planChange)

	// Verify all fields are copied correctly
	if sdkChange.Address != "module.app.aws_instance.main" {
		t.Errorf("Address mismatch: %q", sdkChange.Address)
	}
	if sdkChange.Type != "aws_instance" {
		t.Errorf("Type mismatch: %q", sdkChange.Type)
	}
	if sdkChange.ProviderName != "registry.terraform.io/hashicorp/aws" {
		t.Errorf("ProviderName mismatch: %q", sdkChange.ProviderName)
	}
	if sdkChange.Mode != "managed" {
		t.Errorf("Mode mismatch: %q", sdkChange.Mode)
	}
	if len(sdkChange.Actions) != 2 {
		t.Errorf("Actions length mismatch: %d", len(sdkChange.Actions))
	}
	if sdkChange.Before["ami"] != "ami-old" {
		t.Errorf("Before mismatch: %v", sdkChange.Before)
	}
	if sdkChange.After["ami"] != "ami-new" {
		t.Errorf("After mismatch: %v", sdkChange.After)
	}
	if sdkChange.BeforeSensitive == nil {
		t.Error("BeforeSensitive should not be nil")
	}
	if sdkChange.AfterSensitive == nil {
		t.Error("AfterSensitive should not be nil")
	}
}

func TestHost_RunAnalysis_EmptyTimeout(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{
			PluginTimeout: "", // empty timeout should use default
		},
	}

	host := NewHost(cfg)
	host.plugins = make(map[string]*DiscoveredPlugin)

	changes := []plan.ResourceChange{}

	decisions, err := host.RunAnalysis(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decisions == nil {
		t.Error("expected non-nil decisions slice")
	}
}

func TestSDKVersionConstraints(t *testing.T) {
	// Test that the SDK version constraint is properly set
	if SDKVersionConstraints != ">= 0.4.0" {
		t.Errorf("unexpected SDK version constraint: %s", SDKVersionConstraints)
	}
}
