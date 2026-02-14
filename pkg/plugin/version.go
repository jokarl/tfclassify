// Package plugin provides plugin discovery and lifecycle management.
package plugin

import (
	"fmt"
	"strings"

	goversion "github.com/hashicorp/go-version"
)

// HostVersion is the current version of the tfclassify host.
const HostVersion = "0.0.1"

// PluginInfo contains metadata about a plugin for verification.
type PluginInfo struct {
	Name                  string
	Version               string
	SDKVersion            string
	HostVersionConstraint string
}

// VerifyPlugin performs version negotiation checks between host and plugin.
// Returns an error if the plugin is not compatible with this host.
func VerifyPlugin(expectedName string, info *PluginInfo) error {
	// Verify plugin name matches configuration
	if info.Name != expectedName {
		return fmt.Errorf("plugin name mismatch: expected %q, got %q", expectedName, info.Name)
	}

	// Verify SDK version is compatible
	if info.SDKVersion != "" {
		if err := checkVersionConstraint(info.SDKVersion, SDKVersionConstraints); err != nil {
			return fmt.Errorf("plugin SDK version %q is not compatible: %w", info.SDKVersion, err)
		}
	}

	// Verify host version satisfies plugin's constraint
	if info.HostVersionConstraint != "" {
		if err := checkVersionConstraint(HostVersion, info.HostVersionConstraint); err != nil {
			return fmt.Errorf("host version %q does not satisfy plugin constraint %q: %w",
				HostVersion, info.HostVersionConstraint, err)
		}
	}

	return nil
}

// checkVersionConstraint checks if a version satisfies a constraint.
// Constraint syntax follows semver conventions (e.g., ">= 0.1.0", "~> 1.0", "1.x").
func checkVersionConstraint(versionStr, constraintStr string) error {
	// Parse the version
	version, err := goversion.NewVersion(versionStr)
	if err != nil {
		return fmt.Errorf("invalid version %q: %w", versionStr, err)
	}

	// Parse the constraint
	constraints, err := goversion.NewConstraint(constraintStr)
	if err != nil {
		return fmt.Errorf("invalid constraint %q: %w", constraintStr, err)
	}

	// Check if version satisfies constraint
	if !constraints.Check(version) {
		return fmt.Errorf("version %s does not satisfy constraint %s", versionStr, constraintStr)
	}

	return nil
}

// ParseVersionConstraint parses a version constraint string and returns whether it's valid.
func ParseVersionConstraint(constraint string) error {
	if constraint == "" {
		return nil // Empty constraint matches all versions
	}

	_, err := goversion.NewConstraint(constraint)
	if err != nil {
		return fmt.Errorf("invalid version constraint: %w", err)
	}

	return nil
}

// CompareVersions compares two semantic version strings.
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func CompareVersions(v1, v2 string) (int, error) {
	ver1, err := goversion.NewVersion(v1)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", v1, err)
	}

	ver2, err := goversion.NewVersion(v2)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", v2, err)
	}

	return ver1.Compare(ver2), nil
}

// ExtractMajorMinor extracts the major.minor portion of a version string.
func ExtractMajorMinor(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}
