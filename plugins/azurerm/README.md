# tfclassify Azure Plugin

Deep inspection plugin for Azure Resource Manager (azurerm) resources. Analyzes actual resource attribute values -- role permissions, network sources, key vault grants -- using pattern-based detection to classify changes beyond what resource-type matching can detect.

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
- [Role Resolution](#role-resolution)
- [Data-Plane Detection](#data-plane-detection)
- [Pattern-Based Control-Plane Detection](#pattern-based-control-plane-detection)
- [Building](#building)
- [Development](#development)

## Overview

The azurerm plugin provides three analyzers that inspect Azure-specific resource attributes:

| Analyzer | Resource Type | Detects |
|----------|--------------|---------|
| `privilege-escalation` | `azurerm_role_assignment` | Role changes matching configured action patterns |
| `network-exposure` | `azurerm_network_security_rule` | Inbound rules with overly permissive sources |
| `key-vault-access` | `azurerm_key_vault_access_policy` | Access policies granting destructive permissions |

Plugin decisions are merged with core classification rules via the host's precedence system. A plugin can escalate a resource's classification but never lower it.

## Analyzers

### Privilege Escalation

Detects privilege escalation in Azure role assignments by matching the role's effective permission set against configured action patterns.

**Resource pattern:** `azurerm_role_assignment`

**What it detects:**
- New privileged role assignments (e.g., assigning Owner to a principal)
- Role escalations (e.g., changing Reader to Contributor)
- Data-plane access grants (e.g., storage blob read access)
- Unknown roles whose permissions cannot be resolved

**How it works:**
1. Resolves the role being assigned (see [Role Resolution](#role-resolution))
2. Computes effective actions: `Actions - NotActions` for control-plane, `DataActions - NotDataActions` for data-plane
3. Matches effective actions against configured patterns
4. Optionally filters by ARM scope level

**Example output:**

```
role "Owner" grants control-plane access matching configured patterns
role "Storage Blob Data Owner" grants data-plane access matching configured patterns
unknown role "Custom Role" flagged (role permissions could not be resolved)
```

### Network Exposure

Detects overly permissive network security rules that allow inbound traffic from broad sources.

**Resource pattern:** `azurerm_network_security_rule`

**What it detects:**
- Inbound allow rules where `source_address_prefix` is `*`, `0.0.0.0/0`, or `Internet`
- Also checks `source_address_prefixes` (the array variant)

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

Plugin configuration is defined **per-classification** inside classification blocks. This allows different action patterns and settings for each classification level.

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
      actions      = ["*", "Microsoft.Authorization/*"]  # Control-plane patterns
      data_actions = ["*/read"]                          # Data-plane patterns
      exclude      = ["AcrPush"]                         # Ignore these roles entirely
      roles        = ["Owner", "Contributor"]             # Only analyze these roles
      scopes       = ["subscription", "management_group"] # Only at broad scopes
      flag_unknown_roles = true                           # Flag unresolvable roles
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
| `actions` | list(string) | `[]` | Control-plane action patterns to match (e.g., `["Microsoft.Authorization/*"]`). See [Pattern-Based Control-Plane Detection](#pattern-based-control-plane-detection). |
| `data_actions` | list(string) | `[]` | Data-plane action patterns to match (e.g., `["*/read"]`). See [Data-Plane Detection](#data-plane-detection). |
| `exclude` | list(string) | `[]` | Role names to skip entirely (no decisions emitted) |
| `roles` | list(string) | `[]` | If non-empty, only analyze these specific roles |
| `scopes` | list(string) | `[]` | Scope levels to match: `"management_group"`, `"subscription"`, `"resource_group"`, `"resource"`. Empty matches any scope. |
| `flag_unknown_roles` | bool | `true` | Emit decisions for roles whose permissions cannot be resolved, with diagnostic metadata |

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
- Use different `actions`/`data_actions` patterns per classification for graduated detection (e.g., wildcard patterns for "critical", specific write patterns for "standard")

#### Programmatic Options

The following options are available via `PluginConfig` but are not exposed through HCL config:

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `PermissiveSources` | `[]string` | `["*", "0.0.0.0/0", "Internet"]` | Network sources that trigger network exposure detection |
| `DestructiveKVPermissions` | `[]string` | `["delete", "purge"]` | Key vault permissions considered destructive |
| `CrossReferenceCustomRoles` | `bool` | `true` | Look up `azurerm_role_definition` resources in the plan for custom role pattern matching |

### Full Configuration Example

A `.tfclassify.hcl` that uses the azurerm plugin alongside core classification rules:

```hcl
# ─── Plugin ──────────────────────────────────────────────────────────
plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.1.0"
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

  # Pattern-based detection: wildcard or auth control access
  azurerm {
    privilege_escalation {
      actions = ["*", "Microsoft.Authorization/*"]
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

  # Pattern-based detection: write/delete operations
  azurerm {
    privilege_escalation {
      actions      = ["*/write", "*/delete"]
      data_actions = ["*/write", "*/delete"]
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

precedence = ["critical", "high", "standard", "auto"]

defaults {
  unclassified   = "standard"
  no_changes     = "auto"
  plugin_timeout = "30s"
}
```

## Role Resolution

The privilege escalation analyzer resolves roles through a three-level fallback chain:

1. **Built-in database**: Embedded JSON database of 400+ Azure built-in roles with full permission sets. Roles are looked up by `role_definition_name` or `role_definition_id`. This is the primary source.

2. **Custom roles from plan**: If `CrossReferenceCustomRoles` is enabled (default), the analyzer queries the plan for `azurerm_role_definition` resources and matches their permission sets against configured patterns.

3. **Unknown**: Roles not found through any mechanism are flagged (if `flag_unknown_roles` is true) with diagnostic metadata listing the resolution attempts, or silently skipped (if `flag_unknown_roles` is false).

Decision metadata includes a `role_source` field indicating which resolution path was used: `builtin`, `plan-custom-role`, or `unknown`.

## Data-Plane Detection

> CR-0027: Data-Plane Action Detection

Azure RBAC distinguishes between control-plane actions (`Actions`/`NotActions`) and data-plane actions (`DataActions`/`NotDataActions`). Control-plane actions manage Azure resources; data-plane actions access data within resources (e.g., reading blob contents vs. managing blob containers).

The `data_actions` option enables pattern-based detection for data-plane access. When configured, the analyzer computes effective data actions (`DataActions` minus `NotDataActions`) and matches them against your patterns.

### Configuration

```hcl
classification "critical" {
  azurerm {
    privilege_escalation {
      data_actions = ["*/read"]  # Flag roles with any data-plane read
    }
  }
}
```

### Pattern Syntax

Data-plane patterns use Azure RBAC matching rules:
- `*` -- matches everything
- `Microsoft.Storage/*` -- matches all storage data actions
- `*/read` -- matches any read action
- `*/write` -- matches any write action

### How It Works

1. For each role assignment, resolve the role definition
2. Compute effective data actions: `DataActions - NotDataActions`
3. Match effective actions against configured patterns
4. If ANY action matches ANY pattern, emit a decision

### NotDataActions Subtraction

`NotDataActions` removes permissions before pattern matching. This naturally handles exclusion scenarios:

```
# Banking config: data_actions = ["*/read"]

# Role: Storage Blob Data Owner
# dataActions: ["Microsoft.Storage/.../blobs/*"]
# notDataActions: []
# -> Effective includes reads -> MATCHES -> flagged as critical

# Custom write-only role (notDataActions blocks reads)
# dataActions: ["Microsoft.Storage/.../blobs/*"]
# notDataActions: ["Microsoft.Storage/.../blobs/read"]
# -> Effective: write/delete only -> NO reads -> does NOT match
# -> Write-only is acceptable -- not flagged
```

### Decision Metadata

Data-plane triggers include metadata:
- `trigger`: `"data-plane"`
- `matched_data_actions`: list of effective actions that matched
- `matched_patterns`: list of configured patterns that matched

### Independence from Control-Plane

Control-plane and data-plane triggers are independent -- either can cause a role to be flagged. A role can trigger via:
- Control-plane only (action pattern match)
- Data-plane only (data action pattern match)
- Both (triggers appear as `trigger: "both"`)

## Pattern-Based Control-Plane Detection

> CR-0028: Pattern-Based Control-Plane Detection

The `actions` option provides pattern-based detection for control-plane actions. When configured, it matches effective control-plane actions (`Actions` minus `NotActions`, with wildcard expansion via the action registry) against your patterns.

### Configuration

```hcl
classification "critical" {
  azurerm {
    privilege_escalation {
      actions = ["*", "Microsoft.Authorization/*"]  # Flag wildcard or auth control
    }
  }
}

classification "standard" {
  azurerm {
    privilege_escalation {
      actions = ["*/write", "*/delete"]  # Flag write/delete operations
    }
  }
}
```

### Pattern Syntax

Control-plane patterns use Azure RBAC matching rules:
- `*` -- matches everything (Owner pattern)
- `Microsoft.Authorization/*` -- matches authorization control (User Access Administrator pattern)
- `*/write` -- matches any write action
- `*/read` -- matches any read action
- `Microsoft.Compute/*` -- matches all compute actions

### How It Works

1. For each role assignment, resolve the role definition
2. Compute effective actions: `Actions - NotActions` (with wildcard expansion via the action registry)
3. Match effective actions against configured patterns
4. If ANY action matches ANY pattern, emit a decision

### NotActions Subtraction

`NotActions` removes permissions before pattern matching:

```
# Config: actions = ["Microsoft.Authorization/*"]

# Role: Contributor (actions: ["*"], notActions: ["Microsoft.Authorization/*", ...])
# -> "*" is expanded to all concrete actions via the action registry
# -> Microsoft.Authorization/* actions are subtracted
# -> Does NOT match "Microsoft.Authorization/*" (excluded by notActions)
# -> No decision emitted

# Role: Owner (actions: ["*"], notActions: [])
# -> "*" is expanded to all concrete actions
# -> No subtraction
# -> MATCHES "Microsoft.Authorization/*" (auth actions present in expanded set)
```

### Combined Control-Plane and Data-Plane

Both `actions` and `data_actions` can be configured together:

```hcl
classification "critical" {
  azurerm {
    privilege_escalation {
      actions      = ["*", "Microsoft.Authorization/*"]
      data_actions = ["*/read"]
    }
  }
}
```

A role triggers if it matches EITHER the control-plane patterns OR the data-plane patterns. The decision metadata indicates which triggered via the `trigger` field.

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
go test ./plugins/azurerm/ -run TestAction
```

### Refreshing the Role Database

The embedded role database is generated from the [AzAdvertizer](https://www.azadvertizer.net/) CSV:

```bash
make generate-roles
```

This downloads the latest CSV and converts it to `plugins/azurerm/roledata/roles.json` using the `tools/csv2roles/` tool.

### Refreshing the Action Registry

The embedded action registry is generated from the role database:

```bash
make generate-actions
```

This produces `plugins/azurerm/actiondata/actions.json` using the `tools/md2actions/` tool.

### Module Structure

```
plugins/azurerm/
├── main.go              # Entry point: calls sdk/plugin.Serve()
├── plugin.go            # AzurermPluginSet and PluginConfig
├── privilege.go         # PrivilegeEscalationAnalyzer
├── network.go           # NetworkExposureAnalyzer
├── keyvault.go          # KeyVaultAccessAnalyzer
├── scoring.go           # Pattern matching: actionMatchesPattern, computeEffectiveActions
├── actions.go           # ActionRegistry (embedded JSON, wildcard expansion)
├── scope.go             # ARM scope path parsing
├── roles.go             # RoleDatabase (embedded JSON, lookup by name/ID)
├── helpers.go           # Shared utility functions
├── roledata/
│   └── roles.json       # Embedded Azure built-in role definitions
├── actiondata/
│   └── actions.json     # Embedded Azure RBAC action registry
├── *_test.go            # Tests for each component
├── go.mod               # Module: github.com/jokarl/tfclassify-plugin-azurerm
└── CHANGELOG.md
```

The module depends only on the tfclassify SDK (`github.com/jokarl/tfclassify/sdk`). During development, a `replace` directive in `go.mod` resolves the SDK locally:

```
replace github.com/jokarl/tfclassify/sdk => ../../sdk
```
