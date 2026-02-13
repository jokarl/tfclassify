package plugin

import (
	"archive/zip"
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

func TestExtractBinaryFromZip(t *testing.T) {
	// Create a temp directory for our test
	tmpDir, err := os.MkdirTemp("", "tfclassify-zip-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a zip file with a test binary
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(destDir, 0755)

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}

	zipWriter := zip.NewWriter(zipFile)

	// Add a binary file to the zip
	binaryContent := []byte("#!/bin/sh\necho hello")
	writer, err := zipWriter.Create("tfclassify-plugin-test")
	if err != nil {
		t.Fatalf("failed to create file in zip: %v", err)
	}
	if _, err := writer.Write(binaryContent); err != nil {
		t.Fatalf("failed to write to zip: %v", err)
	}

	zipWriter.Close()
	zipFile.Close()

	// Test extraction
	err = extractBinaryFromZip(zipPath, "tfclassify-plugin-test", destDir)
	if err != nil {
		t.Fatalf("extractBinaryFromZip failed: %v", err)
	}

	// Verify the extracted file exists
	extractedPath := filepath.Join(destDir, "tfclassify-plugin-test")
	if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
		t.Error("expected extracted binary to exist")
	}

	// Verify the content
	content, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if !bytes.Equal(content, binaryContent) {
		t.Errorf("extracted content mismatch: got %q, want %q", content, binaryContent)
	}
}

func TestExtractBinaryFromZip_BinaryNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tfclassify-zip-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a zip file without the expected binary
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(destDir, 0755)

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}

	zipWriter := zip.NewWriter(zipFile)

	// Add a different file to the zip
	writer, err := zipWriter.Create("other-file")
	if err != nil {
		t.Fatalf("failed to create file in zip: %v", err)
	}
	writer.Write([]byte("content"))

	zipWriter.Close()
	zipFile.Close()

	// Test extraction - should fail
	err = extractBinaryFromZip(zipPath, "tfclassify-plugin-missing", destDir)
	if err == nil {
		t.Error("expected error when binary not found in zip")
	}
}

func TestExtractBinaryFromZip_InvalidZip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tfclassify-zip-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an invalid zip file
	invalidZipPath := filepath.Join(tmpDir, "invalid.zip")
	os.WriteFile(invalidZipPath, []byte("not a zip file"), 0644)

	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(destDir, 0755)

	err = extractBinaryFromZip(invalidZipPath, "tfclassify-plugin-test", destDir)
	if err == nil {
		t.Error("expected error for invalid zip file")
	}
}

func TestExtractBinaryFromZip_BinaryInSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tfclassify-zip-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a zip file with binary in subdirectory
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(destDir, 0755)

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}

	zipWriter := zip.NewWriter(zipFile)

	// Add a binary file in a subdirectory
	binaryContent := []byte("#!/bin/sh\necho from subdir")
	writer, err := zipWriter.Create("some/subdir/tfclassify-plugin-test")
	if err != nil {
		t.Fatalf("failed to create file in zip: %v", err)
	}
	if _, err := writer.Write(binaryContent); err != nil {
		t.Fatalf("failed to write to zip: %v", err)
	}

	zipWriter.Close()
	zipFile.Close()

	// Test extraction - should work because we use filepath.Base
	err = extractBinaryFromZip(zipPath, "tfclassify-plugin-test", destDir)
	if err != nil {
		t.Fatalf("extractBinaryFromZip failed: %v", err)
	}

	// Verify the extracted file exists
	extractedPath := filepath.Join(destDir, "tfclassify-plugin-test")
	if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
		t.Error("expected extracted binary to exist")
	}
}
