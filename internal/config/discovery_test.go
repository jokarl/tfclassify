package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscover_ExplicitPath(t *testing.T) {
	path, err := Discover("testdata/valid.hcl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != "testdata/valid.hcl" {
		t.Errorf("expected testdata/valid.hcl, got %s", path)
	}
}

func TestDiscover_ExplicitPathNotFound(t *testing.T) {
	_, err := Discover("testdata/nonexistent.hcl")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}

	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("expected error about file not found, got: %v", err)
	}
}

func TestDiscover_CurrentDir(t *testing.T) {
	// Create a temp directory and config file
	tmpDir, err := os.MkdirTemp("", "tfclassify-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configPath := filepath.Join(tmpDir, ConfigFileName)
	if err := os.WriteFile(configPath, []byte("# test config"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	path, err := Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(path, ConfigFileName) {
		t.Errorf("expected path to end with %s, got %s", ConfigFileName, path)
	}
}

func TestDiscover_NotFound(t *testing.T) {
	// Create a temp directory without a config file
	tmpDir, err := os.MkdirTemp("", "tfclassify-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Temporarily set HOME to the temp directory (so home directory lookup also fails)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	_, err = Discover("")
	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}

	if !strings.Contains(err.Error(), "no configuration file found") {
		t.Errorf("expected error about no config found, got: %v", err)
	}

	if !strings.Contains(err.Error(), "searched paths") {
		t.Errorf("expected error to mention searched paths, got: %v", err)
	}
}
