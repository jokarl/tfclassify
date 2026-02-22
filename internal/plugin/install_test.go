package plugin

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
)

func TestInstallPlugins_BuiltinSkipped(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:    "terraform",
				Enabled: true,
				Source:  "", // builtin, no source
			},
		},
	}

	var buf bytes.Buffer
	err := InstallPlugins(cfg, &buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("builtin (skip)")) {
		t.Errorf("expected builtin skip message, got: %s", buf.String())
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
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Create manifest with the plugin version
	manifest := &Manifest{
		Plugins: map[string]string{
			"test": "1.0.0",
		},
	}

	// Test that existing file with matching version returns true
	if !isInstalledAtVersion(tmpFile.Name(), "test", "1.0.0", manifest) {
		t.Error("expected isInstalledAtVersion to return true for existing file with matching version")
	}

	// Test that existing file with different version returns false
	if isInstalledAtVersion(tmpFile.Name(), "test", "2.0.0", manifest) {
		t.Error("expected isInstalledAtVersion to return false for existing file with different version")
	}

	// Test that non-existent file returns false
	if isInstalledAtVersion("/nonexistent/path/plugin", "test", "1.0.0", manifest) {
		t.Error("expected isInstalledAtVersion to return false for non-existent file")
	}

	// Test that missing plugin in manifest returns false
	if isInstalledAtVersion(tmpFile.Name(), "other", "1.0.0", manifest) {
		t.Error("expected isInstalledAtVersion to return false for plugin not in manifest")
	}
}

func TestInstallPlugins_AlreadyInstalled(t *testing.T) {
	// Create a temp directory and file to simulate installed plugin
	tmpDir, err := os.MkdirTemp("", "tfclassify-plugins")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a fake plugin binary
	pluginPath := filepath.Join(tmpDir, "tfclassify-plugin-test")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write plugin binary: %v", err)
	}

	// Create a manifest with the plugin at the correct version
	manifest := &Manifest{Plugins: map[string]string{"test": "1.0.0"}}
	if err := saveManifest(tmpDir, manifest); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Set TFCLASSIFY_PLUGIN_DIR env var
	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	if err := os.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() { _ = os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv) }()

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
	if err := os.Setenv("TFCLASSIFY_PLUGIN_DIR", "/custom/plugin/dir"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() { _ = os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv) }()

	result := DefaultPluginDir()
	if result != "/custom/plugin/dir" {
		t.Errorf("expected /custom/plugin/dir, got %s", result)
	}

	// Test without env var
	_ = os.Unsetenv("TFCLASSIFY_PLUGIN_DIR")
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
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a zip file with a test binary
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "dest")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}

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

	_ = zipWriter.Close()
	_ = zipFile.Close()

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
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a zip file without the expected binary
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "dest")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}

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
	_, _ = writer.Write([]byte("content"))

	_ = zipWriter.Close()
	_ = zipFile.Close()

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
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create an invalid zip file
	invalidZipPath := filepath.Join(tmpDir, "invalid.zip")
	if err := os.WriteFile(invalidZipPath, []byte("not a zip file"), 0644); err != nil {
		t.Fatalf("failed to write invalid zip: %v", err)
	}

	destDir := filepath.Join(tmpDir, "dest")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}

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
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a zip file with binary in subdirectory
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "dest")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}

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

	_ = zipWriter.Close()
	_ = zipFile.Close()

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

func TestExtractBinaryFromZip_WindowsExe(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tfclassify-zip-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a zip file with .exe extension
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "dest")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}

	zipWriter := zip.NewWriter(zipFile)

	// Add a Windows binary file
	binaryContent := []byte("MZ...") // Fake Windows executable
	writer, err := zipWriter.Create("tfclassify-plugin-test.exe")
	if err != nil {
		t.Fatalf("failed to create file in zip: %v", err)
	}
	if _, err := writer.Write(binaryContent); err != nil {
		t.Fatalf("failed to write to zip: %v", err)
	}

	_ = zipWriter.Close()
	_ = zipFile.Close()

	// Test extraction - should find .exe variant
	err = extractBinaryFromZip(zipPath, "tfclassify-plugin-test", destDir)
	if err != nil {
		t.Fatalf("extractBinaryFromZip failed: %v", err)
	}

	// Verify the extracted file exists
	extractedPath := filepath.Join(destDir, "tfclassify-plugin-test.exe")
	if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
		t.Error("expected extracted binary.exe to exist")
	}
}

func TestDiscoverPlugins_ExternalPluginNotInstalled(t *testing.T) {
	// Create empty temp directory
	tmpDir, err := os.MkdirTemp("", "tfclassify-empty-plugins")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	if err := os.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() { _ = os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv) }()

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:    "external",
				Enabled: true,
				Source:  "github.com/test/tfclassify-plugin-external",
				Version: "1.0.0",
			},
		},
	}

	_, err = DiscoverPlugins(cfg, "/usr/bin/tfclassify")
	if err == nil {
		t.Fatal("expected error for missing external plugin")
	}

	// Verify it's a PluginNotInstalledError
	var notInstalledErr *PluginNotInstalledError
	if !errors.As(err, &notInstalledErr) {
		t.Errorf("expected PluginNotInstalledError, got %T: %v", err, err)
	}

	if notInstalledErr != nil {
		if notInstalledErr.PluginName != "external" {
			t.Errorf("expected plugin name 'external', got %q", notInstalledErr.PluginName)
		}
		if notInstalledErr.Source != "github.com/test/tfclassify-plugin-external" {
			t.Errorf("expected source 'github.com/test/tfclassify-plugin-external', got %q", notInstalledErr.Source)
		}
	}
}

func TestDiscoverPlugins_LocalDir(t *testing.T) {
	// Create temp directory as "current working directory"
	tmpDir, err := os.MkdirTemp("", "tfclassify-workdir")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Resolve symlinks (macOS /var -> /private/var) so paths match os.Getwd()
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	// Create .tfclassify/plugins/ directory
	pluginDir := filepath.Join(tmpDir, ".tfclassify", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	// Create a test plugin
	pluginPath := filepath.Join(pluginDir, "tfclassify-plugin-localtest")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write plugin binary: %v", err)
	}

	// Clear env var so it falls back to local dir
	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	_ = os.Unsetenv("TFCLASSIFY_PLUGIN_DIR")
	defer func() { _ = os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv) }()

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "localtest", Enabled: true, Source: "github.com/test/localtest"},
		},
	}

	discovered, err := DiscoverPlugins(cfg, "/usr/bin/tfclassify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plugin, ok := discovered["localtest"]
	if !ok {
		t.Fatal("expected to find localtest plugin")
	}

	if plugin.Path != pluginPath {
		t.Errorf("expected path %s, got %s", pluginPath, plugin.Path)
	}
}

func TestSearchPaths_Order(t *testing.T) {
	// Test that search paths are returned in correct order
	paths := searchPaths()

	// Should have at least 2 paths (cwd and home)
	if len(paths) < 2 {
		t.Errorf("expected at least 2 search paths, got %d", len(paths))
	}

	// When env var is set, it should be first
	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	if err := os.Setenv("TFCLASSIFY_PLUGIN_DIR", "/custom/path"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() { _ = os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv) }()

	paths = searchPaths()
	if len(paths) < 1 || paths[0] != "/custom/path" {
		t.Errorf("expected first path to be /custom/path, got %v", paths)
	}
}

func TestDefaultPluginDir_NoEnv(t *testing.T) {
	// Clear env var
	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	_ = os.Unsetenv("TFCLASSIFY_PLUGIN_DIR")
	defer func() { _ = os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv) }()

	dir := DefaultPluginDir()

	// Should end with .tfclassify/plugins
	if !strings.HasSuffix(dir, filepath.Join(".tfclassify", "plugins")) {
		t.Errorf("expected dir to end with .tfclassify/plugins, got %s", dir)
	}
}

func TestDownloadFile_Success(t *testing.T) {
	// Create a test HTTP server
	binaryContent := []byte("#!/bin/sh\necho test binary")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(binaryContent)
	}))
	defer server.Close()

	// Download the file
	tempPath, err := downloadFile(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = os.Remove(tempPath) }()

	// Verify the content
	content, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if !bytes.Equal(content, binaryContent) {
		t.Errorf("content mismatch: got %q, want %q", content, binaryContent)
	}
}

func TestDownloadFile_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	_, err := downloadFile(server.URL)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to contain 404, got: %v", err)
	}
}

func TestDownloadFile_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := downloadFile(server.URL)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain 500, got: %v", err)
	}
}

func TestDownloadFile_WithGitHubToken(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content"))
	}))
	defer server.Close()

	// Set GITHUB_TOKEN
	oldToken := os.Getenv("GITHUB_TOKEN")
	if err := os.Setenv("GITHUB_TOKEN", "test-token-12345"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() { _ = os.Setenv("GITHUB_TOKEN", oldToken) }()

	tempPath, err := downloadFile(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = os.Remove(tempPath) }()

	if receivedAuth != "token test-token-12345" {
		t.Errorf("expected auth header 'token test-token-12345', got %q", receivedAuth)
	}
}

func TestDownloadAndInstall_InvalidSource(t *testing.T) {
	// Test with invalid source format
	err := downloadAndInstall("test", "invalid-source", "1.0.0", "/tmp")
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
	if !strings.Contains(err.Error(), "github.com") {
		t.Errorf("expected error to mention github.com, got: %v", err)
	}
}

func TestFetchRelease_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/tags/v1.0.0" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("expected Accept header, got %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"tag_name": "v1.0.0",
			"assets": [
				{"name": "plugin_1.0.0_linux_amd64.zip", "browser_download_url": "https://example.com/plugin.zip"},
				{"name": "checksums.txt", "browser_download_url": "https://example.com/checksums.txt"}
			]
		}`)
	}))
	defer server.Close()

	origBase := githubAPIBase
	githubAPIBase = server.URL
	defer func() { githubAPIBase = origBase }()

	release, err := fetchRelease("owner", "repo", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release.TagName != "v1.0.0" {
		t.Errorf("expected tag v1.0.0, got %s", release.TagName)
	}
	if len(release.Assets) != 2 {
		t.Errorf("expected 2 assets, got %d", len(release.Assets))
	}
}

func TestFetchRelease_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	origBase := githubAPIBase
	githubAPIBase = server.URL
	defer func() { githubAPIBase = origBase }()

	_, err := fetchRelease("nobody", "nonexistent", "v99.99.99")
	if err == nil {
		t.Fatal("expected error for missing release")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to mention 'not found', got: %v", err)
	}
}

func TestFetchRelease_WithGitHubToken(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name": "v1.0.0", "assets": []}`)
	}))
	defer server.Close()

	origBase := githubAPIBase
	githubAPIBase = server.URL
	defer func() { githubAPIBase = origBase }()

	oldToken := os.Getenv("GITHUB_TOKEN")
	if err := os.Setenv("GITHUB_TOKEN", "test-api-token"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() { _ = os.Setenv("GITHUB_TOKEN", oldToken) }()

	_, err := fetchRelease("owner", "repo", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedAuth != "token test-api-token" {
		t.Errorf("expected auth header 'token test-api-token', got %q", receivedAuth)
	}
}

func TestFetchRelease_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	origBase := githubAPIBase
	githubAPIBase = server.URL
	defer func() { githubAPIBase = origBase }()

	_, err := fetchRelease("owner", "repo", "v1.0.0")
	if err == nil {
		t.Fatal("expected error for server error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention 500, got: %v", err)
	}
}

func TestFindAssetURL_Found(t *testing.T) {
	release := &githubRelease{
		TagName: "v1.0.0",
		Assets: []githubAsset{
			{Name: "plugin_1.0.0_linux_amd64.zip", BrowserDownloadURL: "https://example.com/linux.zip"},
			{Name: "plugin_1.0.0_darwin_arm64.zip", BrowserDownloadURL: "https://example.com/darwin.zip"},
		},
	}

	url, err := findAssetURL(release, "plugin_1.0.0_darwin_arm64.zip")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com/darwin.zip" {
		t.Errorf("expected darwin URL, got %s", url)
	}
}

func TestFindAssetURL_NotFound(t *testing.T) {
	release := &githubRelease{
		TagName: "v1.0.0",
		Assets: []githubAsset{
			{Name: "plugin_1.0.0_linux_amd64.zip", BrowserDownloadURL: "https://example.com/linux.zip"},
		},
	}

	_, err := findAssetURL(release, "plugin_1.0.0_windows_amd64.zip")
	if err == nil {
		t.Fatal("expected error for missing asset")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "plugin_1.0.0_linux_amd64.zip") {
		t.Errorf("expected available assets listed in error, got: %v", err)
	}
}

func TestDownloadAndInstall_EndToEnd(t *testing.T) {
	// Create a zip containing a fake plugin binary
	binaryContent := []byte("#!/bin/sh\necho hello")
	assetName := fmt.Sprintf("tfclassify-plugin-test_1.0.0_%s_%s.zip", runtime.GOOS, runtime.GOARCH)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, _ := zw.Create("tfclassify-plugin-test")
	_, _ = w.Write(binaryContent)
	_ = zw.Close()
	zipBytes := zipBuf.Bytes()

	// Mock server handles both API and download requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/"):
			// GitHub Releases API
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"tag_name": "v1.0.0",
				"assets": [{
					"name": %q,
					"browser_download_url": "%s/download/%s"
				}]
			}`, assetName, "PLACEHOLDER", assetName)
		case strings.HasPrefix(r.URL.Path, "/download/"):
			// Asset download
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(zipBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Patch the API response to include the real server URL for download
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"tag_name": "v1.0.0",
				"assets": [{
					"name": %q,
					"browser_download_url": "%s/download/%s"
				}]
			}`, assetName, server.URL, assetName)
		case strings.HasPrefix(r.URL.Path, "/download/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(zipBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server2.Close()

	origBase := githubAPIBase
	githubAPIBase = server2.URL
	defer func() { githubAPIBase = origBase }()

	tmpDir, err := os.MkdirTemp("", "tfclassify-e2e-install")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	err = downloadAndInstall("test", "github.com/owner/tfclassify-plugin-test", "1.0.0", tmpDir)
	if err != nil {
		t.Fatalf("downloadAndInstall failed: %v", err)
	}

	// Verify the binary was extracted
	extractedPath := filepath.Join(tmpDir, "tfclassify-plugin-test")
	content, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("failed to read extracted binary: %v", err)
	}
	if !bytes.Equal(content, binaryContent) {
		t.Errorf("binary content mismatch: got %q, want %q", content, binaryContent)
	}
}

func TestInstallPlugins_DownloadFailure(t *testing.T) {
	// Mock server returns 404 for any release lookup
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	origBase := githubAPIBase
	githubAPIBase = server.URL
	defer func() { githubAPIBase = origBase }()

	tmpDir, err := os.MkdirTemp("", "tfclassify-install-fail")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	if err := os.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() { _ = os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv) }()

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:    "nonexistent",
				Enabled: true,
				Source:  "github.com/nobody/nonexistent-plugin",
				Version: "99.99.99",
			},
		},
	}

	var buf bytes.Buffer
	err = InstallPlugins(cfg, &buf)
	if err == nil {
		t.Fatal("expected error for failed plugin install")
	}
	if !strings.Contains(err.Error(), "failed to install") {
		t.Errorf("expected error to mention 'failed to install', got: %v", err)
	}
}

func TestInstallPlugins_MultiplePlugins(t *testing.T) {
	// Create temp directory with one already installed plugin
	tmpDir, err := os.MkdirTemp("", "tfclassify-multi")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Pre-install one plugin
	pluginPath := filepath.Join(tmpDir, "tfclassify-plugin-existing")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write plugin: %v", err)
	}

	// Create a manifest with the plugin at the correct version
	manifest := &Manifest{Plugins: map[string]string{"existing": "1.0.0"}}
	if err := saveManifest(tmpDir, manifest); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	if err := os.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() { _ = os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv) }()

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "builtin", Enabled: true, Source: ""}, // builtin, no source
			{Name: "disabled", Enabled: false, Source: "github.com/test/disabled", Version: "1.0.0"}, // disabled
			{Name: "existing", Enabled: true, Source: "github.com/test/existing", Version: "1.0.0"}, // already installed
		},
	}

	var buf bytes.Buffer
	err = InstallPlugins(cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "builtin: builtin (skip)") {
		t.Errorf("expected builtin skip message in output")
	}
	if !strings.Contains(output, "disabled: disabled (skip)") {
		t.Errorf("expected disabled skip message in output")
	}
	if !strings.Contains(output, "existing: already installed") {
		t.Errorf("expected already installed message in output")
	}
}

func TestPluginBinaryPrefix(t *testing.T) {
	if PluginBinaryPrefix != "tfclassify-plugin-" {
		t.Errorf("expected PluginBinaryPrefix to be 'tfclassify-plugin-', got %q", PluginBinaryPrefix)
	}
}

func TestAssetNaming(t *testing.T) {
	// Verify the asset naming convention
	name := "test"
	version := "1.0.0"
	expectedAsset := "tfclassify-plugin-" + name + "_" + version + "_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip"

	// This tests that the asset name follows the expected convention
	// as documented in CR-0009
	if !strings.HasPrefix(expectedAsset, "tfclassify-plugin-test_1.0.0_") {
		t.Errorf("unexpected asset name format: %s", expectedAsset)
	}
	if !strings.HasSuffix(expectedAsset, ".zip") {
		t.Errorf("expected asset to end with .zip: %s", expectedAsset)
	}
}

func TestManifest_LoadSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tfclassify-manifest")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Test loading non-existent manifest returns empty manifest
	manifest, err := loadManifest(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error loading non-existent manifest: %v", err)
	}
	if len(manifest.Plugins) != 0 {
		t.Errorf("expected empty plugins map, got %v", manifest.Plugins)
	}

	// Test saving and loading manifest
	manifest.Plugins["test"] = "1.0.0"
	manifest.Plugins["other"] = "2.0.0"
	if err := saveManifest(tmpDir, manifest); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Load it back
	loaded, err := loadManifest(tmpDir)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	if loaded.Plugins["test"] != "1.0.0" {
		t.Errorf("expected test version 1.0.0, got %s", loaded.Plugins["test"])
	}
	if loaded.Plugins["other"] != "2.0.0" {
		t.Errorf("expected other version 2.0.0, got %s", loaded.Plugins["other"])
	}
}

func TestManifest_LoadInvalid(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tfclassify-manifest-invalid")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Write invalid JSON
	manifestPath := filepath.Join(tmpDir, ManifestFileName)
	if err := os.WriteFile(manifestPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write invalid manifest: %v", err)
	}

	_, err = loadManifest(tmpDir)
	if err == nil {
		t.Error("expected error loading invalid manifest")
	}
}

func TestManifest_SaveCreatesDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tfclassify-manifest-create")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Try to save to a non-existent subdirectory
	subDir := filepath.Join(tmpDir, "newdir", "plugins")
	manifest := &Manifest{Plugins: map[string]string{"test": "1.0.0"}}
	if err := saveManifest(subDir, manifest); err != nil {
		t.Fatalf("failed to save manifest to new dir: %v", err)
	}

	// Verify it was created
	if _, err := os.Stat(filepath.Join(subDir, ManifestFileName)); os.IsNotExist(err) {
		t.Error("expected manifest file to exist")
	}
}

func TestInstallPlugins_VersionUpgrade(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho upgraded")
	assetName := fmt.Sprintf("tfclassify-plugin-test_2.0.0_%s_%s.zip", runtime.GOOS, runtime.GOARCH)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, _ := zw.Create("tfclassify-plugin-test")
	_, _ = w.Write(binaryContent)
	_ = zw.Close()
	zipBytes := zipBuf.Bytes()

	// Mock server serves both API and download
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/"):
			rw.Header().Set("Content-Type", "application/json")
			// The download URL points back to this same server
			downloadURL := fmt.Sprintf("http://%s/download/%s", r.Host, assetName)
			_, _ = fmt.Fprintf(rw, `{"tag_name":"tfclassify-plugin-test-v2.0.0","assets":[{"name":%q,"browser_download_url":%q}]}`, assetName, downloadURL)
		case strings.HasPrefix(r.URL.Path, "/download/"):
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write(zipBytes)
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	origBase := githubAPIBase
	githubAPIBase = server.URL
	defer func() { githubAPIBase = origBase }()

	tmpDir, err := os.MkdirTemp("", "tfclassify-upgrade")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Pre-install plugin at old version
	pluginPath := filepath.Join(tmpDir, "tfclassify-plugin-test")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write plugin: %v", err)
	}

	manifest := &Manifest{Plugins: map[string]string{"test": "1.0.0"}}
	if err := saveManifest(tmpDir, manifest); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	if err := os.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() { _ = os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv) }()

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:    "test",
				Enabled: true,
				Source:  "github.com/test/repo",
				Version: "2.0.0",
			},
		},
	}

	var buf bytes.Buffer
	err = InstallPlugins(cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "upgrading from v1.0.0 to v2.0.0") {
		t.Errorf("expected upgrade message, got: %s", output)
	}
	if !strings.Contains(output, "installed") {
		t.Errorf("expected installed message, got: %s", output)
	}

	// Verify the binary was upgraded
	content, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("failed to read plugin binary: %v", err)
	}
	if !bytes.Equal(content, binaryContent) {
		t.Errorf("binary content mismatch after upgrade")
	}

	// Verify manifest was updated
	updatedManifest, err := loadManifest(tmpDir)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if updatedManifest.Plugins["test"] != "2.0.0" {
		t.Errorf("expected manifest version 2.0.0, got %s", updatedManifest.Plugins["test"])
	}
}

func TestResolveReleaseTag(t *testing.T) {
	tests := []struct {
		name    string
		plugin  string
		repo    string
		version string
		want    string
	}{
		{
			name:    "standalone repo",
			plugin:  "azurerm",
			repo:    "tfclassify-plugin-azurerm",
			version: "0.1.0",
			want:    "v0.1.0",
		},
		{
			name:    "monorepo",
			plugin:  "azurerm",
			repo:    "tfclassify",
			version: "0.1.0",
			want:    "tfclassify-plugin-azurerm-v0.1.0",
		},
		{
			name:    "monorepo different plugin",
			plugin:  "aws",
			repo:    "tfclassify",
			version: "2.3.4",
			want:    "tfclassify-plugin-aws-v2.3.4",
		},
		{
			name:    "standalone repo different plugin",
			plugin:  "aws",
			repo:    "tfclassify-plugin-aws",
			version: "1.0.0",
			want:    "v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveReleaseTag(tt.plugin, tt.repo, tt.version)
			if got != tt.want {
				t.Errorf("resolveReleaseTag(%q, %q, %q) = %q, want %q", tt.plugin, tt.repo, tt.version, got, tt.want)
			}
		})
	}
}

func TestDownloadURL_Monorepo(t *testing.T) {
	// Verify URL construction for monorepo source (repo != plugin binary name)
	name := "azurerm"
	source := "github.com/jokarl/tfclassify"
	version := "0.1.0"

	owner, repo, err := parseGitHubSource(source)
	if err != nil {
		t.Fatalf("parseGitHubSource(%q) error: %v", source, err)
	}

	tag := resolveReleaseTag(name, repo, version)
	assetName := fmt.Sprintf("tfclassify-plugin-%s_%s_%s_%s.zip", name, version, runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, assetName)

	expectedTag := "tfclassify-plugin-azurerm-v0.1.0"
	if tag != expectedTag {
		t.Errorf("tag = %q, want %q", tag, expectedTag)
	}

	expectedPrefix := "https://github.com/jokarl/tfclassify/releases/download/tfclassify-plugin-azurerm-v0.1.0/tfclassify-plugin-azurerm_0.1.0_"
	if !strings.HasPrefix(url, expectedPrefix) {
		t.Errorf("url = %q, want prefix %q", url, expectedPrefix)
	}
}

func TestDownloadURL_Standalone(t *testing.T) {
	// Verify URL construction for standalone repo (repo == plugin binary name)
	name := "azurerm"
	source := "github.com/jokarl/tfclassify-plugin-azurerm"
	version := "0.1.0"

	owner, repo, err := parseGitHubSource(source)
	if err != nil {
		t.Fatalf("parseGitHubSource(%q) error: %v", source, err)
	}

	tag := resolveReleaseTag(name, repo, version)
	assetName := fmt.Sprintf("tfclassify-plugin-%s_%s_%s_%s.zip", name, version, runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, assetName)

	expectedTag := "v0.1.0"
	if tag != expectedTag {
		t.Errorf("tag = %q, want %q", tag, expectedTag)
	}

	expectedPrefix := "https://github.com/jokarl/tfclassify-plugin-azurerm/releases/download/v0.1.0/tfclassify-plugin-azurerm_0.1.0_"
	if !strings.HasPrefix(url, expectedPrefix) {
		t.Errorf("url = %q, want prefix %q", url, expectedPrefix)
	}
}

func TestManifest_NilPluginsMap(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tfclassify-manifest-nil")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Write manifest with null plugins
	manifestPath := filepath.Join(tmpDir, ManifestFileName)
	if err := os.WriteFile(manifestPath, []byte(`{"plugins": null}`), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	manifest, err := loadManifest(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should initialize plugins map even if null in JSON
	if manifest.Plugins == nil {
		t.Error("expected plugins map to be initialized, got nil")
	}
}
