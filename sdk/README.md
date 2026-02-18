# tfclassify Plugin SDK

The Plugin SDK provides the public interfaces and types for building tfclassify deep inspection plugins. It is a standalone Go module with minimal dependencies (go-plugin, gRPC, protobuf), keeping plugin binaries lightweight.

```
go get github.com/jokarl/tfclassify/sdk
```

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Interfaces](#interfaces)
  - [Analyzer](#analyzer)
  - [ClassificationAwareAnalyzer](#classificationawareanalyzer)
  - [Runner](#runner)
  - [PluginSet](#pluginset)
- [Types](#types)
  - [ResourceChange](#resourcechange)
  - [Decision](#decision)
  - [ConfigSchemaSpec](#configschemaspec)
- [Helper Structs](#helper-structs)
  - [BuiltinPluginSet](#builtinpluginset)
  - [DefaultAnalyzer](#defaultanalyzer)
- [Writing a Plugin](#writing-a-plugin)
  - [1. Create the Module](#1-create-the-module)
  - [2. Define the Entry Point](#2-define-the-entry-point)
  - [3. Define the PluginSet](#3-define-the-pluginset)
  - [4. Implement an Analyzer](#4-implement-an-analyzer)
  - [5. Emit Decisions](#5-emit-decisions)
  - [6. Build and Install](#6-build-and-install)
- [Inspecting Resource Attributes](#inspecting-resource-attributes)
- [Testing](#testing)
- [Severity Guidelines](#severity-guidelines)
- [gRPC Protocol](#grpc-protocol)
- [Version Compatibility](#version-compatibility)

## Overview

tfclassify uses a three-layer classification model:

| Layer | Mechanism | Runs |
|-------|-----------|------|
| 1. Core rules | Config-driven glob pattern matching | In host process |
| 2. Builtin analyzers | Cross-provider heuristics (deletion, replace, sensitive) | In host process |
| 3. Deep inspection plugins | Provider-specific attribute analysis | Separate process via gRPC |

This SDK is for **Layer 3** -- building plugins that inspect the actual values of resource attributes (role names, CIDR ranges, permission grants, etc.) rather than just matching on resource type and action.

## Architecture

Plugins run as separate processes. The host and plugin communicate bidirectionally over gRPC using [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin):

```
┌─────────────────────┐          gRPC           ┌──────────────────────┐
│     tfclassify      │ ───────────────────────▶ │       Plugin         │
│       (host)        │                          │     (subprocess)     │
│                     │  PluginService.Analyze() │                      │
│                     │ ◀─────────────────────── │  Analyzer.Analyze()  │
│  RunnerService      │  RunnerService calls:    │                      │
│  (serves plan data) │  - GetResourceChanges()  │  runner.EmitDecision │
│                     │  - GetResourceChange()   │                      │
│                     │  - EmitDecision()        │                      │
└─────────────────────┘                          └──────────────────────┘
```

1. Host starts the plugin binary as a subprocess
2. go-plugin handshake and gRPC broker established
3. Host calls `PluginService.Analyze()` with a broker ID
4. Plugin uses the broker ID to dial back and create a `RunnerService` client
5. Plugin queries plan data via `GetResourceChanges` / `GetResourceChange`
6. Plugin reports findings via `EmitDecision`
7. Host aggregates plugin decisions with core rule results

## Interfaces

### Analyzer

The core interface every analyzer must implement. Each analyzer inspects a subset of resource changes and emits classification decisions.

```go
type Analyzer interface {
    // Name returns the unique name within the plugin set.
    Name() string

    // Enabled returns whether this analyzer should run.
    Enabled() bool

    // ResourcePatterns returns glob patterns for resources this analyzer
    // inspects. Return nil or empty to receive all resources.
    ResourcePatterns() []string

    // Analyze inspects resources and emits decisions via the Runner.
    Analyze(runner Runner) error
}
```

Embed `DefaultAnalyzer` to get default implementations for `Enabled()` (returns `true`) and `ResourcePatterns()` (returns `nil`).

### ClassificationAwareAnalyzer

An optional extension of `Analyzer` for analyzers that need per-classification configuration. When implemented, the host calls `AnalyzeWithClassification` once per classification block that contains analyzer config, instead of calling `Analyze` once.

```go
type ClassificationAwareAnalyzer interface {
    Analyzer

    // AnalyzeWithClassification inspects resources with classification context.
    // classification is the name of the classification block (e.g., "critical")
    // analyzerConfig is JSON-encoded per-analyzer configuration from the
    // classification block's plugin sub-block.
    AnalyzeWithClassification(runner Runner, classification string, analyzerConfig []byte) error
}
```

**When to use:** Implement this when your analyzer needs graduated thresholds or different detection patterns per classification level. For example, the azurerm plugin's privilege escalation analyzer uses stricter action patterns for `critical` than for `standard`.

**How config flows:** HCL sub-blocks inside `classification {}` blocks are JSON-encoded and passed as `analyzerConfig`. Given:

```hcl
classification "critical" {
  azurerm {
    privilege_escalation {
      actions = ["Microsoft.Authorization/*"]
    }
  }
}
```

The analyzer receives `classification = "critical"` and `analyzerConfig = {"actions":["Microsoft.Authorization/*"]}`.

If an analyzer only implements `Analyzer` (not `ClassificationAwareAnalyzer`), its `Analyze()` method is called once without classification context.

### Runner

The host-provided interface that plugins call during analysis to query plan data and emit decisions. You do not implement this -- it is passed to your `Analyze()` method.

```go
type Runner interface {
    // GetResourceChanges returns resources matching glob patterns.
    // Empty patterns returns all resources.
    GetResourceChanges(patterns []string) ([]*ResourceChange, error)

    // GetResourceChange returns a single resource by address.
    GetResourceChange(address string) (*ResourceChange, error)

    // EmitDecision records a classification decision for a resource.
    EmitDecision(analyzer Analyzer, change *ResourceChange, decision *Decision) error
}
```

### PluginSet

Defines a collection of analyzers provided by a single plugin binary.

```go
type PluginSet interface {
    PluginSetName() string
    PluginSetVersion() string
    AnalyzerNames() []string
    VersionConstraint() string        // semver constraint on host, e.g. ">= 0.1.0"
    ConfigSchema() *ConfigSchemaSpec  // nil if no validation needed
}
```

Use `BuiltinPluginSet` instead of implementing this directly.

## Types

### ResourceChange

Represents a single resource change from a Terraform plan:

```go
type ResourceChange struct {
    Address         string                 // "azurerm_role_assignment.admin"
    Type            string                 // "azurerm_role_assignment"
    ProviderName    string                 // "registry.terraform.io/hashicorp/azurerm"
    Mode            string                 // "managed" or "data"
    Actions         []string               // ["create"], ["update"], ["delete", "create"]
    Before          map[string]interface{} // State before change (nil for creates)
    After           map[string]interface{} // State after change (nil for deletes)
    BeforeSensitive interface{}            // Sensitive attribute markers (before)
    AfterSensitive  interface{}            // Sensitive attribute markers (after)
}
```

`Before` and `After` are JSON-decoded maps of the resource's attributes. For creates, `Before` is nil. For deletes, `After` is nil. For replacements (destroy-and-recreate), `Actions` contains `["delete", "create"]`.

### Decision

A classification finding emitted by an analyzer:

```go
type Decision struct {
    // Classification level to assign. Leave empty to let the host
    // use severity to determine classification.
    Classification string

    // Human-readable explanation of why this decision was made.
    Reason string

    // Fine-grained risk score (0-100). Higher = more severe.
    Severity int

    // Additional context (optional). Included in JSON output.
    Metadata map[string]interface{}
}
```

### ConfigSchemaSpec

Describes the expected structure of a plugin's `config {}` block for validation:

```go
type ConfigSchemaSpec struct {
    Attributes []ConfigAttribute
}

type ConfigAttribute struct {
    Name        string // Attribute name
    Type        string // HCL type: "string", "number", "bool", "list(string)", etc.
    Required    bool
    Description string
}
```

## Helper Structs

### BuiltinPluginSet

A ready-made `PluginSet` implementation. Embed it and set the fields:

```go
type BuiltinPluginSet struct {
    Name                  string
    Version               string
    Analyzers             []Analyzer
    HostVersionConstraint string            // Optional semver constraint
    Schema                *ConfigSchemaSpec // Optional config validation
}
```

Provides `PluginSetName()`, `PluginSetVersion()`, `AnalyzerNames()`, `VersionConstraint()`, `ConfigSchema()`, and a helper `GetAnalyzer(name string) Analyzer`.

### DefaultAnalyzer

Embed in your analyzer structs to get default implementations:

```go
type DefaultAnalyzer struct{}

func (d DefaultAnalyzer) Enabled() bool          { return true }
func (d DefaultAnalyzer) ResourcePatterns() []string { return nil }
```

Override `Enabled()` to support toggling analyzers via configuration. Override `ResourcePatterns()` to restrict which resources your analyzer receives.

## Writing a Plugin

### 1. Create the Module

```bash
mkdir tfclassify-plugin-aws-securitygroups
cd tfclassify-plugin-aws-securitygroups
go mod init github.com/yourorg/tfclassify-plugin-aws-securitygroups
go get github.com/jokarl/tfclassify/sdk
```

### 2. Define the Entry Point

`main.go` -- calls `plugin.Serve()` with your `PluginSet`:

```go
package main

import (
    sdkplugin "github.com/jokarl/tfclassify/sdk/plugin"
)

func main() {
    sdkplugin.Serve(&sdkplugin.ServeOpts{
        PluginSet: NewSecurityGroupPluginSet(),
    })
}
```

### 3. Define the PluginSet

`plugin.go` -- group your analyzers together:

```go
package main

import "github.com/jokarl/tfclassify/sdk"

const Version = "0.1.0"

type SecurityGroupPluginSet struct {
    *sdk.BuiltinPluginSet
}

func NewSecurityGroupPluginSet() *SecurityGroupPluginSet {
    ps := &SecurityGroupPluginSet{}
    ps.BuiltinPluginSet = &sdk.BuiltinPluginSet{
        Name:    "aws-securitygroups",
        Version: Version,
        Analyzers: []sdk.Analyzer{
            NewOpenIngressAnalyzer(),
        },
    }
    return ps
}
```

### 4. Implement an Analyzer

`open_ingress.go` -- detect security groups allowing traffic from 0.0.0.0/0:

```go
package main

import (
    "fmt"
    "github.com/jokarl/tfclassify/sdk"
)

type OpenIngressAnalyzer struct {
    sdk.DefaultAnalyzer
}

func NewOpenIngressAnalyzer() *OpenIngressAnalyzer {
    return &OpenIngressAnalyzer{}
}

func (a *OpenIngressAnalyzer) Name() string {
    return "open-ingress"
}

func (a *OpenIngressAnalyzer) ResourcePatterns() []string {
    return []string{"aws_security_group_rule"}
}

func (a *OpenIngressAnalyzer) Analyze(runner sdk.Runner) error {
    changes, err := runner.GetResourceChanges(a.ResourcePatterns())
    if err != nil {
        return fmt.Errorf("failed to get resource changes: %w", err)
    }

    for _, change := range changes {
        after := change.After
        if after == nil {
            continue // Being deleted
        }

        ruleType, _ := after["type"].(string)
        cidr, _ := after["cidr_blocks"].([]interface{})

        if ruleType != "ingress" {
            continue
        }

        for _, block := range cidr {
            if s, ok := block.(string); ok && s == "0.0.0.0/0" {
                decision := &sdk.Decision{
                    Reason:   fmt.Sprintf("security group rule allows ingress from %s", s),
                    Severity: 85,
                    Metadata: map[string]interface{}{
                        "analyzer":    "open-ingress",
                        "cidr_block":  s,
                    },
                }
                if err := runner.EmitDecision(a, change, decision); err != nil {
                    return fmt.Errorf("failed to emit decision: %w", err)
                }
            }
        }
    }
    return nil
}
```

### 5. Emit Decisions

Call `runner.EmitDecision()` for each finding. Key fields:

| Field | Usage |
|-------|-------|
| `Classification` | Leave empty (recommended) -- the host maps severity to classifications via precedence. Set explicitly only if you need to force a specific level. |
| `Reason` | Human-readable explanation shown in output. |
| `Severity` | 0-100 risk score. Used for ordering and classification mapping. |
| `Metadata` | Arbitrary key-value pairs included in JSON output. |

### 6. Build and Install

```bash
# Build
go build -o tfclassify-plugin-aws-securitygroups .

# Install locally
mkdir -p .tfclassify/plugins
cp tfclassify-plugin-aws-securitygroups .tfclassify/plugins/

# Or install to home directory
mkdir -p ~/.tfclassify/plugins
cp tfclassify-plugin-aws-securitygroups ~/.tfclassify/plugins/
```

Configure in `.tfclassify.hcl`:

```hcl
plugin "aws-securitygroups" {
  enabled = true
  # For local development, omit source/version.
  # The host discovers the binary from the plugin directories.
}
```

For distribution via GitHub releases, add `source` and `version` so users can run `tfclassify init`:

```hcl
plugin "aws-securitygroups" {
  enabled = true
  source  = "github.com/yourorg/tfclassify-plugin-aws-securitygroups"
  version = "0.1.0"
}
```

## Inspecting Resource Attributes

Resource attributes arrive as `map[string]interface{}` from JSON decoding. Always use safe type assertions:

```go
// String field
if name, ok := change.After["name"].(string); ok {
    // use name
}

// Nested object
if osDisk, ok := change.After["os_disk"].(map[string]interface{}); ok {
    if caching, ok := osDisk["caching"].(string); ok {
        // use caching
    }
}

// List field
if permissions, ok := change.After["secret_permissions"].([]interface{}); ok {
    for _, p := range permissions {
        if perm, ok := p.(string); ok {
            // use perm
        }
    }
}

// Numeric field (JSON numbers decode as float64)
if count, ok := change.After["count"].(float64); ok {
    intCount := int(count)
    // use intCount
}
```

A common pattern for safe string access:

```go
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

## Testing

Create a mock `Runner` for unit tests:

```go
type mockRunner struct {
    changes   []*sdk.ResourceChange
    decisions []*capturedDecision
}

type capturedDecision struct {
    analyzer string
    address  string
    decision *sdk.Decision
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
    return nil, fmt.Errorf("not found: %s", address)
}

func (r *mockRunner) EmitDecision(analyzer sdk.Analyzer, change *sdk.ResourceChange, decision *sdk.Decision) error {
    r.decisions = append(r.decisions, &capturedDecision{
        analyzer: analyzer.Name(),
        address:  change.Address,
        decision: decision,
    })
    return nil
}
```

Example test:

```go
func TestOpenIngress_DetectsWildcardCIDR(t *testing.T) {
    analyzer := NewOpenIngressAnalyzer()

    runner := &mockRunner{
        changes: []*sdk.ResourceChange{
            {
                Address: "aws_security_group_rule.allow_all",
                Type:    "aws_security_group_rule",
                Actions: []string{"create"},
                After: map[string]interface{}{
                    "type":        "ingress",
                    "cidr_blocks": []interface{}{"0.0.0.0/0"},
                    "from_port":   float64(0),
                    "to_port":     float64(65535),
                },
            },
        },
    }

    if err := analyzer.Analyze(runner); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if len(runner.decisions) != 1 {
        t.Fatalf("expected 1 decision, got %d", len(runner.decisions))
    }
    if runner.decisions[0].decision.Severity != 85 {
        t.Errorf("expected severity 85, got %d", runner.decisions[0].decision.Severity)
    }
}
```

## Severity Guidelines

| Range | Use Case | Examples |
|-------|----------|----------|
| 90-100 | Critical security issues | Unrestricted wildcard permissions, Owner role assignment |
| 80-89 | High-risk changes | Destructive KV permissions, open network rules |
| 60-79 | Moderate concerns | Targeted auth write, broad provider wildcards |
| 40-59 | Low-priority findings | De-escalation, limited write access |
| 0-39 | Informational | Read-only changes, metadata updates |

## gRPC Protocol

Defined in [`proto/tfclassify.proto`](../proto/tfclassify.proto). Two services:

**PluginService** (host calls plugin):
- `GetPluginInfo` -- version negotiation
- `GetConfigSchema` -- schema for config validation
- `ApplyConfig` -- forward plugin-specific HCL config
- `Analyze` -- start analysis (passes broker ID for callback)

**RunnerService** (plugin calls host):
- `GetResourceChanges` -- query resources by glob patterns
- `GetResourceChange` -- query a single resource by address
- `EmitDecision` -- record a classification decision

Generated Go code lives in `sdk/pb/`. Plugin authors do not interact with the protobuf types directly -- the SDK handles serialization.

## Version Compatibility

- **`SDKVersion`** (currently `0.0.1`): embedded in every plugin binary and reported to the host during handshake. The host checks this against its supported SDK version constraints.
- **`HostVersionConstraint`**: an optional semver constraint your plugin can set (e.g. `">= 0.1.0"`) to require a minimum host version. Set via `BuiltinPluginSet.HostVersionConstraint`.

Both are checked during the go-plugin handshake. If either fails, the host logs the mismatch and skips the plugin.
