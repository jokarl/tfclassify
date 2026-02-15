# tfclassify Azure Plugin

Deep inspection plugin for Azure Resource Manager (azurerm) resources. Analyzes actual resource attribute values -- role permissions, network sources, key vault grants -- to produce graduated severity scores that go beyond what pattern matching can detect.

## Table of Contents

- [Overview](#overview)
- [Analyzers](#analyzers)
  - [Privilege Escalation](#privilege-escalation)
  - [Network Exposure](#network-exposure)
  - [Key Vault Access](#key-vault-access)
- [Configuration](#configuration)
  - [Enabling the Plugin](#enabling-the-plugin)
  - [Classification-Scoped Plugin Configuration](#classification-scoped-plugin-configuration)
  - [Full Configuration Example](#full-configuration-example)
- [How Scoring Works](#how-scoring-works)
  - [Permission Tiers](#permission-tiers)
  - [Scope Multipliers](#scope-multipliers)
  - [Scoring Example](#scoring-example)
- [Role Resolution](#role-resolution)
- [Building](#building)
- [Development](#development)

## Overview

The azurerm plugin provides three analyzers that inspect Azure-specific resource attributes:

| Analyzer | Resource Type | Detects |
|----------|--------------|---------|
| `privilege-escalation` | `azurerm_role_assignment` | Role changes with permission-based severity scoring |
| `network-exposure` | `azurerm_network_security_rule` | Inbound rules with overly permissive sources |
| `key-vault-access` | `azurerm_key_vault_access_policy` | Access policies granting destructive permissions |

Plugin decisions are merged with core classification rules via the host's precedence system. A plugin can escalate a resource's classification but never lower it.

## Analyzers

### Privilege Escalation

Detects privilege escalation in Azure role assignments by computing a severity score from the role's actual permission set rather than relying on role name matching.

**Resource pattern:** `azurerm_role_assignment`

**What it detects:**
- New privileged role assignments (e.g., assigning Owner to a principal)
- Role escalations (e.g., changing Reader to Contributor)

**How it works:**
1. Resolves the role being assigned (see [Role Resolution](#role-resolution))
2. Scores the role's permissions using tiered analysis (see [Permission Tiers](#permission-tiers))
3. Applies a scope multiplier based on the ARM scope path (see [Scope Multipliers](#scope-multipliers))
4. Compares before/after scores to determine escalation direction

**Example output:**

```
privileged role "Owner" assigned (severity: 95)
role escalated from "Reader" to "Contributor" (severity: 70)
```

### Network Exposure

Detects overly permissive network security rules that allow inbound traffic from broad sources.

**Resource pattern:** `azurerm_network_security_rule`

**What it detects:**
- Inbound allow rules where `source_address_prefix` is `*`, `0.0.0.0/0`, or `Internet`
- Also checks `source_address_prefixes` (the array variant)

**Severity:** 85 (fixed)

**Conditions (all must be true):**
- `direction` is `Inbound`
- `access` is `Allow`
- Source matches one of the configured permissive sources

### Key Vault Access

Detects when key vault access policies grant destructive permissions.

**Resource pattern:** `azurerm_key_vault_access_policy`

**What it detects:**
- `delete` or `purge` in any of these permission fields:
  - `secret_permissions`
  - `key_permissions`
  - `certificate_permissions`
  - `storage_permissions`

**Severity:** 80 (fixed)

## Configuration

### Enabling the Plugin

Add a `plugin` block to `.tfclassify.hcl`:

```hcl
# For local development (binary in plugin search path):
plugin "azurerm" {
  enabled = true
}

# For distributed installation via tfclassify init:
plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.1.0"
}
```

Then install:

```bash
tfclassify init
```

### Classification-Scoped Plugin Configuration

Plugin configuration is now defined **per-classification** inside classification blocks, rather than at the top-level plugin block. This allows different thresholds and settings for each classification level.

Inside each classification block, add an `azurerm {}` block with sub-blocks for each analyzer:

```hcl
classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*"]
    actions  = ["create", "update"]
  }

  # Plugin-specific configuration for this classification level
  azurerm {
    # Privilege escalation analyzer settings
    privilege_escalation {
      score_threshold = 80              # Only emit decisions if severity >= 80
      exclude         = ["AcrPush"]     # Ignore these roles entirely
      roles           = ["Owner", "Contributor"]  # Only analyze these roles
    }

    # Network exposure analyzer (empty = use defaults)
    network_exposure {}

    # Key vault access analyzer
    keyvault_access {}
  }
}
```

#### Analyzer Sub-blocks

Each analyzer has its own sub-block with specific options:

**`privilege_escalation {}`**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `score_threshold` | int | `0` | Only emit decisions when severity >= this value |
| `exclude` | list(string) | `[]` | Role names to skip entirely (no decisions emitted) |
| `roles` | list(string) | `[]` | If non-empty, only analyze these specific roles |

**`network_exposure {}`**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `permissive_sources` | list(string) | `["*", "0.0.0.0/0", "Internet"]` | Network sources that trigger detection |

**`keyvault_access {}`**

Uses default settings. Include an empty block to enable the analyzer for the classification.

#### Behavior Notes

- If no `azurerm {}` block is present in a classification, the plugin does not emit decisions for resources matching that classification
- An analyzer sub-block (e.g., `privilege_escalation {}`) enables that analyzer for the classification
- Empty sub-blocks use default settings
- The `score_threshold` option is useful for tiered classification: use a high threshold (80+) for "critical" and a lower threshold (or no threshold) for "high"

#### Programmatic Options

The following options are available via `PluginConfig` but are not exposed through HCL config:

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `PrivilegedRoles` | `[]string` | `["Owner", "User Access Administrator", "Contributor"]` | Fallback roles when not found in the built-in database |
| `PermissiveSources` | `[]string` | `["*", "0.0.0.0/0", "Internet"]` | Network sources that trigger network exposure detection |
| `DestructiveKVPermissions` | `[]string` | `["delete", "purge"]` | Key vault permissions considered destructive |
| `UnknownPrivilegedSeverity` | `int` | `80` | Severity for roles in PrivilegedRoles but not in the database |
| `UnknownRoleSeverity` | `int` | `50` | Severity for completely unknown roles |
| `CrossReferenceCustomRoles` | `bool` | `true` | Look up `azurerm_role_definition` resources in the plan for custom role scoring |

### Full Configuration Example

A `.tfclassify.hcl` that uses the azurerm plugin alongside core classification rules:

```hcl
# ─── Plugin ──────────────────────────────────────────────────────────
plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.1.0"
  # Note: plugin-specific configuration is defined per-classification
  # inside classification blocks below (not at the top-level plugin block).
}

# ─── Classifications ─────────────────────────────────────────────────
#
# Core rules handle the broad patterns.
# The azurerm plugin adds deep inspection on top -- its decisions
# merge via precedence and can escalate but never lower a classification.

classification "critical" {
  description = "Requires security team approval"

  rule {
    description = "Deleting IAM or role resources"
    resource    = ["*_role_*", "*_iam_*"]
    actions     = ["delete"]
  }

  rule {
    description = "Deleting a key vault destroys all secrets"
    resource    = ["*_key_vault"]
    actions     = ["delete"]
  }

  # Plugin analyzer configuration for "critical" level.
  # High threshold ensures only the most privileged roles trigger critical.
  azurerm {
    privilege_escalation {
      score_threshold = 80  # Only Owner (95) and UAA (85) trigger critical
    }
    network_exposure {}
    keyvault_access {}
  }
}

classification "high" {
  description = "Requires team lead approval"

  rule {
    description = "Non-destructive IAM changes"
    resource    = ["*_role_*", "*_iam_*"]
    actions     = ["create", "update"]
  }

  rule {
    description = "Network security changes"
    resource    = ["*_security_rule", "*_firewall_*"]
  }

  rule {
    description = "Key vault secret/key changes"
    resource    = ["*_key_vault_*"]
  }

  # Plugin analyzer configuration for "high" level.
  # Lower threshold captures mid-tier privilege escalations.
  azurerm {
    privilege_escalation {
      # No threshold = any privilege escalation triggers "high"
    }
    network_exposure {}
    keyvault_access {}
  }
}

classification "standard" {
  description = "Standard change process"

  rule {
    resource = ["*"]
  }
}

classification "auto" {
  description = "No approval needed"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

# With the azurerm plugin enabled, a simple role assignment create
# that core rules classify as "high" may be escalated to "critical"
# if the plugin detects a high-severity permission set (e.g., Owner).
precedence = ["critical", "high", "standard", "auto"]

defaults {
  unclassified   = "standard"
  no_changes     = "auto"
  plugin_timeout = "30s"
}
```

## How Scoring Works

The privilege escalation analyzer uses a multi-factor scoring system to compute graduated severity for role assignments. The algorithm is implemented in `scoring.go` and `scope.go`.

### Overview

Scoring happens in three steps:

1. **Permission analysis** — score the role's permission set against tiered risk patterns
2. **Scope weighting** — multiply the base score by a factor based on the ARM scope path
3. **Clamping** — constrain the result to [0, 100]

When a role definition has multiple permission blocks, each block is scored independently and the highest score wins.

### Permission Tiers

Roles are scored based on their effective permission set. Azure RBAC roles define `actions` (allowed operations) and `notActions` (excluded operations). The scoring algorithm examines these lists using Azure's pattern matching rules:

- `*` matches all operations
- `Microsoft.Compute/*` matches all operations under a provider
- `*/read` matches any read operation

The algorithm classifies each permission block into one of eight tiers:

| Tier | Score | Pattern | Detection Logic | Example Roles |
|------|-------|---------|-----------------|---------------|
| 1 | 95 | Unrestricted wildcard `*` without auth exclusion | `actions` contains `*` AND `notActions` does not cover `Microsoft.Authorization` write | Owner |
| 2 | 85 | `Microsoft.Authorization/*` control | `actions` contains `Microsoft.Authorization/*` (not via wildcard) AND not excluded by `notActions` | User Access Administrator |
| 3 | 75 | Targeted role assignment write | `actions` contains `Microsoft.Authorization/roleAssignments/write` or `.../roleAssignments/*` without broader auth access | Custom roles granting role assignment write |
| 4 | 70 | Wildcard with auth excluded | `actions` contains `*` AND `notActions` covers `Microsoft.Authorization` write operations | Contributor |
| 5 | 50–65 | Provider wildcards | `actions` contains patterns like `Microsoft.Compute/*` (ends with `/*`, contains `.`) | Custom roles with broad provider access |
| 6 | 30 | Limited write access | Has non-read actions but does not match any higher tier | Custom roles with specific write actions |
| 7 | 15 | Read-only access | All actions end with `/read` | Reader, custom read-only roles |
| 8 | 0 | No permissions | Empty `actions` and `dataActions` | Roles with no actions defined |

**Authorization exclusion detection:** The `notActions` list is checked for patterns that cover `Microsoft.Authorization` write operations. Any of these patterns qualify as an auth exclusion: `Microsoft.Authorization/*`, `Microsoft.Authorization/*/Write`, or `Microsoft.Authorization/*/Delete`. This is how the algorithm distinguishes Owner (tier 1, no exclusion) from Contributor (tier 4, auth excluded).

**Provider wildcard scoring:** Tier 5 scores scale with the number of provider wildcards: `50 + min(count × 5, 15)`. A role with one provider wildcard scores 55; three or more score 65. This captures the intuition that broader provider access is riskier.

### Scope Multipliers

After computing a base permission score, a multiplier is applied based on the ARM scope path of the role assignment. The scope is read from the `scope` attribute of the `azurerm_role_assignment` resource in the Terraform plan.

| Scope Level | Multiplier | Detection Rule |
|-------------|------------|----------------|
| Management Group | 1.1× | Path contains `microsoft.management/managementgroups` (case-insensitive) |
| Subscription | 1.0× | Path starts with `/subscriptions/` with no `/resourceGroups/` segment |
| Resource Group | 0.8× | Path contains `/resourceGroups/` with no `/providers/` segment after it |
| Resource | 0.6× | Path contains `/providers/` after the `/resourceGroups/` segment |
| Unknown | 0.9× | Does not match any of the above patterns |

**Scope parsing** is case-insensitive and trims trailing slashes. The parser checks for management group first, then subscription, then resource group vs. resource. If the path does not start with `/subscriptions/` and is not a management group, it falls through to unknown.

The multiplier reflects that the same role at a broader scope is riskier: Owner at management group scope (1.1×) affects all subscriptions beneath it, while Owner at resource scope (0.6×) is tightly constrained.

The final score is `round(base × multiplier)`, clamped to [0, 100]. A base score of 0 always stays 0 regardless of scope.

### Scoring Examples

**Contributor at subscription scope:**

1. Contributor has `Actions: ["*"]`, `NotActions: ["Microsoft.Authorization/*/Write", "Microsoft.Authorization/*/Delete", ...]`
2. `actions` contains `*` → wildcard detected
3. `notActions` covers `Microsoft.Authorization` write → auth excluded
4. Tier 4 match → base score **70**
5. Subscription scope → multiplier **1.0×**
6. Final severity: **70**

**Owner at resource group scope:**

1. Owner has `Actions: ["*"]`, `NotActions: []`
2. `actions` contains `*` → wildcard detected
3. `notActions` is empty → auth NOT excluded
4. Tier 1 match → base score **95**
5. Resource group scope → multiplier **0.8×**
6. Final severity: **76** (round(95 × 0.8))

**Custom role with `Microsoft.Compute/*` and `Microsoft.Network/*` at subscription scope:**

1. Two provider wildcards detected
2. Tier 5 match → base score **60** (50 + 2×5)
3. Subscription scope → multiplier **1.0×**
4. Final severity: **60**

**Reader at management group scope:**

1. Reader has `Actions: ["*/read"]`, `NotActions: []`
2. All actions end with `/read` → read-only
3. Tier 7 match → base score **15**
4. Management group scope → multiplier **1.1×**
5. Final severity: **17** (round(15 × 1.1))

### Score Interpretation

| Score Range | Risk Level | Typical Roles |
|-------------|-----------|---------------|
| 90–100 | Very high | Owner, User Access Administrator at broad scope |
| 70–89 | High | Contributor, roles with auth write, broad custom roles |
| 50–69 | Medium | Roles with provider-level wildcards |
| 20–49 | Low | Roles with limited write access |
| 1–19 | Minimal | Read-only roles |
| 0 | None | No permissions or no role specified |

## Role Resolution

The privilege escalation analyzer resolves roles through a four-level fallback chain:

1. **Built-in database**: Embedded JSON database of 400+ Azure built-in roles with full permission sets. Roles are looked up by `role_definition_name` or `role_definition_id`. This is the primary source.

2. **Custom roles from plan**: If `CrossReferenceCustomRoles` is enabled (default), the analyzer queries the plan for `azurerm_role_definition` resources and scores their permission sets.

3. **Config fallback**: If the role name appears in `PrivilegedRoles` but is not in the database or plan, it gets `UnknownPrivilegedSeverity` (default 80).

4. **Unknown**: Roles not found through any mechanism get `UnknownRoleSeverity` (default 50).

Decision metadata includes a `role_source` field indicating which resolution path was used: `builtin`, `plan-custom-role`, `config-fallback`, or `unknown`.

## Building

From the repository root:

```bash
# Build the plugin binary
go build -o bin/tfclassify-plugin-azurerm ./plugins/azurerm

# Or use the Makefile
make build-all
```

For local testing, copy the binary to a plugin search path:

```bash
mkdir -p .tfclassify/plugins
cp bin/tfclassify-plugin-azurerm .tfclassify/plugins/
```

## Development

### Running Tests

```bash
# All plugin tests
go test ./plugins/azurerm/...

# Specific test
go test ./plugins/azurerm/ -run TestPrivilege
go test ./plugins/azurerm/ -run TestNetwork
go test ./plugins/azurerm/ -run TestKeyVault
go test ./plugins/azurerm/ -run TestScoring
go test ./plugins/azurerm/ -run TestScope
```

### Refreshing the Role Database

The embedded role database is generated from the [AzAdvertizer](https://www.azadvertizer.net/) CSV:

```bash
make generate-roles
```

This downloads the latest CSV and converts it to `plugins/azurerm/roledata/roles.json` using the `tools/csv2roles/` tool.

### Module Structure

```
plugins/azurerm/
├── main.go              # Entry point: calls sdk/plugin.Serve()
├── plugin.go            # AzurermPluginSet and PluginConfig
├── privilege.go         # PrivilegeEscalationAnalyzer
├── network.go           # NetworkExposureAnalyzer
├── keyvault.go          # KeyVaultAccessAnalyzer
├── scoring.go           # Permission-based risk scoring
├── scope.go             # ARM scope path parsing and multipliers
├── roles.go             # RoleDatabase (embedded JSON, lookup by name/ID)
├── helpers.go           # Shared utility functions
├── roledata/
│   └── roles.json       # Embedded Azure built-in role definitions
├── *_test.go            # Tests for each component
├── go.mod               # Module: github.com/jokarl/tfclassify-plugin-azurerm
└── CHANGELOG.md
```

The module depends only on the tfclassify SDK (`github.com/jokarl/tfclassify/sdk`). During development, a `replace` directive in `go.mod` resolves the SDK locally:

```
replace github.com/jokarl/tfclassify/sdk => ../../sdk
```
