package plan

import (
	"bytes"
	"testing"
)

// FuzzParseJSON fuzzes the JSON plan parser with malformed input.
// Seed corpus includes a minimal valid plan.
func FuzzParseJSON(f *testing.F) {
	// Seed with a valid minimal plan
	f.Add([]byte(`{
		"format_version": "1.2",
		"terraform_version": "1.9.0",
		"resource_changes": []
	}`))

	// Seed with a plan that has resources
	f.Add([]byte(`{
		"format_version": "1.2",
		"terraform_version": "1.9.0",
		"resource_changes": [{
			"address": "azurerm_resource_group.test",
			"type": "azurerm_resource_group",
			"provider_name": "registry.terraform.io/hashicorp/azurerm",
			"mode": "managed",
			"change": {
				"actions": ["create"],
				"before": null,
				"after": {"name": "test", "location": "eastus"},
				"before_sensitive": {},
				"after_sensitive": {}
			}
		}]
	}`))

	// Seed with empty JSON
	f.Add([]byte(`{}`))

	// Seed with truncated JSON
	f.Add([]byte(`{"format_version": "1.`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Parse should not panic on any input
		_, _ = Parse(bytes.NewReader(data))
	})
}
