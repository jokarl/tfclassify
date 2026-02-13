// Package plugin provides plugin discovery and lifecycle management.
package plugin

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jokarl/tfclassify/pkg/config"
)

// InstallPlugins downloads and installs external plugins declared in the config.
// Bundled plugins and disabled plugins are skipped.
func InstallPlugins(cfg *config.Config, w io.Writer) error {
	pluginDir := DefaultPluginDir()

	for _, p := range cfg.Plugins {
		if p.Source == "" {
			fmt.Fprintf(w, "  %s: bundled (skip)\n", p.Name)
			continue
		}

		if !p.Enabled {
			fmt.Fprintf(w, "  %s: disabled (skip)\n", p.Name)
			continue
		}

		binaryName := PluginBinaryPrefix + p.Name
		binaryPath := filepath.Join(pluginDir, binaryName)

		if isInstalledAtVersion(binaryPath, p.Version) {
			fmt.Fprintf(w, "  %s: already installed (v%s)\n", p.Name, p.Version)
			continue
		}

		fmt.Fprintf(w, "  %s: installing v%s from %s...\n", p.Name, p.Version, p.Source)
		if err := downloadAndInstall(p.Name, p.Source, p.Version, pluginDir); err != nil {
			return fmt.Errorf("failed to install plugin %q: %w", p.Name, err)
		}
		fmt.Fprintf(w, "  %s: installed\n", p.Name)
	}

	return nil
}

// isInstalledAtVersion checks if a plugin binary exists at the given path.
// For now, we just check existence. Version tracking would require a manifest.
func isInstalledAtVersion(binaryPath, version string) bool {
	_, err := os.Stat(binaryPath)
	return err == nil
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
	assetName := fmt.Sprintf("tfclassify-plugin-%s_%s_%s_%s.zip", name, version, runtime.GOOS, runtime.GOARCH)
	assetURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/%s", owner, repo, version, assetName)

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
		return "", fmt.Errorf("download failed with status %d: %s", resp.StatusCode, url)
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
