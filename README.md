# tfclassify

Classify Terraform plan changes based on organization-defined rules. tfclassify analyzes `terraform show -json` output and categorizes each resource change (critical, review, standard, auto-approved, etc.) so you can automate change approval workflows.

## Table of Contents

- [How It Works](#how-it-works)
- [Quick Start](#quick-start)
- [CLI Reference](#cli-reference)
  - [Root Command](#root-command)
  - [Init Command](#init-command)
  - [Output Formats](#output-formats)
- [Configuration](#configuration)
  - [Config Discovery](#config-discovery)
  - [Classification Rules](#classification-rules)
  - [Rule Fields](#rule-fields)
  - [Precedence](#precedence)
  - [Defaults](#defaults)
  - [Plugin Declarations](#plugin-declarations)
- [Plan File Formats](#plan-file-formats)
- [Three-Layer Classification Model](#three-layer-classification-model)
  - [Layer 1: Core Rules](#layer-1-core-rules)
  - [Layer 2: Builtin Analyzers](#layer-2-builtin-analyzers)
  - [Layer 3: Deep Inspection Plugins](#layer-3-deep-inspection-plugins)
- [Examples](#examples)
- [E2E Test Scenarios](#e2e-test-scenarios)
- [Project Structure](#project-structure)
- [Development](#development)
- [Architecture Decisions](#architecture-decisions)

## How It Works

1. You define classification rules in `.tfclassify.hcl` using glob patterns on resource types and actions
2. tfclassify parses a Terraform plan and evaluates each resource change against your rules
3. Rules are checked in precedence order — the first match wins
4. The overall classification (and exit code) is determined by the highest-precedence match across all resources

```
# Binary plans work directly
tfclassify -p tfplan

# Or use JSON
terraform show -json tfplan > plan.json
tfclassify -p plan.json
```

By default, tfclassify exits `0` for any successful classification (CI-friendly). Use `--detailed-exitcode` to get classification-based exit codes for pipeline gating:

```bash
# Default: exit 0 on success, parse output for classification
tfclassify -p tfplan --output json | jq .overall

# With --detailed-exitcode: non-zero exit for non-auto classifications
tfclassify -p tfplan --detailed-exitcode
```

## Quick Start

### Build

```bash
make build
# Output: bin/tfclassify
```

### Configure

Create `.tfclassify.hcl` in your project root:

```hcl
# "critical" — any deletion of identity/access resources.
# The "resource" field accepts glob patterns matched against the Terraform
# resource type (e.g. "azurerm_role_assignment", "aws_iam_role").
classification "critical" {
  description = "Requires security team approval"

  rule {
    description = "Deleting IAM or role resources requires security review"
    resource    = ["*_role_*", "*_iam_*"]
    actions     = ["delete"]
    # Omit to match all actions.
  }
}

# "standard" — everything not caught above.
# Using resource = ["*"] as a catch-all is safe because the classifier evaluates
# rules in precedence order: critical is checked first, so this only catches
# resources that didn't match anything above.
classification "standard" {
  description = "Standard change process"

  rule {
    description = "All infrastructure changes not covered above"
    resource    = ["*"]
  }
}

# "auto" — no-op changes (Terraform evaluated but found no changes).
classification "auto" {
  description = "Automatic approval"

  rule {
    description = "No actual changes detected"
    resource    = ["*"]
    actions     = ["no-op"]
  }
}

# Precedence controls evaluation order AND exit codes.
# Exit codes: auto=0, standard=1, critical=2
precedence = ["critical", "standard", "auto"]

defaults {
  unclassified = "standard"   # Resources matching no rule
  no_changes   = "auto"       # Plans with zero resource changes
}
```

### Run

```bash
# Use binary plan directly (auto-detected)
tfclassify -p tfplan -v

# Or use JSON
terraform show -json tfplan > plan.json
tfclassify -p plan.json -v
```

### Output

```
Classification: critical
Exit code: 2
Resources: 3

[critical] (1 resources)
  Requires security team approval
  - azurerm_role_assignment.admin (azurerm_role_assignment) [delete]
    Rule: Deleting IAM or role resources requires security review

[standard] (2 resources)
  Standard change process
  - azurerm_virtual_network.main (azurerm_virtual_network) [create]
    Rule: All infrastructure changes not covered above
  - azurerm_resource_group.production (azurerm_resource_group) [create]
    Rule: All infrastructure changes not covered above
```

## CLI Reference

### Root Command

```
tfclassify [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--plan` | `-p` | (required) | Path to Terraform plan file (JSON or binary) |
| `--config` | `-c` | auto-discover | Path to `.tfclassify.hcl` config file |
| `--output` | `-o` | `text` | Output format: `text`, `json`, `github` |
| `--verbose` | `-v` | `false` | Show per-resource rule match details |
| `--detailed-exitcode` | `-d` | `false` | Use classification-based exit codes (see below) |

### Exit Codes

By default, tfclassify exits `0` for any successful classification, making it CI-friendly. Non-zero exit codes are reserved for errors (e.g., config load failure, plan parse failure).

With `--detailed-exitcode`, tfclassify uses classification-based exit codes:

| Precedence position | Exit code | Typical meaning |
|---------------------|-----------|-----------------|
| 1st (highest) | N-1 | Critical — block pipeline |
| 2nd | N-2 | Review — require approval |
| ... | ... | ... |
| Last (lowest) | 0 | Auto — proceed |

The `exit_code` field in JSON and GitHub output formats always contains the precedence-derived code, regardless of the `--detailed-exitcode` flag.

### Init Command

```
tfclassify init [flags]
```

Downloads and installs plugin binaries declared in your configuration from GitHub releases.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | auto-discover | Path to `.tfclassify.hcl` config file |

Supports `GITHUB_TOKEN` environment variable for authenticated requests.

### Output Formats

**Text** (default) — human-readable summary. With `-v`, groups resources by classification and shows matched rules.

**JSON** — machine-readable output for CI/CD integration:

```json
{
  "overall": "critical",
  "overall_description": "Requires security team approval",
  "exit_code": 2,
  "no_changes": false,
  "resources": [
    {
      "address": "azurerm_role_assignment.admin",
      "type": "azurerm_role_assignment",
      "actions": ["delete"],
      "classification": "critical",
      "classification_description": "Requires security team approval",
      "matched_rule": "Deleting IAM or role resources requires security review"
    }
  ]
}
```

**GitHub** — sets GitHub Actions output variables (`classification`, `exit_code`, `no_changes`, `resource_count`) in both legacy `::set-output` and `GITHUB_OUTPUT` file formats.

## Configuration

### Config Discovery

Config files are discovered in order:

1. Explicit path via `--config`
2. `.tfclassify.hcl` in the current directory
3. `.tfclassify.hcl` in the home directory

### Classification Rules

Each `classification` block contains one or more `rule` blocks. A resource matches a classification if it matches **any** of its rules.

```hcl
classification "review" {
  description = "Requires team lead review"

  # Rule 1: any change to security or firewall rules
  rule {
    description = "Network security changes affect access controls"
    resource    = ["*_security_rule", "*_firewall_*"]
  }

  # Rule 2: any change to key vault children
  rule {
    description = "Key vault secret/key changes need review"
    resource    = ["*_key_vault_*"]
  }
}
```

### Rule Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Optional. Appears in verbose and JSON output next to each classified resource, explaining **why** the rule matched. Without it, an auto-generated description is used (e.g. `critical rule 1 (resource: *_role_*, ...)`). |
| `resource` | list of globs | Resource type must match at least one pattern. |
| `not_resource` | list of globs | Resource type must match **none** of the patterns. Cannot combine with `resource` in the same rule. |
| `actions` | list of strings | Terraform plan action. Omit to match all actions. See **Action Values** below. |

### Action Values

These values come directly from the Terraform plan JSON format — tfclassify does not define or remap them.

| Action | When Terraform emits it |
|--------|------------------------|
| `create` | A new resource will be created. |
| `update` | An existing resource will be modified in-place (no recreation). |
| `delete` | An existing resource will be destroyed. Also appears as part of replacement (see below). |
| `read` | A data source will be read during apply. Only appears for `data` resources whose values are not yet known at plan time. |
| `no-op` | Terraform evaluated the resource and found no difference between config and state. The resource appears in the plan but nothing will change. |

**Replacement** is not a single action — it is a composite of `["delete", "create"]` (destroy-then-create) or `["create", "delete"]` (create-before-destroy, when `lifecycle { create_before_destroy = true }` is set). A rule with `actions = ["delete"]` will match replacements because the action list contains `"delete"`.

Glob patterns use `*` to match any sequence of characters. Some useful patterns:

| Pattern | Matches | Does Not Match |
|---------|---------|----------------|
| `*_role_*` | `azurerm_role_assignment`, `aws_iam_role_policy` | `azurerm_resource_group` |
| `*_key_vault` | `azurerm_key_vault` | `azurerm_key_vault_secret` |
| `*_key_vault_*` | `azurerm_key_vault_secret`, `azurerm_key_vault_key` | `azurerm_key_vault` |
| `*` | Everything | — |

### Precedence

The `precedence` list controls two things:

1. **Evaluation order** — rules in the first classification are checked before the second, and so on. First match wins.
2. **Exit codes** — last entry = 0, codes increase toward the first entry.

```hcl
precedence = ["critical", "review", "standard", "auto"]
# Exit codes:  3          2         1           0
```

The overall exit code is the highest across all resources. One critical resource makes the entire plan exit 3.

### Defaults

```hcl
defaults {
  unclassified   = "standard"   # Resources matching no rule
  no_changes     = "auto"       # Plans with zero resource changes
  plugin_timeout = "30s"        # Timeout for external plugin execution
}
```

`unclassified` and `no_changes` must reference classification names from the precedence list. `plugin_timeout` accepts Go duration strings (e.g. `"10s"`, `"2m30s"`).

### Plugin Declarations

```hcl
plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"   # GitHub repo for download
  version = "0.1.0"                            # Semantic version

  config {
    # Plugin-specific options (opaque to core, forwarded via gRPC)
    privilege_enabled = true
    network_enabled   = true
    keyvault_enabled  = true
  }
}
```

See the [full-reference example](docs/examples/full-reference/.tfclassify.hcl) for an annotated configuration demonstrating every field.

## Plan File Formats

tfclassify accepts both JSON and binary Terraform plan files:

| Format | How to generate | Detection |
|--------|-----------------|-----------|
| JSON | `terraform show -json tfplan > plan.json` | First byte is `{` |
| Binary | `terraform plan -out=tfplan` | ZIP magic bytes (`PK`) |

Supported format versions: `0.2`, `1.0`, `1.1`, `1.2`.

When a binary plan is detected, tfclassify automatically invokes `terraform show -json` to convert it. The `terraform` (or `tofu`) binary must be on PATH, or set via `TERRAFORM_PATH` env var.

```bash
# Direct binary plan support — no manual conversion needed
terraform plan -out=tfplan
tfclassify -p tfplan
```

## Three-Layer Classification Model

tfclassify classifies resources through three progressively deeper layers:

### Layer 1: Core Rules

Config-driven pattern matching. Glob patterns on resource types and actions, evaluated against the precedence order from `.tfclassify.hcl`. This is the fast path — no plugins involved.

### Layer 2: Builtin Analyzers

Cross-provider heuristics that run in-process after core rules. These detect Terraform-level concepts regardless of provider:

| Analyzer | Detects | Severity |
|----------|---------|----------|
| `deletion` | Standalone resource deletions (not replacements) | 80 |
| `replace` | Resource replacements (destroy + recreate) | 75 |
| `sensitive` | Changes to Terraform-marked sensitive attributes | 70 |

Builtin analyzers are always enabled and require no configuration.

### Layer 3: Deep Inspection Plugins

Provider-specific analysis via external plugins. Plugins run as separate processes communicating over gRPC ([hashicorp/go-plugin](https://github.com/hashicorp/go-plugin)). They inspect actual resource attribute values — role permissions, network CIDRs, access grants — to produce graduated severity scores.

**Available plugins:**

| Plugin | Documentation | Detects |
|--------|--------------|---------|
| [azurerm](plugins/azurerm/) | [plugins/azurerm/README.md](plugins/azurerm/README.md) | Privilege escalation, network exposure, destructive key vault permissions |

**Plugin discovery:** plugins are discovered as `tfclassify-plugin-{name}` binaries in:

1. `TFCLASSIFY_PLUGIN_DIR` environment variable
2. `.tfclassify/plugins/` in the current directory
3. `~/.tfclassify/plugins/` in the home directory

**Decision aggregation:** plugin decisions are merged with core results via the precedence system. A plugin can escalate a resource's classification but never lower it.

**Building custom plugins:** see the [Plugin SDK documentation](sdk/README.md) and [plugin authoring guide](docs/plugin-authoring.md).

## Examples

Working examples with sample plan JSON, annotated config files, and expected output:

| Example | Demonstrates |
|---------|-------------|
| [basic-classification](docs/examples/basic-classification/) | Resource type glob matching, `resource = ["*"]` catch-all, exit codes |
| [action-filtering](docs/examples/action-filtering/) | Action-specific rules, same type with different classifications based on action |
| [mixed-changes](docs/examples/mixed-changes/) | Multiple rules per classification, glob precision (`*_key_vault` vs `*_key_vault_*`) |
| [full-reference](docs/examples/full-reference/) | Every configurable field annotated: plugins, `not_resource`, `plugin_timeout`, five precedence levels |

Each example directory contains a `.tfclassify.hcl`, a `plan.json`, and a `README.md` with run instructions and expected output. Run any example:

```bash
tfclassify \
  -p docs/examples/basic-classification/plan.json \
  -c docs/examples/basic-classification/.tfclassify.hcl \
  -v
```

## E2E Test Scenarios

The [`testdata/e2e/`](testdata/e2e/) directory contains end-to-end test scenarios that run against real Azure infrastructure via GitHub Actions CI. Each scenario has a `main.tf` (Terraform config), `.tfclassify.hcl` (classification rules), and `expected.json` (expected exit codes for create and destroy phases).

These scenarios demonstrate real-world classification behavior across all three layers — core rules, builtin analyzers, and deep inspection plugins.

### Core Rule Scenarios

| Scenario | Config | What It Tests |
|----------|--------|---------------|
| [route-table](testdata/e2e/route-table/) | Glob rules only, no plugins | Baseline: route table + route classified as `standard` by `resource = ["*"]` catch-all. No plugin involvement. |
| [role-assignment-reader](testdata/e2e/role-assignment-reader/) | Glob rules only, no plugins | Reader role assignment classified as `standard` on create (catch-all), `critical` on destroy (matches `*_role_*` + `delete`). |

### Deep Inspection: Privilege Escalation

| Scenario | Config | What It Tests |
|----------|--------|---------------|
| [role-assignment-privileged](testdata/e2e/role-assignment-privileged/) | Plugin enabled, default config | Owner role assignment at RG scope. Plugin scores permissions (tier 1, score 95 * 0.8 = 76) and emits escalation decision. No `score_threshold` configured so any score triggers. |
| [role-escalation-threshold](testdata/e2e/role-escalation-threshold/) | `score_threshold = 70` on critical, default on standard | **Graduated thresholds.** Owner (76 at RG) triggers `critical` (>= 70). Contributor (56 at RG) skips critical, falls through to `standard` (any score). Demonstrates per-classification threshold gating. |
| [role-exclusion](testdata/e2e/role-exclusion/) | `exclude = ["AcrPush"]` on both critical and standard | AcrPush role is excluded from privilege escalation detection in all classifications. Falls through to core rule `standard` on create, `critical` on destroy (glob `*_role_*` + `delete`). |

### Deep Inspection: Network Exposure

| Scenario | Config | What It Tests |
|----------|--------|---------------|
| [nsg-open-inbound](testdata/e2e/nsg-open-inbound/) | Glob rules only (no plugin block) | NSG rule allowing `*` inbound from `*`. Classified as `standard` on create (no network exposure plugin configured), `critical` on destroy (glob `*_security_rule` + `delete`). |

### Deep Inspection: Key Vault

| Scenario | Config | What It Tests |
|----------|--------|---------------|
| [keyvault-destructive](testdata/e2e/keyvault-destructive/) | `keyvault_access {}` on critical | Key vault access policy granting `Delete` and `Purge` secret permissions. Plugin detects destructive permissions and emits `critical`. Demonstrates attribute-level deep inspection of permission arrays. |

### Deep Inspection: Data-Plane and Control-Plane Patterns

| Scenario | Config | What It Tests |
|----------|--------|---------------|
| [data-plane-detection](testdata/e2e/data-plane-detection/) | `data_actions = ["Microsoft.Storage/*"]`, `score_threshold = 100` on critical | **CR-0027.** Storage Blob Data Owner triggers `critical` via data-plane pattern matching. Reader (no data actions) falls through to `standard`. Control-plane threshold set to 100 to isolate data-plane triggering. |
| [control-plane-patterns](testdata/e2e/control-plane-patterns/) | `actions = ["Microsoft.Authorization/*", "*"]` on critical, `actions = ["*/read"]` on standard | **CR-0028.** User Access Administrator (has `Microsoft.Authorization/*`) triggers `critical`. Reader (only `*/read`) triggers `standard`. Demonstrates pattern-based control-plane detection replacing score thresholds. |

### How E2E Tests Run

Each scenario is executed by the reusable [e2e.yml](.github/workflows/e2e.yml) workflow:

1. `terraform plan -out=create.tfplan` against real Azure infrastructure
2. `tfclassify` classifies the create plan and compares exit code to `expected.json`
3. `terraform apply` to create the resources
4. `terraform plan -destroy -out=destroy.tfplan`
5. `tfclassify` classifies the destroy plan and compares exit code
6. `terraform destroy` to clean up

CI ([ci.yml](.github/workflows/ci.yml)) runs all scenarios on PRs and pushes to main, building from source with both JSON and binary plan formats. The nightly [verify.yml](.github/workflows/verify.yml) runs against published releases.

## Project Structure

The repository uses Go workspaces (`go.work`) with three modules:

| Module | Path | Documentation | Purpose |
|--------|------|---------------|---------|
| `github.com/jokarl/tfclassify` | `.` | This file | CLI, core engine, config, plan parsing, plugin host |
| `github.com/jokarl/tfclassify/sdk` | [`sdk/`](sdk/) | [sdk/README.md](sdk/README.md) | Plugin authoring SDK (Analyzer, Runner, PluginSet interfaces) |
| `github.com/jokarl/tfclassify-plugin-azurerm` | [`plugins/azurerm/`](plugins/azurerm/) | [plugins/azurerm/README.md](plugins/azurerm/README.md) | Azure deep inspection plugin |

```
tfclassify/
├── cmd/tfclassify/        # CLI entry point (Cobra)
├── pkg/
│   ├── classify/          # Core classification engine (Layer 1 + 2)
│   ├── config/            # HCL config loading, validation, discovery
│   ├── output/            # Output formatters (text, json, github)
│   ├── plan/              # Terraform plan JSON/binary parsing
│   └── plugin/            # Plugin discovery, installation, lifecycle
├── sdk/                   # Plugin SDK — see sdk/README.md
│   ├── plugin/            # gRPC plugin server entry point
│   └── pb/                # Generated protobuf code
├── plugins/
│   └── azurerm/           # Azure plugin — see plugins/azurerm/README.md
├── proto/                 # gRPC protocol definitions
├── docs/
│   ├── adr/               # Architecture Decision Records
│   ├── cr/                # Change Requests
│   ├── examples/          # Working examples with sample plans
│   └── plugin-authoring.md
├── go.work                # Go workspace tying all three modules together
└── Makefile
```

## Development

```bash
make build          # Build CLI → bin/tfclassify
make build-all      # Build CLI + azurerm plugin
make test           # Run all tests across workspace (go test ./...)
make vet            # Static analysis (go vet ./...)
make lint           # golangci-lint run ./...
make generate-roles # Refresh Azure role database from AzAdvertizer CSV
make clean          # Remove build artifacts
```

Run a single test:

```bash
go test ./pkg/classify/ -run TestClassifier_Deletion
go test ./plugins/azurerm/ -run TestPrivilege
```

Before committing, run vulnerability check (enforced by CI):

```bash
govulncheck ./...
```

Protobuf code generation (output in `sdk/pb/`):

```bash
protoc --go_out=. --go-grpc_out=. proto/tfclassify.proto
```

## Architecture Decisions

| ADR | Decision |
|-----|----------|
| [ADR-0001](docs/adr/ADR-0001-monorepo-with-go-workspaces.md) | Monorepo with Go workspaces |
| [ADR-0002](docs/adr/ADR-0002-grpc-plugin-architecture.md) | gRPC plugin architecture via hashicorp/go-plugin |
| [ADR-0003](docs/adr/ADR-0003-provider-agnostic-core-with-deep-inspection-plugins.md) | Provider-agnostic core with deep inspection plugins |
| [ADR-0004](docs/adr/ADR-0004-hcl-configuration-format.md) | HCL configuration format |
| [ADR-0005](docs/adr/ADR-0005-plugin-sdk-versioning-and-protocol-compatibility.md) | Plugin SDK versioning and protocol compatibility |
| [ADR-0006](docs/adr/ADR-0006-permission-based-privilege-escalation-detection.md) | Permission-based privilege escalation detection |
