package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary builds the tfclassify binary and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	binaryPath := filepath.Join(tmpDir, "tfclassify")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = filepath.Join(testProjectRoot(t), "cmd", "tfclassify")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, output)
	}

	return binaryPath
}

// testProjectRoot returns the project root directory.
func testProjectRoot(t *testing.T) string {
	t.Helper()

	// Walk up from cmd/tfclassify to project root
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	// We're in cmd/tfclassify, so go up two levels
	return filepath.Join(dir, "..", "..")
}

// writeTestConfig creates a test configuration file and returns its path.
func writeTestConfig(t *testing.T, dir string) string {
	t.Helper()

	configContent := `
classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*", "*_iam_*"]
    actions  = ["delete"]
  }
}

classification "standard" {
  description = "Standard change process"

  rule {
    resource = ["*"]
  }
}

classification "auto" {
  description = "Automatic approval"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

precedence = ["critical", "standard", "auto"]

defaults {
  unclassified = "standard"
  no_changes   = "auto"
}
`
	configPath := filepath.Join(dir, ".tfclassify.hcl")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return configPath
}

// writeTestPlan creates a test plan JSON file and returns its path.
func writeTestPlan(t *testing.T, dir string) string {
	t.Helper()

	planContent := `{
  "format_version": "1.2",
  "terraform_version": "1.5.0",
  "resource_changes": [
    {
      "address": "azurerm_role_assignment.example",
      "mode": "managed",
      "type": "azurerm_role_assignment",
      "name": "example",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["delete"],
        "before": {"role_definition_name": "Contributor"},
        "after": null,
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_virtual_network.main",
      "mode": "managed",
      "type": "azurerm_virtual_network",
      "name": "main",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["update"],
        "before": {"name": "vnet-old"},
        "after": {"name": "vnet-new"},
        "before_sensitive": false,
        "after_sensitive": false
      }
    }
  ]
}`
	planPath := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatalf("failed to write test plan: %v", err)
	}
	return planPath
}

// writeEmptyPlan creates an empty plan JSON file (no resource changes).
func writeEmptyPlan(t *testing.T, dir string) string {
	t.Helper()

	planContent := `{
  "format_version": "1.2",
  "terraform_version": "1.5.0",
  "resource_changes": []
}`
	planPath := filepath.Join(dir, "empty_plan.json")
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatalf("failed to write empty plan: %v", err)
	}
	return planPath
}

func TestCLI_TextOutput(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)
	planPath := writeTestPlan(t, tmpDir)

	cmd := exec.Command(binary, "--plan", planPath, "--config", configPath, "--output", "text")
	output, _ := cmd.CombinedOutput()
	// We ignore the error because the CLI exits with non-zero exit code for non-auto classifications

	outputStr := string(output)

	if !strings.Contains(outputStr, "Classification: critical") {
		t.Errorf("expected text output to contain 'Classification: critical', got:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "Resources: 2") {
		t.Errorf("expected text output to contain 'Resources: 2', got:\n%s", outputStr)
	}
}

func TestCLI_JSONOutput(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)
	planPath := writeTestPlan(t, tmpDir)

	cmd := exec.Command(binary, "--plan", planPath, "--config", configPath, "--output", "json")
	output, _ := cmd.CombinedOutput()

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\nOutput:\n%s", err, string(output))
	}

	if result["overall"] != "critical" {
		t.Errorf("expected overall 'critical', got %v", result["overall"])
	}

	resources, ok := result["resources"].([]interface{})
	if !ok {
		t.Fatalf("expected resources to be array, got %T", result["resources"])
	}
	if len(resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(resources))
	}
}

func TestCLI_GitHubOutput(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)
	planPath := writeTestPlan(t, tmpDir)

	cmd := exec.Command(binary, "--plan", planPath, "--config", configPath, "--output", "github")
	output, _ := cmd.CombinedOutput()

	outputStr := string(output)
	if !strings.Contains(outputStr, "classification=critical") {
		t.Errorf("expected github output to contain 'classification=critical', got:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "resource_count=2") {
		t.Errorf("expected github output to contain 'resource_count=2', got:\n%s", outputStr)
	}
}

func TestCLI_VerboseOutput(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)
	planPath := writeTestPlan(t, tmpDir)

	cmd := exec.Command(binary, "--plan", planPath, "--config", configPath, "--verbose")
	output, _ := cmd.CombinedOutput()

	outputStr := string(output)
	// Verbose mode should include classification descriptions and rule details
	if !strings.Contains(outputStr, "critical") {
		t.Errorf("expected verbose output to contain 'critical', got:\n%s", outputStr)
	}
}

func TestCLI_EmptyPlan(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)
	planPath := writeEmptyPlan(t, tmpDir)

	cmd := exec.Command(binary, "--plan", planPath, "--config", configPath, "--output", "json")
	output, _ := cmd.CombinedOutput()

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\nOutput:\n%s", err, string(output))
	}

	if result["overall"] != "auto" {
		t.Errorf("expected overall 'auto' for empty plan, got %v", result["overall"])
	}
	if result["no_changes"] != true {
		t.Errorf("expected no_changes true, got %v", result["no_changes"])
	}
}

func TestCLI_NonexistentPlan(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)

	cmd := exec.Command(binary, "--plan", "/nonexistent/plan.json", "--config", configPath)
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for nonexistent plan")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "failed to parse plan") {
		t.Errorf("expected error about parsing plan, got:\n%s", outputStr)
	}
}

func TestCLI_NonexistentConfig(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	planPath := writeTestPlan(t, tmpDir)

	cmd := exec.Command(binary, "--plan", planPath, "--config", "/nonexistent/config.hcl")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for nonexistent config")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "failed to load configuration") {
		t.Errorf("expected error about loading configuration, got:\n%s", outputStr)
	}
}

func TestCLI_MissingPlanFlag(t *testing.T) {
	binary := buildBinary(t)

	cmd := exec.Command(binary)
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code when --plan is missing")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "plan") {
		t.Errorf("expected error about missing plan flag, got:\n%s", outputStr)
	}
}

func TestCLI_VersionFlag(t *testing.T) {
	binary := buildBinary(t)

	cmd := exec.Command(binary, "--version")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("expected zero exit code for --version, got error: %v\n%s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "tfclassify") {
		t.Errorf("expected version output to contain 'tfclassify', got:\n%s", outputStr)
	}
}

func TestCLI_InitSubcommand_NoPlugins(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)

	cmd := exec.Command(binary, "init", "--config", configPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("expected zero exit code for init with no external plugins: %v\n%s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Installing plugins") {
		t.Errorf("expected 'Installing plugins' message, got:\n%s", outputStr)
	}
}

func TestCLI_ExitCodes(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)

	// Empty plan should exit 0 (auto)
	emptyPlanPath := writeEmptyPlan(t, tmpDir)
	cmd := exec.Command(binary, "--plan", emptyPlanPath, "--config", configPath)
	if err := cmd.Run(); err != nil {
		t.Errorf("expected exit code 0 for empty plan, got error: %v", err)
	}
}

func TestCLI_DefaultExitCodeZeroOnSuccess(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)
	planPath := writeTestPlan(t, tmpDir) // Plan with critical changes

	// Without --detailed-exitcode, should exit 0 even with critical classification
	cmd := exec.Command(binary, "--plan", planPath, "--config", configPath, "--output", "json")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("expected exit code 0 without --detailed-exitcode, got error: %v\nOutput:\n%s", err, output)
	}

	// Verify the output still contains "critical"
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\nOutput:\n%s", err, string(output))
	}

	if result["overall"] != "critical" {
		t.Errorf("expected overall 'critical', got %v", result["overall"])
	}
}

func TestCLI_DetailedExitCodeFlag(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)
	planPath := writeTestPlan(t, tmpDir) // Plan with critical changes

	// With --detailed-exitcode, should exit with non-zero code for critical
	cmd := exec.Command(binary, "--plan", planPath, "--config", configPath, "--output", "json", "--detailed-exitcode")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Errorf("expected non-zero exit code with --detailed-exitcode for critical classification")
	}

	// Verify the exit code is correct (critical = 2 for 3-level precedence)
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("expected exit code 2 for critical, got %d", exitErr.ExitCode())
	}

	// Verify the output still contains correct classification info
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\nOutput:\n%s", err, string(output))
	}

	if result["overall"] != "critical" {
		t.Errorf("expected overall 'critical', got %v", result["overall"])
	}
	// The exit_code in JSON should always contain the precedence-derived code
	if int(result["exit_code"].(float64)) != 2 {
		t.Errorf("expected exit_code 2 in JSON, got %v", result["exit_code"])
	}
}

func TestCLI_ErrorExitCodeUnaffectedByFlag(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)

	// Without --detailed-exitcode
	cmd := exec.Command(binary, "--plan", "/nonexistent/plan.json", "--config", configPath)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit code for nonexistent plan")
	}
	outputStr := string(output)
	if !strings.Contains(outputStr, "failed to parse plan") {
		t.Errorf("expected error about parsing plan, got:\n%s", outputStr)
	}

	// With --detailed-exitcode - same behavior for errors
	cmd = exec.Command(binary, "--plan", "/nonexistent/plan.json", "--config", configPath, "--detailed-exitcode")
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit code for nonexistent plan with --detailed-exitcode")
	}
	outputStr = string(output)
	if !strings.Contains(outputStr, "failed to parse plan") {
		t.Errorf("expected error about parsing plan, got:\n%s", outputStr)
	}
}

func TestCLI_ValidateCmd_ValidConfig(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)

	cmd := exec.Command(binary, "validate", "--config", configPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("expected exit 0 for valid config, got error: %v\n%s", err, output)
	}

	if !strings.Contains(string(output), "Configuration valid.") {
		t.Errorf("expected 'Configuration valid.' in output, got:\n%s", output)
	}
}

func TestCLI_ValidateCmd_InvalidConfig(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write config with precedence referencing nonexistent classification
	configContent := `
classification "standard" {
  description = "Standard"
  rule {
    resource = ["*"]
  }
}

precedence = ["critical", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
`
	configPath := filepath.Join(tmpDir, ".tfclassify.hcl")
	os.WriteFile(configPath, []byte(configContent), 0644)

	cmd := exec.Command(binary, "validate", "--config", configPath)
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for invalid config")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
	}

	if !strings.Contains(string(output), "undefined classification") {
		t.Errorf("expected error about undefined classification, got:\n%s", output)
	}
}

func TestCLI_ValidateCmd_InvalidGlob(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `
classification "critical" {
  description = "Critical"
  rule {
    resource = ["*_role_[*"]
  }
}

classification "standard" {
  description = "Standard"
  rule {
    resource = ["*"]
  }
}

precedence = ["critical", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
`
	configPath := filepath.Join(tmpDir, ".tfclassify.hcl")
	os.WriteFile(configPath, []byte(configContent), 0644)

	cmd := exec.Command(binary, "validate", "--config", configPath)
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for invalid glob pattern")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
	}

	if !strings.Contains(string(output), "invalid resource pattern") {
		t.Errorf("expected error about invalid pattern, got:\n%s", output)
	}
}

func TestCLI_ValidateCmd_WarningsExitZero(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Config with empty classification (warning-level issue)
	configContent := `
classification "empty" {
  description = "Empty"
}

classification "standard" {
  description = "Standard"
  rule {
    resource = ["*"]
  }
}

precedence = ["empty", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
`
	configPath := filepath.Join(tmpDir, ".tfclassify.hcl")
	os.WriteFile(configPath, []byte(configContent), 0644)

	cmd := exec.Command(binary, "validate", "--config", configPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("expected exit 0 with warnings, got error: %v\n%s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Configuration valid.") {
		t.Errorf("expected 'Configuration valid.' in output, got:\n%s", outputStr)
	}
}

func TestCLI_ValidateCmd_NoConfigFile(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command(binary, "validate", "--config", filepath.Join(tmpDir, "nonexistent.hcl"))
	_, err = cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for nonexistent config")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
	}
}

func TestCLI_InvalidPlanJSON(t *testing.T) {
	binary := buildBinary(t)

	tmpDir, err := os.MkdirTemp("", "tfclassify-cli")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := writeTestConfig(t, tmpDir)

	// Write invalid JSON
	invalidPlanPath := filepath.Join(tmpDir, "invalid.json")
	os.WriteFile(invalidPlanPath, []byte("this is not json"), 0644)

	cmd := exec.Command(binary, "--plan", invalidPlanPath, "--config", configPath)
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit code for invalid JSON plan")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "failed to parse plan") {
		t.Errorf("expected error about parsing plan, got:\n%s", outputStr)
	}
}
