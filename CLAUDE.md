# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build          # Build CLI → bin/tfclassify
make build-all      # Build CLI + azurerm plugin
make test           # Run all tests across workspace (go test ./...)
make vet            # Static analysis (go vet ./...)
make lint           # golangci-lint run ./...
make generate-roles # Refresh Azure role database from AzAdvertizer CSV
make clean          # Remove build artifacts
```

Go workspace mode means all commands run across all three modules from the repo root.

**Run a single test:**
```bash
go test ./pkg/classify/ -run TestClassifier_Deletion
go test ./plugins/azurerm/ -run TestPrivilege
```

**Protobuf code generation** (output in `sdk/pb/`):
```bash
protoc --go_out=. --go-grpc_out=. proto/tfclassify.proto
```

## Architecture

tfclassify classifies Terraform plan changes into organizational categories (critical, review, standard, auto-approved) using a three-layer model:

**Layer 1 — Core Engine** (`pkg/classify/`): Config-driven pattern matching. Glob patterns on resource types + actions, evaluated against precedence order from `.tfclassify.hcl`. This is the fast path — no plugins involved.

**Layer 2 — Builtin Analyzers** (`pkg/classify/deletion.go`, `replace.go`, `sensitive.go`): Cross-provider heuristics that run in-process. Implement `classify.BuiltinAnalyzer` interface. Detect deletions, destroy-and-recreate, and sensitive attribute changes.

**Layer 3 — External Plugins** (`sdk/` + `pkg/plugin/`): Provider-specific deep inspection via gRPC (hashicorp/go-plugin). Plugins run as separate processes, communicate bidirectionally — host calls `PluginService.Analyze()`, plugin calls back `RunnerService.GetResourceChanges()` and `RunnerService.EmitDecision()`. The broker ID in `AnalyzeRequest` enables the plugin to dial back to the host's Runner server.

**Decision aggregation**: Plugin/analyzer decisions override core pattern matches based on the `precedence` list in config (lower index = higher precedence).

## Module Structure (Go Workspaces)

| Module | Path | Purpose |
|--------|------|---------|
| Host/CLI | `.` | Core engine, config, plan parsing, plugin loading, CLI |
| SDK | `sdk/` | Public interfaces for plugin authors (`Analyzer`, `Runner`, `PluginSet`) |
| Azure plugin | `plugins/azurerm/` | Reference plugin: privilege escalation, network exposure, keyvault |

The SDK and plugin have their own `go.mod`. The plugin uses a `replace` directive to resolve the SDK locally. `go.work` ties them together for development.

## Key Interfaces

- `sdk.Analyzer` — Plugin-side analysis logic. Receives a `Runner` to query resources and emit decisions.
- `sdk.Runner` — Host-side API exposed to plugins via gRPC. Methods: `GetResourceChanges`, `GetResourceChange`, `EmitDecision`.
- `sdk.PluginSet` — Collection of analyzers provided by a plugin binary.
- `classify.BuiltinAnalyzer` — In-process analyzer (Layer 2). Simpler interface: takes `[]plan.ResourceChange`, returns `[]ResourceDecision`.

## gRPC Protocol

Defined in `proto/tfclassify.proto`, generated code in `sdk/pb/`. Two services:
- `PluginService` (host → plugin): `GetPluginInfo`, `GetConfigSchema`, `ApplyConfig`, `Analyze`
- `RunnerService` (plugin → host): `GetResourceChanges`, `GetResourceChange`, `EmitDecision`

`before`/`after` fields on `ResourceChange` are JSON-encoded bytes (not structured protobuf) to handle arbitrary Terraform resource schemas.

## Configuration

HCL format (`.tfclassify.hcl`), parsed by `pkg/config/` using `hashicorp/hcl/v2`. Key blocks: `classification` (rules with resource glob + actions), `plugin` (source, version, enabled, config), `precedence` list, `defaults`.

## Plugin System

Discovery order (`pkg/plugin/discovery.go`): config `plugin_dir` → `TFCLASSIFY_PLUGIN_DIR` env → `.tfclassify/plugins/` → `~/.tfclassify/plugins/`. Binaries named `tfclassify-plugin-{name}`.

Version negotiation: host checks `SDKVersionConstraints` against plugin's reported SDK version, and plugin can specify `HostVersionConstraint` (semver). Both checked during handshake.

## Governance

ADRs in `docs/adr/`, CRs in `docs/cr/`. Use the `/governance` skill to create new ones. Checkpoint commits follow `checkpoint(CR-xxxx): {summary}` format. CRs use Gherkin acceptance criteria and RFC 2119 keywords.
