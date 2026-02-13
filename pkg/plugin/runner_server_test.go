package plugin

import (
	"context"
	"testing"

	"github.com/jokarl/tfclassify/pkg/config"
	"github.com/jokarl/tfclassify/pkg/plan"
	"github.com/jokarl/tfclassify/sdk"
	sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
)

func TestRunnerServiceServer_GetResourceChanges(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{
			Address: "azurerm_role_assignment.admin",
			Type:    "azurerm_role_assignment",
			Actions: []string{"create"},
			After:   map[string]interface{}{"role_definition_name": "Owner"},
		},
		{
			Address: "azurerm_virtual_network.main",
			Type:    "azurerm_virtual_network",
			Actions: []string{"create"},
		},
	}

	runner := NewRunner(host)
	server := NewRunnerServiceServer(runner)

	tests := []struct {
		name     string
		patterns []string
		want     int
	}{
		{"all patterns", []string{"*"}, 2},
		{"role pattern", []string{"*_role_*"}, 1},
		{"vnet pattern", []string{"*_virtual_network"}, 1},
		{"no match", []string{"*_subnet_*"}, 0},
		{"multiple patterns", []string{"*_role_*", "*_virtual_network"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &sdkplugin.GetResourceChangesRequest{Patterns: tt.patterns}
			resp, err := server.GetResourceChanges(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Changes) != tt.want {
				t.Errorf("expected %d changes, got %d", tt.want, len(resp.Changes))
			}
		})
	}
}

func TestRunnerServiceServer_GetResourceChange(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)
	host.changes = []plan.ResourceChange{
		{
			Address: "azurerm_role_assignment.admin",
			Type:    "azurerm_role_assignment",
			Actions: []string{"create"},
		},
	}

	runner := NewRunner(host)
	server := NewRunnerServiceServer(runner)

	// Test found
	req := &sdkplugin.GetResourceChangeRequest{Address: "azurerm_role_assignment.admin"}
	resp, err := server.GetResourceChange(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Change == nil {
		t.Fatal("expected change, got nil")
	}
	if resp.Change.Address != "azurerm_role_assignment.admin" {
		t.Errorf("expected address 'azurerm_role_assignment.admin', got %q", resp.Change.Address)
	}

	// Test not found
	req = &sdkplugin.GetResourceChangeRequest{Address: "nonexistent.resource"}
	_, err = server.GetResourceChange(context.Background(), req)
	if err == nil {
		t.Error("expected error for nonexistent resource")
	}
}

func TestRunnerServiceServer_EmitDecision(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)
	runner := NewRunner(host)
	server := NewRunnerServiceServer(runner)

	change := &sdkplugin.ResourceChange{
		Address: "test.resource",
		Type:    "test_type",
		Actions: []string{"create"},
	}

	decision := &sdkplugin.Decision{
		Classification: "critical",
		Reason:         "test reason",
		Severity:       90,
	}

	req := &sdkplugin.EmitDecisionRequest{
		AnalyzerName: "test-analyzer",
		Change:       change,
		Decision:     decision,
	}

	resp, err := server.EmitDecision(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Error("expected non-nil response")
	}

	// Verify decision was recorded
	if len(host.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(host.decisions))
	}

	d := host.decisions[0]
	if d.Address != "test.resource" {
		t.Errorf("expected address 'test.resource', got %q", d.Address)
	}
	if d.Classification != "critical" {
		t.Errorf("expected classification 'critical', got %q", d.Classification)
	}
}

func TestProtoConversions_ResourceChange(t *testing.T) {
	// Test nil
	if sdkToProtoResourceChange(nil) != nil {
		t.Error("expected nil for nil SDK change")
	}
	if protoToSDKResourceChange(nil) != nil {
		t.Error("expected nil for nil proto change")
	}

	// Test conversion
	sdkChange := &sdk.ResourceChange{
		Address:      "test.resource",
		Type:         "test_type",
		ProviderName: "test_provider",
		Mode:         "managed",
		Actions:      []string{"create"},
		Before:       nil,
		After:        map[string]interface{}{"key": "value"},
	}

	proto := sdkToProtoResourceChange(sdkChange)
	if proto.Address != "test.resource" {
		t.Errorf("expected address 'test.resource', got %q", proto.Address)
	}
	if proto.Type != "test_type" {
		t.Errorf("expected type 'test_type', got %q", proto.Type)
	}

	// Round-trip
	converted := protoToSDKResourceChange(proto)
	if converted.Address != sdkChange.Address {
		t.Errorf("round-trip failed for address: expected %q, got %q", sdkChange.Address, converted.Address)
	}
}

func TestProtoConversions_Decision(t *testing.T) {
	// Test nil
	if protoToSDKDecision(nil) != nil {
		t.Error("expected nil for nil proto decision")
	}

	// Test conversion
	protoDecision := &sdkplugin.Decision{
		Classification: "critical",
		Reason:         "test reason",
		Severity:       90,
		Metadata:       []byte(`{"key":"value"}`),
	}

	sdk := protoToSDKDecision(protoDecision)
	if sdk.Classification != "critical" {
		t.Errorf("expected classification 'critical', got %q", sdk.Classification)
	}
	if sdk.Severity != 90 {
		t.Errorf("expected severity 90, got %d", sdk.Severity)
	}
	if sdk.Metadata == nil {
		t.Error("expected non-nil metadata")
	}
}

func TestPluginAnalyzerWrapper(t *testing.T) {
	wrapper := &pluginAnalyzerWrapper{name: "test-analyzer"}

	if wrapper.Name() != "test-analyzer" {
		t.Errorf("expected name 'test-analyzer', got %q", wrapper.Name())
	}
	if !wrapper.Enabled() {
		t.Error("expected Enabled() to return true")
	}
	if wrapper.ResourcePatterns() != nil {
		t.Error("expected ResourcePatterns() to return nil")
	}
	if err := wrapper.Analyze(nil); err != nil {
		t.Errorf("unexpected error from Analyze: %v", err)
	}
}

func TestEmitDecisionWithMetadata(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)
	runner := NewRunner(host)

	change := &sdk.ResourceChange{
		Address: "test.resource",
		Type:    "test_type",
		Actions: []string{"create"},
	}

	// Test metadata-only decision (empty classification)
	decision := &sdk.Decision{
		Classification: "",
		Reason:         "metadata reason",
		Severity:       70,
	}

	err := runner.EmitDecisionWithMetadata("test-analyzer", change, decision)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(host.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(host.decisions))
	}

	d := host.decisions[0]
	if d.Classification != "" {
		t.Errorf("expected empty classification, got %q", d.Classification)
	}
	if d.MatchedRule == "" {
		t.Error("expected non-empty MatchedRule")
	}

	// Test with classification
	host.decisions = nil
	decision.Classification = "critical"
	err = runner.EmitDecisionWithMetadata("test-analyzer", change, decision)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d = host.decisions[0]
	if d.Classification != "critical" {
		t.Errorf("expected classification 'critical', got %q", d.Classification)
	}
}

func TestNewRunnerServiceServer(t *testing.T) {
	cfg := &config.Config{}
	host := NewHost(cfg)
	runner := NewRunner(host)

	server := NewRunnerServiceServer(runner)
	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.runner != runner {
		t.Error("expected runner to be set")
	}
}
