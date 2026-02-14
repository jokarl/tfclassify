// Package config provides HCL configuration loading for tfclassify.
package config

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// Load discovers and loads the configuration file.
// If explicitPath is provided, it uses that path directly.
// Otherwise, it searches for config files in standard locations.
func Load(explicitPath string) (*Config, error) {
	path, err := Discover(explicitPath)
	if err != nil {
		return nil, err
	}

	return LoadFile(path)
}

// LoadFile loads configuration from a specific file path.
func LoadFile(path string) (*Config, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	return Parse(src, path)
}

// Parse parses HCL configuration from bytes.
func Parse(src []byte, filename string) (*Config, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(src, filename)
	if diags.HasErrors() {
		return nil, formatDiagnostics(diags, filename)
	}

	var cfg Config
	decodeDiags := gohcl.DecodeBody(file.Body, nil, &cfg)
	if decodeDiags.HasErrors() {
		return nil, formatDiagnostics(decodeDiags, filename)
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// formatDiagnostics converts HCL diagnostics into a readable error.
func formatDiagnostics(diags hcl.Diagnostics, filename string) error {
	var errMsg string
	for _, diag := range diags {
		if diag.Severity == hcl.DiagError {
			if diag.Subject != nil {
				errMsg += fmt.Sprintf("%s:%d:%d: %s: %s\n",
					filename,
					diag.Subject.Start.Line,
					diag.Subject.Start.Column,
					diag.Summary,
					diag.Detail)
			} else {
				errMsg += fmt.Sprintf("%s: %s: %s\n", filename, diag.Summary, diag.Detail)
			}
		}
	}
	return fmt.Errorf("configuration error:\n%s", errMsg)
}
