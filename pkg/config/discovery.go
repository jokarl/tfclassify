// Package config provides HCL configuration loading for tfclassify.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigFileName is the default name for the configuration file.
const ConfigFileName = ".tfclassify.hcl"

// Discover finds the configuration file path.
// Search order:
// 1. Explicit path from --config flag (if provided)
// 2. .tfclassify.hcl in current working directory
// 3. .tfclassify.hcl in home directory
func Discover(explicitPath string) (string, error) {
	if explicitPath != "" {
		if _, err := os.Stat(explicitPath); err != nil {
			return "", fmt.Errorf("config file not found at %s: %w", explicitPath, err)
		}
		return explicitPath, nil
	}

	searchPaths := getSearchPaths()
	searchedPaths := make([]string, 0, len(searchPaths))

	for _, p := range searchPaths {
		searchedPaths = append(searchedPaths, p)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("no configuration file found; searched paths: %v", searchedPaths)
}

// getSearchPaths returns the ordered list of paths to search for config files.
func getSearchPaths() []string {
	paths := []string{}

	// Current working directory
	cwd, err := os.Getwd()
	if err == nil {
		paths = append(paths, filepath.Join(cwd, ConfigFileName))
	}

	// Home directory
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ConfigFileName))
	}

	return paths
}
