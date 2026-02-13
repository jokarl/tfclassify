// Package plan provides Terraform plan JSON parsing functionality.
package plan

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
)

// supportedFormatVersions defines the Terraform plan JSON format versions that are supported.
var supportedFormatVersions = map[string]bool{
	"0.2": true,
	"1.0": true,
	"1.1": true,
	"1.2": true,
}

// ParseFile reads and parses a Terraform plan JSON file.
func ParseFile(path string) (*ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plan file: %w", err)
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads and parses Terraform plan JSON from a reader.
func Parse(r io.Reader) (*ParseResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan data: %w", err)
	}

	var plan tfjson.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	if err := validateFormatVersion(plan.FormatVersion); err != nil {
		return nil, err
	}

	changes := extractResourceChanges(&plan)

	return &ParseResult{
		FormatVersion: plan.FormatVersion,
		Changes:       changes,
	}, nil
}

// validateFormatVersion checks that the plan format version is supported.
func validateFormatVersion(version string) error {
	if version == "" {
		return fmt.Errorf("plan format_version is missing")
	}

	// Check exact match first
	if supportedFormatVersions[version] {
		return nil
	}

	// Check major.minor prefix for forward compatibility
	for supported := range supportedFormatVersions {
		if strings.HasPrefix(version, supported) {
			return nil
		}
	}

	return fmt.Errorf("unsupported plan format_version %q; supported versions are: 0.2, 1.0, 1.1, 1.2", version)
}

// extractResourceChanges converts terraform-json resource changes to our internal type.
func extractResourceChanges(plan *tfjson.Plan) []ResourceChange {
	if plan.ResourceChanges == nil {
		return []ResourceChange{}
	}

	changes := make([]ResourceChange, 0, len(plan.ResourceChanges))
	for _, rc := range plan.ResourceChanges {
		change := ResourceChange{
			Address:      rc.Address,
			Type:         rc.Type,
			ProviderName: rc.ProviderName,
			Mode:         string(rc.Mode),
			Actions:      actionsToStrings(rc.Change.Actions),
		}

		if rc.Change != nil {
			change.Before = convertToMap(rc.Change.Before)
			change.After = convertToMap(rc.Change.After)
			change.BeforeSensitive = rc.Change.BeforeSensitive
			change.AfterSensitive = rc.Change.AfterSensitive
		}

		changes = append(changes, change)
	}

	return changes
}

// actionsToStrings converts terraform-json Actions to string slice.
func actionsToStrings(actions tfjson.Actions) []string {
	result := make([]string, len(actions))
	for i, a := range actions {
		result[i] = string(a)
	}
	return result
}

// convertToMap converts an interface{} to map[string]interface{}.
// If the input is already a map, it returns it directly.
// Otherwise, it returns nil.
func convertToMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}
