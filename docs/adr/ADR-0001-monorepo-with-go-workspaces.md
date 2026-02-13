---
status: approved
date: 2026-02-13
decision-makers: Johan
---

# Monorepo with Go Workspaces for Future Multi-Repo Extraction

## Context and Problem Statement

tfclassify consists of multiple logical components: a CLI, a plugin SDK, and plugins. These will eventually live in separate repositories (CLI, SDK, and one repo per plugin) to allow independent versioning and distribution. However, starting with multiple repos introduces unnecessary coordination overhead during early development.

How should we structure the repository to enable rapid development now while making future extraction into separate repositories straightforward?

## Decision Drivers

* Rapid iteration during initial development without cross-repo coordination
* Future extraction into separate repos must not require changing import paths
* Plugin SDK must be independently importable (`go get github.com/jokarl/tfclassify/sdk`)
* Each component should have its own dependency tree to avoid pulling unnecessary transitive dependencies

## Considered Options

* Go workspaces with separate go.mod per component
* Single go.mod at repository root
* Multi-repo from the start

## Decision Outcome

Chosen option: "Go workspaces with separate go.mod per component", because it mirrors the future repository boundaries from day one while keeping everything in a single repo for development convenience.

### Module Boundaries

| Module | Path | Future Repo |
|--------|------|-------------|
| `github.com/jokarl/tfclassify` | `cmd/tfclassify/` | `tfclassify` |
| `github.com/jokarl/tfclassify/sdk` | `sdk/` | `tfclassify-sdk` |
| `github.com/jokarl/tfclassify/plugin-terraform` | `plugins/terraform/` | `tfclassify-plugin-terraform` |

A `go.work` file at the repository root ties the modules together for local development, allowing cross-module references without publishing.

### Consequences

* Good, because import paths remain stable through extraction - no consumer-facing changes needed
* Good, because each module manages its own dependencies, keeping the SDK lean for plugin authors
* Good, because `go.work` enables local development across modules without replace directives in go.mod
* Bad, because slightly more complex initial setup compared to a single go.mod
* Bad, because CI must be aware of the workspace structure

### Confirmation

* Each module directory contains its own `go.mod` with the correct module path
* `go.work` at root references all modules
* `go build ./...` succeeds from the workspace root
* Each module can be built independently when `go.work` is absent (simulating post-extraction)

## Pros and Cons of the Options

### Go workspaces with separate go.mod per component

* Good, because module paths match future repository import paths
* Good, because dependency isolation prevents SDK consumers from pulling CLI dependencies
* Good, because `go.work` handles local cross-module references transparently
* Neutral, because requires Go 1.18+ (widely available)
* Bad, because slightly more initial boilerplate (multiple go.mod files)

### Single go.mod at repository root

* Good, because simplest possible setup with zero Go module complexity
* Bad, because extraction requires changing import paths or introducing replace directives
* Bad, because SDK consumers would pull all transitive dependencies including CLI-only ones
* Bad, because harder to enforce module boundaries - any package can import any other

### Multi-repo from the start

* Good, because each component is independently versioned and released from the beginning
* Bad, because cross-repo changes during early development require coordinated PRs and releases
* Bad, because slows down initial development velocity significantly
* Bad, because premature - component boundaries may shift during early design

## More Information

Related: [ADR-0002](ADR-0002-grpc-plugin-architecture.md) defines the plugin architecture that drives the module boundaries chosen here.

Go workspaces documentation: https://go.dev/doc/tutorial/workspaces
