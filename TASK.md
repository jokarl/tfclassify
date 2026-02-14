# tfclassify - Terraform Plan Change Classification CLI

A standalone Go CLI with plugin architecture for deep inspection of Terraform plan changes.

**Inspired by [TFLint](https://github.com/terraform-linters/tflint)** - we adopt their proven plugin patterns including gRPC-based communication, SDK structure, and plugin discovery conventions.

## Core Features

- Parse `terraform show -json` output
- Classify changes by resource type, action, and attribute analysis
- **Plugin system** for deep resource inspection (provider-agnostic)
- Configurable classification rules
- Multiple output formats (JSON, GitHub Actions, text)
- Exit codes for CI/CD integration

## Plugin Architecture (TFLint-Inspired)

### Overview

Like TFLint, tfclassify uses a bidirectional gRPC architecture where:
- Plugins are standalone executables (`tfclassify-plugin-{name}`)
- Communication uses [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin)
- Plugins can query the host for resource data via a Runner interface
- Plugins emit classification decisions back to the host

```
┌─────────────────────────────────────────────────────────────────┐
│                        tfclassify (host)                        │
│  ┌─────────────┐    ┌──────────────┐    ┌─────────────────┐     │
│  │ Plan Parser │ →  │ Plugin Loader│ →  │   Classifier    │     │
│  └─────────────┘    └──────┬───────┘    └─────────────────┘     │
│                            │ gRPC                               │
│         ┌──────────────────┼──────────────────┐                 │
│         ↓                  ↓                  ↓                 │
│   ┌───────────┐      ┌───────────┐      ┌───────────┐           │
│   │ Plugin A  │      │ Plugin B  │      │ Plugin C  │           │
│   │ (process) │      │ (process) │      │ (process) │           │
│   └───────────┘      └───────────┘      └───────────┘           │
└─────────────────────────────────────────────────────────────────┘
```

### Plugin SDK Interface (TFLint-Style)

Adopting TFLint's pattern of RuleSet → Rule, we use PluginSet → Analyzer:

```go
package sdk

// PluginSet defines a collection of analyzers (like TFLint's RuleSet)
type PluginSet interface {
    // PluginSetName returns the plugin identifier
    PluginSetName() string

    // PluginSetVersion returns the plugin version
    PluginSetVersion() string

    // AnalyzerNames returns list of analyzer names in this plugin
    AnalyzerNames() []string

    // ConfigSchema returns the schema for plugin configuration
    ConfigSchema() *hclext.BodySchema

    // ApplyConfig applies plugin-specific configuration
    ApplyConfig(config *hclext.BodyContent) error
}

// Analyzer inspects resource changes (like TFLint's Rule)
type Analyzer interface {
    // Name returns the analyzer identifier
    Name() string

    // Enabled returns whether this analyzer is enabled by default
    Enabled() bool

    // ResourcePatterns returns glob patterns this analyzer handles
    // e.g., ["*_role_assignment", "*_iam_*"]
    ResourcePatterns() []string

    // Analyze inspects changes and emits decisions via the Runner
    Analyze(runner Runner) error
}

// Runner provides access to plan data and emits decisions (like TFLint's Runner)
type Runner interface {
    // GetResourceChanges returns changes matching the given patterns
    GetResourceChanges(patterns []string) ([]*ResourceChange, error)

    // GetResourceChange returns a specific change by address
    GetResourceChange(address string) (*ResourceChange, error)

    // EvaluateExpr evaluates a Terraform expression from the plan
    EvaluateExpr(expr string, target interface{}) error

    // EmitDecision reports a classification decision for a resource
    EmitDecision(analyzer Analyzer, change *ResourceChange, decision *Decision) error
}

// ResourceChange contains the full change information
type ResourceChange struct {
    Address         string
    Type            string
    Provider        string
    Mode            string // "managed" or "data"
    Actions         []string
    Before          map[string]interface{}
    After           map[string]interface{}
    BeforeSensitive map[string]interface{}
    AfterSensitive  map[string]interface{}
}

// Decision is the analyzer's classification result
type Decision struct {
    Classification string // "emergency", "normal", "standard", "automated"
    Reason         string
    Severity       int    // 0-100 for fine-grained ordering
    Metadata       map[string]interface{}
}

// BuiltinPluginSet provides default implementations (like TFLint's BuiltinRuleSet)
type BuiltinPluginSet struct {
    Name      string
    Version   string
    Analyzers []Analyzer
}

// DefaultAnalyzer provides default implementations (like TFLint's DefaultRule)
type DefaultAnalyzer struct{}
```

### Plugin Discovery (TFLint-Style)

Plugin binaries named `tfclassify-plugin-{name}` are discovered in:

1. `plugin_dir` specified in config
2. `TFCLASSIFY_PLUGIN_DIR` environment variable
3. `./.tfclassify/plugins/`
4. `~/.tfclassify/plugins/`

Auto-installed plugins use versioned paths:
```
~/.tfclassify/plugins/
└── github.com/
    └── myorg/
        └── tfclassify-plugin-azure/
            └── 0.1.0/
                └── tfclassify-plugin-azure
```

### Plugin Execution Sequence

Following TFLint's bidirectional pattern:

1. **Host** starts plugin process via go-plugin
2. **Host** sends plugin configuration via `ApplyConfig()`
3. **Host** starts Runner gRPC server
4. **Host** calls plugin's `Analyze()` method
5. **Plugin** queries Runner for resource changes via `GetResourceChanges()`
6. **Plugin** inspects before/after state
7. **Plugin** emits decisions via `EmitDecision()`
8. **Plugin** returns completion status
9. **Host** aggregates decisions from all plugins

### Configuration Schema

```json
{
  "$schema": "https://tfclassify.dev/schema/v1.json",
  "version": "1.0",

  "plugins": {
    "azure-rbac": {
      "enabled": true,
      "source": "github.com/myorg/tfclassify-plugin-azure",
      "version": "0.1.0",
      "config": {
        "privileged_roles": ["Owner", "User Access Administrator"],
        "sensitive_scopes": ["/subscriptions/"]
      }
    },
    "networking": {
      "enabled": true,
      "source": "builtin",
      "config": {
        "critical_cidrs": ["0.0.0.0/0", "::/0"]
      }
    }
  },

  "classifications": {
    "emergency": {
      "description": "Requires immediate escalation",
      "rules": [
        { "resource": ["*_role_*"], "actions": ["delete"] }
      ]
    },
    "normal": {
      "description": "Standard approval required",
      "rules": [
        { "resource": ["*_role_*", "*_iam_*", "*_identity*"] }
      ]
    },
    "standard": {
      "description": "Pre-approved with plan review",
      "rules": [
        { "notResource": ["*_role_*", "*_iam_*", "*_key_vault*"] }
      ]
    },
    "automated": {
      "description": "Auto-approved, low risk",
      "rules": [
        { "notResource": ["*_role_*", "*_iam_*"], "actions": ["create", "update"] }
      ]
    }
  },

  "precedence": ["emergency", "normal", "standard", "automated"],

  "defaults": {
    "unclassified": "normal",
    "noChanges": "automated",
    "pluginTimeout": "30s"
  }
}
```

### Writing a Custom Plugin

Following the TFLint pattern with `plugin.Serve()`:

```go
package main

import (
    "github.com/org/tfclassify/sdk"
    "github.com/org/tfclassify/sdk/plugin"
)

func main() {
    plugin.Serve(&plugin.ServeOpts{
        PluginSet: &MyPluginSet{
            BuiltinPluginSet: sdk.BuiltinPluginSet{
                Name:    "my-plugin",
                Version: "0.1.0",
                Analyzers: []sdk.Analyzer{
                    &RoleAssignmentAnalyzer{},
                },
            },
        },
    })
}

type RoleAssignmentAnalyzer struct {
    sdk.DefaultAnalyzer
    privilegedRoles []string // From plugin config
}

func (a *RoleAssignmentAnalyzer) Name() string {
    return "role_assignment"
}

func (a *RoleAssignmentAnalyzer) ResourcePatterns() []string {
    return []string{"*_role_assignment", "*_iam_*"}
}

func (a *RoleAssignmentAnalyzer) Analyze(runner sdk.Runner) error {
    changes, err := runner.GetResourceChanges(a.ResourcePatterns())
    if err != nil {
        return err
    }

    for _, change := range changes {
        // Analyze before/after state
        beforeRole := getString(change.Before, "role_definition_name")
        afterRole := getString(change.After, "role_definition_name")

        if isPrivilegeEscalation(beforeRole, afterRole, a.privilegedRoles) {
            runner.EmitDecision(a, change, &sdk.Decision{
                Classification: "emergency",
                Reason:         fmt.Sprintf("Privilege escalation: %s → %s", beforeRole, afterRole),
                Severity:       95,
                Metadata: map[string]interface{}{
                    "before_role": beforeRole,
                    "after_role":  afterRole,
                },
            })
        }
    }
    return nil
}
```

### Built-in Plugins

Bundled with tfclassify (can be disabled):

**1. Role Analyzer** (`builtin:role`)
- Patterns: `*_role_assignment`, `*_iam_*`, `google_*_iam_*`
- Detects: privilege escalation, scope expansion, sensitive role grants

**2. Network Analyzer** (`builtin:network`)
- Patterns: `*_security_rule`, `*_firewall_rule`, `*_security_group*`
- Detects: opening to 0.0.0.0/0, removing deny rules, exposing ports

**3. Secrets Analyzer** (`builtin:secrets`)
- Patterns: `*_secret*`, `*_key_vault*`, `*_kms*`
- Detects: key deletion, access policy changes

## CLI Interface

```bash
tfclassify [flags]

Flags:
  -p, --plan string       Path to terraform plan JSON (or stdin)
  -c, --config string     Path to config (default: .tfclassify.hcl)
  -o, --output string     Output format: json|github|text (default: text)
      --plugin-dir string Additional plugin directory
      --no-plugins        Disable all plugins, use base rules only
  -v, --verbose           Include plugin analysis details
      --version           Print version
  -h, --help              Print help
```

### Exit Codes

| Code | Classification | Meaning |
|------|---------------|---------|
| 0 | automated / no-changes | Safe to auto-apply |
| 1 | standard | Needs plan review |
| 2 | normal | Needs approval |
| 3 | emergency | Needs escalation |
| 10+ | - | Error codes |

## Project Structure

```
tfclassify/
├── cmd/
│   └── tfclassify/
│       └── main.go
├── pkg/
│   ├── classify/           # Classification engine
│   │   ├── classifier.go
│   │   └── result.go
│   ├── config/             # Config loading
│   │   ├── config.go
│   │   └── validation.go
│   ├── plan/               # Plan parsing (wraps tfjson)
│   │   └── parser.go
│   ├── plugin/             # Plugin host (like tflint/plugin)
│   │   ├── discovery.go    # Find plugin binaries
│   │   ├── loader.go       # Start plugins via go-plugin
│   │   ├── host2plugin/    # Host → Plugin gRPC
│   │   │   ├── client.go
│   │   │   └── server.go
│   │   └── plugin2host/    # Plugin → Host gRPC (Runner)
│   │       ├── client.go
│   │       └── server.go
│   └── output/
│       └── formatter.go
├── sdk/                    # Published SDK (like tflint-plugin-sdk)
│   ├── pluginset.go        # PluginSet interface
│   ├── analyzer.go         # Analyzer interface
│   ├── runner.go           # Runner interface
│   ├── types.go            # ResourceChange, Decision
│   ├── builtin.go          # BuiltinPluginSet, DefaultAnalyzer
│   └── plugin/
│       └── serve.go        # plugin.Serve() entry point
├── plugins/                # Built-in plugin implementations
│   ├── role/
│   ├── network/
│   └── secrets/
├── proto/                  # gRPC protocol definitions
│   └── tfclassify.proto
└── go.mod
```

## Key Dependencies

```go
require (
    github.com/hashicorp/terraform-json v0.22.0
    github.com/hashicorp/go-plugin v1.6.0
    github.com/hashicorp/hcl/v2 v2.19.0    // HCL config (like TFLint)
    github.com/spf13/cobra v1.8.0
    github.com/gobwas/glob v0.2.3
    google.golang.org/grpc v1.60.0
    google.golang.org/protobuf v1.32.0
)
```

## Implementation Steps

1. **Core engine** (no plugins)
   - Plan parsing with terraform-json
   - Config loading (HCL format like TFLint)
   - Base rule classification
   - CLI and output formatters

2. **Plugin SDK**
   - Define PluginSet, Analyzer, Runner interfaces
   - Create BuiltinPluginSet, DefaultAnalyzer helpers
   - Implement `plugin.Serve()` entry point

3. **gRPC protocol**
   - Define proto for host↔plugin communication
   - Implement host2plugin (ApplyConfig, Analyze)
   - Implement plugin2host (GetResourceChanges, EmitDecision)

4. **Plugin host**
   - Plugin discovery (following TFLint conventions)
   - Plugin lifecycle management via go-plugin
   - Decision aggregation

5. **Built-in plugins**
   - Role analyzer
   - Network analyzer
   - Secrets analyzer

6. **Testing**
   - Unit tests per package
   - Integration tests with real plans
   - Plugin SDK tests

## Verification

```bash
# Build
go build -o tfclassify ./cmd/tfclassify

# Run tests
go test ./...

# Test with a plan
terraform plan -out=tfplan
terraform show -json tfplan > tfplan.json
./tfclassify -p tfplan.json -c .tfclassify.hcl -v

# Test plugin development
cd plugins/role && go build -o tfclassify-plugin-role
TFCLASSIFY_PLUGIN_DIR=./plugins ./tfclassify -p tfplan.json
```

## Distribution

- **Binary releases**: GitHub Releases with goreleaser
- **Plugin SDK**: `go get github.com/org/tfclassify/sdk`
- **Official plugins**: Separate repos, auto-installable via config
