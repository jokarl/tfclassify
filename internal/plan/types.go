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

	// OriginalActions is set when Actions were rewritten by ignore_attributes
	// preprocessing. Contains the original Terraform-reported actions (e.g., ["update"]).
	OriginalActions []string
	// IgnoredAttributes lists the specific attribute paths that were filtered
	// by ignore_attributes (e.g., ["tags.tf-module-l2"]).
	IgnoredAttributes []string
}

// ParseResult contains the parsed plan data.
type ParseResult struct {
	FormatVersion string
	Changes       []ResourceChange
	DriftChanges  []ResourceChange // from plan.ResourceDrift
	Config        *tfjson.Config   // full Terraform config (dependency graph)
}
