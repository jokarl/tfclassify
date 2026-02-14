package plugin

import (
	"testing"
)

func TestVerifyPlugin_NameMismatch(t *testing.T) {
	info := &PluginInfo{
		Name:       "wrong-name",
		Version:    "0.1.0",
		SDKVersion: "0.1.0",
	}

	err := VerifyPlugin("expected-name", info)
	if err == nil {
		t.Fatal("expected error for name mismatch")
	}

	if err.Error() != `plugin name mismatch: expected "expected-name", got "wrong-name"` {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestVerifyPlugin_SDKTooOld(t *testing.T) {
	info := &PluginInfo{
		Name:       "test-plugin",
		Version:    "1.0.0",
		SDKVersion: "0.0.0", // Older than required 0.0.1
	}

	err := VerifyPlugin("test-plugin", info)
	if err == nil {
		t.Fatal("expected error for old SDK version")
	}

	// The error should mention SDK version
	if !containsString(err.Error(), "SDK version") {
		t.Errorf("expected error to mention SDK version, got: %v", err)
	}
}

func TestVerifyPlugin_SDKCompatible(t *testing.T) {
	tests := []struct {
		name       string
		sdkVersion string
	}{
		{"exact match", "0.0.1"},
		{"newer patch", "0.0.5"},
		{"newer minor", "0.1.0"},
		{"newer major", "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &PluginInfo{
				Name:       "test-plugin",
				Version:    "1.0.0",
				SDKVersion: tt.sdkVersion,
			}

			err := VerifyPlugin("test-plugin", info)
			if err != nil {
				t.Errorf("unexpected error for SDK version %s: %v", tt.sdkVersion, err)
			}
		})
	}
}

func TestVerifyPlugin_HostConstraintNotMet(t *testing.T) {
	info := &PluginInfo{
		Name:                  "test-plugin",
		Version:               "1.0.0",
		SDKVersion:            "0.1.0",
		HostVersionConstraint: ">= 99.0.0", // Requires future version
	}

	err := VerifyPlugin("test-plugin", info)
	if err == nil {
		t.Fatal("expected error for unmet host constraint")
	}

	if !containsString(err.Error(), "host version") {
		t.Errorf("expected error to mention host version, got: %v", err)
	}
}

func TestVerifyPlugin_HostConstraintMet(t *testing.T) {
	info := &PluginInfo{
		Name:                  "test-plugin",
		Version:               "1.0.0",
		SDKVersion:            "0.0.1",
		HostVersionConstraint: ">= 0.0.1", // Current host should satisfy this
	}

	err := VerifyPlugin("test-plugin", info)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVerifyPlugin_EmptyConstraints(t *testing.T) {
	info := &PluginInfo{
		Name:                  "test-plugin",
		Version:               "1.0.0",
		SDKVersion:            "", // Empty should be allowed
		HostVersionConstraint: "", // Empty should be allowed
	}

	err := VerifyPlugin("test-plugin", info)
	if err != nil {
		t.Errorf("unexpected error for empty constraints: %v", err)
	}
}

func TestCheckVersionConstraint_Valid(t *testing.T) {
	tests := []struct {
		version    string
		constraint string
	}{
		{"1.0.0", ">= 1.0.0"},
		{"1.5.0", ">= 1.0.0"},
		{"2.0.0", ">= 1.0.0"},
		{"1.0.0", "~> 1.0"},
		{"1.9.0", "~> 1.0"},
		{"1.0.0", "= 1.0.0"},
		{"0.1.0", ">= 0.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.version+" "+tt.constraint, func(t *testing.T) {
			err := checkVersionConstraint(tt.version, tt.constraint)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCheckVersionConstraint_Invalid(t *testing.T) {
	tests := []struct {
		version    string
		constraint string
	}{
		{"0.9.0", ">= 1.0.0"},
		{"0.1.0", "> 1.0.0"},
		{"2.0.0", "~> 1.0"},
		{"1.0.1", "= 1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.version+" "+tt.constraint, func(t *testing.T) {
			err := checkVersionConstraint(tt.version, tt.constraint)
			if err == nil {
				t.Error("expected error for unsatisfied constraint")
			}
		})
	}
}

func TestCheckVersionConstraint_InvalidVersion(t *testing.T) {
	err := checkVersionConstraint("not-a-version", ">= 1.0.0")
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestCheckVersionConstraint_InvalidConstraint(t *testing.T) {
	err := checkVersionConstraint("1.0.0", "not-a-constraint")
	if err == nil {
		t.Fatal("expected error for invalid constraint")
	}
}

func TestParseVersionConstraint_Valid(t *testing.T) {
	validConstraints := []string{
		">= 1.0.0",
		"~> 1.0",
		"= 1.0.0",
		"> 0.9, < 2.0",
		"",
	}

	for _, c := range validConstraints {
		err := ParseVersionConstraint(c)
		if err != nil {
			t.Errorf("unexpected error for constraint %q: %v", c, err)
		}
	}
}

func TestParseVersionConstraint_Invalid(t *testing.T) {
	err := ParseVersionConstraint("not valid")
	if err == nil {
		t.Fatal("expected error for invalid constraint")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.1.0", "1.0.9", 1},
	}

	for _, tt := range tests {
		t.Run(tt.v1+" vs "+tt.v2, func(t *testing.T) {
			result, err := CompareVersions(tt.v1, tt.v2)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestCompareVersions_InvalidVersion(t *testing.T) {
	_, err := CompareVersions("invalid", "1.0.0")
	if err == nil {
		t.Fatal("expected error for invalid version")
	}

	_, err = CompareVersions("1.0.0", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestExtractMajorMinor(t *testing.T) {
	tests := []struct {
		version  string
		expected string
	}{
		{"1.2.3", "1.2"},
		{"1.0.0", "1.0"},
		{"0.1.5", "0.1"},
		{"1.2", "1.2"},
		{"1", "1"},
	}

	for _, tt := range tests {
		result := ExtractMajorMinor(tt.version)
		if result != tt.expected {
			t.Errorf("ExtractMajorMinor(%q) = %q, want %q", tt.version, result, tt.expected)
		}
	}
}

func TestHostVersion(t *testing.T) {
	// Verify HostVersion is a valid semver
	err := ParseVersionConstraint(">= " + HostVersion)
	if err != nil {
		t.Errorf("HostVersion %q is not valid semver: %v", HostVersion, err)
	}
}

func TestSDKVersionConstraints_Valid(t *testing.T) {
	// Verify SDKVersionConstraints is a valid constraint
	err := ParseVersionConstraint(SDKVersionConstraints)
	if err != nil {
		t.Errorf("SDKVersionConstraints %q is not valid: %v", SDKVersionConstraints, err)
	}
}

// containsString is a helper to check if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
