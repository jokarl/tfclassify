# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build          # Build CLI → bin/tfclassify
make build-all      # Build CLI + azurerm plugin
make test           # Run all tests across workspace (go test ./...)
make vet            # Static analysis (go vet ./...)
make lint           # golangci-lint run ./...
make ci             # Run all CI checks (build-all + test + vet + lint + govulncheck)
make generate-roles           # Refresh Azure role database from AzAdvertizer CSV
make generate-actions         # Refresh Azure action registry from Microsoft Docs + role database
make generate-actions-offline # Refresh Azure action registry from role database only (no network)
make clean                    # Remove build artifacts
```

Go workspace mode means all commands run across all three modules from the repo root.

The CI pipeline enforces four checks on every PR. Run `make ci` locally before pushing — it runs all of them in one command:

```bash
make ci              # build-all + test + vet + lint + govulncheck
```

The `vuln` job fails the PR if `govulncheck` finds reachable vulnerabilities. Fix by bumping the Go version in `go.work` and all three `go.mod` files, or by upgrading affected dependencies. The `lint` job fails the PR if `golangci-lint` finds any violation.

**Run a single test:**
```bash
go test ./internal/classify/ -run TestClassifier_Deletion
go test ./plugins/azurerm/ -run TestPrivilege
```

**E2E fixture tests** (fast, no Azure needed):
```bash
bash testdata/e2e/run.sh --build --fixtures         # classify committed plan fixtures
bash testdata/e2e/run.sh --build --capture          # refresh fixtures (requires Azure)
```

Fixture tests run against committed plan JSON files in `testdata/e2e/*/fixtures/`. CI runs fixtures on every PR. Live Azure e2e tests run nightly via `verify.yml`.

**Protobuf code generation** (output in `sdk/pb/`):
```bash
protoc --go_out=. --go-grpc_out=. proto/tfclassify.proto
```

## Product Philosophy

tfclassify classifies **changes**, not final state. This is the core differentiator from Trivy, Checkov, and tflint — those tools evaluate whether a resource IS compliant. tfclassify classifies whether a CHANGE needs elevated approval. "This resource has TLS 1.0" is a compliance finding (their domain). "This PR downgrades TLS from 1.2 to 1.0" is a risky change that needs a different approval workflow (our domain).

### What We Are

A change classification and approval routing engine for Terraform plans. We answer: "Given this set of infrastructure changes, what level of review and approval is required?"

### What We Are NOT

A policy engine, compliance scanner, or static analysis tool. We do not maintain databases of "correct" attribute values, check hundreds of resource properties, or enforce configuration standards. That is what Trivy, Checkov, and tflint do — and they do it well.

### Analyzer Depth Principle

**Either bring semantic depth or don't build it.** The bar for a new analyzer is: "Does this require domain knowledge that Trivy/Checkov cannot replicate?"

The `privilege_escalation` analyzer is the reference — it resolves Azure role definitions from a built-in database, computes effective permissions with wildcard expansion and NotActions, separates data-plane from control-plane, cross-references custom role definitions from the plan, and supports graduated thresholds per classification. Users cannot replicate this with a Checkov policy.

A shallow attribute check ("if `default_action == Allow`, flag it") does not clear this bar. That is a policy check, and policy tools already do it across hundreds of resource types.

### Classification Naming Convention

Classifications can be named after compliance controls (e.g., `classification "CIS-6"`) and rule descriptions can reference specific control IDs. No dedicated compliance annotation feature is needed — the existing model handles it through naming. See `testdata/e2e/cis-azure-foundations/` for a working example.

## Architecture

tfclassify classifies Terraform plan changes into organizational categories (critical, review, standard, auto-approved) using a three-layer model:

**Layer 1 — Core Engine** (`internal/classify/`): Config-driven pattern matching. Glob patterns on resource types + actions, evaluated against precedence order from `.tfclassify.hcl`. This is the fast path — no plugins involved.

**Layer 2 — Builtin Analyzers** (`internal/classify/deletion.go`, `replace.go`, `sensitive.go`, `blast_radius.go`, `drift.go`, `topology.go`): Cross-provider heuristics that run in-process. Simple analyzers implement `classify.BuiltinAnalyzer` interface; plan-aware analyzers implement `classify.PlanAwareAnalyzer` (receives full `ParseResult`). Detect deletions, destroy-and-recreate, sensitive attribute changes, plan-wide blast radius thresholds, drift corrections, and dependency graph topology.

**Layer 3 — External Plugins** (`sdk/` + `internal/plugin/`): Provider-specific deep inspection via gRPC (hashicorp/go-plugin). Plugins run as separate processes, communicate bidirectionally — host calls `PluginService.Analyze()`, plugin calls back `RunnerService.GetResourceChanges()` and `RunnerService.EmitDecision()`. The broker ID in `AnalyzeRequest` enables the plugin to dial back to the host's Runner server. Currently serves one deep analyzer (`privilege_escalation`) as the reference implementation.

**Decision aggregation**: Plugin/analyzer decisions override core pattern matches based on the `precedence` list in config (lower index = higher precedence).

### Output Pipeline

`internal/output/` handles result formatting (text, JSON, GitHub Actions, SARIF). The evidence system (`output/evidence.go`) produces a self-contained JSON artifact with input hashes, timestamps, and optional Ed25519 signature for tamper evidence. Evidence can include the full explain trace for audit retention.

## Module Structure (Go Workspaces)

| Module | Path | Purpose |
|--------|------|---------|
| Host/CLI | `.` | Core engine, config, plan parsing, plugin loading, CLI |
| SDK | `sdk/` | Public interfaces for plugin authors (`Analyzer`, `Runner`, `PluginSet`) |
| Azure plugin | `plugins/azurerm/` | Reference plugin: privilege escalation (deep RBAC analysis) |

The SDK and plugin have their own `go.mod`. The plugin uses a `replace` directive to resolve the SDK locally. `go.work` ties them together for development.

## Key Interfaces

- `sdk.Analyzer` — Plugin-side analysis logic. Receives a `Runner` to query resources and emit decisions.
- `sdk.Runner` — Host-side API exposed to plugins via gRPC. Methods: `GetResourceChanges`, `GetResourceChange`, `EmitDecision`.
- `sdk.PluginSet` — Collection of analyzers provided by a plugin binary.
- `classify.BuiltinAnalyzer` — In-process analyzer (Layer 2). Simpler interface: takes `[]plan.ResourceChange`, returns `[]ResourceDecision`.
- `classify.PlanAwareAnalyzer` — In-process analyzer that receives the full `*plan.ParseResult` (for analyzers needing plan-level context like drift data or dependency graph).

## gRPC Protocol

Defined in `proto/tfclassify.proto`, generated code in `sdk/pb/`. Two services:
- `PluginService` (host → plugin): `GetPluginInfo`, `GetConfigSchema`, `ApplyConfig`, `Analyze`
- `RunnerService` (plugin → host): `GetResourceChanges`, `GetResourceChange`, `EmitDecision`

`before`/`after` fields on `ResourceChange` are JSON-encoded bytes (not structured protobuf) to handle arbitrary Terraform resource schemas.

## Configuration

HCL format (`.tfclassify.hcl`), parsed by `internal/config/` using `hashicorp/hcl/v2`. Key blocks: `classification` (rules with resource/not_resource glob + actions/not_actions + module/not_module, blast_radius thresholds, topology thresholds), `plugin` (source, version, enabled, config), `precedence` list, `defaults` (unclassified, no_changes, drift_classification, ignore_attributes), `evidence` (artifact output with optional Ed25519 signing).

## Plugin System

Discovery order (`internal/plugin/discovery.go`): `TFCLASSIFY_PLUGIN_DIR` env → `.tfclassify/plugins/` → `~/.tfclassify/plugins/`. Binaries named `tfclassify-plugin-{name}`.

Version negotiation: host checks `SDKVersionConstraints` against plugin's reported SDK version, and plugin can specify `HostVersionConstraint` (semver). Both checked during handshake.

## CLI Subcommands

- `tfclassify --plan <file>` — Classify a Terraform plan (root command). Accepts `--evidence-file <path>` to write a signed evidence artifact alongside normal output.
- `tfclassify init` — Install plugins declared in configuration
- `tfclassify validate` — Check `.tfclassify.hcl` for errors without a plan. Exits 0 if valid (warnings to stderr), exits 1 on errors. Accepts `--config` / `-c` flag.
- `tfclassify explain --plan <file>` — Trace classification decisions for each resource through the full pipeline (core rules, builtin analyzers, plugins). Accepts `--resource` / `-r` (repeatable) to filter, `--output` / `-o` for json/text, `--config` / `-c`.
- `tfclassify verify --evidence-file <file> --public-key <file>` — Verify the Ed25519 signature of an evidence artifact. Exits 0 if valid, 1 if invalid.

## E2E Tests

E2E test scenarios live in `testdata/e2e/`. Each scenario has `main.tf`, `.tfclassify.hcl`, and `expected.json`.

**Two tiers:**

1. **Fixture tests (fast, every PR):** Committed plan JSON fixtures in `testdata/e2e/*/fixtures/`. CI runs `run.sh --build --fixtures` which classifies fixtures and checks exit codes against `expected.json`. No Azure credentials needed.

2. **Live e2e (slow, periodic):** Runs `terraform plan` against real Azure infrastructure via `verify.yml`. Tests both JSON and binary `.tfplan` formats. If plan shapes drift from committed fixtures, verify can detect the difference.

E2E tests must be kept in sync with code changes. When modifying plugin analyzers, config parsing, or classification logic, check whether existing e2e scenarios need updating and whether new scenarios are needed. The CI matrix in `.github/workflows/ci.yml` must include all scenarios.

**Run fixture tests locally:**
```bash
bash testdata/e2e/run.sh --build --fixtures               # fast, no Azure
```

**Refresh fixtures** (requires Azure credentials):
```bash
bash testdata/e2e/run.sh --build --capture                # all scenarios
bash testdata/e2e/run.sh --build --capture -t route-table  # specific scenario
```

**Run live e2e locally** with `testdata/e2e/run.sh`. Use `--build` for development — it compiles both the CLI and azurerm plugin from source:
```bash
bash testdata/e2e/run.sh --build --plan-only -t blast-radius -t role-assignment-privileged
```

Use `--version` to test against a published GitHub release. This downloads the CLI via `gh release download` and installs plugins via `tfclassify init`:
```bash
bash testdata/e2e/run.sh --version 0.4.0 --plan-only -t blast-radius
```

Add `--plan-only` to skip apply/destroy (faster iteration). Use `-t NAME` (repeatable) to run specific scenarios.

**Verify e2e in CI** by triggering the workflow on your branch:
```bash
gh workflow run ci.yml --ref $(git branch --show-current)
gh run watch    # watch the latest run
```

## Governance

ADRs in `docs/adr/`, CRs in `docs/cr/`. Use the `/governance` skill to create new ones. Checkpoint commits follow `checkpoint(CR-xxxx): {summary}` format. CRs use Gherkin acceptance criteria and RFC 2119 keywords.
