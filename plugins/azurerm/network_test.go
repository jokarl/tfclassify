package main

import (
	"errors"
	"testing"

	"github.com/jokarl/tfclassify/sdk"
)

func TestNetworkExposure_WildcardSource(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_network_security_rule.test",
				Type:    "azurerm_network_security_rule",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name":                  "allow-all-inbound",
					"direction":             "Inbound",
					"access":                "Allow",
					"source_address_prefix": "*",
					"protocol":              "Tcp",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
	}

	decision := runner.decisions[0]
	if decision.Severity != 85 {
		t.Errorf("expected severity 85, got %d", decision.Severity)
	}

	if decision.Metadata["source"] != "*" {
		t.Errorf("expected source *, got %v", decision.Metadata["source"])
	}
}

func TestNetworkExposure_CIDRZero(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_network_security_rule.test",
				Type:    "azurerm_network_security_rule",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name":                  "allow-all-inbound",
					"direction":             "Inbound",
					"access":                "Allow",
					"source_address_prefix": "0.0.0.0/0",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for 0.0.0.0/0, got %d", len(runner.decisions))
	}
}

func TestNetworkExposure_CIDRSource(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_network_security_rule.test",
				Type:    "azurerm_network_security_rule",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name":                  "allow-private",
					"direction":             "Inbound",
					"access":                "Allow",
					"source_address_prefix": "10.0.0.0/8", // Private CIDR - not permissive
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for specific CIDR, got %d", len(runner.decisions))
	}
}

func TestNetworkExposure_OutboundIgnored(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_network_security_rule.test",
				Type:    "azurerm_network_security_rule",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name":                  "allow-all-outbound",
					"direction":             "Outbound", // Outbound should be ignored
					"access":                "Allow",
					"source_address_prefix": "*",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for outbound rules, got %d", len(runner.decisions))
	}
}

func TestNetworkExposure_DenyIgnored(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_network_security_rule.test",
				Type:    "azurerm_network_security_rule",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name":                  "deny-all-inbound",
					"direction":             "Inbound",
					"access":                "Deny", // Deny rules should be ignored
					"source_address_prefix": "*",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for deny rules, got %d", len(runner.decisions))
	}
}

func TestNetworkExposure_Deletion(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_network_security_rule.test",
				Type:    "azurerm_network_security_rule",
				Actions: []string{"delete"},
				Before: map[string]interface{}{
					"name":                  "allow-all-inbound",
					"direction":             "Inbound",
					"access":                "Allow",
					"source_address_prefix": "*",
				},
				After: nil, // Being deleted
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Deletions are fine - we only care about what's being created/updated
	if len(runner.decisions) != 0 {
		t.Errorf("expected 0 decisions for deletion, got %d", len(runner.decisions))
	}
}

func TestNetworkExposure_Internet(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_network_security_rule.test",
				Type:    "azurerm_network_security_rule",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name":                  "allow-internet",
					"direction":             "Inbound",
					"access":                "Allow",
					"source_address_prefix": "Internet",
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for Internet source, got %d", len(runner.decisions))
	}
}

func TestNetworkExposure_GetResourceChangesError(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)
	runner := &mockRunner{err: errors.New("test error")}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNetworkExposure_EmitDecisionError(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)
	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_network_security_rule.test",
				Type:    "azurerm_network_security_rule",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name":                  "allow-all-inbound",
					"direction":             "Inbound",
					"access":                "Allow",
					"source_address_prefix": "*",
				},
			},
		},
		emitErr: errors.New("emit error"),
	}

	err := analyzer.Analyze(runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNetworkExposure_Name(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)

	if analyzer.Name() != "network-exposure" {
		t.Errorf("expected name 'network-exposure', got %q", analyzer.Name())
	}
}

func TestNetworkExposure_SourceAddressPrefixes(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewNetworkExposureAnalyzer(config)

	runner := &mockRunner{
		changes: []*sdk.ResourceChange{
			{
				Address: "azurerm_network_security_rule.test",
				Type:    "azurerm_network_security_rule",
				Actions: []string{"create"},
				After: map[string]interface{}{
					"name":                    "allow-all-inbound",
					"direction":               "Inbound",
					"access":                  "Allow",
					"source_address_prefixes": []interface{}{"10.0.0.0/8", "*"},
				},
			},
		},
	}

	err := analyzer.Analyze(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.decisions) != 1 {
		t.Fatalf("expected 1 decision for wildcard in prefixes array, got %d", len(runner.decisions))
	}
}
