# Project Structure Review (golang-pro skill)

## What's done well

**Module separation is excellent.** The three-module workspace (`host`, `sdk/`, `plugins/azurerm/`) follows the golang-pro monorepo pattern precisely. The SDK has zero dependency on host code, the plugin only depends on the SDK, and the host can use everything. No circular dependencies anywhere.

**Package naming is idiomatic.** All lowercase, single-word packages. `package main` only where it belongs (CLI + plugin binary). Unexported helpers stay unexported.

**Exported API is lean.** Only what consumers need is exported. Internal mechanics like `compileRules`, `matchesResource` are properly lowercase.

**Test organization is clean.** `_test.go` colocated with implementation, `testdata/` subdirectories, e2e scenarios in root `testdata/e2e/`.

## Things worth discussing

1. ~~**Empty `internal/` directory**~~ — **Resolved.** All host-internal packages (`classify`, `config`, `output`, `plan`, `plugin`) moved from `pkg/` to `internal/`, enforcing Go's import restriction. The empty `pkg/` directory has been removed.

2. **`cmd/tfclassify/main.go` does a lot** — it's 179 lines combining flag definitions, the `run()` function, `runInit()`, and helpers like `builtinAnalyzers()` and `hasExternalPlugins()`. The golang-pro skill suggests `cmd/` should be a thin entry point that wires things together. The orchestration logic in `run()` could live in a dedicated package (e.g., `internal/app`).

3. **`tools/` has no `tools.go`** — the golang-pro skill recommends a blank-import `tools.go` for pinning tool dependencies. However, the tools here are standalone CLI programs invoked via `make`, not `go generate` dependencies, so the pattern doesn't apply. This is fine as-is.

4. **`proto/` at root, generated code in `sdk/pb/`** — this is a good layout. The golang-pro skill's `api/` directory convention would suggest `api/proto/`, but `proto/` at root is equally common and clear.

5. **No version injection in plugin binary** — the host binary gets `-X main.Version={{.Version}}` via goreleaser, but `plugins/azurerm/main.go` has `var Version = "dev"` with no ldflags in the Makefile's `build-all` target. Goreleaser already handles this via `.goreleaser.azurerm.yml`.
