# tfclassify

Classify Terraform plan changes based on organization-defined rules. tfclassify analyzes `terraform show -json` output and categorizes each resource change (critical, review, standard, auto-approved, etc.) so you can automate change approval workflows.

**How is this different from Trivy or Checkov?** Tools like [Trivy](https://trivy.dev/) and [Checkov](https://www.checkov.io/) scan Terraform configs and plans for *misconfigurations* — "is this storage account public?", "does this security group allow 0.0.0.0/0?". Even when scanning plan files, they evaluate the planned final state against security checks. They answer **"is this resource configured correctly?"** tfclassify answers a different question: **"how risky is this change?"** It analyzes the *diff* between before and after states and classifies changes into organizational categories that drive your approval workflow. A storage account might pass every Checkov check but still require security team sign-off because it's being *deleted* in production. tfclassify handles that routing; misconfiguration scanners don't.

## Table of Contents

- [How It Works](#how-it-works)
- [Quick Start](#quick-start)
- [CLI Reference](#cli-reference)
  - [Root Command](#root-command)
  - [Init Command](#init-command)
  - [Validate Command](#validate-command)
  - [Explain Command](#explain-command)
  - [Verify Command](#verify-command)
  - [Output Formats](#output-formats)
- [Configuration](#configuration)
  - [Config Discovery](#config-discovery)
  - [Classification Rules](#classification-rules)
  - [Rule Fields](#rule-fields)
  - [Precedence](#precedence)
  - [Defaults](#defaults)
  - [Blast Radius](#blast-radius)
  - [Plugin Declarations](#plugin-declarations)
  - [Evidence](#evidence)
- [Plan File Formats](#plan-file-formats)
- [Three-Layer Classification Model](#three-layer-classification-model)
  - [Layer 1: Core Rules](#layer-1-core-rules)
  - [Layer 2: Builtin Analyzers](#layer-2-builtin-analyzers)
  - [Layer 3: Deep Inspection Plugins](#layer-3-deep-inspection-plugins)
- [Examples](#examples)
- [E2E Test Scenarios](#e2e-test-scenarios)
- [CI/CD Integration](#cicd-integration)
- [Project Structure](#project-structure)
- [Development](#development)
- [Architecture Decisions](#architecture-decisions)
- [Known Limitations](#known-limitations)

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
| `--evidence-file` | | | Write evidence artifact to file (see [Evidence](#evidence)) |

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

### Validate Command

```
tfclassify validate [flags]
```

Checks `.tfclassify.hcl` for errors without requiring a Terraform plan. Validates HCL syntax, classification references in precedence/defaults, precedence ordering, glob patterns, and plugin references.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | auto-discover | Path to `.tfclassify.hcl` config file |

**Exit codes:**
- `0` — config is valid (warnings, if any, are printed to stderr)
- `1` — config has errors

```bash
# Validate before committing config changes
tfclassify validate -c .tfclassify.hcl

# Use in CI to catch config drift early (no plan needed)
tfclassify validate
```

### Explain Command

```
tfclassify explain [flags]
```

Traces classification decisions for each resource through the full pipeline — core rules, builtin analyzers, and plugins. Useful for debugging why a resource received a particular classification.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--plan` | `-p` | (required) | Path to Terraform plan file (JSON or binary) |
| `--config` | `-c` | auto-discover | Path to `.tfclassify.hcl` config file |
| `--output` | `-o` | `text` | Output format: `text`, `json` |
| `--resource` | `-r` | (all) | Filter to specific resource addresses (repeatable) |

```bash
# Explain all resources
tfclassify explain -p tfplan

# Explain specific resources with JSON output
tfclassify explain -p tfplan -r azurerm_role_assignment.admin -o json

# Filter multiple resources
tfclassify explain -p tfplan -r azurerm_role_assignment.admin -r azurerm_key_vault.main
```

### Verify Command

```
tfclassify verify [flags]
```

Verifies the Ed25519 signature of a tfclassify evidence artifact. Exits 0 if the signature is valid, 1 if invalid.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--evidence-file` | `-e` | (required) | Path to evidence artifact JSON |
| `--public-key` | `-k` | (required) | Path to Ed25519 PEM public key |

```bash
# Verify an evidence artifact
tfclassify verify --evidence-file evidence.json --public-key signing.pub
```

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
      "matched_rules": ["Deleting IAM or role resources requires security review"]
    }
  ]
}
```

**GitHub** — sets GitHub Actions output variables (`classification`, `exit_code`, `no_changes`, `resource_count`) in `GITHUB_OUTPUT` file format.

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
| `not_actions` | list of strings | Inverse of `actions` — matches all actions EXCEPT those listed. Cannot combine with `actions` in the same rule. Same valid values as `actions`. |

### Action Values

These values come directly from the Terraform plan JSON format — tfclassify does not define or remap them.

| Action | When Terraform emits it |
|--------|------------------------|
| `create` | A new resource will be created. |
| `update` | An existing resource will be modified in-place (no recreation). |
| `delete` | An existing resource will be destroyed. Also appears as part of replacement (see below). |
| `read` | A data source will be read during apply. Only appears for `data` resources whose values are not yet known at plan time. |
| `no-op` | Terraform evaluated the resource and found no difference between config and state. The resource appears in the plan but nothing will change. |

`not_actions` is useful when you want to match "everything except" a small set. For example, matching all real changes (excluding no-ops):

```hcl
rule {
  resource    = ["*"]
  not_actions = ["no-op"]
}
```

`actions` and `not_actions` are mutually exclusive — specifying both in the same rule is a validation error. Omitting both matches all actions.

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

### Blast Radius

The optional `blast_radius` block inside a classification triggers when plan-wide change counts exceed configured thresholds. When any threshold is exceeded, **all** resources in the plan receive a decision at that classification level.

```hcl
classification "critical" {
  description = "Requires security team approval"

  blast_radius {
    max_deletions    = 5    # Standalone deletions (delete without create)
    max_replacements = 10   # Replacements (delete + create pairs)
    max_changes      = 50   # All resources with non-no-op actions
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `max_deletions` | int | Trigger when standalone deletions exceed this count |
| `max_replacements` | int | Trigger when replacements (destroy + recreate) exceed this count |
| `max_changes` | int | Trigger when total non-no-op changes exceed this count |

All fields are optional. Omitted fields are not evaluated. Values must be positive integers. Multiple classifications can define different thresholds for graduated blast radius detection.

### Plugin Declarations

```hcl
plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"   # GitHub repo for download
  version = "0.1.0"                            # Semantic version
}

# Plugin analyzer configuration lives inside classification blocks.
# Each plugin can define per-analyzer sub-blocks:
classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  azurerm {
    privilege_escalation {
      actions = ["Microsoft.Authorization/*"]   # Control-plane patterns
    }
  }
}
```

See the [full-reference example](docs/examples/full-reference/.tfclassify.hcl) for an annotated configuration demonstrating every field.

### Evidence

The optional `evidence` block configures evidence artifact output for audit retention. When present, tfclassify produces a self-contained JSON file alongside the normal output, including input hashes, timestamps, classification results, and an optional Ed25519 signature for tamper evidence.

```hcl
evidence {
  include_resources = true     # Include per-resource decisions (default: true)
  include_trace     = false    # Include full explain trace (default: false)
  signing_key       = "$TFCLASSIFY_SIGNING_KEY"  # Ed25519 PEM private key path
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `include_resources` | bool | `true` | Include per-resource classification decisions in the artifact |
| `include_trace` | bool | `false` | Include full explain trace (every rule evaluated per resource) |
| `signing_key` | string | | Path to Ed25519 PEM private key. Supports `$ENV_VAR` expansion. When set, the artifact includes `signature` and `signed_content_hash` fields |

Use `--evidence-file` on the root command to specify the output path. If the `evidence` block is present but `--evidence-file` is omitted, tfclassify writes to the current directory with an auto-generated filename and warns to stderr.

Verify a signed artifact with:

```bash
tfclassify verify --evidence-file evidence.json --public-key signing.pub
```

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

| Analyzer | Detects |
|----------|---------|
| `deletion` | Standalone resource deletions (not part of a replacement) |
| `replace` | Resource replacements (destroy + recreate) |
| `sensitive` | Changes to Terraform-marked sensitive attributes |
| `blast_radius` | Plan-wide change counts exceeding configured thresholds |

Builtin analyzers are always enabled and require no configuration, except `blast_radius` which requires a `blast_radius {}` block inside a classification (see [Blast Radius](#blast-radius)).

### Layer 3: Deep Inspection Plugins

Provider-specific analysis via external plugins. Plugins run as separate processes communicating over gRPC ([hashicorp/go-plugin](https://github.com/hashicorp/go-plugin)). They inspect actual resource attribute values — such as role permissions and effective actions — using pattern-based detection configured per-classification.

**Available plugins:**

| Plugin | Documentation | Detects |
|--------|--------------|---------|
| [azurerm](plugins/azurerm/) | [plugins/azurerm/README.md](plugins/azurerm/README.md) | Privilege escalation |

**Plugin discovery:** plugins are discovered as `tfclassify-plugin-{name}` binaries in:

1. `TFCLASSIFY_PLUGIN_DIR` environment variable
2. `.tfclassify/plugins/` in the current directory
3. `~/.tfclassify/plugins/` in the home directory

**Decision aggregation:** plugin decisions are merged with core results via the precedence system. A plugin can escalate a resource's classification but never lower it.

**Building custom plugins:** see the [Plugin SDK documentation](sdk/README.md) and [plugin authoring guide](docs/plugin-authoring.md).

## Examples

### Full Reference Configuration

The [full-reference example](docs/examples/full-reference/) is the canonical annotated configuration demonstrating every `.tfclassify.hcl` capability: five precedence levels, multiple rules per classification, `not_resource` exclusions, classification-scoped plugin config with graduated thresholds, and all `defaults` options.

```bash
tfclassify \
  -p docs/examples/full-reference/plan.json \
  -c docs/examples/full-reference/.tfclassify.hcl \
  -v
```

### Learning Path via E2E Scenarios

The [e2e test scenarios](testdata/e2e/) serve as a progressive learning path from simple to advanced. Each scenario is CI-tested against real Azure infrastructure:

| Concept | Scenario | What It Shows |
|---------|----------|---------------|
| Glob matching basics | [route-table](testdata/e2e/route-table/) | `resource = ["*"]` catch-all, no plugins |
| Action filtering | [role-assignment-reader](testdata/e2e/role-assignment-reader/) | Same type classified differently based on action |
| Plugin deep inspection | [role-assignment-privileged](testdata/e2e/role-assignment-privileged/) | Permission-based detection via azurerm plugin |
| Graduated thresholds | [role-escalation-threshold](testdata/e2e/role-escalation-threshold/) | Different action patterns per classification level |
| Role exclusions | [role-exclusion](testdata/e2e/role-exclusion/) | `exclude` list bypasses plugin detection |
| Module support | [modules-pluginless](testdata/e2e/modules-pluginless/) | Resources inside Terraform modules |
| CIS benchmark mapping | [cis-azure-foundations](testdata/e2e/cis-azure-foundations/) | Classifications named after CIS controls |

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
| [role-assignment-privileged](testdata/e2e/role-assignment-privileged/) | `actions = ["Microsoft.Authorization/*"]` on critical | Owner role at RG scope. Owner's effective actions include `Microsoft.Authorization/*`, matching the critical pattern. |
| [role-escalation-threshold](testdata/e2e/role-escalation-threshold/) | `actions = ["Microsoft.Authorization/*"]` on critical, `actions = ["*/write", "*/delete", "*/action"]` on standard | **Graduated patterns.** Owner (has `Microsoft.Authorization/*`) triggers `critical`. Contributor (has write/delete but `NotActions: Microsoft.Authorization/*`) falls to `standard`. |
| [role-exclusion](testdata/e2e/role-exclusion/) | `exclude = ["AcrPush"]` on both critical and standard | AcrPush role is excluded from privilege escalation detection in all classifications. Falls through to core rule `standard` on create, `critical` on destroy (glob `*_role_*` + `delete`). |
| [custom-role-cross-reference](testdata/e2e/custom-role-cross-reference/) | `actions = ["Microsoft.Authorization/*"]` on critical | Custom role with `Microsoft.Authorization/roleAssignments/write`. Plugin cross-references the role definition from the plan to resolve effective actions, matching the critical pattern. |

### Deep Inspection: Data-Plane and Control-Plane Patterns

| Scenario | Config | What It Tests |
|----------|--------|---------------|
| [data-plane-detection](testdata/e2e/data-plane-detection/) | `data_actions = ["Microsoft.Storage/*"]` on critical | **CR-0027.** Storage Blob Data Owner triggers `critical` via data-plane pattern matching (`Microsoft.Storage/*/blobs/*` matches `Microsoft.Storage/*`). Reader (no data actions) falls through to `standard` via control-plane `*/read` pattern. |
| [control-plane-patterns](testdata/e2e/control-plane-patterns/) | `actions = ["Microsoft.Authorization/*", "*"]` on critical, `actions = ["*/read"]` on standard | **CR-0028.** User Access Administrator (has `Microsoft.Authorization/*`) triggers `critical`. Reader (only `*/read`) triggers `standard`. Demonstrates pattern-based control-plane detection. |

### Module Support

| Scenario | Config | What It Tests |
|----------|--------|---------------|
| [modules-pluginless](testdata/e2e/modules-pluginless/) | Glob rules only, no plugins | Resources created inside Terraform modules are classified correctly. Module-expanded addresses (e.g., `module.network.azurerm_network_security_group.nsg`) match glob patterns on the resource type. |
| [modules-plugin](testdata/e2e/modules-plugin/) | Plugin with `privilege_escalation` | Resources inside modules are passed to plugins for deep inspection. Module-expanded role assignments are analyzed for privilege escalation patterns. |

### Compliance Benchmark Mapping

| Scenario | Config | What It Tests |
|----------|--------|---------------|
| [cis-azure-foundations](testdata/e2e/cis-azure-foundations/) | `privilege_escalation` across CIS-named classifications | Classifications named after CIS Azure Foundations Benchmark sections. Demonstrates mapping CIS 1.23 (no privileged role assignments) to tfclassify analyzers. No special compliance feature needed — classification and rule names carry the CIS references directly. |

### How E2E Tests Run

Each scenario is executed by the reusable [e2e.yml](.github/workflows/e2e.yml) workflow:

1. `terraform plan -out=create.tfplan` against real Azure infrastructure
2. `tfclassify` classifies the create plan and compares exit code to `expected.json`
3. `terraform apply` to create the resources
4. `terraform plan -destroy -out=destroy.tfplan`
5. `tfclassify` classifies the destroy plan and compares exit code
6. `terraform destroy` to clean up

CI ([ci.yml](.github/workflows/ci.yml)) runs all scenarios on PRs and pushes to main, building from source with both JSON and binary plan formats. The nightly [verify.yml](.github/workflows/verify.yml) runs against published releases.

## CI/CD Integration

### GitHub Actions

```yaml
- name: Install tfclassify
  run: |
    curl -sSL https://github.com/jokarl/tfclassify/releases/latest/download/tfclassify_linux_amd64.tar.gz | tar xz
    chmod +x tfclassify
    sudo mv tfclassify /usr/local/bin/

- name: Install plugins
  run: tfclassify init

- name: Validate config
  run: tfclassify validate

- name: Classify plan
  id: classify
  run: tfclassify -p tfplan --output github --detailed-exitcode
  continue-on-error: true

- name: Gate on classification
  run: |
    echo "Classification: ${{ steps.classify.outputs.classification }}"
    echo "Exit code: ${{ steps.classify.outputs.exit_code }}"
    if [ "${{ steps.classify.outputs.classification }}" = "critical" ]; then
      echo "::error::Critical changes detected — requires security team approval"
      exit 1
    fi
```

### Generic CI

```bash
# Validate config (no plan needed — fast, catches config drift early)
tfclassify validate

# Generate and classify the plan
terraform plan -out=tfplan
tfclassify -p tfplan --output json --detailed-exitcode > classification.json
EXIT_CODE=$?

# Route based on exit code
case $EXIT_CODE in
  0) echo "Auto-approved" ;;
  1) echo "Standard review required" ;;
  2) echo "Critical — blocking pipeline" && exit 1 ;;
esac
```

The `--output github` format sets GitHub Actions output variables (`classification`, `exit_code`, `no_changes`, `resource_count`) via the `GITHUB_OUTPUT` file. The `--output json` format produces machine-readable JSON for other CI systems.

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
├── internal/
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
├── testdata/e2e/          # End-to-end test scenarios (CI-tested)
├── docs/
│   ├── adr/               # Architecture Decision Records
│   ├── cr/                # Change Requests
│   ├── examples/          # Annotated reference configuration
│   └── plugin-authoring.md
├── go.work                # Go workspace tying all three modules together
└── Makefile
```

## Development

```bash
make build                    # Build CLI → bin/tfclassify
make build-all                # Build CLI + azurerm plugin
make test                     # Run all tests across workspace (go test ./...)
make vet                      # Static analysis (go vet ./...)
make lint                     # golangci-lint run ./...
make generate-roles           # Refresh Azure role database from AzAdvertizer CSV
make generate-actions         # Refresh Azure action registry from Microsoft Docs + role database
make generate-actions-offline # Refresh Azure action registry from role database only (no network)
make clean                    # Remove build artifacts
```

Run a single test:

```bash
go test ./internal/classify/ -run TestClassifier_Deletion
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

## Known Limitations

- **Sensitive attribute detection is top-level only.** The builtin `sensitive` analyzer detects changes to Terraform-marked sensitive attributes at the top level of a resource. Sensitive values nested inside objects or lists are not detected.

- **Blast radius thresholds count all resources including no-ops.** The `max_changes` threshold in `blast_radius` applies to every resource with a non-no-op action. There is no way to scope thresholds to specific resource types or exclude certain actions from the count.

- **Plugin analyzer errors are silently skipped.** If an external plugin analyzer returns an error during analysis, the error is logged but does not fail the classification run. This means plugin failures may produce incomplete results without an obvious indication in the output. Check verbose (`-v`) output or the explain command if you suspect a plugin is not producing expected decisions.
