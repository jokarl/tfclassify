package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
)

func TestDiscoverPlugins_SourcelessPluginDiscovered(t *testing.T) {
	// Plugins without source should still be discovered if binary exists.
	tmpDir, err := os.MkdirTemp("", "tfclassify-plugins")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pluginPath := filepath.Join(tmpDir, "tfclassify-plugin-test")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write plugin binary: %v", err)
	}

	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	os.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir)
	defer os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv)

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "test", Enabled: true}, // no source, but binary exists
		},
	}

	discovered, err := DiscoverPlugins(cfg, "/usr/bin/tfclassify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(discovered) != 1 {
		t.Errorf("expected 1 plugin discovered, got %d", len(discovered))
	}
}

func TestDiscoverPlugins_SourcelessPluginNotFound(t *testing.T) {
	// Plugins without source should return error if binary is not found.
	tmpDir, err := os.MkdirTemp("", "tfclassify-empty")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	os.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir)
	defer os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv)

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "test", Enabled: true}, // no source, no binary
		},
	}

	_, err = DiscoverPlugins(cfg, "/usr/bin/tfclassify")
	if err == nil {
		t.Fatal("expected error for plugin not found")
	}
}

func TestDiscoverPlugins_DisabledSkipped(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "terraform", Enabled: false},
		},
	}

	discovered, err := DiscoverPlugins(cfg, "/usr/bin/tfclassify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(discovered) != 0 {
		t.Errorf("expected no plugins discovered, got %d", len(discovered))
	}
}

func TestDiscoverPlugins_EnvVar(t *testing.T) {
	// Create temp directory with plugin binary
	tmpDir, err := os.MkdirTemp("", "tfclassify-plugins")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pluginPath := filepath.Join(tmpDir, "tfclassify-plugin-test")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write plugin binary: %v", err)
	}

	// Set environment variable
	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	os.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir)
	defer os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv)

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "test", Enabled: true, Source: "github.com/test/repo"},
		},
	}

	discovered, err := DiscoverPlugins(cfg, "/usr/bin/tfclassify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plugin, ok := discovered["test"]
	if !ok {
		t.Fatal("expected to find test plugin")
	}

	if plugin.Path != pluginPath {
		t.Errorf("expected path %s, got %s", pluginPath, plugin.Path)
	}
}

func TestDiscoverPlugins_NotFound(t *testing.T) {
	// Set environment variable to empty directory
	tmpDir, err := os.MkdirTemp("", "tfclassify-empty")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	os.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir)
	defer os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv)

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "nonexistent", Enabled: true, Source: "github.com/test/nonexistent"},
		},
	}

	_, err = DiscoverPlugins(cfg, "/usr/bin/tfclassify")
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
}

func TestDiscoverPlugins_Precedence(t *testing.T) {
	// Create two temp directories with plugin binaries
	dir1, err := os.MkdirTemp("", "tfclassify-plugins1")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir1)

	dir2, err := os.MkdirTemp("", "tfclassify-plugins2")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir2)

	// Create plugin in both directories
	pluginPath1 := filepath.Join(dir1, "tfclassify-plugin-test")
	pluginPath2 := filepath.Join(dir2, "tfclassify-plugin-test")
	if err := os.WriteFile(pluginPath1, []byte("#!/bin/sh\n# dir1"), 0755); err != nil {
		t.Fatalf("failed to write plugin binary: %v", err)
	}
	if err := os.WriteFile(pluginPath2, []byte("#!/bin/sh\n# dir2"), 0755); err != nil {
		t.Fatalf("failed to write plugin binary: %v", err)
	}

	// Set environment variable to dir1 (should take precedence)
	oldEnv := os.Getenv("TFCLASSIFY_PLUGIN_DIR")
	os.Setenv("TFCLASSIFY_PLUGIN_DIR", dir1)
	defer os.Setenv("TFCLASSIFY_PLUGIN_DIR", oldEnv)

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "test", Enabled: true, Source: "github.com/test/repo"},
		},
	}

	discovered, err := DiscoverPlugins(cfg, "/usr/bin/tfclassify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plugin := discovered["test"]
	if plugin.Path != pluginPath1 {
		t.Errorf("expected env var path to take precedence, got %s", plugin.Path)
	}
}
