package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/internal/classify"
)

func TestFormatJSON(t *testing.T) {
	result := &classify.Result{
		Overall:         "critical",
		OverallExitCode: 2,
		NoChanges:       false,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "azurerm_role_assignment.example",
				ResourceType:   "azurerm_role_assignment",
				Actions:        []string{"delete"},
				Classification: "critical",
				MatchedRules:   []string{"critical rule 1 (resource: *_role_*)"},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatJSON, false)
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse output
	var output JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if output.Overall != "critical" {
		t.Errorf("expected overall 'critical', got '%s'", output.Overall)
	}

	if output.ExitCode != 2 {
		t.Errorf("expected exit_code 2, got %d", output.ExitCode)
	}

	if len(output.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(output.Resources))
	}

	if output.Resources[0].Address != "azurerm_role_assignment.example" {
		t.Errorf("unexpected resource address: %s", output.Resources[0].Address)
	}

	if output.Resources[0].Classification != "critical" {
		t.Errorf("expected resource classification 'critical', got '%s'",
			output.Resources[0].Classification)
	}
}

func TestFormatText(t *testing.T) {
	result := &classify.Result{
		Overall:         "standard",
		OverallExitCode: 1,
		NoChanges:       false,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "azurerm_virtual_network.main",
				ResourceType:   "azurerm_virtual_network",
				Actions:        []string{"update"},
				Classification: "standard",
				MatchedRules:   []string{"standard rule 1 (resource: *)"},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatText, false)
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Classification: standard") {
		t.Errorf("expected output to contain 'Classification: standard', got:\n%s", output)
	}

	if !strings.Contains(output, "Exit code: 1") {
		t.Errorf("expected output to contain 'Exit code: 1', got:\n%s", output)
	}

	if !strings.Contains(output, "azurerm_virtual_network.main") {
		t.Errorf("expected output to contain resource address, got:\n%s", output)
	}
}

func TestFormatText_NoChanges(t *testing.T) {
	result := &classify.Result{
		Overall:           "auto",
		OverallExitCode:   0,
		NoChanges:         true,
		ResourceDecisions: []classify.ResourceDecision{},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatText, false)
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "No resource changes") {
		t.Errorf("expected output to indicate no changes, got:\n%s", output)
	}
}

func TestFormatText_NoChangesWithDowngradedVerbose(t *testing.T) {
	result := &classify.Result{
		Overall:         "auto",
		OverallExitCode: 0,
		NoChanges:       true,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:           "azurerm_monitor_diagnostic_setting.this",
				ResourceType:      "azurerm_monitor_diagnostic_setting",
				Actions:           []string{"no-op"},
				Classification:    "minor",
				OriginalActions:   []string{"update"},
				IgnoredAttributes: []string{"log_analytics_destination_type"},
			},
			{
				Address:        "azurerm_resource_group.rg",
				ResourceType:   "azurerm_resource_group",
				Actions:        []string{"no-op"},
				Classification: "minor",
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatText, true) // verbose
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "No resource changes") {
		t.Errorf("expected 'No resource changes', got:\n%s", output)
	}
	if !strings.Contains(output, "Downgraded to no-op by ignore_attributes") {
		t.Errorf("expected downgraded section, got:\n%s", output)
	}
	if !strings.Contains(output, "azurerm_monitor_diagnostic_setting.this") {
		t.Errorf("expected downgraded resource address, got:\n%s", output)
	}
	if !strings.Contains(output, "log_analytics_destination_type") {
		t.Errorf("expected ignored attribute name, got:\n%s", output)
	}
	// Non-downgraded resource should NOT appear
	if strings.Contains(output, "azurerm_resource_group.rg") {
		t.Errorf("non-downgraded resource should not appear in downgraded section, got:\n%s", output)
	}
}

func TestFormatText_NoChangesWithDowngradedNonVerbose(t *testing.T) {
	result := &classify.Result{
		Overall:         "auto",
		OverallExitCode: 0,
		NoChanges:       true,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:           "azurerm_monitor_diagnostic_setting.this",
				ResourceType:      "azurerm_monitor_diagnostic_setting",
				Actions:           []string{"no-op"},
				OriginalActions:   []string{"update"},
				IgnoredAttributes: []string{"log_analytics_destination_type"},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatText, false) // non-verbose
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Non-verbose should NOT show downgraded section
	if strings.Contains(output, "Downgraded") {
		t.Errorf("non-verbose should not show downgraded section, got:\n%s", output)
	}
}

// Mixed real + no-op resources: the "hidden" line must break the no-op count
// down by classification so the reader can see what ignore_attributes filtered
// out. Without this, an output showing `Classification: minor` next to an opaque
// "(95 no-op resources hidden)" forces the reader to guess what matched.
func TestFormatText_MixedNoOpBreakdownVerbose(t *testing.T) {
	result := &classify.Result{
		Overall:         "minor",
		OverallExitCode: 0,
		NoChanges:       false,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "data.azapi_resource_action.keys[0]",
				ResourceType:   "azapi_resource_action",
				Actions:        []string{"read"},
				Classification: "minor",
				MatchedRules:   []string{"Data source reads"},
			},
			{
				Address:           "azurerm_key_vault_key.cmk",
				ResourceType:      "azurerm_key_vault_key",
				Actions:           []string{"no-op"},
				OriginalActions:   []string{"update"},
				IgnoredAttributes: []string{"tags.tf-module-l2"},
				Classification:    "major",
			},
			{
				Address:           "azurerm_resource_group.rg",
				ResourceType:      "azurerm_resource_group",
				Actions:           []string{"no-op"},
				OriginalActions:   []string{"update"},
				IgnoredAttributes: []string{"tags.tf-module-l2"},
				Classification:    "minor",
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatText, true)
	if err := formatter.Format(result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "(2 no-op resources hidden — minor: 1, major: 1)") {
		t.Errorf("expected breakdown of hidden no-op resources by classification, got:\n%s", output)
	}
}

func TestFormatGitHub(t *testing.T) {
	result := &classify.Result{
		Overall:         "critical",
		OverallExitCode: 2,
		NoChanges:       false,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "test.resource",
				Classification: "critical",
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatGitHub, false)
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check for GITHUB_OUTPUT file format
	if !strings.Contains(output, "classification=critical") {
		t.Errorf("expected GITHUB_OUTPUT format for classification, got:\n%s", output)
	}

	if !strings.Contains(output, "exit_code=2") {
		t.Errorf("expected GITHUB_OUTPUT format for exit_code, got:\n%s", output)
	}

	// Legacy ::set-output should not be present
	if strings.Contains(output, "::set-output") {
		t.Errorf("unexpected legacy ::set-output syntax in output:\n%s", output)
	}
}

func TestFormatText_Verbose(t *testing.T) {
	result := &classify.Result{
		Overall:         "standard",
		OverallExitCode: 1,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "azurerm_virtual_network.main",
				ResourceType:   "azurerm_virtual_network",
				Actions:        []string{"update"},
				Classification: "standard",
				MatchedRules:   []string{"standard rule 1 (resource: *)"},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatText, true) // verbose=true
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verbose output should include the rule
	if !strings.Contains(output, "Rule:") {
		t.Errorf("expected verbose output to contain rule info, got:\n%s", output)
	}
}

func TestFormatJSON_WithDescriptions(t *testing.T) {
	result := &classify.Result{
		Overall:            "critical",
		OverallDescription: "Requires security team approval",
		OverallExitCode:    2,
		NoChanges:          false,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:                   "azurerm_role_assignment.example",
				ResourceType:              "azurerm_role_assignment",
				Actions:                   []string{"delete"},
				Classification:            "critical",
				ClassificationDescription: "IAM role change requiring security approval",
				MatchedRules:              []string{"critical rule 1 (resource: *_role_*)"},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatJSON, false)
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse output
	var output JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Check overall_description
	if output.OverallDescription != "Requires security team approval" {
		t.Errorf("expected overall_description 'Requires security team approval', got '%s'", output.OverallDescription)
	}

	// Check classification_description per resource
	if len(output.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(output.Resources))
	}

	if output.Resources[0].ClassificationDescription != "IAM role change requiring security approval" {
		t.Errorf("expected resource classification_description 'IAM role change requiring security approval', got '%s'",
			output.Resources[0].ClassificationDescription)
	}
}

func TestFormatJSON_OmitsEmptyDescriptions(t *testing.T) {
	result := &classify.Result{
		Overall:         "standard",
		OverallExitCode: 1,
		NoChanges:       false,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "test.resource",
				Classification: "standard",
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatJSON, false)
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that empty descriptions are omitted (using omitempty)
	output := buf.String()
	if strings.Contains(output, "overall_description") {
		t.Errorf("expected overall_description to be omitted when empty, got:\n%s", output)
	}
	if strings.Contains(output, "classification_description") {
		t.Errorf("expected classification_description to be omitted when empty, got:\n%s", output)
	}
}

func TestFormatGitHub_WithDescription(t *testing.T) {
	result := &classify.Result{
		Overall:            "critical",
		OverallDescription: "Security approval required",
		OverallExitCode:    2,
		NoChanges:          false,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "test.resource",
				Classification: "critical",
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatGitHub, false)
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check for GITHUB_OUTPUT file format
	if !strings.Contains(output, "classification_description=Security approval required") {
		t.Errorf("expected GITHUB_OUTPUT format for classification_description, got:\n%s", output)
	}
}

func TestFormatGitHub_OmitsEmptyDescription(t *testing.T) {
	result := &classify.Result{
		Overall:         "standard",
		OverallExitCode: 1,
		NoChanges:       false,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "test.resource",
				Classification: "standard",
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatGitHub, false)
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check that classification_description is NOT present when empty
	if strings.Contains(output, "classification_description") {
		t.Errorf("expected classification_description to be omitted when empty, got:\n%s", output)
	}
}

func TestFormatText_VerboseWithDescriptions(t *testing.T) {
	result := &classify.Result{
		Overall:            "critical",
		OverallDescription: "Requires security team approval",
		OverallExitCode:    2,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:                   "azurerm_role_assignment.admin",
				ResourceType:              "azurerm_role_assignment",
				Actions:                   []string{"delete"},
				Classification:            "critical",
				ClassificationDescription: "IAM changes require security approval",
				MatchedRules:              []string{"critical rule 1 (resource: *_role_*)"},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatText, true) // verbose=true
	err := formatter.Format(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check that overall description appears under the classification header
	if !strings.Contains(output, "Requires security team approval") {
		t.Errorf("expected verbose output to contain overall description, got:\n%s", output)
	}

	// Check that classification description appears under the group header
	if !strings.Contains(output, "IAM changes require security approval") {
		t.Errorf("expected verbose output to contain classification description under group header, got:\n%s", output)
	}
}
