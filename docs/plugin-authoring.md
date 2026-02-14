# Plugin Authoring Guide

This guide walks you through creating a tfclassify deep inspection plugin. Plugins enable provider-specific analysis of Terraform resource attributes — going beyond pattern matching to inspect actual field values like role names, network CIDR ranges, and permission grants.

## Overview

tfclassify has a three-layer classification model:

1. **Layer 1 (Core):** Config-driven pattern matching on resource types and actions
2. **Layer 2 (Builtin analyzers):** Cross-provider analysis of Terraform concepts (deletions, sensitive attributes, replacements) — runs in-process, no plugin needed
3. **Layer 3 (Deep inspection plugins):** Provider-specific analysis of resource field semantics

This guide focuses on Layer 3 — writing plugins that inspect resource attribute values.

## Prerequisites

- Go 1.24 or later
- Familiarity with Terraform resource structures
- Understanding of the provider you're targeting (Azure, AWS, GCP, etc.)

## Project Structure

A plugin is a standalone Go module that depends on the tfclassify SDK:

```
tfclassify-plugin-yourprovider-yourusecase/
├── go.mod
├── main.go              # Entry point
├── plugin.go            # PluginSet definition
├── analyzer1.go         # First analyzer
├── analyzer2.go         # Second analyzer (optional)
├── analyzer1_test.go    # Tests
└── analyzer2_test.go
```

Plugin names follow the pattern `tfclassify-plugin-{provider}-{usecase}`. Each plugin targets a specific use case for a provider rather than covering an entire provider's resources. For example, `tfclassify-plugin-azurerm-roleassignment` focuses on deep inspection of Azure role assignments.

## Step 1: Create the Module

```bash
mkdir tfclassify-plugin-yourprovider-yourusecase
cd tfclassify-plugin-yourprovider-yourusecase
go mod init github.com/yourorg/tfclassify-plugin-yourprovider-yourusecase
go get github.com/jokarl/tfclassify/sdk
```

## Step 2: Create the Entry Point

`main.go` is minimal — it just serves your plugin:

```go
package main

import (
    sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
)

func main() {
    sdkplugin.Serve(&sdkplugin.ServeOpts{
        PluginSet: NewRoleAssignmentPluginSet(),
    })
}
```

## Step 3: Define the PluginSet

The `PluginSet` groups related analyzers together. Use the SDK's `BuiltinPluginSet` for convenience:

```go
package main

import "github.com/jokarl/tfclassify/sdk"

const Version = "0.1.0"

type RoleAssignmentPluginSet struct {
    *sdk.BuiltinPluginSet
    config *PluginConfig
}

type PluginConfig struct {
    // Plugin-specific configuration fields
    SomeThreshold int
    EnabledFlags  map[string]bool
}

func DefaultConfig() *PluginConfig {
    return &PluginConfig{
        SomeThreshold: 100,
        EnabledFlags:  map[string]bool{"feature": true},
    }
}

func NewRoleAssignmentPluginSet() *RoleAssignmentPluginSet {
    config := DefaultConfig()
    ps := &RoleAssignmentPluginSet{config: config}

    ps.BuiltinPluginSet = &sdk.BuiltinPluginSet{
        Name:    "azurerm-roleassignment",
        Version: Version,
        Analyzers: []sdk.Analyzer{
            NewPrivilegeEscalationAnalyzer(config),
            NewScopeAnalyzer(config),
        },
    }

    return ps
}
```

## Step 4: Implement Analyzers

Each analyzer implements the `sdk.Analyzer` interface:

```go
type Analyzer interface {
    Name() string
    Enabled() bool
    ResourcePatterns() []string
    Analyze(runner Runner) error
}
```

### Example: Privilege Escalation Analyzer

```go
package main

import (
    "fmt"
    "github.com/jokarl/tfclassify/sdk"
)

type PrivilegeEscalationAnalyzer struct {
    sdk.DefaultAnalyzer  // Provides default implementations
    config *PluginConfig
}

func NewPrivilegeEscalationAnalyzer(config *PluginConfig) *PrivilegeEscalationAnalyzer {
    return &PrivilegeEscalationAnalyzer{config: config}
}

func (a *PrivilegeEscalationAnalyzer) Name() string {
    return "privilege-escalation"
}

func (a *PrivilegeEscalationAnalyzer) Enabled() bool {
    return a.config.EnabledFlags["privilege"]
}

func (a *PrivilegeEscalationAnalyzer) ResourcePatterns() []string {
    return []string{"azurerm_role_assignment"}
}

func (a *PrivilegeEscalationAnalyzer) Analyze(runner sdk.Runner) error {
    // Get matching resources
    changes, err := runner.GetResourceChanges(a.ResourcePatterns())
    if err != nil {
        return fmt.Errorf("failed to get resource changes: %w", err)
    }

    for _, change := range changes {
        // Inspect field values
        beforeRole := stringField(change.Before, "role_definition_name")
        afterRole := stringField(change.After, "role_definition_name")

        if isEscalation(beforeRole, afterRole) {
            decision := &sdk.Decision{
                Reason:   fmt.Sprintf("role escalated from %q to %q", beforeRole, afterRole),
                Severity: 90,
                Metadata: map[string]interface{}{
                    "before_role": beforeRole,
                    "after_role":  afterRole,
                },
            }

            if err := runner.EmitDecision(a, change, decision); err != nil {
                return fmt.Errorf("failed to emit decision: %w", err)
            }
        }
    }

    return nil
}

// Helper to extract string fields
func stringField(m map[string]interface{}, key string) string {
    if m == nil {
        return ""
    }
    if v, ok := m[key].(string); ok {
        return v
    }
    return ""
}
```

## Step 5: Understanding ResourceChange

The `sdk.ResourceChange` structure contains the Terraform plan data:

```go
type ResourceChange struct {
    Address         string                 // e.g., "azurerm_role_assignment.admin"
    Type            string                 // e.g., "azurerm_role_assignment"
    ProviderName    string                 // e.g., "registry.terraform.io/hashicorp/azurerm"
    Mode            string                 // "managed" or "data"
    Actions         []string               // e.g., ["create"], ["update"], ["delete", "create"]
    Before          map[string]interface{} // State before change (nil for creates)
    After           map[string]interface{} // State after change (nil for deletes)
    BeforeSensitive interface{}            // Sensitive markers for before state
    AfterSensitive  interface{}            // Sensitive markers for after state
}
```

### Inspecting Field Values

Resources in Terraform plans are represented as nested maps. Access fields using type assertions:

```go
// Simple string field
name := change.After["name"].(string)

// Nested object
if osDisk, ok := change.After["os_disk"].(map[string]interface{}); ok {
    caching := osDisk["caching"].(string)
}

// List/array field
if permissions, ok := change.After["secret_permissions"].([]interface{}); ok {
    for _, p := range permissions {
        if perm, ok := p.(string); ok {
            // Use perm
        }
    }
}
```

## Step 6: Emitting Decisions

Use `runner.EmitDecision()` to record findings:

```go
decision := &sdk.Decision{
    // Classification is usually left empty - the host uses severity
    Classification: "",

    // Human-readable explanation
    Reason: "inbound allow rule with source * detected",

    // Severity (0-100) determines ordering within a classification
    // Higher = more severe
    Severity: 85,

    // Additional context (optional)
    Metadata: map[string]interface{}{
        "analyzer": "network-exposure",
        "source":   "*",
        "rule":     "allow-all-inbound",
    },
}

runner.EmitDecision(a, change, decision)
```

### Severity Guidelines

| Severity | Use Case |
|----------|----------|
| 90-100 | Critical security issues (privilege escalation, data exposure) |
| 80-89 | High-risk changes (destructive permissions, wide network access) |
| 60-79 | Moderate concerns (configuration drift, non-compliance) |
| 40-59 | Low-priority findings (de-escalation, cleanup) |
| 0-39 | Informational (logging, metadata changes) |

## Step 7: Testing Your Analyzer

Create a mock runner for unit tests:

```go
type mockRunner struct {
    changes   []*sdk.ResourceChange
    decisions []*sdk.Decision
}

func (r *mockRunner) GetResourceChanges(patterns []string) ([]*sdk.ResourceChange, error) {
    return r.changes, nil
}

func (r *mockRunner) GetResourceChange(address string) (*sdk.ResourceChange, error) {
    for _, c := range r.changes {
        if c.Address == address {
            return c, nil
        }
    }
    return nil, nil
}

func (r *mockRunner) EmitDecision(analyzer sdk.Analyzer, change *sdk.ResourceChange, decision *sdk.Decision) error {
    r.decisions = append(r.decisions, decision)
    return nil
}
```

### Example Test

```go
func TestPrivilegeEscalation_ReaderToOwner(t *testing.T) {
    config := DefaultConfig()
    analyzer := NewPrivilegeEscalationAnalyzer(config)

    runner := &mockRunner{
        changes: []*sdk.ResourceChange{
            {
                Address: "azurerm_role_assignment.test",
                Type:    "azurerm_role_assignment",
                Actions: []string{"update"},
                Before:  map[string]interface{}{"role_definition_name": "Reader"},
                After:   map[string]interface{}{"role_definition_name": "Owner"},
            },
        },
    }

    err := analyzer.Analyze(runner)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if len(runner.decisions) != 1 {
        t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
    }

    if runner.decisions[0].Severity != 90 {
        t.Errorf("expected severity 90, got %d", runner.decisions[0].Severity)
    }
}
```

## Step 8: Building the Plugin

```bash
go build -o tfclassify-plugin-azurerm-roleassignment .
```

The binary name must follow the pattern `tfclassify-plugin-<provider>-<usecase>` where the combined name matches the plugin name in `.tfclassify.hcl`.

## Step 9: Installing the Plugin

Copy the binary to one of these locations (searched in order):

1. Directory specified by `TFCLASSIFY_PLUGIN_DIR` environment variable
2. `.tfclassify/plugins/` in the current directory
3. `~/.tfclassify/plugins/`

Or distribute via GitHub releases and let users run `tfclassify init`.

## Step 10: Configuring the Plugin

Users configure your plugin in `.tfclassify.hcl`:

```hcl
plugin "azurerm-roleassignment" {
  enabled = true
  source  = "github.com/yourorg/tfclassify-plugin-azurerm-roleassignment"
  version = "0.1.0"

  config {
    some_threshold = 50
    enabled_flags = {
      privilege = true
      scope     = false
    }
  }
}
```

## Distribution

### Release Asset Naming

For GitHub releases, name assets following this pattern:

```
tfclassify-plugin-<provider>-<usecase>_<version>_<os>_<arch>.zip
```

Examples:
- `tfclassify-plugin-azurerm-roleassignment_0.1.0_linux_amd64.zip`
- `tfclassify-plugin-azurerm-roleassignment_0.1.0_darwin_arm64.zip`
- `tfclassify-plugin-azurerm-roleassignment_0.1.0_windows_amd64.zip`

The ZIP should contain the plugin binary at the root level.

## Reference Implementation

The `plugins/azurerm/` directory in the tfclassify repository contains a complete reference plugin. Study it for patterns on:
- Structuring a multi-analyzer plugin (`plugin.go`, `privilege.go`, `network.go`, `keyvault.go`)
- Handling different action types (create, update, delete)
- Extracting nested field values with safe type assertions
- Embedding data files (`//go:embed` for role database)
- Comprehensive test coverage with mock runners

## Common Patterns

### Checking for Specific Actions

```go
func hasAction(actions []string, target string) bool {
    for _, a := range actions {
        if a == target {
            return true
        }
    }
    return false
}

// Only analyze creates
if hasAction(change.Actions, "create") {
    // ...
}
```

### Handling Deletions

Deletions have `After = nil`. Check before accessing fields:

```go
if change.After == nil {
    continue // Being deleted, skip
}
```

### Building Sets from Config

```go
func toSet(slice []string) map[string]bool {
    set := make(map[string]bool)
    for _, s := range slice {
        set[s] = true
    }
    return set
}

privilegedRoles := toSet(config.PrivilegedRoles)
if privilegedRoles[role] {
    // role is privileged
}
```

## Troubleshooting

### Plugin Not Found

Ensure:
1. Binary name matches `tfclassify-plugin-<provider>-<usecase>`
2. Binary is in a search path directory
3. Binary is executable (`chmod +x`)

### No Decisions Emitted

Check:
1. `Enabled()` returns `true`
2. `ResourcePatterns()` matches the resources in the plan
3. Your detection logic triggers for the test case

### Type Assertion Panics

Always use safe type assertions:

```go
// Bad - will panic if type is wrong
value := change.After["field"].(string)

// Good - safe assertion
if value, ok := change.After["field"].(string); ok {
    // use value
}
```
