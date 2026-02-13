---
status: approved
date: 2026-02-13
decision-makers: Johan
---

# gRPC-Based Plugin Architecture Using hashicorp/go-plugin

## Context and Problem Statement

tfclassify needs an extensible architecture where third parties can add deep inspection logic for provider-specific resources (e.g., analyzing Azure role assignments or AWS IAM policies). The core engine handles config-driven pattern matching, but deep analysis of before/after resource state requires domain-specific knowledge that must be pluggable.

How should tfclassify support extensible, provider-specific analysis while maintaining process isolation and a stable plugin contract?

## Decision Drivers

* Process isolation - a misbehaving plugin must not crash the host
* Language agnosticism in the long term (plugins could be written in languages other than Go)
* Proven pattern in the Terraform ecosystem (familiarity for contributors)
* Bidirectional communication - plugins need to query the host for plan data
* Plugin discovery and lifecycle must be well-defined

## Considered Options

* hashicorp/go-plugin with gRPC (TFLint pattern)
* In-process Go plugin system (Go's `plugin` package)
* REST/HTTP plugin protocol
* WebAssembly (Wasm) plugins

## Decision Outcome

Chosen option: "hashicorp/go-plugin with gRPC", because it provides process isolation, a proven bidirectional communication pattern, and direct alignment with TFLint's architecture that our target users already understand.

### Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                     tfclassify (host)                        │
│  ┌───────────┐   ┌──────────────┐   ┌──────────────────-┐    │
│  │Plan Parser│ → │ Core Engine  │ → │Decision Aggregator│    │
│  └───────────┘   │(pattern match│   └──────────────────-┘    │
│                  │ from config) │                            │
│                  └──────┬───────┘                            │
│                         │ gRPC (go-plugin)                   │
│           ┌─────────────┼─────────────┐                      │
│           ↓             ↓             ↓                      │
│     ┌───────────┐ ┌───────────┐ ┌───────────┐                │
│     │ Bundled   │ │ Plugin A  │ │ Plugin B  │                │
│     │ terraform │ │ (process) │ │ (process) │                │
│     │ (process) │ └───────────┘ └───────────┘                │
│     └───────────┘                                            │
└──────────────────────────────────────────────────────────────┘
```

### Bidirectional Communication

Following TFLint's pattern:

1. **Host → Plugin** (host2plugin): `ApplyConfig()`, `Analyze()`
2. **Plugin → Host** (plugin2host): `GetResourceChanges()`, `GetResourceChange()`, `EmitDecision()`

The host starts a Runner gRPC server that plugins call back into during analysis.

### Plugin Discovery

Plugin binaries named `tfclassify-plugin-{name}` are discovered in order:

1. `plugin_dir` specified in config
2. `TFCLASSIFY_PLUGIN_DIR` environment variable
3. `./.tfclassify/plugins/`
4. `~/.tfclassify/plugins/`

### Key Interfaces

| Interface | Purpose | TFLint Equivalent |
|-----------|---------|-------------------|
| `PluginSet` | Collection of analyzers, config schema | `RuleSet` |
| `Analyzer` | Inspects resource changes, emits decisions | `Rule` |
| `Runner` | Host-side API for plan data access and decision emission | `Runner` |

### Plugin Execution Sequence

1. Host starts plugin process via go-plugin
2. Host sends plugin configuration via `ApplyConfig()`
3. Host starts Runner gRPC server
4. Host calls plugin's `Analyze()` method
5. Plugin queries Runner for resource changes via `GetResourceChanges()`
6. Plugin inspects before/after state
7. Plugin emits decisions via `EmitDecision()`
8. Plugin returns completion status
9. Host aggregates decisions from all plugins

### Consequences

* Good, because process isolation prevents plugin crashes from affecting the host
* Good, because gRPC enables language-agnostic plugins in the future
* Good, because TFLint users will recognize the pattern and plugin development workflow
* Good, because hashicorp/go-plugin handles plugin lifecycle (health checks, graceful shutdown)
* Bad, because gRPC adds protobuf compilation step and code generation
* Bad, because plugin startup has process spawn overhead (mitigated by running plugins in parallel)
* Bad, because debugging across process boundaries is harder than in-process

### Confirmation

* A plugin can be built as a standalone executable and discovered by the host
* Bidirectional gRPC communication works: host calls Analyze, plugin calls back GetResourceChanges and EmitDecision
* Plugin crash does not crash the host process
* Integration test demonstrates full host-plugin lifecycle

## Pros and Cons of the Options

### hashicorp/go-plugin with gRPC

* Good, because battle-tested in Terraform, TFLint, Vault, Packer
* Good, because process isolation with automatic cleanup
* Good, because bidirectional gRPC enables the Runner callback pattern
* Good, because built-in health checking, versioning, and protocol negotiation
* Neutral, because requires protobuf definitions and code generation
* Bad, because adds hashicorp/go-plugin and gRPC as dependencies

### In-process Go plugin system

Uses Go's built-in `plugin` package to load shared objects at runtime.

* Good, because no IPC overhead - direct function calls
* Good, because simpler implementation with no gRPC layer
* Bad, because Go `plugin` package is poorly maintained and has significant limitations
* Bad, because plugins must be compiled with exact same Go version and dependency versions
* Bad, because no process isolation - plugin panic crashes the host
* Bad, because Linux/macOS only (no Windows support)

### REST/HTTP plugin protocol

Plugins expose HTTP endpoints, host communicates via REST.

* Good, because language agnostic from day one
* Good, because easy to debug with standard HTTP tools
* Bad, because no built-in plugin lifecycle management (must handle process spawn separately)
* Bad, because HTTP overhead is higher than gRPC for frequent bidirectional calls
* Bad, because no standard for bidirectional communication (would need webhooks or polling)

### WebAssembly (Wasm) plugins

Plugins compiled to Wasm, executed in an embedded runtime.

* Good, because sandboxed execution with fine-grained capability control
* Good, because single binary distribution (no separate processes)
* Bad, because Go-to-Wasm compilation produces large binaries with limited stdlib support
* Bad, because Wasm ecosystem for Go is less mature
* Bad, because accessing host data requires complex host function bindings
* Bad, because no established pattern in the Terraform ecosystem

## More Information

Related: [ADR-0001](ADR-0001-monorepo-with-go-workspaces.md) defines module boundaries aligned with this plugin architecture.

Related: [ADR-0003](ADR-0003-provider-agnostic-core-with-deep-inspection-plugins.md) defines the classification model that plugins participate in.

References:
- TFLint plugin architecture: https://github.com/terraform-linters/tflint
- hashicorp/go-plugin: https://github.com/hashicorp/go-plugin
