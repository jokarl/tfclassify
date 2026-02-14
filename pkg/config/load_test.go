package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_WithExplicitPath(t *testing.T) {
	cfg, err := Load("testdata/valid.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Classifications) != 3 {
		t.Errorf("expected 3 classifications, got %d", len(cfg.Classifications))
	}

	if cfg.Classifications[0].Name != "critical" {
		t.Errorf("expected first classification 'critical', got %q", cfg.Classifications[0].Name)
	}
}

func TestLoad_WithDiscovery(t *testing.T) {
	// Create a temp directory with a .tfclassify.hcl file
	tmpDir, err := os.MkdirTemp("", "tfclassify-load-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `
classification "standard" {
  description = "Standard"
  rule {
    resource = ["*"]
  }
}

precedence = ["standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
`
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory so Discover finds the config
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Classifications) != 1 {
		t.Errorf("expected 1 classification, got %d", len(cfg.Classifications))
	}
	if cfg.Classifications[0].Name != "standard" {
		t.Errorf("expected 'standard', got %q", cfg.Classifications[0].Name)
	}
}

func TestLoad_DiscoveryFails(t *testing.T) {
	// Create a temp directory with NO config file
	tmpDir, err := os.MkdirTemp("", "tfclassify-noconfig-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	_, err = Load("")
	if err == nil {
		t.Fatal("expected error when no config file found")
	}
	if !strings.Contains(err.Error(), "no configuration file found") {
		t.Errorf("expected 'no configuration file found' error, got: %v", err)
	}
}

func TestLoad_ExplicitPathNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.hcl")
	if err == nil {
		t.Fatal("expected error for nonexistent explicit path")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("expected 'config file not found' error, got: %v", err)
	}
}

func TestLoadFile_UnreadableFile(t *testing.T) {
	// Create a temp directory with an unreadable file
	tmpDir, err := os.MkdirTemp("", "tfclassify-unreadable-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "unreadable.hcl")
	if err := os.WriteFile(configPath, []byte("content"), 0000); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = LoadFile(configPath)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
	// Error could be a read error or a parse error depending on OS/user permissions
	errStr := err.Error()
	if !strings.Contains(errStr, "failed to read config file") && !strings.Contains(errStr, "configuration error") {
		t.Errorf("expected read or parse error, got: %v", err)
	}
}

func TestLoadFile_FileNotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/file.hcl")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("expected 'failed to read config file' error, got: %v", err)
	}
}

func TestParse_EmptyBytes(t *testing.T) {
	// Empty bytes should fail validation (no precedence, etc.)
	_, err := Parse([]byte(""), "empty.hcl")
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}

func TestDiscover_HomeDirectory(t *testing.T) {
	// Create a temp directory to use as HOME with a config file
	tmpHome, err := os.MkdirTemp("", "tfclassify-home-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Create another temp dir to use as CWD (without config)
	tmpCwd, err := os.MkdirTemp("", "tfclassify-cwd-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpCwd)

	// Put config in HOME
	configPath := filepath.Join(tmpHome, ConfigFileName)
	if err := os.WriteFile(configPath, []byte("# test"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to CWD without config
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpCwd)

	// Set HOME to our temp home
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	path, err := Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != configPath {
		t.Errorf("expected path %q, got %q", configPath, path)
	}
}
