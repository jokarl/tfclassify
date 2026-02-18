# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build          # Build CLI → bin/tfclassify
make build-all      # Build CLI + azurerm plugin
make test           # Run all tests across workspace (go test ./...)
make vet            # Static analysis (go vet ./...)
make lint           # golangci-lint run ./...
make generate-roles           # Refresh Azure role database from AzAdvertizer CSV
make generate-actions         # Refresh Azure action registry from Microsoft Docs + role database
make generate-actions-offline # Refresh Azure action registry from role database only (no network)
make clean                    # Remove build artifacts
```

Go workspace mode means all commands run across all three modules from the repo root.

**Before committing**, run vulnerability check:
```bash
govulncheck ./...
```
CI enforces this — the `vuln` job fails the PR if `govulncheck` finds reachable vulnerabilities. Fix by bumping the Go version in `go.work` and all three `go.mod` files, or by upgrading affected dependencies.

**Run a single test:**
```bash
go test ./internal/classify/ -run TestClassifier_Deletion
go test ./plugins/azurerm/ -run TestPrivilege
```

**Protobuf code generation** (output in `sdk/pb/`):
```bash
protoc --go_out=. --go-grpc_out=. proto/tfclassify.proto
```

## Architecture

tfclassify classifies Terraform plan changes into organizational categories (critical, review, standard, auto-approved) using a three-layer model:

**Layer 1 — Core Engine** (`internal/classify/`): Config-driven pattern matching. Glob patterns on resource types + actions, evaluated against precedence order from `.tfclassify.hcl`. This is the fast path — no plugins involved.

**Layer 2 — Builtin Analyzers** (`internal/classify/deletion.go`, `replace.go`, `sensitive.go`): Cross-provider heuristics that run in-process. Implement `classify.BuiltinAnalyzer` interface. Detect deletions, destroy-and-recreate, and sensitive attribute changes.

**Layer 3 — External Plugins** (`sdk/` + `internal/plugin/`): Provider-specific deep inspection via gRPC (hashicorp/go-plugin). Plugins run as separate processes, communicate bidirectionally — host calls `PluginService.Analyze()`, plugin calls back `RunnerService.GetResourceChanges()` and `RunnerService.EmitDecision()`. The broker ID in `AnalyzeRequest` enables the plugin to dial back to the host's Runner server.

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

HCL format (`.tfclassify.hcl`), parsed by `internal/config/` using `hashicorp/hcl/v2`. Key blocks: `classification` (rules with resource glob + actions), `plugin` (source, version, enabled, config), `precedence` list, `defaults`.

## Plugin System

Discovery order (`internal/plugin/discovery.go`): `TFCLASSIFY_PLUGIN_DIR` env → `.tfclassify/plugins/` → `~/.tfclassify/plugins/`. Binaries named `tfclassify-plugin-{name}`.

Version negotiation: host checks `SDKVersionConstraints` against plugin's reported SDK version, and plugin can specify `HostVersionConstraint` (semver). Both checked during handshake.

## CLI Subcommands

- `tfclassify --plan <file>` — Classify a Terraform plan (root command)
- `tfclassify init` — Install plugins declared in configuration
- `tfclassify validate` — Check `.tfclassify.hcl` for errors without a plan. Exits 0 if valid (warnings to stderr), exits 1 on errors. Accepts `--config` / `-c` flag.
- `tfclassify explain --plan <file>` — Trace classification decisions for each resource through the full pipeline (core rules, builtin analyzers, plugins). Accepts `--resource` / `-r` (repeatable) to filter, `--output` / `-o` for json/text, `--config` / `-c`.

## E2E Tests

E2E test scenarios live in `testdata/e2e/`. Each scenario has `main.tf`, `.tfclassify.hcl`, and `expected.json`. These run against real Azure infrastructure in CI.

**E2e tests must be kept in sync with code changes.** When modifying plugin analyzers, config parsing, or classification logic, check whether existing e2e scenarios need updating and whether new scenarios are needed. The CI matrix in `.github/workflows/ci.yml` must include all scenarios.

**Verify e2e on your branch** by triggering the CI workflow:
```bash
gh workflow run ci.yml --ref $(git branch --show-current)
```

Monitor the run:
```bash
gh run list --workflow=ci.yml --branch=$(git branch --show-current) --limit=1
gh run watch                    # watch the latest run
```

E2e tests build from source and test both JSON and binary plan formats. Each scenario runs create and destroy phases, comparing exit codes against `expected.json`.

## Governance

ADRs in `docs/adr/`, CRs in `docs/cr/`. Use the `/governance` skill to create new ones. Checkpoint commits follow `checkpoint(CR-xxxx): {summary}` format. CRs use Gherkin acceptance criteria and RFC 2119 keywords.
