package plugin

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestIsInstalledAtVersion(t *testing.T) {
	// Create a temp file to test "installed" case
	tmpFile, err := os.CreateTemp("", "tfclassify-plugin-test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Test that existing file returns true
	if !isInstalledAtVersion(tmpFile.Name(), "1.0.0") {
		t.Error("expected isInstalledAtVersion to return true for existing file")
	}

	// Test that non-existent file returns false
	if isInstalledAtVersion("/nonexistent/path/plugin", "1.0.0") {
		t.Error("expected isInstalledAtVersion to return false for non-existent file")
	}
}

func TestInstallPlugins_AlreadyInstalled(t *testing.T) {
	// Create a temp directory and file to simulate installed plugin
	tmpDir, err := os.MkdirTemp("", "tfclassify-plugins")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a fake plugin binary
	pluginPath := filepath.Join(tmpDir, "tfclassify-plugin-test")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write plugin binary: %v", err)
	}

	// Set TFCLASSIFY_PLUGIN_DIR env var
	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	os.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir)
	defer os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv)

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:    "test",
				Enabled: true,
				Source:  "github.com/test/repo",
				Version: "1.0.0",
			},
		},
	}

	var buf bytes.Buffer
	err = InstallPlugins(cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("already installed")) {
		t.Errorf("expected already installed message, got: %s", buf.String())
	}
}

func TestDefaultPluginDir(t *testing.T) {
	// Test with env var set
	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	os.Setenv("TFCLASSIFY_PLUGIN_DIR", "/custom/plugin/dir")
	defer os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv)

	result := DefaultPluginDir()
	if result != "/custom/plugin/dir" {
		t.Errorf("expected /custom/plugin/dir, got %s", result)
	}

	// Test without env var
	os.Unsetenv("TFCLASSIFY_PLUGIN_DIR")
	result = DefaultPluginDir()
	cwd, _ := os.Getwd()
	expected := filepath.Join(cwd, ".tfclassify", "plugins")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}
