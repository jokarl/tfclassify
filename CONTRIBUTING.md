# Contributing to tfclassify

Thanks for your interest in contributing to tfclassify. This guide covers development setup, testing, plugin authoring, and the PR process.

## Development Setup

### Prerequisites

- Go 1.24 or later
- `golangci-lint` for linting
- `protoc` with Go plugins for protobuf changes
- `govulncheck` for vulnerability scanning

### Clone and Build

```bash
git clone https://github.com/jokarl/tfclassify.git
cd tfclassify
```

The repository uses [Go workspaces](https://go.dev/doc/tutorial/workspaces) (`go.work`) with three modules:

| Module | Path | Purpose |
|--------|------|---------|
| Host/CLI | `.` | Core engine, config, plan parsing, plugin loading, CLI |
| SDK | `sdk/` | Public interfaces for plugin authors |
| Azure plugin | `plugins/azurerm/` | Reference plugin for Azure deep inspection |

All `make` commands run across the full workspace from the repo root.

```bash
make build          # Build CLI -> bin/tfclassify
make build-all      # Build CLI + azurerm plugin
```

## Running Tests

### Unit Tests

```bash
make test           # Run all tests across workspace
make vet            # Static analysis
make lint           # Linter
```

Run a single test:

```bash
go test ./internal/classify/ -run TestClassifier_Deletion
go test ./plugins/azurerm/ -run TestPrivilege
```

### End-to-End Tests

E2E scenarios live in `testdata/e2e/`. Each has `main.tf`, `.tfclassify.hcl`, and `expected.json`. Use the runner script with `--build` to compile from source:

```bash
bash testdata/e2e/run.sh --build --plan-only -t blast-radius -t role-assignment-privileged
```

Add `--plan-only` to skip apply/destroy for faster iteration. Use `-t NAME` (repeatable) to run specific scenarios.

To run all E2E scenarios against a published release:

```bash
bash testdata/e2e/run.sh --version 0.4.0 --plan-only -t blast-radius
```

### Vulnerability Check

CI enforces this -- run before committing:

```bash
govulncheck ./...
```

## Plugin Authoring

Plugins provide provider-specific deep inspection (Layer 3) via gRPC. For a full guide, see [docs/plugin-authoring.md](docs/plugin-authoring.md) and the [SDK README](sdk/README.md).

Quick overview:

1. Create a new Go module depending on `github.com/jokarl/tfclassify/sdk`
2. Implement `sdk.Analyzer` (or `sdk.ClassificationAwareAnalyzer` for graduated thresholds)
3. Group analyzers in a `sdk.PluginSet` (embed `sdk.BuiltinPluginSet` for defaults)
4. Serve via `sdkplugin.Serve()` in `main.go`
5. Build as `tfclassify-plugin-{name}` and place in a plugin search path

The `plugins/azurerm/` directory is the reference implementation.

## Pull Request Guidelines

### Before Submitting

1. Run the full test and lint suite:
   ```bash
   make test && make vet && make lint && govulncheck ./...
   ```
2. If you changed plugin analyzers, config parsing, or classification logic, verify that existing E2E scenarios still pass and add new scenarios if needed.
3. If you changed protobuf definitions, regenerate:
   ```bash
   protoc --go_out=. --go-grpc_out=. proto/tfclassify.proto
   ```

### Governance Process

This project uses Architecture Decision Records (ADRs) and Change Requests (CRs) for tracking decisions and changes:

- **ADRs** (`docs/adr/`): Record significant architectural decisions. Use the MADR format with status frontmatter (`proposed`, `accepted`, `deprecated`, `superseded`).
- **CRs** (`docs/cr/`): Track implementation work. Use Gherkin acceptance criteria and RFC 2119 keywords. Checkpoint commits follow `checkpoint(CR-xxxx): {summary}` format.

If your change involves a new analyzer, a new CLI command, or a structural change, open a CR first. For architectural shifts (new plugin protocol, new output format), open an ADR.

### PR Structure

- Keep PRs focused on a single concern.
- Reference related CRs and ADRs in the PR description.
- E2E test scenarios in `testdata/e2e/` must be kept in sync with code changes. The CI matrix in `.github/workflows/ci.yml` must include all scenarios.

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- The project uses `golangci-lint` -- run `make lint` to check.
- Prefer table-driven tests.
- Use safe type assertions when working with `map[string]interface{}` from Terraform plan data.
- Plugin SDK interfaces are in `sdk/` -- keep them minimal. Use `BuiltinPluginSet` to provide defaults so plugin authors only implement what they need.

## Reporting Issues

Use the [GitHub issue tracker](https://github.com/jokarl/tfclassify/issues). See the issue templates for bug reports and feature requests.
