package plan

import (
	"os"
	"strings"
	"testing"
)

func TestParse_ValidPlan(t *testing.T) {
	result, err := ParseFile("testdata/valid_plan.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FormatVersion != "1.2" {
		t.Errorf("expected format version 1.2, got %s", result.FormatVersion)
	}

	if len(result.Changes) != 2 {
		t.Fatalf("expected 2 resource changes, got %d", len(result.Changes))
	}

	// Check first resource
	rc1 := result.Changes[0]
	if rc1.Address != "azurerm_role_assignment.example" {
		t.Errorf("expected address azurerm_role_assignment.example, got %s", rc1.Address)
	}
	if rc1.Type != "azurerm_role_assignment" {
		t.Errorf("expected type azurerm_role_assignment, got %s", rc1.Type)
	}
	if rc1.Mode != "managed" {
		t.Errorf("expected mode managed, got %s", rc1.Mode)
	}
	if len(rc1.Actions) != 1 || rc1.Actions[0] != "create" {
		t.Errorf("expected actions [create], got %v", rc1.Actions)
	}
	if rc1.Before != nil {
		t.Errorf("expected Before to be nil, got %v", rc1.Before)
	}
	if rc1.After == nil {
		t.Error("expected After to be non-nil")
	}

	// Check second resource
	rc2 := result.Changes[1]
	if rc2.Address != "azurerm_virtual_network.main" {
		t.Errorf("expected address azurerm_virtual_network.main, got %s", rc2.Address)
	}
	if len(rc2.Actions) != 1 || rc2.Actions[0] != "update" {
		t.Errorf("expected actions [update], got %v", rc2.Actions)
	}
}

func TestParse_EmptyPlan(t *testing.T) {
	result, err := ParseFile("testdata/empty_plan.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Changes == nil {
		t.Error("expected Changes to be non-nil empty slice, got nil")
	}

	if len(result.Changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(result.Changes))
	}
}

func TestParse_MalformedJSON(t *testing.T) {
	reader := strings.NewReader("this is not valid JSON")
	_, err := Parse(reader)
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}

	if !strings.Contains(err.Error(), "failed to parse plan JSON") {
		t.Errorf("expected error message to contain 'failed to parse plan JSON', got: %v", err)
	}
}

func TestParse_UnsupportedVersion(t *testing.T) {
	_, err := ParseFile("testdata/unsupported_version.json")
	if err == nil {
		t.Error("expected error for unsupported version, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported plan format_version") {
		t.Errorf("expected error to mention unsupported format_version, got: %v", err)
	}

	if !strings.Contains(err.Error(), "0.1") {
		t.Errorf("expected error to include the actual version, got: %v", err)
	}
}

func TestParse_SensitiveValues(t *testing.T) {
	result, err := ParseFile("testdata/sensitive_values.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}

	rc := result.Changes[0]
	if rc.BeforeSensitive == nil {
		t.Error("expected BeforeSensitive to be non-nil")
	}
	if rc.AfterSensitive == nil {
		t.Error("expected AfterSensitive to be non-nil")
	}

	// Check that sensitive markers are preserved
	beforeSens, ok := rc.BeforeSensitive.(map[string]interface{})
	if !ok {
		t.Fatalf("expected BeforeSensitive to be map, got %T", rc.BeforeSensitive)
	}
	if beforeSens["value"] != true {
		t.Errorf("expected BeforeSensitive[value] to be true, got %v", beforeSens["value"])
	}
}

func TestParse_DataSource(t *testing.T) {
	result, err := ParseFile("testdata/data_source.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}

	rc := result.Changes[0]
	if rc.Mode != "data" {
		t.Errorf("expected mode 'data', got %s", rc.Mode)
	}
	if rc.Type != "azurerm_subscription" {
		t.Errorf("expected type azurerm_subscription, got %s", rc.Type)
	}
}

func TestParse_Actions(t *testing.T) {
	result, err := ParseFile("testdata/all_actions.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Changes) != 4 {
		t.Fatalf("expected 4 changes, got %d", len(result.Changes))
	}

	tests := []struct {
		address string
		actions []string
	}{
		{"azurerm_resource_group.create", []string{"create"}},
		{"azurerm_resource_group.update", []string{"update"}},
		{"azurerm_resource_group.delete", []string{"delete"}},
		{"azurerm_resource_group.replace", []string{"delete", "create"}},
	}

	for i, tt := range tests {
		rc := result.Changes[i]
		if rc.Address != tt.address {
			t.Errorf("change %d: expected address %s, got %s", i, tt.address, rc.Address)
		}
		if len(rc.Actions) != len(tt.actions) {
			t.Errorf("change %d: expected %d actions, got %d", i, len(tt.actions), len(rc.Actions))
			continue
		}
		for j, action := range tt.actions {
			if rc.Actions[j] != action {
				t.Errorf("change %d action %d: expected %s, got %s", i, j, action, rc.Actions[j])
			}
		}
	}
}

func TestParseFile_FileNotFound(t *testing.T) {
	_, err := ParseFile("testdata/nonexistent.json")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to open plan file") {
		t.Errorf("expected error to mention 'failed to open plan file', got: %v", err)
	}
}

func TestParse_PreservesBeforeAfter(t *testing.T) {
	result, err := ParseFile("testdata/nested_values.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}

	rc := result.Changes[0]
	after := rc.After
	if after == nil {
		t.Fatal("expected After to be non-nil")
	}

	// Check nested map (os_disk)
	osDisk, ok := after["os_disk"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected os_disk to be map, got %T", after["os_disk"])
	}
	if osDisk["caching"] != "ReadWrite" {
		t.Errorf("expected os_disk.caching to be 'ReadWrite', got %v", osDisk["caching"])
	}

	// Check slice value (network_interface_ids)
	nics, ok := after["network_interface_ids"].([]interface{})
	if !ok {
		t.Fatalf("expected network_interface_ids to be slice, got %T", after["network_interface_ids"])
	}
	if len(nics) != 2 {
		t.Errorf("expected 2 NICs, got %d", len(nics))
	}

	// Check nested map (tags)
	tags, ok := after["tags"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tags to be map, got %T", after["tags"])
	}
	if tags["environment"] != "production" {
		t.Errorf("expected tags.environment to be 'production', got %v", tags["environment"])
	}
}

func TestParse_FromReader(t *testing.T) {
	f, err := os.Open("testdata/valid_plan.json")
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	result, err := Parse(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Changes) != 2 {
		t.Errorf("expected 2 changes, got %d", len(result.Changes))
	}
}
