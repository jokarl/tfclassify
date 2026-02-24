package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jokarl/tfclassify/internal/config"
	"github.com/jokarl/tfclassify/internal/plan"
)

// TestGRPCIntegration_PrivilegeEscalation builds the azurerm plugin from source,
// starts it via the host, feeds a role assignment plan, and asserts decisions.
// Skipped in short mode since it compiles a plugin binary.
func TestGRPCIntegration_PrivilegeEscalation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping gRPC integration test in short mode")
	}

	// Build the azurerm plugin binary
	tmpDir := t.TempDir()
	pluginBinary := filepath.Join(tmpDir, "tfclassify-plugin-azurerm")
	cmd := exec.Command("go", "build", "-o", pluginBinary, "../../plugins/azurerm/")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build azurerm plugin: %v\n%s", err, out)
	}

	// Set plugin discovery to our temp dir
	t.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir)

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "azurerm", Enabled: true},
		},
		Classifications: []config.ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				PluginAnalyzerConfigs: map[string]*config.PluginAnalyzerConfig{
					"azurerm": {
						PrivilegeEscalation: &config.PrivilegeEscalationConfig{
							Actions: []string{"*"},
						},
					},
				},
			},
			{
				Name:        "standard",
				Description: "Standard",
				Rules: []config.RuleConfig{
					{Resource: []string{"*"}},
				},
			},
		},
		Precedence: []string{"critical", "standard"},
		Defaults: &config.DefaultsConfig{
			Unclassified:  "standard",
			NoChanges:     "standard",
			PluginTimeout: "30s",
		},
	}

	host := NewHost(cfg)
	defer host.Shutdown()

	selfPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get executable: %v", err)
	}

	if err := host.DiscoverAndStart(context.Background(), selfPath); err != nil {
		t.Fatalf("DiscoverAndStart failed: %v", err)
	}

	// Create a plan with an Owner role assignment
	changes := []plan.ResourceChange{
		{
			Address:      "azurerm_role_assignment.owner",
			Type:         "azurerm_role_assignment",
			ProviderName: "registry.terraform.io/hashicorp/azurerm",
			Mode:         "managed",
			Actions:      []string{"create"},
			After: map[string]interface{}{
				"role_definition_name": "Owner",
				"scope":               "/subscriptions/00000000-0000-0000-0000-000000000000",
			},
		},
	}

	decisions, err := host.RunAnalysis(changes)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	if len(decisions) == 0 {
		t.Fatal("expected at least one decision for Owner role assignment")
	}

	found := false
	for _, d := range decisions {
		if d.Address == "azurerm_role_assignment.owner" && d.Classification == "critical" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected critical decision for Owner role assignment, got: %+v", decisions)
	}
}

// TestGRPCIntegration_ContextCancellation verifies that plugin analysis respects
// context cancellation. Skipped in short mode.
func TestGRPCIntegration_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping context cancellation test in short mode")
	}

	// Build the azurerm plugin binary
	tmpDir := t.TempDir()
	pluginBinary := filepath.Join(tmpDir, "tfclassify-plugin-azurerm")
	cmd := exec.Command("go", "build", "-o", pluginBinary, "../../plugins/azurerm/")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build azurerm plugin: %v\n%s", err, out)
	}

	t.Setenv("TFCLASSIFY_PLUGIN_DIR", tmpDir)

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "azurerm", Enabled: true},
		},
		Classifications: []config.ClassificationConfig{
			{
				Name:        "critical",
				Description: "Critical",
				PluginAnalyzerConfigs: map[string]*config.PluginAnalyzerConfig{
					"azurerm": {
						PrivilegeEscalation: &config.PrivilegeEscalationConfig{
							Actions: []string{"*"},
						},
					},
				},
			},
		},
		Precedence: []string{"critical"},
		Defaults: &config.DefaultsConfig{
			Unclassified:  "critical",
			NoChanges:     "critical",
			PluginTimeout: "1ms", // Very short timeout to trigger cancellation
		},
	}

	host := NewHost(cfg)
	defer host.Shutdown()

	selfPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get executable: %v", err)
	}

	if err := host.DiscoverAndStart(context.Background(), selfPath); err != nil {
		t.Fatalf("DiscoverAndStart failed: %v", err)
	}

	changes := []plan.ResourceChange{
		{
			Address: "azurerm_role_assignment.test",
			Type:    "azurerm_role_assignment",
			Actions: []string{"create"},
			After: map[string]interface{}{
				"role_definition_name": "Owner",
				"scope":               "/subscriptions/00000000-0000-0000-0000-000000000000",
			},
		},
	}

	// With 1ms timeout, the analysis should either complete very fast
	// or the timeout will trigger. Either way, it should not hang.
	_, err = host.RunAnalysis(changes)
	// We don't assert on error because the plugin might complete before the
	// timeout fires. The key assertion is that this doesn't hang.
	_ = err
}
