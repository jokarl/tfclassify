# Test Coverage Analysis

## Current Coverage Summary

| Module / Package | Coverage | Notes |
|---|---|---|
| `pkg/output` | **100.0%** | Fully covered |
| `plugins/azurerm` | **99.1%** | Excellent |
| `internal/bundled` | **96.8%** | Very good |
| `plugins/terraform` | **94.7%** | Very good |
| `pkg/classify` | **93.8%** | Good |
| `pkg/config` | **89.3%** | Good |
| `pkg/plan` | **86.7%** | Moderate |
| `pkg/plugin` | **69.2%** | Needs improvement |
| `sdk` (root) | **100.0%** | Fully covered |
| `sdk/plugin` | **30.1%** | Needs significant improvement |
| `cmd/tfclassify` | **0.0%** | No tests |
| `proto/gen` | N/A | No test files (generated code) |

**Overall: 76.9% statement coverage** (main module only; SDK at 36.7% overall)

---

## Areas Needing Improvement

### 1. `sdk/plugin` (grpc.go / serve.go) — 30.1% coverage

**What's untested:**

- `Serve()` — the main plugin entrypoint (`serve.go:37`). This bootstraps the hashicorp/go-plugin server and is the single most important function in the SDK for plugin authors.
- `Analyze()` — the gRPC handler that runs all enabled analyzers (`grpc.go:78`). This is the core analysis loop on the plugin side.
- `RunnerClient` methods — `GetResourceChanges`, `GetResourceChange`, `EmitDecision` (`grpc.go:137-195`). These are the gRPC client stubs that plugins use to call back to the host. Zero coverage on all three.
- `NewPluginServiceClient` (`grpc.go:119`) — factory for the client side.
- All four `pluginService*Handler` functions (`grpc.go:423-493`) — the gRPC handler wrappers.
- `GetConfigSchema` only partially covered (42.9%) — the branch that converts schema attributes is not fully exercised.

**Why it matters:** The SDK is the public API for plugin authors. Untested gRPC client/server roundtrip paths mean that breaking changes to the protocol could go undetected. The `Analyze` function is the core execution path for every plugin.

**Recommended tests:**
- Integration test that starts a gRPC server with `RegisterPluginServiceServer`, connects a client, and exercises the full `GetPluginInfo` → `ApplyConfig` → `Analyze` flow using an in-process gRPC connection (e.g., `bufconn`).
- Unit tests for `RunnerClient` methods using a mock gRPC server.
- Test `GetConfigSchema` with a `PluginSet` that returns a non-nil schema with multiple attributes.

---

### 2. `pkg/plugin` (loader.go) — 69.2% coverage

**What's untested:**

- `runPluginAnalysis` (`loader.go:116`) — 0% coverage. This is the function that actually connects to a plugin process via gRPC, performs version negotiation, applies config, and calls `Analyze`. It's the core orchestration function.
- `getPluginInfo` (`loader.go:182`) — 0% coverage. Plugin version negotiation.
- `applyPluginConfig` (`loader.go:212`) — 0% coverage. Configuration delivery to plugins.
- `callAnalyze` (`loader.go:235`) — 0% coverage. The RPC call to trigger analysis.
- `RunAnalysis` (`loader.go:87`) — only 64.3% covered. The timeout parsing is tested but the actual plugin execution loop (calling `runPluginAnalysis`) is never hit because tests only use empty plugin maps.

**Why it matters:** These functions are the host-side counterpart to the SDK's plugin-side code. Together they form the complete plugin lifecycle. The fact that neither side of the gRPC protocol is tested end-to-end is the single largest coverage gap in the project.

**Recommended tests:**
- End-to-end integration test that builds the terraform plugin binary, starts it as a subprocess, and validates the full classify → plugin → callback → decision flow.
- Alternatively, a unit test for `runPluginAnalysis` using `bufconn` or a mock gRPC connection to avoid subprocess overhead.
- Test `RunAnalysis` with a real (or mock) plugin that returns decisions, to cover the main loop body.

---

### 3. `cmd/tfclassify` (main.go) — 0% coverage

**What's untested:**

- `main()` — the CLI entrypoint
- `init()` — Cobra flag registration
- `run()` — the main command handler that orchestrates config loading → plan parsing → classification → plugin execution → output formatting
- `runInit()` — the `init` subcommand handler
- `runBundledPlugin()` — the bundled plugin mode entrypoint

**Why it matters:** The `run()` function is the integration point where all packages come together. Bugs in the wiring between packages (e.g., passing wrong arguments, missing error handling) can only be caught by testing this layer.

**Recommended tests:**
- CLI integration tests using `exec.Command` to invoke the built binary with various flag combinations and validate stdout/stderr/exit codes.
- Test cases:
  - `--plan valid.json` with a valid config produces correct output
  - `--plan valid.json --output json` produces valid JSON output
  - `--plan valid.json --output github` produces GitHub-formatted output
  - `--plan nonexistent.json` produces an appropriate error
  - `--config nonexistent.hcl` produces an appropriate error
  - `init` subcommand with no external plugins skips gracefully
  - `--act-as-bundled-plugin` flag routes correctly
  - Missing `--plan` flag produces a usage error

---

### 4. `pkg/plugin/runner_server.go` — partial coverage

**What's untested:**

- `RegisterRunnerServiceServer` (`runner_server.go:86`) — 0% coverage. The gRPC service registration function.
- All four `runnerService*Handler` functions (`runner_server.go:112-168`) — 0% coverage. These are the gRPC handler wrappers identical in pattern to the SDK ones.
- `sdkToProtoResourceChange` (`runner_server.go:168`) — 75% coverage. The `BeforeSensitive`/`AfterSensitive` marshaling branches are not tested.
- `protoToSDKResourceChange` (`runner_server.go:201`) — 76.9% coverage. Same sensitive field branches missing.

**Recommended tests:**
- Test the proto↔SDK conversion functions with resources that have `BeforeSensitive` and `AfterSensitive` fields populated.
- Test `RegisterRunnerServiceServer` with a real `grpc.Server` instance.

---

### 5. `pkg/plan/parser.go` — `parseBinaryPlan` at 30% coverage

**What's untested:**

- The successful path of `parseBinaryPlan` — only the error case (terraform not found) is tested. The actual `terraform show -json` execution and stdout parsing is never tested.
- `findTerraform` PATH fallback to `tofu` binary.

**Why it matters:** Binary plan support is a user-facing feature. If `terraform show -json` output format changes or the command invocation breaks, there are no tests to catch it.

**Recommended tests:**
- Integration test with a real Terraform binary (conditionally skipped if terraform isn't available via `t.Skip`).
- Unit test using a mock script that outputs known JSON to validate the parsing path without needing real Terraform.

---

### 6. `pkg/config/loader.go` — `Load` at 0%, `LoadFile` at 75%

**What's untested:**

- `Load()` function (`loader.go:16`) — the combined discover + load path is never called in tests. Tests call `Parse()` or `LoadFile()` directly.
- `LoadFile` read error path (file exists but is unreadable).

**Recommended tests:**
- Test `Load("")` with a config file in the expected discovery path.
- Test `Load("explicit/path.hcl")` with an explicit path.
- Test `LoadFile` with a file that has restrictive permissions (unreadable).

---

## Structural Gaps (Not Captured by Line Coverage)

### A. No integration tests

The project has no integration or end-to-end tests. Every test is a unit test within a single package. This means:
- The wiring between `config.Load` → `plan.ParseFile` → `classify.New` → `classify.Classify` → `plugin.RunAnalysis` → `output.Format` is never validated as a whole.
- Plugin lifecycle (start → version check → configure → analyze → collect decisions → shutdown) is never tested end-to-end.

**Recommendation:** Add an `integration_test.go` (or `_test/` directory) that runs the full pipeline with known fixture inputs and validates the complete output.

### B. No negative/adversarial tests for plugin protocol

The plugin system communicates over gRPC with subprocess plugins. There are no tests for:
- Plugin returning malformed responses
- Plugin timing out mid-analysis
- Plugin crashing during analysis
- Plugin returning decisions for resources that don't exist
- Version negotiation failures (SDK version mismatch)

**Recommendation:** Add protocol robustness tests using mock plugins that exhibit these behaviors.

### C. No tests for concurrent plugin execution

`RunAnalysis` runs plugins sequentially, but `Runner` uses a `sync.Mutex` suggesting concurrency was considered. There are no tests validating thread safety of `EmitDecision` or `GetResourceChanges` under concurrent access.

### D. Missing edge cases in classifier

- `AddPluginDecisions` with a plugin decision for a resource not in the original plan (the "shouldn't happen" branch at `classifier.go:157`)
- `AddPluginDecisions` where plugin has lower precedence than core (should not downgrade)
- `compileRules` with invalid glob patterns (only the happy path is tested through `New()`)

---

## Priority Ranking

| Priority | Area | Current | Impact |
|---|---|---|---|
| **P0** | Plugin gRPC roundtrip (sdk/plugin + pkg/plugin) | 30-69% | Core feature, protocol correctness |
| **P1** | CLI integration tests (cmd/tfclassify) | 0% | User-facing entry point |
| **P1** | End-to-end pipeline test | None | Wiring correctness |
| **P2** | Binary plan parsing (`parseBinaryPlan`) | 30% | User-facing feature |
| **P2** | Proto/SDK conversion edge cases | 75-77% | Data integrity |
| **P3** | `config.Load` discovery path | 0% | Config resolution |
| **P3** | Plugin robustness/adversarial tests | None | Production resilience |
