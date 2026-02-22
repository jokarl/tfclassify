// Package plan provides Terraform plan JSON parsing functionality.
package plan

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
}

// ParseResult contains the parsed plan data.
type ParseResult struct {
	FormatVersion string
	Changes       []ResourceChange
}
