package plugin

import (
	"testing"

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
