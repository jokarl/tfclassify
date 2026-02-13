package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/pkg/classify"
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
				MatchedRule:    "critical rule 1 (resource: *_role_*)",
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
				MatchedRule:    "standard rule 1 (resource: *)",
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

	// Check for ::set-output syntax
	if !strings.Contains(output, "::set-output name=classification::critical") {
		t.Errorf("expected GitHub Actions set-output for classification, got:\n%s", output)
	}

	if !strings.Contains(output, "::set-output name=exit_code::2") {
		t.Errorf("expected GitHub Actions set-output for exit_code, got:\n%s", output)
	}

	// Check for GITHUB_OUTPUT file format
	if !strings.Contains(output, "classification=critical") {
		t.Errorf("expected GITHUB_OUTPUT format for classification, got:\n%s", output)
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
				MatchedRule:    "standard rule 1 (resource: *)",
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
