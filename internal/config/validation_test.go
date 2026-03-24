package config

import (
	"bytes"
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

func TestWarnRedundantNotResource_FullyRedundant(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules: []RuleConfig{
					{Resource: []string{"*_role_*", "*_iam_*"}},
				},
			},
			{
				Name:        "standard",
				Description: "Standard",
				Rules: []RuleConfig{
					// This not_resource list is fully covered by critical's resource patterns
					{NotResource: []string{"*_role_*", "*_iam_*"}},
				},
			},
		},
		Precedence: []string{"critical", "standard"},
	}

	var buf bytes.Buffer
	WarnRedundantNotResource(cfg, &buf)

	if !strings.Contains(buf.String(), "Warning: classification \"standard\" rule 1") {
		t.Errorf("expected warning for standard rule 1, got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "resource = [\"*\"]") {
		t.Errorf("expected suggestion to use resource = [\"*\"], got: %q", buf.String())
	}
}

func TestWarnRedundantNotResource_PartiallyNew(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules: []RuleConfig{
					{Resource: []string{"*_role_*"}},
				},
			},
			{
				Name:        "standard",
				Description: "Standard",
				Rules: []RuleConfig{
					// This not_resource list has patterns not in critical
					{NotResource: []string{"*_role_*", "*_custom_*"}},
				},
			},
		},
		Precedence: []string{"critical", "standard"},
	}

	var buf bytes.Buffer
	WarnRedundantNotResource(cfg, &buf)

	if buf.Len() != 0 {
		t.Errorf("expected no warning when not_resource has unique patterns, got: %q", buf.String())
	}
}

func TestWarnRedundantNotResource_NoNotResource(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules: []RuleConfig{
					{Resource: []string{"*_role_*"}},
				},
			},
			{
				Name:        "standard",
				Description: "Standard",
				Rules: []RuleConfig{
					{Resource: []string{"*"}},
				},
			},
		},
		Precedence: []string{"critical", "standard"},
	}

	var buf bytes.Buffer
	WarnRedundantNotResource(cfg, &buf)

	if buf.Len() != 0 {
		t.Errorf("expected no warning when no not_resource rules exist, got: %q", buf.String())
	}
}

func TestValidateGlobPatterns_Valid(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name: "critical",
				Rules: []RuleConfig{
					{Resource: []string{"*_role_*", "*"}},
				},
			},
		},
	}

	if err := ValidateGlobPatterns(cfg); err != nil {
		t.Errorf("expected no error for valid patterns, got: %v", err)
	}
}

func TestValidateGlobPatterns_Invalid(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name: "critical",
				Rules: []RuleConfig{
					{Resource: []string{"*_role_[*"}},
				},
			},
		},
	}

	err := ValidateGlobPatterns(cfg)
	if err == nil {
		t.Fatal("expected error for invalid glob pattern, got nil")
	}

	if !strings.Contains(err.Error(), "critical") {
		t.Errorf("expected error to mention classification name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "*_role_[*") {
		t.Errorf("expected error to mention pattern, got: %v", err)
	}
}

func TestValidateGlobPatterns_InvalidNotResource(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name: "standard",
				Rules: []RuleConfig{
					{NotResource: []string{"[bad"}},
				},
			},
		},
	}

	err := ValidateGlobPatterns(cfg)
	if err == nil {
		t.Fatal("expected error for invalid not_resource pattern, got nil")
	}
	if !strings.Contains(err.Error(), "not_resource") {
		t.Errorf("expected error to mention not_resource, got: %v", err)
	}
}

func TestValidateWarnings_UnreachableRule(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name: "critical",
				Rules: []RuleConfig{
					{Resource: []string{"*"}},
				},
			},
			{
				Name: "standard",
				Rules: []RuleConfig{
					{Resource: []string{"*_role_*"}},
				},
			},
		},
		Precedence: []string{"critical", "standard"},
	}

	warnings := ValidateWarnings(cfg)

	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "unreachable") && w.Classification == "standard" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unreachable rule warning for standard, got: %v", warnings)
	}
}

func TestValidateWarnings_UnreachableRule_WithActions(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name: "critical",
				Rules: []RuleConfig{
					{Resource: []string{"*"}, Actions: []string{"delete"}},
				},
			},
			{
				Name: "standard",
				Rules: []RuleConfig{
					{Resource: []string{"*_role_*"}},
				},
			},
		},
		Precedence: []string{"critical", "standard"},
	}

	warnings := ValidateWarnings(cfg)

	for _, w := range warnings {
		if strings.Contains(w.Message, "unreachable") {
			t.Errorf("expected no unreachable warning when catch-all has actions, got: %v", w)
		}
	}
}

func TestValidateRules_ActionsAndNotActionsMutuallyExclusive(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "standard",
				Description: "Standard",
				Rules: []RuleConfig{
					{
						Resource:   []string{"*"},
						Actions:    []string{"create"},
						NotActions: []string{"no-op"},
					},
				},
			},
		},
		Precedence: []string{"standard"},
		Defaults:   &DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for rule with both actions and not_actions, got nil")
	}

	if !strings.Contains(err.Error(), "cannot combine actions and not_actions") {
		t.Errorf("expected mutual exclusivity error, got: %v", err)
	}
}

func TestValidateRules_NotActionsInvalidValue(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "standard",
				Description: "Standard",
				Rules: []RuleConfig{
					{
						Resource:   []string{"*"},
						NotActions: []string{"destroy"},
					},
				},
			},
		},
		Precedence: []string{"standard"},
		Defaults:   &DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid not_actions value, got nil")
	}

	if !strings.Contains(err.Error(), "invalid not_actions value") {
		t.Errorf("expected error about invalid not_actions value, got: %v", err)
	}
	if !strings.Contains(err.Error(), "destroy") {
		t.Errorf("expected error to mention 'destroy', got: %v", err)
	}
}

func TestValidateRules_NotActionsValid(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "standard",
				Description: "Standard",
				Rules: []RuleConfig{
					{
						Resource:   []string{"*"},
						NotActions: []string{"no-op"},
					},
				},
			},
		},
		Precedence: []string{"standard"},
		Defaults:   &DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	err := Validate(cfg)
	if err != nil {
		t.Errorf("expected no error for valid not_actions, got: %v", err)
	}
}

func TestValidateWarnings_EmptyClassification(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:  "empty",
				Rules: nil,
			},
			{
				Name: "standard",
				Rules: []RuleConfig{
					{Resource: []string{"*"}},
				},
			},
		},
		Precedence: []string{"empty", "standard"},
	}

	warnings := ValidateWarnings(cfg)

	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "no rules") && w.Classification == "empty" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected empty classification warning, got: %v", warnings)
	}
}

func TestValidate_SARIFLevel_Valid(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				SARIFLevel:  "error",
				Rules:       []RuleConfig{{Resource: []string{"*_role_*"}}},
			},
			{
				Name:        "standard",
				Description: "Standard",
				SARIFLevel:  "note",
				Rules:       []RuleConfig{{Resource: []string{"*"}}},
			},
		},
		Precedence: []string{"critical", "standard"},
		Defaults:   &DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	if err := Validate(cfg); err != nil {
		t.Errorf("expected no error for valid sarif_level values, got: %v", err)
	}
}

func TestValidate_SARIFLevel_Invalid(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				SARIFLevel:  "critical",
				Rules:       []RuleConfig{{Resource: []string{"*"}}},
			},
		},
		Precedence: []string{"critical"},
		Defaults:   &DefaultsConfig{Unclassified: "critical", NoChanges: "critical"},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid sarif_level value, got nil")
	}

	if !strings.Contains(err.Error(), "sarif_level") {
		t.Errorf("expected error to mention sarif_level, got: %v", err)
	}
	if !strings.Contains(err.Error(), "critical") {
		t.Errorf("expected error to mention the invalid value, got: %v", err)
	}
}

func TestValidate_SARIFLevel_Empty(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "standard",
				Description: "Standard",
				Rules:       []RuleConfig{{Resource: []string{"*"}}},
			},
		},
		Precedence: []string{"standard"},
		Defaults:   &DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	if err := Validate(cfg); err != nil {
		t.Errorf("expected no error when sarif_level is omitted, got: %v", err)
	}
}

func TestValidate_DuplicatePrecedence(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules:       []RuleConfig{{Resource: []string{"*_role_*"}}},
			},
			{
				Name:        "standard",
				Description: "Standard",
				Rules:       []RuleConfig{{Resource: []string{"*"}}},
			},
		},
		Precedence: []string{"critical", "standard", "critical"},
		Defaults:   &DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate precedence entry, got nil")
	}

	if !strings.Contains(err.Error(), "duplicate entry") {
		t.Errorf("expected error about duplicate entry, got: %v", err)
	}
	if !strings.Contains(err.Error(), "critical") {
		t.Errorf("expected error to mention 'critical', got: %v", err)
	}
}

func TestValidate_DuplicatePrecedence_NoDuplicates(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules:       []RuleConfig{{Resource: []string{"*_role_*"}}},
			},
			{
				Name:        "standard",
				Description: "Standard",
				Rules:       []RuleConfig{{Resource: []string{"*"}}},
			},
		},
		Precedence: []string{"critical", "standard"},
		Defaults:   &DefaultsConfig{Unclassified: "standard", NoChanges: "standard"},
	}

	err := Validate(cfg)
	if err != nil {
		t.Errorf("unexpected error for unique precedence entries: %v", err)
	}
}

func TestValidateRules_ModuleAndNotModuleMutuallyExclusive(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				Rules: []RuleConfig{
					{
						Resource:  []string{"*"},
						Module:    []string{"module.production"},
						NotModule: []string{"module.staging"},
					},
				},
			},
		},
		Precedence: []string{"critical"},
		Defaults:   &DefaultsConfig{Unclassified: "critical", NoChanges: "critical"},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for rule with both module and not_module, got nil")
	}

	if !strings.Contains(err.Error(), "cannot combine module and not_module") {
		t.Errorf("expected mutual exclusivity error, got: %v", err)
	}
}

func TestValidateGlobPatterns_ModulePatterns(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name: "critical",
				Rules: []RuleConfig{
					{Resource: []string{"*"}, Module: []string{"module.production", "module.production.**"}},
				},
			},
		},
	}

	if err := ValidateGlobPatterns(cfg); err != nil {
		t.Errorf("expected no error for valid module patterns, got: %v", err)
	}
}

func TestValidateGlobPatterns_InvalidModulePattern(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name: "critical",
				Rules: []RuleConfig{
					{Resource: []string{"*"}, Module: []string{"[bad"}},
				},
			},
		},
	}

	err := ValidateGlobPatterns(cfg)
	if err == nil {
		t.Fatal("expected error for invalid module pattern, got nil")
	}
	if !strings.Contains(err.Error(), "module pattern") {
		t.Errorf("expected error to mention module pattern, got: %v", err)
	}
}

func TestValidateGlobPatterns_InvalidNotModulePattern(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name: "critical",
				Rules: []RuleConfig{
					{Resource: []string{"*"}, NotModule: []string{"[bad"}},
				},
			},
		},
	}

	err := ValidateGlobPatterns(cfg)
	if err == nil {
		t.Fatal("expected error for invalid not_module pattern, got nil")
	}
	if !strings.Contains(err.Error(), "not_module pattern") {
		t.Errorf("expected error to mention not_module pattern, got: %v", err)
	}
}

func TestValidate_IgnoreAttributesEmpty(t *testing.T) {
	_, err := LoadFile("testdata/ignore_attributes_empty.hcl")
	if err == nil {
		t.Fatal("expected error for empty ignore_attributes entry, got nil")
	}

	if !strings.Contains(err.Error(), "ignore_attributes") {
		t.Errorf("expected error about ignore_attributes, got: %v", err)
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("expected error about empty entry, got: %v", err)
	}
}

func TestValidate_IgnoreAttributesValid(t *testing.T) {
	cfg, err := LoadFile("testdata/ignore_attributes_valid.hcl")
	if err != nil {
		t.Fatalf("expected no error for valid ignore_attributes, got: %v", err)
	}

	if len(cfg.Defaults.IgnoreAttributes) != 2 {
		t.Errorf("expected 2 ignore_attributes entries, got %d", len(cfg.Defaults.IgnoreAttributes))
	}
	if cfg.Defaults.IgnoreAttributes[0] != "tags" || cfg.Defaults.IgnoreAttributes[1] != "tags_all" {
		t.Errorf("expected [tags, tags_all], got %v", cfg.Defaults.IgnoreAttributes)
	}
}

func TestValidateWarnings_EmptyClassification_WithPlugin(t *testing.T) {
	cfg := &Config{
		Classifications: []ClassificationConfig{
			{
				Name:  "critical",
				Rules: nil,
				PluginAnalyzerConfigs: map[string]*PluginAnalyzerConfig{
					"azurerm": {PrivilegeEscalation: &PrivilegeEscalationConfig{}},
				},
			},
		},
		Precedence: []string{"critical"},
	}

	warnings := ValidateWarnings(cfg)

	for _, w := range warnings {
		if strings.Contains(w.Message, "no rules") && w.Classification == "critical" {
			t.Errorf("expected no empty classification warning when plugin is configured, got: %v", w)
		}
	}
}
