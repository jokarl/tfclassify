package config

import (
	"strings"
	"testing"
)

func TestValidate_PrecedenceMismatch(t *testing.T) {
	_, err := LoadFile("testdata/precedence_mismatch.hcl")
	if err == nil {
		t.Fatal("expected error for precedence mismatch, got nil")
	}

	if !strings.Contains(err.Error(), "precedence references undefined classification") {
		t.Errorf("expected error about undefined classification, got: %v", err)
	}

	if !strings.Contains(err.Error(), "critical") {
		t.Errorf("expected error to mention 'critical', got: %v", err)
	}
}

func TestValidate_UnclassifiedMismatch(t *testing.T) {
	_, err := LoadFile("testdata/unclassified_mismatch.hcl")
	if err == nil {
		t.Fatal("expected error for unclassified mismatch, got nil")
	}

	if !strings.Contains(err.Error(), "defaults.unclassified references undefined classification") {
		t.Errorf("expected error about undefined classification, got: %v", err)
	}
}

func TestValidate_EmptyPrecedence(t *testing.T) {
	_, err := LoadFile("testdata/empty_precedence.hcl")
	if err == nil {
		t.Fatal("expected error for empty precedence, got nil")
	}

	if !strings.Contains(err.Error(), "precedence must not be empty") {
		t.Errorf("expected error about empty precedence, got: %v", err)
	}
}

func TestValidate_DuplicateClassification(t *testing.T) {
	_, err := LoadFile("testdata/duplicate_classification.hcl")
	if err == nil {
		t.Fatal("expected error for duplicate classification, got nil")
	}

	if !strings.Contains(err.Error(), "duplicate classification") {
		t.Errorf("expected error about duplicate classification, got: %v", err)
	}
}

func TestValidate_RuleRequiresPattern(t *testing.T) {
	_, err := LoadFile("testdata/rule_missing_pattern.hcl")
	if err == nil {
		t.Fatal("expected error for rule missing pattern, got nil")
	}

	if !strings.Contains(err.Error(), "rule must specify resource or not_resource") {
		t.Errorf("expected error about missing pattern, got: %v", err)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "standard",
				Description: "Standard",
				Rules: []RuleConfig{
					{Resource: []string{"*"}},
				},
			},
		},
		Precedence: []string{"standard"},
		Defaults: &DefaultsConfig{
			Unclassified: "standard",
			NoChanges:    "standard",
		},
	}

	err := Validate(cfg)
	if err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}
