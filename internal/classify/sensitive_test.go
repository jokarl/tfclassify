package classify

import (
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/internal/plan"
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
	if !strings.Contains(decisions[0].MatchedRules[0], "builtin: sensitive") {
		t.Errorf("expected MatchedRule to contain 'builtin: sensitive', got %q", decisions[0].MatchedRules[0])
	}
	if !strings.Contains(decisions[0].MatchedRules[0], "value") {
		t.Errorf("expected MatchedRule to mention 'value' attribute, got %q", decisions[0].MatchedRules[0])
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
	if strings.Contains(decisions[0].MatchedRules[0], sensitiveValue) {
		t.Errorf("MatchedRule should NOT expose sensitive value, got: %s", decisions[0].MatchedRules[0])
	}
	if !strings.Contains(decisions[0].MatchedRules[0], "value") {
		t.Errorf("MatchedRule should mention attribute name 'value', got: %s", decisions[0].MatchedRules[0])
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
	if !strings.Contains(decisions[0].MatchedRules[0], "password") {
		t.Errorf("expected MatchedRule to mention password, got: %s", decisions[0].MatchedRules[0])
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
	rule := decisions[0].MatchedRules[0]
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

// Recursive sensitive detection tests

func TestSensitiveAnalyzer_NestedSensitive(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_app_service.main",
			Type:    "azurerm_app_service",
			Actions: []string{"update"},
			Before: map[string]interface{}{
				"settings": map[string]interface{}{
					"password": "old-secret",
					"name":     "myapp",
				},
			},
			After: map[string]interface{}{
				"settings": map[string]interface{}{
					"password": "new-secret",
					"name":     "myapp",
				},
			},
			BeforeSensitive: map[string]interface{}{
				"settings": map[string]interface{}{
					"password": true,
				},
			},
			AfterSensitive: map[string]interface{}{
				"settings": map[string]interface{}{
					"password": true,
				},
			},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	rule := decisions[0].MatchedRules[0]
	if !strings.Contains(rule, "settings.password") {
		t.Errorf("expected MatchedRule to mention 'settings.password', got: %s", rule)
	}
}

func TestSensitiveAnalyzer_DeeplyNestedSensitive(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_resource.deep",
			Type:    "azurerm_resource",
			Actions: []string{"update"},
			Before: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"secret_key": "old-key",
					},
				},
			},
			After: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"secret_key": "new-key",
					},
				},
			},
			BeforeSensitive: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"secret_key": true,
					},
				},
			},
			AfterSensitive: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"secret_key": true,
					},
				},
			},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	rule := decisions[0].MatchedRules[0]
	if !strings.Contains(rule, "level1.level2.secret_key") {
		t.Errorf("expected MatchedRule to mention 'level1.level2.secret_key', got: %s", rule)
	}
}

func TestSensitiveAnalyzer_ListIndexedSensitive(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_resource.list",
			Type:    "azurerm_resource",
			Actions: []string{"update"},
			Before: map[string]interface{}{
				"config": []interface{}{
					map[string]interface{}{"secret_key": "old-key", "name": "first"},
				},
			},
			After: map[string]interface{}{
				"config": []interface{}{
					map[string]interface{}{"secret_key": "new-key", "name": "first"},
				},
			},
			BeforeSensitive: map[string]interface{}{
				"config": []interface{}{
					map[string]interface{}{"secret_key": true},
				},
			},
			AfterSensitive: map[string]interface{}{
				"config": []interface{}{
					map[string]interface{}{"secret_key": true},
				},
			},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	rule := decisions[0].MatchedRules[0]
	if !strings.Contains(rule, "config.0.secret_key") {
		t.Errorf("expected MatchedRule to mention 'config.0.secret_key', got: %s", rule)
	}
}

func TestSensitiveAnalyzer_NestedUnchanged(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_resource.unchanged",
			Type:    "azurerm_resource",
			Actions: []string{"update"},
			Before: map[string]interface{}{
				"settings": map[string]interface{}{
					"password": "same-pass",
					"name":     "old-name",
				},
			},
			After: map[string]interface{}{
				"settings": map[string]interface{}{
					"password": "same-pass",
					"name":     "new-name",
				},
			},
			BeforeSensitive: map[string]interface{}{
				"settings": map[string]interface{}{
					"password": true,
				},
			},
			AfterSensitive: map[string]interface{}{
				"settings": map[string]interface{}{
					"password": true,
				},
			},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for unchanged nested sensitive, got %d", len(decisions))
	}
}

func TestSensitiveAnalyzer_MixedTopLevelAndNested(t *testing.T) {
	a := &SensitiveAnalyzer{}
	changes := []plan.ResourceChange{
		{
			Address: "azurerm_resource.mixed",
			Type:    "azurerm_resource",
			Actions: []string{"update"},
			Before: map[string]interface{}{
				"api_key": "old-key",
				"settings": map[string]interface{}{
					"password": "old-pass",
				},
			},
			After: map[string]interface{}{
				"api_key": "new-key",
				"settings": map[string]interface{}{
					"password": "new-pass",
				},
			},
			BeforeSensitive: map[string]interface{}{
				"api_key": true,
				"settings": map[string]interface{}{
					"password": true,
				},
			},
			AfterSensitive: map[string]interface{}{
				"api_key": true,
				"settings": map[string]interface{}{
					"password": true,
				},
			},
		},
	}

	decisions := a.Analyze(changes)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	rule := decisions[0].MatchedRules[0]
	if !strings.Contains(rule, "api_key") {
		t.Errorf("expected MatchedRule to mention 'api_key', got: %s", rule)
	}
	if !strings.Contains(rule, "settings.password") {
		t.Errorf("expected MatchedRule to mention 'settings.password', got: %s", rule)
	}
}

// Helper function tests

func TestValueChanged_BothNil(t *testing.T) {
	if valueChanged(nil, nil) {
		t.Error("expected false when both are nil")
	}
}

func TestValueChanged_Added(t *testing.T) {
	if !valueChanged(nil, "new") {
		t.Error("expected true when value is added")
	}
}

func TestValueChanged_Removed(t *testing.T) {
	if !valueChanged("old", nil) {
		t.Error("expected true when value is removed")
	}
}

func TestValueChanged_Changed(t *testing.T) {
	if !valueChanged("old", "new") {
		t.Error("expected true when value changed")
	}
}

func TestValueChanged_Unchanged(t *testing.T) {
	if valueChanged("same", "same") {
		t.Error("expected false when value unchanged")
	}
}
