package tfclassify_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/internal/classify"
	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/output"
	"github.com/jokarl/tfclassify/internal/plan"
)

// TestEndToEnd_FullPipeline tests the complete config → parse → classify → output pipeline.
func TestEndToEnd_FullPipeline(t *testing.T) {
	configHCL := `
classification "critical" {
  description = "Requires security team approval"
  rule {
    resource = ["*_role_*", "*_iam_*"]
    actions  = ["delete"]
  }
}

classification "standard" {
  description = "Standard change process"
  rule {
    resource = ["*"]
  }
}

classification "auto" {
  description = "Automatic approval"
  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

precedence = ["critical", "standard", "auto"]

defaults {
  unclassified = "standard"
  no_changes   = "auto"
}
`

	planJSON := `{
  "format_version": "1.2",
  "terraform_version": "1.5.0",
  "resource_changes": [
    {
      "address": "azurerm_role_assignment.admin",
      "mode": "managed",
      "type": "azurerm_role_assignment",
      "name": "admin",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["delete"],
        "before": {"role_definition_name": "Owner"},
        "after": null,
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_virtual_network.main",
      "mode": "managed",
      "type": "azurerm_virtual_network",
      "name": "main",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["update"],
        "before": {"name": "vnet-old"},
        "after": {"name": "vnet-new"},
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_resource_group.rg",
      "mode": "managed",
      "type": "azurerm_resource_group",
      "name": "rg",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["create"],
        "before": null,
        "after": {"name": "rg-test", "location": "eastus"},
        "before_sensitive": false,
        "after_sensitive": false
      }
    }
  ]
}`

	// Step 1: Parse config
	cfg, err := config.Parse([]byte(configHCL), "test.hcl")
	if err != nil {
		t.Fatalf("config.Parse failed: %v", err)
	}

	// Step 2: Parse plan
	planResult, err := plan.Parse(strings.NewReader(planJSON))
	if err != nil {
		t.Fatalf("plan.Parse failed: %v", err)
	}

	if len(planResult.Changes) != 3 {
		t.Fatalf("expected 3 resource changes, got %d", len(planResult.Changes))
	}

	// Step 3: Classify
	classifier, err := classify.New(cfg)
	if err != nil {
		t.Fatalf("classify.New failed: %v", err)
	}

	result := classifier.Classify(planResult.Changes)

	// Step 4: Verify classification results
	if result.Overall != "critical" {
		t.Errorf("expected overall 'critical', got %q", result.Overall)
	}
	if result.OverallDescription != "Requires security team approval" {
		t.Errorf("expected overall description, got %q", result.OverallDescription)
	}
	if result.NoChanges {
		t.Error("expected NoChanges to be false")
	}

	// Verify individual decisions
	if len(result.ResourceDecisions) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(result.ResourceDecisions))
	}

	decisions := make(map[string]classify.ResourceDecision)
	for _, d := range result.ResourceDecisions {
		decisions[d.Address] = d
	}

	// Role deletion should be critical
	roleDecision := decisions["azurerm_role_assignment.admin"]
	if roleDecision.Classification != "critical" {
		t.Errorf("expected role assignment to be 'critical', got %q", roleDecision.Classification)
	}

	// VNet update should be standard
	vnetDecision := decisions["azurerm_virtual_network.main"]
	if vnetDecision.Classification != "standard" {
		t.Errorf("expected virtual network to be 'standard', got %q", vnetDecision.Classification)
	}

	// Resource group create should be standard
	rgDecision := decisions["azurerm_resource_group.rg"]
	if rgDecision.Classification != "standard" {
		t.Errorf("expected resource group to be 'standard', got %q", rgDecision.Classification)
	}

	// Step 5: Verify output formats
	// JSON output
	var jsonBuf bytes.Buffer
	jsonFormatter := output.NewFormatter(&jsonBuf, output.FormatJSON, false)
	if err := jsonFormatter.Format(result); err != nil {
		t.Fatalf("JSON formatter failed: %v", err)
	}

	var jsonOutput map[string]interface{}
	if err := json.Unmarshal(jsonBuf.Bytes(), &jsonOutput); err != nil {
		t.Fatalf("JSON output is invalid: %v\nOutput: %s", err, jsonBuf.String())
	}
	if jsonOutput["overall"] != "critical" {
		t.Errorf("JSON overall should be 'critical', got %v", jsonOutput["overall"])
	}

	// Text output
	var textBuf bytes.Buffer
	textFormatter := output.NewFormatter(&textBuf, output.FormatText, false)
	if err := textFormatter.Format(result); err != nil {
		t.Fatalf("text formatter failed: %v", err)
	}
	textOutput := textBuf.String()
	if !strings.Contains(textOutput, "Classification: critical") {
		t.Errorf("text output should contain 'Classification: critical', got:\n%s", textOutput)
	}

	// GitHub output
	var ghBuf bytes.Buffer
	ghFormatter := output.NewFormatter(&ghBuf, output.FormatGitHub, false)
	if err := ghFormatter.Format(result); err != nil {
		t.Fatalf("github formatter failed: %v", err)
	}
	ghOutput := ghBuf.String()
	if !strings.Contains(ghOutput, "classification=critical") {
		t.Errorf("github output should contain 'classification=critical', got:\n%s", ghOutput)
	}
}

// TestEndToEnd_NoChanges tests the pipeline with an empty plan.
func TestEndToEnd_NoChanges(t *testing.T) {
	configHCL := `
classification "standard" {
  description = "Standard"
  rule {
    resource = ["*"]
  }
}

precedence = ["standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
`

	planJSON := `{
  "format_version": "1.2",
  "terraform_version": "1.5.0",
  "resource_changes": []
}`

	cfg, err := config.Parse([]byte(configHCL), "test.hcl")
	if err != nil {
		t.Fatalf("config.Parse failed: %v", err)
	}

	planResult, err := plan.Parse(strings.NewReader(planJSON))
	if err != nil {
		t.Fatalf("plan.Parse failed: %v", err)
	}

	classifier, err := classify.New(cfg)
	if err != nil {
		t.Fatalf("classify.New failed: %v", err)
	}

	result := classifier.Classify(planResult.Changes)

	if !result.NoChanges {
		t.Error("expected NoChanges to be true")
	}
	if result.OverallExitCode != 0 {
		t.Errorf("expected exit code 0 for no changes, got %d", result.OverallExitCode)
	}
}

// TestEndToEnd_PluginDecisionsMerge tests that plugin decisions are properly merged with core decisions.
func TestEndToEnd_PluginDecisionsMerge(t *testing.T) {
	configHCL := `
classification "critical" {
  description = "Critical"
  rule {
    resource = ["*_role_*"]
  }
}

classification "standard" {
  description = "Standard"
  rule {
    resource = ["*"]
  }
}

precedence = ["critical", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
`

	planJSON := `{
  "format_version": "1.2",
  "terraform_version": "1.5.0",
  "resource_changes": [
    {
      "address": "azurerm_virtual_network.main",
      "mode": "managed",
      "type": "azurerm_virtual_network",
      "name": "main",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["update"],
        "before": {"name": "vnet-old"},
        "after": {"name": "vnet-new"},
        "before_sensitive": false,
        "after_sensitive": false
      }
    }
  ]
}`

	cfg, err := config.Parse([]byte(configHCL), "test.hcl")
	if err != nil {
		t.Fatalf("config.Parse failed: %v", err)
	}

	planResult, err := plan.Parse(strings.NewReader(planJSON))
	if err != nil {
		t.Fatalf("plan.Parse failed: %v", err)
	}

	classifier, err := classify.New(cfg)
	if err != nil {
		t.Fatalf("classify.New failed: %v", err)
	}

	result := classifier.Classify(planResult.Changes)

	// Initially should be "standard"
	if result.Overall != "standard" {
		t.Errorf("expected initial overall 'standard', got %q", result.Overall)
	}

	// Simulate plugin upgrading the classification
	pluginDecisions := []classify.ResourceDecision{
		{
			Address:        "azurerm_virtual_network.main",
			ResourceType:   "azurerm_virtual_network",
			Actions:        []string{"update"},
			Classification: "critical",
			MatchedRules:   []string{"plugin: sensitive network change"},
		},
	}

	classifier.AddPluginDecisions(result, pluginDecisions)

	// Should be upgraded to "critical"
	if result.Overall != "critical" {
		t.Errorf("expected overall 'critical' after plugin decision, got %q", result.Overall)
	}
	if result.ResourceDecisions[0].Classification != "critical" {
		t.Errorf("expected resource classification 'critical', got %q",
			result.ResourceDecisions[0].Classification)
	}
}

// TestEndToEnd_NotResourceRule tests that not_resource rules work correctly in the full pipeline.
func TestEndToEnd_NotResourceRule(t *testing.T) {
	configHCL := `
classification "critical" {
  description = "Critical changes"
  rule {
    resource = ["*_role_*"]
  }
}

classification "standard" {
  description = "Standard changes"
  rule {
    not_resource = ["*_role_*"]
  }
}

precedence = ["critical", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
`

	planJSON := `{
  "format_version": "1.2",
  "terraform_version": "1.5.0",
  "resource_changes": [
    {
      "address": "azurerm_virtual_network.main",
      "mode": "managed",
      "type": "azurerm_virtual_network",
      "name": "main",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["create"],
        "before": null,
        "after": {"name": "vnet"},
        "before_sensitive": false,
        "after_sensitive": false
      }
    }
  ]
}`

	cfg, err := config.Parse([]byte(configHCL), "test.hcl")
	if err != nil {
		t.Fatalf("config.Parse failed: %v", err)
	}

	planResult, err := plan.Parse(strings.NewReader(planJSON))
	if err != nil {
		t.Fatalf("plan.Parse failed: %v", err)
	}

	classifier, err := classify.New(cfg)
	if err != nil {
		t.Fatalf("classify.New failed: %v", err)
	}

	result := classifier.Classify(planResult.Changes)

	// VNet should match "standard" via not_resource (it's not a role resource)
	if result.ResourceDecisions[0].Classification != "standard" {
		t.Errorf("expected 'standard' for non-role resource, got %q",
			result.ResourceDecisions[0].Classification)
	}
}
