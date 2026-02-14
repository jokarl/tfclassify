# tfclassify

Classify Terraform plan changes based on organization-defined rules. tfclassify analyzes `terraform show -json` output and categorizes each resource change (critical, review, standard, auto-approved, etc.) so you can automate change approval workflows.

## How it works

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

Exit codes map to precedence position, making tfclassify usable as a gate in CI/CD pipelines:

| Precedence position | Exit code | Typical meaning |
|---------------------|-----------|-----------------|
| 1st (highest) | N-1 | Critical — block pipeline |
| 2nd | N-2 | Review — require approval |
| ... | ... | ... |
| Last (lowest) | 0 | Auto — proceed |

## Quick start

### Build

```bash
make build
# Output: bin/tfclassify
```

### Configure

Create `.tfclassify.hcl` in your project root:

```hcl
classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*", "*_iam_*"]
    actions  = ["delete"]
  }
}

classification "standard" {
  description = "Standard change process"

  rule {
    resource = ["*"]
  }
}

classification "auto" {
  description = "Automatic approval"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

precedence = ["critical", "standard", "auto"]

defaults {
  unclassified = "standard"
  no_changes   = "auto"
}
```

Rules are evaluated in precedence order. `resource = ["*"]` on `standard` is safe as a catch-all because `critical` is checked first.

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
  - azurerm_role_assignment.admin (azurerm_role_assignment) [delete]
    Rule: critical rule 1 (resource: *_role_*, ...)

[standard] (2 resources)
  - azurerm_virtual_network.main (azurerm_virtual_network) [create]
    Rule: standard rule 1 (resource: *)
  - azurerm_resource_group.production (azurerm_resource_group) [create]
    Rule: standard rule 1 (resource: *)
```

## CLI reference

```
tfclassify [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--plan` | `-p` | (required) | Path to Terraform plan file (JSON or binary) |
| `--config` | `-c` | auto-discover | Path to `.tfclassify.hcl` config file |
| `--output` | `-o` | `text` | Output format: `text`, `json`, `github` |
| `--verbose` | `-v` | `false` | Show per-resource rule match details |

## Configuration

tfclassify uses HCL configuration (`.tfclassify.hcl`). Config files are discovered in order:

1. Explicit path via `--config`
2. `.tfclassify.hcl` in the current directory
3. `.tfclassify.hcl` in the home directory

### Classification rules

Each `classification` block contains one or more `rule` blocks. A resource matches a classification if it matches **any** of its rules.

```hcl
classification "review" {
  description = "Requires team lead review"

  # Rule 1: any change to security or firewall rules
  rule {
    resource = ["*_security_rule", "*_firewall_*"]
  }

  # Rule 2: any change to key vault children
  rule {
    resource = ["*_key_vault_*"]
  }
}
```

**Rule fields:**

| Field | Type | Description |
|-------|------|-------------|
| `resource` | list of globs | Resource type must match at least one pattern |
| `not_resource` | list of globs | Resource type must match none of the patterns |
| `actions` | list of strings | Terraform action must be one of these (`create`, `update`, `delete`, `read`, `no-op`). Omit to match all actions |

### Precedence

The `precedence` list controls two things:

1. **Evaluation order** — rules in the first classification are checked before the second, and so on. First match wins.
2. **Exit codes** — last entry = 0, codes increase toward the first entry.

```hcl
precedence = ["critical", "review", "standard", "auto"]
# Exit codes:  3          2         1           0
```

### Defaults

```hcl
defaults {
  unclassified = "standard"   # Resources matching no rule
  no_changes   = "auto"       # Plans with zero resource changes
}
```

## Plan file formats

tfclassify accepts both JSON and binary Terraform plan files:

| Format | How to generate | Detection |
|--------|-----------------|-----------|
| JSON | `terraform show -json tfplan > plan.json` | First byte is `{` |
| Binary | `terraform plan -out=tfplan` | ZIP magic bytes (`PK`) |

When a binary plan is detected, tfclassify automatically invokes `terraform show -json` to convert it. The `terraform` binary must be on PATH (or set `TERRAFORM_PATH` env var).

```bash
# Direct binary plan support — no manual conversion needed
terraform plan -out=tfplan
tfclassify -p tfplan
```

## Plugin system

tfclassify supports plugins for deep inspection beyond pattern matching. Plugins run as separate processes communicating over gRPC.

### Installing plugins

External plugins are installed using `tfclassify init`, which reads plugin declarations from your config and downloads binaries from GitHub releases:

```bash
tfclassify init
```

Example config with an external plugin:

```hcl
plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify-plugin-azurerm"
  version = "0.1.0"
}
```

Plugin binaries are placed in `.tfclassify/plugins/` (or `TFCLASSIFY_PLUGIN_DIR`).

### Builtin analyzers

tfclassify includes three cross-provider analyzers that run in-process (no plugin required):

| Analyzer | Detects | Severity |
|----------|---------|----------|
| `deletion` | Standalone resource deletions (not replacements) | 80 |
| `replace` | Resource replacements (destroy + recreate) | 75 |
| `sensitive` | Changes to Terraform-marked sensitive attributes | 70 |

These are enabled by default and operate on Terraform-level concepts (actions, sensitive markers) rather than provider-specific semantics.

### Example plugin: azurerm

The `azurerm` plugin in `plugins/azurerm/` demonstrates deep inspection (Layer 3) analysis for Azure resources. It includes three analyzers:

| Analyzer | Detects | Severity |
|----------|---------|----------|
| `privilege-escalation` | Role changes with permission-based severity scoring (e.g., Reader → Owner) | 40-95 (graduated) |
| `network-exposure` | Permissive network rules (0.0.0.0/0, *, Internet) | 85 |
| `key-vault-access` | Destructive Key Vault permissions (purge, delete) | 80 |

The privilege escalation analyzer uses an embedded database of 400+ Azure built-in roles with full permission sets, scope-based weighting (management group > subscription > resource group > resource), and cross-references custom role definitions from the Terraform plan.

See the [plugin authoring guide](docs/plugin-authoring.md) for details on building custom plugins.

### Plugin discovery

External plugins are discovered as `tfclassify-plugin-{name}` binaries in:

1. `TFCLASSIFY_PLUGIN_DIR` environment variable
2. `.tfclassify/plugins/` in the current directory
3. `~/.tfclassify/plugins/` in the home directory

## Examples

Working examples with sample plan JSON and expected output:

| Example | Demonstrates |
|---------|-------------|
| [basic-classification](docs/examples/basic-classification/) | Resource type glob matching, `not_resource` exclusion, exit codes |
| [action-filtering](docs/examples/action-filtering/) | Action-specific rules, same type with different classifications |
| [mixed-changes](docs/examples/mixed-changes/) | Multiple rules per classification, glob precision, sensitive attributes |

Each example directory contains a `.tfclassify.hcl`, a `plan.json`, and a `README.md` with run instructions and expected output.

## Project structure

```
tfclassify/
├── cmd/tfclassify/        # CLI entry point (Cobra)
├── pkg/
│   ├── classify/          # Core classification engine (precedence-ordered rule evaluation)
│   ├── config/            # HCL config loading, validation, and discovery
│   ├── output/            # Output formatters (text, json, github)
│   ├── plan/              # Terraform plan JSON parsing
│   └── plugin/            # Plugin discovery and lifecycle management
├── sdk/                   # Public plugin SDK (Analyzer, Runner, PluginSet interfaces)
│   └── plugin/            # Plugin gRPC server entry point
├── plugins/
│   └── azurerm/           # Example Azure deep inspection plugin (privilege, network, keyvault)
├── proto/                 # gRPC protocol definitions
└── docs/
    ├── adr/               # Architecture Decision Records
    ├── cr/                # Change Requests
    └── examples/          # Working examples with sample plans
```

The repository uses Go workspaces (`go.work`) with three modules:

| Module | Path | Purpose |
|--------|------|---------|
| `github.com/jokarl/tfclassify` | `.` | CLI and core packages |
| `github.com/jokarl/tfclassify/sdk` | `sdk/` | Plugin authoring SDK (minimal dependencies) |
| `github.com/jokarl/tfclassify-plugin-azurerm` | `plugins/azurerm/` | Example Azure deep inspection plugin |

## Development

```bash
make build          # Build binary to bin/tfclassify
make build-all      # Build CLI and azurerm plugin
make test           # Run all tests across workspace
make vet            # Run go vet
make lint           # Run golangci-lint
make generate-roles # Refresh Azure role database from AzAdvertizer
make clean          # Remove build artifacts
```

## Architecture decisions

| ADR | Decision |
|-----|----------|
| [ADR-0001](docs/adr/ADR-0001-monorepo-with-go-workspaces.md) | Monorepo with Go workspaces |
| [ADR-0002](docs/adr/ADR-0002-grpc-plugin-architecture.md) | gRPC plugin architecture via hashicorp/go-plugin |
| [ADR-0003](docs/adr/ADR-0003-provider-agnostic-core-with-deep-inspection-plugins.md) | Provider-agnostic core with deep inspection plugins |
| [ADR-0004](docs/adr/ADR-0004-hcl-configuration-format.md) | HCL configuration format |
| [ADR-0005](docs/adr/ADR-0005-plugin-sdk-versioning-and-protocol-compatibility.md) | Plugin SDK versioning and protocol compatibility |
| [ADR-0006](docs/adr/ADR-0006-permission-based-privilege-escalation-detection.md) | Permission-based privilege escalation detection |
