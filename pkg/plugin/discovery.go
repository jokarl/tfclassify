// Package plugin provides plugin discovery and lifecycle management.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jokarl/tfclassify/pkg/config"
)

// PluginBinaryPrefix is the prefix for plugin binary names.
const PluginBinaryPrefix = "tfclassify-plugin-"

// DiscoveredPlugin contains information about a discovered plugin.
type DiscoveredPlugin struct {
	Name      string
	Path      string
	IsBundled bool
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
			return nil, fmt.Errorf("failed to discover plugin %q: %w", pluginCfg.Name, err)
		}

		result[pluginCfg.Name] = discovered
	}

	return result, nil
}

// discoverPlugin finds the binary for a single plugin.
func discoverPlugin(pluginCfg *config.PluginConfig, cfg *config.Config, selfPath string) (*DiscoveredPlugin, error) {
	// Special case: bundled terraform plugin
	if pluginCfg.Name == "terraform" && pluginCfg.Source == "" {
		return &DiscoveredPlugin{
			Name:      pluginCfg.Name,
			Path:      selfPath,
			IsBundled: true,
			PluginCfg: pluginCfg,
		}, nil
	}

	// Search for external plugin binary
	binaryName := PluginBinaryPrefix + pluginCfg.Name
	paths := searchPaths()

	for _, dir := range paths {
		candidate := filepath.Join(dir, binaryName)
		if _, err := os.Stat(candidate); err == nil {
			return &DiscoveredPlugin{
				Name:      pluginCfg.Name,
				Path:      candidate,
				IsBundled: false,
				PluginCfg: pluginCfg,
			}, nil
		}
	}

	return nil, fmt.Errorf("plugin binary %q not found in search paths: %v", binaryName, paths)
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
