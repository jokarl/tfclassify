// Package plugin provides plugin discovery and lifecycle management.
package plugin

import (
	"archive/zip"
	"context"
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
func InstallPlugins(ctx context.Context, cfg *config.Config, w io.Writer) error {
	pluginDir := DefaultPluginDir()

	// Load existing manifest
	manifest, err := loadManifest(pluginDir)
	if err != nil {
		return fmt.Errorf("failed to load plugin manifest: %w", err)
	}

	manifestChanged := false

	for _, p := range cfg.Plugins {
		if p.Source == "" {
			_, _ = fmt.Fprintf(w, "  %s: builtin (skip)\n", p.Name)
			continue
		}

		if !p.Enabled {
			_, _ = fmt.Fprintf(w, "  %s: disabled (skip)\n", p.Name)
			continue
		}

		binaryName := PluginBinaryPrefix + p.Name
		binaryPath := filepath.Join(pluginDir, binaryName)

		if isInstalledAtVersion(binaryPath, p.Name, p.Version, manifest) {
			_, _ = fmt.Fprintf(w, "  %s: already installed (v%s)\n", p.Name, p.Version)
			continue
		}

		// Check if a different version is installed
		if installedVersion, exists := manifest.Plugins[p.Name]; exists {
			_, _ = fmt.Fprintf(w, "  %s: upgrading from v%s to v%s...\n", p.Name, installedVersion, p.Version)
		} else {
			_, _ = fmt.Fprintf(w, "  %s: installing v%s from %s...\n", p.Name, p.Version, p.Source)
		}

		if err := downloadAndInstall(ctx, p.Name, p.Source, p.Version, pluginDir); err != nil {
			return fmt.Errorf("failed to install plugin %q: %w", p.Name, err)
		}

		// Update manifest with new version
		manifest.Plugins[p.Name] = p.Version
		manifestChanged = true
		_, _ = fmt.Fprintf(w, "  %s: installed\n", p.Name)
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

// githubRelease represents a GitHub release from the API.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

// githubAsset represents an asset within a GitHub release.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// httpClient is the HTTP client used for GitHub API and download requests.
// It can be overridden in tests.
var httpClient = &http.Client{}

// githubAPIBase is the base URL for the GitHub API. Overridden in tests.
var githubAPIBase = "https://api.github.com"

// fetchRelease queries the GitHub Releases API for a specific tag and returns the release metadata.
func fetchRelease(ctx context.Context, owner, repo, tag string) (*githubRelease, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", githubAPIBase, owner, repo, tag)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query GitHub Releases API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release %q not found in %s/%s", tag, owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d for release %q", resp.StatusCode, tag)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release response: %w", err)
	}

	return &release, nil
}

// findAssetURL searches a release's assets for the expected plugin zip and returns its download URL.
func findAssetURL(release *githubRelease, assetName string) (string, error) {
	for _, a := range release.Assets {
		if a.Name == assetName {
			return a.BrowserDownloadURL, nil
		}
	}

	available := make([]string, 0, len(release.Assets))
	for _, a := range release.Assets {
		available = append(available, a.Name)
	}
	return "", fmt.Errorf("asset %q not found in release %q (available: %v)", assetName, release.TagName, available)
}

// downloadAndInstall downloads a plugin from GitHub releases and installs it.
func downloadAndInstall(ctx context.Context, name, source, version, pluginDir string) error {
	owner, repo, err := parseGitHubSource(source)
	if err != nil {
		return err
	}

	tag := resolveReleaseTag(name, repo, version)
	assetName := fmt.Sprintf("tfclassify-plugin-%s_%s_%s_%s.zip", name, version, runtime.GOOS, runtime.GOARCH)

	// Query the GitHub Releases API for this tag
	release, err := fetchRelease(ctx, owner, repo, tag)
	if err != nil {
		return fmt.Errorf("failed to find release: %w", err)
	}

	// Find the matching asset
	assetURL, err := findAssetURL(release, assetName)
	if err != nil {
		return fmt.Errorf("failed to find plugin asset: %w", err)
	}

	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	tempFile, err := downloadFile(ctx, assetURL)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}
	defer func() { _ = os.Remove(tempFile) }()

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
func downloadFile(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	// Add GitHub token for authentication if available
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	req.Header.Set("Accept", "application/octet-stream")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = tempFile.Close() }()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		_ = os.Remove(tempFile.Name())
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
	defer func() { _ = reader.Close() }()

	for _, file := range reader.File {
		// Look for the plugin binary (may be at root or in a subdirectory)
		fileName := filepath.Base(file.Name)
		if fileName == binaryName || fileName == binaryName+".exe" {
			rc, err := file.Open()
			if err != nil {
				return err
			}
			defer func() { _ = rc.Close() }()

			destPath := filepath.Join(destDir, fileName)
			dest, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer func() { _ = dest.Close() }()

			if _, err := io.Copy(dest, rc); err != nil {
				return err
			}

			return nil
		}
	}

	return fmt.Errorf("binary %q not found in zip archive", binaryName)
}
