package output

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jokarl/tfclassify/internal/classify"
)

func TestFormatSARIF_Basic(t *testing.T) {
	result := &classify.Result{
		Overall:         "critical",
		OverallExitCode: 2,
		NoChanges:       false,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:                   "azurerm_role_assignment.admin",
				ResourceType:              "azurerm_role_assignment",
				Actions:                   []string{"create"},
				Classification:            "critical",
				ClassificationDescription: "Requires security approval",
				MatchedRules:              []string{"IAM role assignment"},
			},
			{
				Address:                   "azurerm_virtual_network.main",
				ResourceType:              "azurerm_virtual_network",
				Actions:                   []string{"update"},
				Classification:            "standard",
				ClassificationDescription: "Standard change process",
				MatchedRules:              []string{"network update"},
			},
		},
	}

	levels := map[string]string{
		"critical": "error",
		"standard": "warning",
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatSARIF, false,
		WithVersion("1.0.0"),
		WithSARIFLevels(levels),
	)
	if err := formatter.Format(result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc sarifDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse SARIF output: %v", err)
	}

	// Schema and version
	if doc.Schema != "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json" {
		t.Errorf("unexpected schema: %s", doc.Schema)
	}
	if doc.Version != "2.1.0" {
		t.Errorf("unexpected SARIF version: %s", doc.Version)
	}

	// Runs
	if len(doc.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(doc.Runs))
	}
	run := doc.Runs[0]

	// Tool driver
	if run.Tool.Driver.Name != "tfclassify" {
		t.Errorf("unexpected tool name: %s", run.Tool.Driver.Name)
	}
	if run.Tool.Driver.Version != "1.0.0" {
		t.Errorf("unexpected tool version: %s", run.Tool.Driver.Version)
	}

	// Rules
	if len(run.Tool.Driver.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(run.Tool.Driver.Rules))
	}
	if run.Tool.Driver.Rules[0].ID != "critical" {
		t.Errorf("expected first rule ID 'critical', got %q", run.Tool.Driver.Rules[0].ID)
	}
	if run.Tool.Driver.Rules[0].DefaultConfiguration.Level != "error" {
		t.Errorf("expected first rule level 'error', got %q", run.Tool.Driver.Rules[0].DefaultConfiguration.Level)
	}
	if run.Tool.Driver.Rules[1].ID != "standard" {
		t.Errorf("expected second rule ID 'standard', got %q", run.Tool.Driver.Rules[1].ID)
	}

	// Results
	if len(run.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(run.Results))
	}

	r0 := run.Results[0]
	if r0.RuleID != "critical" {
		t.Errorf("expected ruleId 'critical', got %q", r0.RuleID)
	}
	if r0.RuleIndex != 0 {
		t.Errorf("expected ruleIndex 0, got %d", r0.RuleIndex)
	}
	if r0.Level != "error" {
		t.Errorf("expected level 'error', got %q", r0.Level)
	}
	if r0.Message.Text != "IAM role assignment" {
		t.Errorf("unexpected message: %q", r0.Message.Text)
	}

	// Logical locations
	if len(r0.Locations) != 1 || len(r0.Locations[0].LogicalLocations) != 1 {
		t.Fatalf("expected 1 logical location")
	}
	loc := r0.Locations[0].LogicalLocations[0]
	if loc.Name != "azurerm_role_assignment.admin" {
		t.Errorf("unexpected location name: %q", loc.Name)
	}
	if loc.Kind != "resource" {
		t.Errorf("unexpected location kind: %q", loc.Kind)
	}

	// Second result
	r1 := run.Results[1]
	if r1.RuleID != "standard" {
		t.Errorf("expected ruleId 'standard', got %q", r1.RuleID)
	}
	if r1.RuleIndex != 1 {
		t.Errorf("expected ruleIndex 1, got %d", r1.RuleIndex)
	}
	if r1.Level != "warning" {
		t.Errorf("expected level 'warning', got %q", r1.Level)
	}
}

func TestFormatSARIF_NoChanges(t *testing.T) {
	result := &classify.Result{
		Overall:           "auto",
		OverallExitCode:   0,
		NoChanges:         true,
		ResourceDecisions: []classify.ResourceDecision{},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatSARIF, false, WithVersion("1.0.0"))
	if err := formatter.Format(result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc sarifDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse SARIF output: %v", err)
	}

	if len(doc.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(doc.Runs))
	}

	// Results must be an empty array, not null
	if doc.Runs[0].Results == nil {
		t.Error("expected non-nil results array")
	}
	if len(doc.Runs[0].Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(doc.Runs[0].Results))
	}
}

func TestFormatSARIF_DefaultLevels(t *testing.T) {
	result := &classify.Result{
		Overall:         "critical",
		OverallExitCode: 2,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "test.a",
				Classification: "critical",
			},
			{
				Address:        "test.b",
				Classification: "standard",
			},
		},
	}

	// No custom levels — everything defaults to "warning"
	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatSARIF, false, WithVersion("1.0.0"))
	if err := formatter.Format(result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc sarifDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse SARIF output: %v", err)
	}

	for _, r := range doc.Runs[0].Results {
		if r.Level != "warning" {
			t.Errorf("expected default level 'warning' for %q, got %q", r.RuleID, r.Level)
		}
	}
}

func TestFormatSARIF_CustomLevels(t *testing.T) {
	result := &classify.Result{
		Overall:         "critical",
		OverallExitCode: 2,
		ResourceDecisions: []classify.ResourceDecision{
			{Address: "test.a", Classification: "critical"},
			{Address: "test.b", Classification: "review"},
			{Address: "test.c", Classification: "auto"},
		},
	}

	levels := map[string]string{
		"critical": "error",
		"review":   "warning",
		"auto":     "none",
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatSARIF, false,
		WithVersion("1.0.0"),
		WithSARIFLevels(levels),
	)
	if err := formatter.Format(result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc sarifDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse SARIF output: %v", err)
	}

	expected := map[string]string{
		"critical": "error",
		"review":   "warning",
		"auto":     "none",
	}
	for _, r := range doc.Runs[0].Results {
		if r.Level != expected[r.RuleID] {
			t.Errorf("expected level %q for %q, got %q", expected[r.RuleID], r.RuleID, r.Level)
		}
	}
}

func TestFormatSARIF_Fingerprints(t *testing.T) {
	result := &classify.Result{
		Overall:         "critical",
		OverallExitCode: 2,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "azurerm_role_assignment.admin",
				Classification: "critical",
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatSARIF, false, WithVersion("1.0.0"))
	if err := formatter.Format(result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc sarifDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse SARIF output: %v", err)
	}

	r := doc.Runs[0].Results[0]
	expected := sha256.Sum256([]byte("azurerm_role_assignment.admin/critical"))
	expectedHex := fmt.Sprintf("%x", expected)

	fp, ok := r.PartialFingerprints["primaryLocationFingerprint"]
	if !ok {
		t.Fatal("expected primaryhamburg found in partialFingerprints")
	}
	if fp != expectedHex {
		t.Errorf("expected fingerprint %q, got %q", expectedHex, fp)
	}
}

func TestFormatSARIF_RuleIndex(t *testing.T) {
	result := &classify.Result{
		Overall:         "critical",
		OverallExitCode: 2,
		ResourceDecisions: []classify.ResourceDecision{
			{Address: "a.1", Classification: "critical"},
			{Address: "a.2", Classification: "standard"},
			{Address: "a.3", Classification: "critical"},
			{Address: "a.4", Classification: "auto"},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatSARIF, false, WithVersion("1.0.0"))
	if err := formatter.Format(result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc sarifDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse SARIF output: %v", err)
	}

	run := doc.Runs[0]

	// Build expected rule ID → index
	ruleIdx := make(map[string]int)
	for i, rule := range run.Tool.Driver.Rules {
		ruleIdx[rule.ID] = i
	}

	// Verify each result references the correct rule
	for _, r := range run.Results {
		expectedIdx, ok := ruleIdx[r.RuleID]
		if !ok {
			t.Errorf("result ruleId %q not found in rules", r.RuleID)
			continue
		}
		if r.RuleIndex != expectedIdx {
			t.Errorf("result for %q has ruleIndex %d, expected %d", r.RuleID, r.RuleIndex, expectedIdx)
		}
	}
}

func TestFormatSARIF_MessageFallback(t *testing.T) {
	result := &classify.Result{
		Overall:         "standard",
		OverallExitCode: 1,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "test.resource",
				Classification: "standard",
				MatchedRules:   nil, // no matched rules
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatSARIF, false, WithVersion("1.0.0"))
	if err := formatter.Format(result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc sarifDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse SARIF output: %v", err)
	}

	msg := doc.Runs[0].Results[0].Message.Text
	if msg != "Resource classified as standard" {
		t.Errorf("expected fallback message, got %q", msg)
	}
}

func TestFormatSARIF_MultipleMatchedRulesJoined(t *testing.T) {
	result := &classify.Result{
		Overall:         "critical",
		OverallExitCode: 2,
		ResourceDecisions: []classify.ResourceDecision{
			{
				Address:        "test.resource",
				Classification: "critical",
				MatchedRules:   []string{"rule A", "rule B"},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf, FormatSARIF, false, WithVersion("1.0.0"))
	if err := formatter.Format(result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc sarifDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse SARIF output: %v", err)
	}

	msg := doc.Runs[0].Results[0].Message.Text
	if msg != "rule A; rule B" {
		t.Errorf("expected joined rules, got %q", msg)
	}
}
