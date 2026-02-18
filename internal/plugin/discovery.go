// Package plugin provides plugin discovery and lifecycle management.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jokarl/tfclassify/internal/config"
)

// PluginBinaryPrefix is the prefix for plugin binary names.
const PluginBinaryPrefix = "tfclassify-plugin-"

// PluginNotInstalledError indicates an enabled external plugin is not installed.
type PluginNotInstalledError struct {
	PluginName string
	Source     string
	Version    string
}

func (e *PluginNotInstalledError) Error() string {
	return fmt.Sprintf("plugin %q is not installed (source: %s, version: %s)", e.PluginName, e.Source, e.Version)
}

// DiscoveredPlugin contains information about a discovered plugin.
type DiscoveredPlugin struct {
	Name      string
	Path      string
	PluginCfg *config.PluginConfig
}

// DiscoverPlugins finds plugin binaries for each enabled plugin in the config.
// Returns a map from plugin name to discovered plugin info.
func DiscoverPlugins(cfg *config.Config, selfPath string) (map[string]*DiscoveredPlugin, error) {
	result := make(map[string]*DiscoveredPlugin)

	for i := range cfg.Plugins {
		pluginCfg := &cfg.Plugins[i]
		if !pluginCfg.Enabled {
			continue
		}

		discovered, err := discoverPlugin(pluginCfg, cfg, selfPath)
		if err != nil {
			return nil, err
		}

		result[pluginCfg.Name] = discovered
	}

	return result, nil
}

// discoverPlugin finds the binary for a single plugin.
func discoverPlugin(pluginCfg *config.PluginConfig, cfg *config.Config, selfPath string) (*DiscoveredPlugin, error) {
	// Search for external plugin binary
	binaryName := PluginBinaryPrefix + pluginCfg.Name
	paths := searchPaths()

	for _, dir := range paths {
		candidate := filepath.Join(dir, binaryName)
		if _, err := os.Stat(candidate); err == nil {
			return &DiscoveredPlugin{
				Name:      pluginCfg.Name,
				Path:      candidate,
				PluginCfg: pluginCfg,
			}, nil
		}
	}

	return nil, &PluginNotInstalledError{
		PluginName: pluginCfg.Name,
		Source:     pluginCfg.Source,
		Version:    pluginCfg.Version,
	}
}

// searchPaths returns the ordered list of directories to search for plugins.
// Search order:
// 1. TFCLASSIFY_PLUGIN_DIR environment variable
// 2. .tfclassify/plugins/ in current directory
// 3. ~/.tfclassify/plugins/
func searchPaths() []string {
	var paths []string

	// Environment variable
	if envDir := os.Getenv("TFCLASSIFY_PLUGIN_DIR"); envDir != "" {
		paths = append(paths, envDir)
	}

	// Current directory
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".tfclassify", "plugins"))
	}

	// Home directory
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".tfclassify", "plugins"))
	}

	return paths
}

// DefaultPluginDir returns the default plugin installation directory.
// Uses TFCLASSIFY_PLUGIN_DIR env var if set, otherwise .tfclassify/plugins/ in cwd.
func DefaultPluginDir() string {
	if envDir := os.Getenv("TFCLASSIFY_PLUGIN_DIR"); envDir != "" {
		return envDir
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, ".tfclassify", "plugins")
	}
	return ".tfclassify/plugins"
}
