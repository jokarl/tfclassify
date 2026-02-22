package plugin

import (
	"context"
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
	"github.com/jokarl/tfclassify/sdk"
)

func TestRunPluginAnalysis_ClientNotFound(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)

	err := host.runPluginAnalysis(context.Background(), "missing", "", nil)
	if err == nil {
		t.Fatal("expected error when plugin client not found")
	}
	if err.Error() != "plugin client not found: missing" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHost_RunAnalysis_PluginNotStarted(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{
			PluginTimeout: "1s",
		},
	}
	host := NewHost(cfg)

	// Add a plugin entry but don't register a client for it
	host.plugins = map[string]*DiscoveredPlugin{
		"ghost": {Name: "ghost", Path: "/nonexistent"},
	}

	changes := []plan.ResourceChange{
		{Address: "test.resource", Type: "test_type", Actions: []string{"create"}},
	}

	// RunAnalysis should not error — it logs warnings but continues
	decisions, err := host.RunAnalysis(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 0 decisions since the plugin failed
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions, got %d", len(decisions))
	}
}

func TestRunner_ConcurrentAccess(t *testing.T) {
	cfg := &config.Config{
		Defaults: &config.DefaultsConfig{},
	}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{Address: "test.resource", Type: "test_type", Actions: []string{"create"}},
	}

	runner := NewRunner(host)

	// Run concurrent GetResourceChanges and EmitDecision calls
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = runner.GetResourceChanges([]string{"*"})
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			analyzer := &mockAnalyzer{name: "concurrent"}
			change := &sdk.ResourceChange{Address: "test.resource", Type: "test_type"}
			decision := &sdk.Decision{Classification: "standard", Reason: "concurrent test"}
			_ = runner.EmitDecision(analyzer, change, decision)
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should have 10 decisions from concurrent emits
	if len(host.decisions) != 10 {
		t.Errorf("expected 10 decisions from concurrent access, got %d", len(host.decisions))
	}
}
