// Package plugin provides plugin discovery and lifecycle management.
package plugin

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jokarl/tfclassify/internal/config"
)

// ManifestFileName is the name of the plugin manifest file.
const ManifestFileName = "manifest.json"

// Manifest tracks installed plugin versions.
type Manifest struct {
	Plugins map[string]string `json:"plugins"` // plugin name -> version
}

// loadManifest loads the manifest file from the plugin directory.
func loadManifest(pluginDir string) (*Manifest, error) {
	manifestPath := filepath.Join(pluginDir, ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Plugins: make(map[string]string)}, nil
		}
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}
	if m.Plugins == nil {
		m.Plugins = make(map[string]string)
	}
	return &m, nil
}

// saveManifest saves the manifest file to the plugin directory.
func saveManifest(pluginDir string, m *Manifest) error {
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	manifestPath := filepath.Join(pluginDir, ManifestFileName)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}
	return nil
}

// InstallPlugins downloads and installs external plugins declared in the config.
// Builtin plugins (no source) and disabled plugins are skipped.
func InstallPlugins(cfg *config.Config, w io.Writer) error {
	pluginDir := DefaultPluginDir()

	// Load existing manifest
	manifest, err := loadManifest(pluginDir)
	if err != nil {
		return fmt.Errorf("failed to load plugin manifest: %w", err)
	}

	manifestChanged := false

	for _, p := range cfg.Plugins {
		if p.Source == "" {
			fmt.Fprintf(w, "  %s: builtin (skip)\n", p.Name)
			continue
		}

		if !p.Enabled {
			fmt.Fprintf(w, "  %s: disabled (skip)\n", p.Name)
			continue
		}

		binaryName := PluginBinaryPrefix + p.Name
		binaryPath := filepath.Join(pluginDir, binaryName)

		if isInstalledAtVersion(binaryPath, p.Name, p.Version, manifest) {
			fmt.Fprintf(w, "  %s: already installed (v%s)\n", p.Name, p.Version)
			continue
		}

		// Check if a different version is installed
		if installedVersion, exists := manifest.Plugins[p.Name]; exists {
			fmt.Fprintf(w, "  %s: upgrading from v%s to v%s...\n", p.Name, installedVersion, p.Version)
		} else {
			fmt.Fprintf(w, "  %s: installing v%s from %s...\n", p.Name, p.Version, p.Source)
		}

		if err := downloadAndInstall(p.Name, p.Source, p.Version, pluginDir); err != nil {
			return fmt.Errorf("failed to install plugin %q: %w", p.Name, err)
		}

		// Update manifest with new version
		manifest.Plugins[p.Name] = p.Version
		manifestChanged = true
		fmt.Fprintf(w, "  %s: installed\n", p.Name)
	}

	// Save manifest if changed
	if manifestChanged {
		if err := saveManifest(pluginDir, manifest); err != nil {
			return fmt.Errorf("failed to save plugin manifest: %w", err)
		}
	}

	return nil
}

// isInstalledAtVersion checks if a plugin is installed at the requested version.
// It checks both the binary existence and the manifest version.
func isInstalledAtVersion(binaryPath, name, version string, manifest *Manifest) bool {
	// Check if binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		return false
	}

	// Check if manifest has this plugin at the correct version
	installedVersion, exists := manifest.Plugins[name]
	if !exists {
		return false
	}

	return installedVersion == version
}

// resolveReleaseTag returns the GitHub release tag for a plugin.
// If the repo name matches the plugin binary name (standalone repo), it uses "v{version}".
// Otherwise (monorepo), it uses "{binaryName}-v{version}".
func resolveReleaseTag(name, repo, version string) string {
	if repo == PluginBinaryPrefix+name {
		return "v" + version
	}
	return PluginBinaryPrefix + name + "-v" + version
}

// downloadAndInstall downloads a plugin from GitHub releases and installs it.
func downloadAndInstall(name, source, version, pluginDir string) error {
	// Parse the source to get owner/repo
	// Expected format: github.com/owner/repo
	owner, repo, err := parseGitHubSource(source)
	if err != nil {
		return err
	}

	// Construct the release asset URL
	tag := resolveReleaseTag(name, repo, version)
	assetName := fmt.Sprintf("tfclassify-plugin-%s_%s_%s_%s.zip", name, version, runtime.GOOS, runtime.GOARCH)
	assetURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, assetName)

	// Create plugin directory if it doesn't exist
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Download the asset
	tempFile, err := downloadFile(assetURL)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}
	defer os.Remove(tempFile)

	// Extract the binary from the zip
	binaryName := PluginBinaryPrefix + name
	if err := extractBinaryFromZip(tempFile, binaryName, pluginDir); err != nil {
		return fmt.Errorf("failed to extract plugin: %w", err)
	}

	return nil
}

// parseGitHubSource parses a source like "github.com/owner/repo" into owner and repo.
func parseGitHubSource(source string) (string, string, error) {
	// Remove https:// prefix if present
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")

	// Remove github.com/ prefix
	if !strings.HasPrefix(source, "github.com/") {
		return "", "", fmt.Errorf("source must be a GitHub repository (github.com/owner/repo), got: %s", source)
	}

	path := strings.TrimPrefix(source, "github.com/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub repository path: %s", source)
	}

	return parts[0], parts[1], nil
}

// downloadFile downloads a file from a URL and returns the path to a temp file.
func downloadFile(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// Add GitHub token for authentication if available
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read a limited amount of the response body for error context
		bodyBytes := make([]byte, 256)
		n, _ := io.ReadFull(resp.Body, bodyBytes)
		if n > 0 {
			bodyPreview := string(bodyBytes[:n])
			return "", fmt.Errorf("download failed with status %d from %s: %s", resp.StatusCode, url, bodyPreview)
		}
		return "", fmt.Errorf("download failed with status %d from %s", resp.StatusCode, url)
	}

	// Create temp file
	tempFile, err := os.CreateTemp("", "tfclassify-plugin-*.zip")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// extractBinaryFromZip extracts a binary from a zip file.
func extractBinaryFromZip(zipPath, binaryName, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		// Look for the plugin binary (may be at root or in a subdirectory)
		fileName := filepath.Base(file.Name)
		if fileName == binaryName || fileName == binaryName+".exe" {
			rc, err := file.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			destPath := filepath.Join(destDir, fileName)
			dest, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer dest.Close()

			if _, err := io.Copy(dest, rc); err != nil {
				return err
			}

			return nil
		}
	}

	return fmt.Errorf("binary %q not found in zip archive", binaryName)
}
