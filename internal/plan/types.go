// Package plan provides Terraform plan JSON parsing functionality.
package plan

import tfjson "github.com/hashicorp/terraform-json"

// ResourceChange represents a single resource change from a Terraform plan.
type ResourceChange struct {
	Address         string
	Type            string
	ProviderName    string
	Mode            string // "managed" or "data"
	Actions         []string
	Before          map[string]interface{}
	After           map[string]interface{}
	BeforeSensitive map[string]interface{}
	AfterSensitive  map[string]interface{}
	ModuleAddress   string // e.g. "module.production.module.network", "" for root
}

// ParseResult contains the parsed plan data.
type ParseResult struct {
	FormatVersion string
	Changes       []ResourceChange
	DriftChanges  []ResourceChange // from plan.ResourceDrift
	Config        *tfjson.Config   // full Terraform config (dependency graph)
}
