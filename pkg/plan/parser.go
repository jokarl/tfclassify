// Package plan provides Terraform plan JSON parsing functionality.
package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

// BinaryPlanMagicBytes is the magic bytes prefix for Terraform binary plan files.
// Terraform binary plans are zip files containing plan.tfplan and other files.
var BinaryPlanMagicBytes = []byte("PK") // ZIP file magic bytes

// ParseFile reads and parses a Terraform plan file (JSON or binary).
func ParseFile(path string) (*ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plan file: %w", err)
	}
	defer f.Close()

	// Read first few bytes to detect format
	header := make([]byte, 4)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read plan header: %w", err)
	}

	// Reset file position
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek plan file: %w", err)
	}

	// Check if it's a binary plan (ZIP format)
	if n >= 2 && bytes.HasPrefix(header, BinaryPlanMagicBytes) {
		return parseBinaryPlan(path)
	}

	// Try to parse as JSON
	return Parse(f)
}

// parseBinaryPlan converts a binary Terraform plan to JSON using `terraform show -json`.
func parseBinaryPlan(path string) (*ParseResult, error) {
	// Find terraform binary
	terraformPath, err := findTerraform()
	if err != nil {
		return nil, fmt.Errorf("binary plan detected but terraform CLI not found: %w", err)
	}

	// Resolve to absolute path so the basename works from the plan's directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve plan path: %w", err)
	}

	// Run terraform show -json from the plan file's directory so that
	// terraform can find the .terraform/ provider plugins created by init.
	cmd := exec.Command(terraformPath, "show", "-json", filepath.Base(absPath))
	cmd.Dir = filepath.Dir(absPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to convert binary plan: %s (stderr: %s)", err, stderr.String())
	}

	return Parse(bytes.NewReader(stdout.Bytes()))
}

// findTerraform finds the terraform binary in PATH.
func findTerraform() (string, error) {
	// Check for TERRAFORM_PATH env var first
	if envPath := os.Getenv("TERRAFORM_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
		return "", fmt.Errorf("TERRAFORM_PATH set to %q but file not found", envPath)
	}

	// Look in PATH
	path, err := exec.LookPath("terraform")
	if err != nil {
		// Also check for tofu (OpenTofu)
		path, err = exec.LookPath("tofu")
		if err != nil {
			return "", fmt.Errorf("terraform or tofu not found in PATH")
		}
	}
	return path, nil
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
// Forward compatible: accepts any patch version of supported major.minor versions.
func validateFormatVersion(version string) error {
	if version == "" {
		return fmt.Errorf("plan format_version is missing")
	}

	// Check exact match first
	if supportedFormatVersions[version] {
		return nil
	}

	// Extract major.minor prefix for forward compatibility
	// For example, "1.2.3" should be accepted if "1.2" is supported
	majorMinor := extractMajorMinor(version)
	if majorMinor != "" && supportedFormatVersions[majorMinor] {
		return nil
	}

	return fmt.Errorf("unsupported plan format_version %q; supported versions are: 0.2, 1.0, 1.1, 1.2", version)
}

// extractMajorMinor extracts the major.minor portion of a semver-like version string.
// For example: "1.2.3" -> "1.2", "1.0" -> "1.0", "2" -> ""
func extractMajorMinor(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "." + parts[1]
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
		}

		if rc.Change != nil {
			change.Actions = actionsToStrings(rc.Change.Actions)
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
