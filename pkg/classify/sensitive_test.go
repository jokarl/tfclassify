package classify

import (
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/pkg/plan"
)

func TestSensitiveAnalyzer_Name(t *testing.T) {
	a := &SensitiveAnalyzer{}
	if a.Name() != "sensitive" {
		t.Errorf("expected name 'sensitive', got %q", a.Name())
	}
}

func TestSensitiveAnalyzer_DetectsChange(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_key_vault_secret.password",
			Type:    "azurerm_key_vault_secret",
			Actions: []string{"update"},
			Before:  map[string]interface{}{"value": "old-secret"},
			After:   map[string]interface{}{"value": "new-secret"},
			BeforeSensitive: map[string]interface{}{"value": true},
			AfterSensitive:  map[string]interface{}{"value": true},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if !strings.Contains(decisions[0].MatchedRule, "builtin: sensitive") {
		t.Errorf("expected MatchedRule to contain 'builtin: sensitive', got %q", decisions[0].MatchedRule)
	}
	if !strings.Contains(decisions[0].MatchedRule, "value") {
		t.Errorf("expected MatchedRule to mention 'value' attribute, got %q", decisions[0].MatchedRule)
	}
}

func TestSensitiveAnalyzer_NoSensitive(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address:         "rg.one",
			Type:            "azurerm_resource_group",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"name": "old-name"},
			After:           map[string]interface{}{"name": "new-name"},
			BeforeSensitive: nil,
			AfterSensitive:  nil,
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for non-sensitive change, got %d", len(decisions))
	}
}

func TestSensitiveAnalyzer_NoValueExposed(t *testing.T) {
	a := &SensitiveAnalyzer{}
	sensitiveValue := "super-secret-password-12345"

	changes := []plan.ResourceChange{
		{
			Address:         "kv.secret",
			Type:            "azurerm_key_vault_secret",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"value": sensitiveValue},
			After:           map[string]interface{}{"value": "new-secret"},
			BeforeSensitive: map[string]interface{}{"value": true},
			AfterSensitive:  map[string]interface{}{"value": true},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if strings.Contains(decisions[0].MatchedRule, sensitiveValue) {
		t.Errorf("MatchedRule should NOT expose sensitive value, got: %s", decisions[0].MatchedRule)
	}
	if !strings.Contains(decisions[0].MatchedRule, "value") {
		t.Errorf("MatchedRule should mention attribute name 'value', got: %s", decisions[0].MatchedRule)
	}
}

func TestSensitiveAnalyzer_NewSensitiveAttribute(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address:         "db.main",
			Type:            "aws_db_instance",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"password": "old-pass"},
			After:           map[string]interface{}{"password": "new-pass"},
			BeforeSensitive: nil,
			AfterSensitive:  map[string]interface{}{"password": true},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if !strings.Contains(decisions[0].MatchedRule, "password") {
		t.Errorf("expected MatchedRule to mention password, got: %s", decisions[0].MatchedRule)
	}
}

func TestSensitiveAnalyzer_RemovedAttribute(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address:         "db.main",
			Type:            "aws_db_instance",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"password": "old-pass"},
			After:           map[string]interface{}{},
			BeforeSensitive: map[string]interface{}{"password": true},
			AfterSensitive:  nil,
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision for removed sensitive attr, got %d", len(decisions))
	}
}

func TestSensitiveAnalyzer_UnchangedValue(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address:         "db.main",
			Type:            "aws_db_instance",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"password": "same-pass", "name": "old-name"},
			After:           map[string]interface{}{"password": "same-pass", "name": "new-name"},
			BeforeSensitive: map[string]interface{}{"password": true},
			AfterSensitive:  map[string]interface{}{"password": true},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for unchanged sensitive attr, got %d", len(decisions))
	}
}

func TestSensitiveAnalyzer_MultipleSensitive(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address: "db.main",
			Type:    "aws_db_instance",
			Actions: []string{"update"},
			Before:  map[string]interface{}{"password": "old-pass", "api_key": "old-key"},
			After:   map[string]interface{}{"password": "new-pass", "api_key": "new-key"},
			BeforeSensitive: map[string]interface{}{"password": true, "api_key": true},
			AfterSensitive:  map[string]interface{}{"password": true, "api_key": true},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision (aggregated), got %d", len(decisions))
	}
	rule := decisions[0].MatchedRule
	if !strings.Contains(rule, "password") || !strings.Contains(rule, "api_key") {
		t.Errorf("expected MatchedRule to mention both attrs, got: %s", rule)
	}
}

func TestSensitiveAnalyzer_NoChanges(t *testing.T) {
	a := &SensitiveAnalyzer{}
	decisions := a.Analyze([]plan.ResourceChange{})
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for no changes, got %d", len(decisions))
	}
}

func TestSensitiveAnalyzer_NilChanges(t *testing.T) {
	a := &SensitiveAnalyzer{}
	decisions := a.Analyze(nil)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for nil changes, got %d", len(decisions))
	}
}

func TestSensitiveAnalyzer_EmptyClassification(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address:         "test.resource",
			Type:            "test_type",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"secret": "old"},
			After:           map[string]interface{}{"secret": "new"},
			BeforeSensitive: map[string]interface{}{"secret": true},
			AfterSensitive:  map[string]interface{}{"secret": true},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Classification != "" {
		t.Errorf("expected empty classification, got %q", decisions[0].Classification)
	}
}

func TestSensitiveAnalyzer_MultipleResources(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address:         "db.one",
			Type:            "aws_db_instance",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"password": "old"},
			After:           map[string]interface{}{"password": "new"},
			BeforeSensitive: map[string]interface{}{"password": true},
			AfterSensitive:  map[string]interface{}{"password": true},
		},
		{
			Address:         "vnet.one",
			Type:            "azurerm_virtual_network",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"name": "old"},
			After:           map[string]interface{}{"name": "new"},
			BeforeSensitive: nil,
			AfterSensitive:  nil,
		},
		{
			Address:         "db.two",
			Type:            "aws_db_instance",
			Actions:         []string{"update"},
			Before:          map[string]interface{}{"api_key": "old"},
			After:           map[string]interface{}{"api_key": "new"},
			BeforeSensitive: map[string]interface{}{"api_key": true},
			AfterSensitive:  map[string]interface{}{"api_key": true},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions (two sensitive resources), got %d", len(decisions))
	}
}

// Helper function tests

func TestAsBoolMap_Nil(t *testing.T) {
	if asBoolMap(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

func TestAsBoolMap_NotMap(t *testing.T) {
	if asBoolMap("not a map") != nil {
		t.Error("expected nil for string input")
	}
	if asBoolMap([]string{"a"}) != nil {
		t.Error("expected nil for slice input")
	}
	if asBoolMap(42) != nil {
		t.Error("expected nil for int input")
	}
	if asBoolMap(true) != nil {
		t.Error("expected nil for bool input")
	}
}

func TestAsBoolMap_ValidMap(t *testing.T) {
	input := map[string]interface{}{"password": true}
	result := asBoolMap(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["password"] != true {
		t.Errorf("expected password=true, got %v", result["password"])
	}
}

func TestHasAttributeChanged_BothNil(t *testing.T) {
	if hasAttributeChanged("test", nil, nil) {
		t.Error("expected false when both are nil")
	}
}

func TestHasAttributeChanged_Added(t *testing.T) {
	if !hasAttributeChanged("password", map[string]interface{}{}, map[string]interface{}{"password": "new"}) {
		t.Error("expected true when attribute is added")
	}
}

func TestHasAttributeChanged_Removed(t *testing.T) {
	if !hasAttributeChanged("password", map[string]interface{}{"password": "old"}, map[string]interface{}{}) {
		t.Error("expected true when attribute is removed")
	}
}

func TestHasAttributeChanged_Changed(t *testing.T) {
	if !hasAttributeChanged("password", map[string]interface{}{"password": "old"}, map[string]interface{}{"password": "new"}) {
		t.Error("expected true when value changed")
	}
}

func TestHasAttributeChanged_Unchanged(t *testing.T) {
	if hasAttributeChanged("password", map[string]interface{}{"password": "same"}, map[string]interface{}{"password": "same"}) {
		t.Error("expected false when value unchanged")
	}
}

func TestHasAttributeChanged_BeforeNilAfterPresent(t *testing.T) {
	if !hasAttributeChanged("password", nil, map[string]interface{}{"password": "new"}) {
		t.Error("expected true when before is nil and after has attribute")
	}
}

func TestHasAttributeChanged_BeforePresentAfterNil(t *testing.T) {
	if !hasAttributeChanged("password", map[string]interface{}{"password": "old"}, nil) {
		t.Error("expected true when before has attribute and after is nil")
	}
}
