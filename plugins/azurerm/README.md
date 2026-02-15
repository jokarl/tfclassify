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
  - [Plugin Config Options](#plugin-config-options)
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
- Role de-escalations (e.g., removing Owner, always severity 40)

**How it works:**
1. Resolves the role being assigned (see [Role Resolution](#role-resolution))
2. Scores the role's permissions using tiered analysis (see [Permission Tiers](#permission-tiers))
3. Applies a scope multiplier based on the ARM scope path (see [Scope Multipliers](#scope-multipliers))
4. Compares before/after scores to determine escalation direction

**Example output:**

```
privileged role "Owner" assigned (severity: 95)
role escalated from "Reader" to "Contributor" (severity: 70)
role de-escalated from "Owner" to "Reader" (severity: 40)
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

### Plugin Config Options

All options are set inside the `config {}` block of the plugin declaration. Every option has a sensible default -- you only need to specify values you want to override.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `privilege_enabled` | bool | `true` | Enable the privilege escalation analyzer |
| `network_enabled` | bool | `true` | Enable the network exposure analyzer |
| `keyvault_enabled` | bool | `true` | Enable the key vault access analyzer |

The following options are available programmatically via `PluginConfig` but are not currently exposed through HCL config:

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

  config {
    privilege_enabled = true
    network_enabled   = true
    keyvault_enabled  = true
  }
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

The privilege escalation analyzer uses a multi-factor scoring system to compute graduated severity for role assignments.

### Permission Tiers

Roles are scored based on their effective permission set (actions minus notActions):

| Tier | Score | Pattern | Example Roles |
|------|-------|---------|---------------|
| 1 | 95 | Unrestricted wildcard `*` without auth exclusion | Owner |
| 2 | 85 | `Microsoft.Authorization/*` control | User Access Administrator |
| 3 | 75 | Targeted `Microsoft.Authorization/roleAssignments/write` | Custom roles with role assignment write |
| 4 | 70 | Wildcard `*` with `Microsoft.Authorization` excluded | Contributor |
| 5 | 50-65 | Provider wildcards (`Microsoft.Compute/*`, etc.) | Custom roles with broad provider access |
| 6 | 30 | Limited write access | Custom roles with specific write actions |
| 7 | 15 | Read-only access | Reader, custom read-only roles |
| 8 | 0 | No permissions | No actions defined |

### Scope Multipliers

After computing a base score, a multiplier is applied based on the ARM scope path of the role assignment:

| Scope Level | Multiplier | Detection |
|-------------|------------|-----------|
| Management Group | 1.1x | Path contains `microsoft.management/managementgroups` |
| Subscription | 1.0x | Path starts with `/subscriptions/` with no resource group |
| Resource Group | 0.8x | Path contains `/resourceGroups/` with no `/providers/` after |
| Resource | 0.6x | Path contains `/providers/` after resource group |
| Unknown | 0.9x | Unrecognized scope format |

The final score is clamped to [0, 100].

### Scoring Example

Assigning the **Contributor** role at **subscription** scope:

1. Contributor has `Actions: ["*"]`, `NotActions: ["Microsoft.Authorization/*/Write", ...]`
2. Tier 4 match: wildcard with auth excluded → base score **70**
3. Subscription scope → multiplier **1.0**
4. Final severity: **70**

Assigning the **Owner** role at **resource group** scope:

1. Owner has `Actions: ["*"]`, `NotActions: []`
2. Tier 1 match: unrestricted wildcard → base score **95**
3. Resource group scope → multiplier **0.8**
4. Final severity: **76** (95 * 0.8, rounded)

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
