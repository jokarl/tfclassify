package plugin

import (
	"bytes"
	"testing"

	"github.com/jokarl/tfclassify/pkg/config"
)

func TestInstallPlugins_BundledSkipped(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:    "terraform",
				Enabled: true,
				Source:  "", // bundled
			},
		},
	}

	var buf bytes.Buffer
	err := InstallPlugins(cfg, &buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("bundled (skip)")) {
		t.Errorf("expected bundled skip message, got: %s", buf.String())
	}
}

func TestInstallPlugins_DisabledSkipped(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:    "azurerm",
				Enabled: false,
				Source:  "github.com/jokarl/tfclassify-plugin-azurerm",
				Version: "0.1.0",
			},
		},
	}

	var buf bytes.Buffer
	err := InstallPlugins(cfg, &buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("disabled (skip)")) {
		t.Errorf("expected disabled skip message, got: %s", buf.String())
	}
}

func TestParseGitHubSource_Valid(t *testing.T) {
	tests := []struct {
		source string
		owner  string
		repo   string
	}{
		{"github.com/owner/repo", "owner", "repo"},
		{"https://github.com/owner/repo", "owner", "repo"},
		{"http://github.com/owner/repo", "owner", "repo"},
	}

	for _, tt := range tests {
		owner, repo, err := parseGitHubSource(tt.source)
		if err != nil {
			t.Errorf("parseGitHubSource(%q) error: %v", tt.source, err)
			continue
		}
		if owner != tt.owner || repo != tt.repo {
			t.Errorf("parseGitHubSource(%q) = (%q, %q), want (%q, %q)", tt.source, owner, repo, tt.owner, tt.repo)
		}
	}
}

func TestParseGitHubSource_Invalid(t *testing.T) {
	tests := []string{
		"gitlab.com/owner/repo",
		"owner/repo",
		"github.com/owner",
	}

	for _, source := range tests {
		_, _, err := parseGitHubSource(source)
		if err == nil {
			t.Errorf("parseGitHubSource(%q) should fail", source)
		}
	}
}

func TestPluginNotInstalledError(t *testing.T) {
	err := &PluginNotInstalledError{
		PluginName: "azurerm",
		Source:     "github.com/jokarl/tfclassify-plugin-azurerm",
		Version:    "0.1.0",
	}

	errStr := err.Error()
	if !bytes.Contains([]byte(errStr), []byte("azurerm")) {
		t.Errorf("error message should contain plugin name: %s", errStr)
	}
	if !bytes.Contains([]byte(errStr), []byte("not installed")) {
		t.Errorf("error message should indicate not installed: %s", errStr)
	}
}
