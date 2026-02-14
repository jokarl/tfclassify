---
status: accepted
date: 2026-02-13
decision-makers: Johan
---

# Permission-Based Privilege Escalation Detection for Azure Role Assignments

## Context and Problem Statement

The azurerm deep inspection plugin's privilege escalation analyzer (ADR-0003, Layer 3) currently uses a name-based allowlist to detect privilege changes in `azurerm_role_assignment` resources. It checks whether `role_definition_name` appears in a flat list of `["Owner", "User Access Administrator", "Contributor"]` and emits a fixed severity of 90 for all escalations.

This approach is not useful for real-world Azure RBAC analysis because:

- **Owner and Contributor are treated identically.** Owner has unrestricted `*` actions. Contributor has `*` actions but explicitly excludes `Microsoft.Authorization/*` via `notActions` — meaning it cannot assign roles or modify access control. These represent fundamentally different risk levels.
- **`Microsoft.Authorization` actions are not weighted.** The ability to write role assignments (`Microsoft.Authorization/roleAssignments/write`) or create custom role definitions (`Microsoft.Authorization/roleDefinitions/write`) is the primary privilege escalation vector in Azure — a principal with these permissions can grant itself any other permission. The current analyzer has no concept of this.
- **Custom roles are invisible.** Organizations frequently create custom roles with broad permissions, including wildcard actions. Unless someone manually adds every custom role name to the config, these are undetected — even a custom role with `*` actions would be ignored.
- **Scope is not considered.** An Owner assignment at subscription scope (`/subscriptions/{id}`) has vastly greater blast radius than Owner at a single resource scope. The analyzer emits severity 90 regardless.
- **No graduated severity.** Every escalation — Reader to Contributor, Reader to Owner, no-role to User Access Administrator — receives the same severity, making triage impossible.

How should the privilege escalation analyzer determine the risk level of Azure role changes?

## Decision Drivers

* The analyzer must score roles based on their actual permissions (actions, notActions, dataActions, notDataActions), not just display names
* `Microsoft.Authorization/*` actions must be recognized as the highest-risk permission class because they enable privilege escalation chains
* Wildcard actions (`*`) must be distinguished from scoped actions, and `notActions` exclusions must reduce the effective risk (e.g., Contributor's `notActions: ["Microsoft.Authorization/*"]`)
* Scope (management group > subscription > resource group > resource) must influence severity — the same role is more dangerous at a broader scope
* Custom roles defined in the same Terraform plan (`azurerm_role_definition`) must be analyzed by inspecting their permission blocks
* Built-in role permission data must stay current as Azure evolves — new roles are added and existing roles are occasionally modified
* The solution must work offline — analysis cannot depend on Azure API access at runtime since tfclassify runs in CI pipelines and air-gapped environments

## Considered Options

* Permission-based analysis with an embedded role database refreshed via CI
* Configurable role tier system (enhanced name-based)
* Runtime Azure API lookups for role definitions

## Decision Outcome

Chosen option: "Permission-based analysis with an embedded role database refreshed via CI", because it provides accurate, permission-level risk scoring that works offline, handles both built-in and custom roles, and stays current through automation rather than manual configuration.

### Embedded Role Database

Azure built-in role definitions (with full permission sets) are fetched from [AzAdvertizer](https://www.azadvertizer.net/azrolesadvertizer_all.html), a publicly available dataset that requires no Azure authentication. A small Go tool downloads the CSV (`https://www.azadvertizer.net/azrolesadvertizer-comma.csv`) and transforms it to a compact JSON format stored at `plugins/azurerm/roledata/roles.json`. This file is embedded into the plugin binary at compile time using `//go:embed`. A Makefile target regenerates the file, and a nightly CI job commits updates when Azure role definitions change.

Each role entry contains the full permission set:

```json
{
  "id": "8e3af657-a8ff-443c-a75c-2fe8c4bcb635",
  "roleName": "Owner",
  "roleType": "BuiltInRole",
  "permissions": [{
    "actions": ["*"],
    "notActions": [],
    "dataActions": [],
    "notDataActions": []
  }]
}
```

The database provides lookup by display name (case-insensitive) and by role definition ID (GUID or full ARM path), since Terraform `azurerm_role_assignment` may reference roles via either `role_definition_name` or `role_definition_id`.

### Permission Scoring Algorithm

A scoring function computes a 0–100 risk score from a role's permission set. The algorithm first computes **effective actions** by subtracting `notActions` from `actions` using Azure's pattern matching semantics, then scores based on what the effective actions grant.

**Scoring tiers:**

| Effective Permission Pattern | Base Score | Rationale |
|------------------------------|-----------|-----------|
| `*` (unrestricted, no relevant notActions) | 95 | Full control including authorization — can do anything |
| `Microsoft.Authorization/*` | 85 | Full authorization control — the core escalation vector |
| `Microsoft.Authorization/roleAssignments/write` | 75 | Can create role assignments — targeted escalation |
| `*` with `notActions: ["Microsoft.Authorization/*"]` | 70 | Broad control but cannot escalate (Contributor pattern) |
| Multiple provider-level wildcards (`Microsoft.X/*`) | 50–65 | Significant write access across services |
| Read-only patterns (`*/read`) | 15 | Low risk — observation only |

The `notActions` field in Azure RBAC performs **exclusion, not deny** — it subtracts permissions from the actions list. The scoring algorithm must compute effective permissions before scoring. This is what makes Contributor (actions `["*"]`, notActions `["Microsoft.Authorization/*", ...]`) fundamentally different from Owner (actions `["*"]`, no notActions) despite both having the wildcard action.

Pattern matching follows Azure's semantics: `*` matches everything, `Microsoft.Compute/*` matches all actions under that provider namespace, and `Microsoft.Compute/virtualMachines/read` matches a single action.

**Expected scores for well-known built-in roles:**

| Role | Score | Key Factor |
|------|-------|-----------|
| Owner | ~95 | Unrestricted `*` |
| User Access Administrator | ~85 | `Microsoft.Authorization/*` |
| Contributor | ~70 | `*` minus `Microsoft.Authorization/*` |
| Key Vault Administrator | ~55 | Broad provider-scoped access |
| Reader | ~15 | Read-only |

### Scope Weighting

The `scope` field on an `azurerm_role_assignment` determines the blast radius of the role. The ARM path is parsed to determine scope level, and a multiplier adjusts the permission score.

| Scope Level | Multiplier | Example Path |
|-------------|-----------|-------------|
| Management group | 1.1 | `/providers/Microsoft.Management/managementGroups/{name}` |
| Subscription | 1.0 (baseline) | `/subscriptions/{id}` |
| Resource group | 0.8 | `/subscriptions/{id}/resourceGroups/{name}` |
| Individual resource | 0.6 | `.../providers/Microsoft.Compute/virtualMachines/{name}` |

Final severity = `clamp(permissionScore * scopeMultiplier, 0, 100)`.

This means Owner at subscription scope scores ~95, but Owner scoped to a single resource scores ~57 — accurately reflecting the reduced blast radius.

### Custom Role Cross-Reference

When the Terraform plan contains `azurerm_role_definition` resources (custom roles being created or modified in the same plan), the analyzer cross-references them. It uses `runner.GetResourceChanges(["azurerm_role_definition"])` to retrieve custom role definitions from the plan, parses their `permissions` blocks, and applies the same scoring algorithm.

This is the first cross-resource analysis in the plugin codebase. The SDK's `Runner.GetResourceChanges` already supports querying arbitrary resource types, so no SDK changes are required. The analyzer's `ResourcePatterns()` continues to return only `["azurerm_role_assignment"]` — the custom role lookup is an internal cross-reference within `Analyze()`, not a pattern declaration.

### Role Resolution Chain

When analyzing a role assignment change, the analyzer resolves role permissions through a fallback chain:

1. **Built-in database lookup** — match `role_definition_name` or `role_definition_id` against the embedded role database
2. **Custom role from plan** — if the role is an `azurerm_role_definition` in the same Terraform plan, parse its permissions
3. **Configured fallback** — if the role name appears in the existing `config.PrivilegedRoles` list (backward compatibility), use a configurable default severity
4. **Unknown role** — emit a moderate severity flag for manual review, since the role's permissions cannot be determined

### Severity Semantics

The emitted severity represents the **absolute risk of the after-state role**, not the delta between before and after. Reader-to-Owner and Contributor-to-Owner both result in Owner access, so both emit Owner's score. The decision metadata includes both `before_score` and `after_score` for consumers that need the delta.

For de-escalation (privileged to less-privileged), a fixed low severity (40) is emitted regardless of the from-role, since de-escalation is risk-reducing.

### Consequences

* Good, because severity now reflects actual role risk — Owner (~95) is clearly distinguished from Contributor (~70) and Reader (~15)
* Good, because `Microsoft.Authorization` actions are explicitly recognized as the highest-risk permission class
* Good, because custom roles are analyzed based on their actual permissions, not ignored
* Good, because scope weighting prevents a resource-scoped Owner from being treated the same as a subscription-scoped Owner
* Good, because the embedded database works offline — no runtime Azure API dependency
* Good, because the `PrivilegedRoles` config is preserved as a fallback, maintaining backward compatibility
* Bad, because the plugin now depends on an external data source (Azure role definitions) that must be kept current
* Bad, because the permission scoring algorithm introduces complexity — the wildcard/notActions interaction and pattern matching must be carefully tested
* Bad, because the embedded role database adds to binary size (Azure has 400+ built-in roles; the JSON is estimated at ~500KB–1MB)
* Neutral, because this is the first cross-resource analysis in the codebase, establishing a pattern other analyzers may follow

### Confirmation

* Owner scores ~95, Contributor scores ~70, Reader scores ~15 — the `notActions` exclusion correctly reduces Contributor's score
* The same role at subscription scope scores higher than at resource-group scope
* A custom `azurerm_role_definition` with `Microsoft.Authorization/roleAssignments/write` in its actions is detected and scored ~75
* A role not found in the database or plan is flagged with moderate severity and metadata indicating `"role_source": "unknown"`
* All existing tests continue to pass (backward-compatible config)
* `go test ./...` in `plugins/azurerm/` passes with new scoring, scope, and cross-reference tests
* The `roledata/roles.json` file can be regenerated from the AzAdvertizer CSV without Azure credentials

## Pros and Cons of the Options

### Permission-based analysis with an embedded role database refreshed via CI

Azure built-in role definitions (including full permission sets) are fetched from a publicly available source (AzAdvertizer CSV), transformed to a compact JSON format, and committed to the plugin source tree. The JSON is embedded into the binary at compile time. A scoring algorithm computes risk from effective permissions. Custom roles from the plan are cross-referenced and scored identically.

* Good, because scoring is based on what roles can actually do, not human-curated name lists
* Good, because `notActions` are correctly handled — Contributor's `Microsoft.Authorization/*` exclusion is reflected in the score
* Good, because new Azure built-in roles are automatically picked up by the nightly CI refresh
* Good, because custom roles are analyzed without any configuration — their permissions are right there in the plan
* Good, because works offline and in air-gapped environments — all data is embedded at build time
* Good, because the scoring algorithm is deterministic and testable — given a role definition, the score is always the same
* Neutral, because the data source (AzAdvertizer) is a third-party project — if it becomes unavailable, the existing committed data remains valid and generation can be switched to an alternative source
* Neutral, because pattern matching for Azure action strings (wildcards, provider namespaces) adds implementation complexity but is well-defined
* Bad, because the embedded JSON grows with Azure's role catalog — currently ~400 built-in roles, likely to grow
* Bad, because Azure could in theory change a built-in role's permissions between refreshes, creating a brief window of inaccuracy (mitigated by nightly refresh)

### Configurable role tier system (enhanced name-based)

Instead of analyzing permissions, expand the current name-based approach by allowing users to assign roles to risk tiers in configuration: `critical = ["Owner", "User Access Administrator"]`, `high = ["Contributor"]`, `medium = [...]`, `low = ["Reader"]`. Each tier maps to a severity range.

* Good, because simple to implement — extends the existing pattern with no new data sources
* Good, because gives organizations full control over risk classification
* Good, because no external data dependency — all configuration is local
* Bad, because organizations must manually maintain the role-to-tier mapping for every role they use
* Bad, because custom roles must be manually added to the config — there is no automatic analysis
* Bad, because it cannot distinguish between roles within a tier — two "high" roles may have very different actual permissions
* Bad, because `notActions` nuance is invisible — Contributor and a hypothetical "Contributor-Plus" without `notActions` would be in the same tier
* Bad, because new Azure built-in roles are silently unclassified until someone updates the config

### Runtime Azure API lookups for role definitions

At analysis time, query the Azure REST API (`GET /providers/Microsoft.Authorization/roleDefinitions`) to fetch the actual role definitions referenced in the plan. Score based on the live permission data.

* Good, because always current — no embedded data to refresh, always uses Azure's latest role definitions
* Good, because handles all roles including custom organization roles that are not in the plan (fetched from Azure by ID)
* Bad, because requires Azure authentication at runtime — service principal credentials, managed identity, or interactive login
* Bad, because does not work in air-gapped or restricted CI environments
* Bad, because introduces a network dependency — analysis fails if Azure is unreachable
* Bad, because adds latency to every analysis run (API calls for each unique role)
* Bad, because the Azure REST API for role definitions requires at least Reader access at some scope — not always available to CI service principals

## More Information

### Azure RBAC Permission Model

Azure role definitions use a four-field permission model:

- **actions**: Control plane operations allowed (e.g., `Microsoft.Compute/virtualMachines/start/action`)
- **notActions**: Control plane operations excluded from actions (subtraction, not deny)
- **dataActions**: Data plane operations allowed (e.g., `Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read`)
- **notDataActions**: Data plane operations excluded from dataActions

The `notActions` field is critical to understand: it is **not a deny**. It subtracts from the `actions` list. If a separate role assignment grants the excluded permission, the principal still has it. This means `notActions` reduces a single role's effective permissions but does not prevent access granted through other role assignments.

For the scoring algorithm, this means we score each role definition in isolation — `notActions` reduces that role's score, but we cannot account for the cumulative effect of multiple role assignments on the same principal (that would require analyzing all assignments for a principal, which is beyond the scope of plan-level analysis).

### Data Source: Azure Built-in Role Definitions

The primary data source is [AzAdvertizer](https://www.azadvertizer.net/azrolesadvertizer_all.html), which publishes all Azure built-in role definitions with full permission sets as a publicly downloadable CSV at `https://www.azadvertizer.net/azrolesadvertizer-comma.csv`. The CSV includes Actions, NotActions, DataActions, and NotDataActions for each role. No authentication is required — the data can be fetched with a simple HTTP GET, making it usable in any CI environment without Azure credentials or service principals.

Azure also publishes built-in role definitions through the ARM REST API (`az role definition list --custom-role-only false`), but this requires Azure CLI authentication which adds credential management overhead and prevents contributors without Azure subscriptions from regenerating the data.

The `microsoft/azure-roles` GitHub repository provides a name-to-GUID mapping that is updated daily via GitHub Actions, but it does not include permission sets — insufficient for permission-based scoring.

Reference: TFLint's `tflint-ruleset-azurerm` uses a similar pattern — it pulls Azure API specifications from `Azure/azure-rest-api-specs` as a git submodule and regenerates validation rules from them.

### Impact on Plugin Architecture

This change introduces the first cross-resource analysis in the plugin codebase (the privilege analyzer querying `azurerm_role_definition` resources while processing `azurerm_role_assignment`). The SDK already supports this — `Runner.GetResourceChanges` accepts arbitrary patterns — but no existing analyzer uses this capability. This establishes a pattern that other analyzers may follow (e.g., a network analyzer cross-referencing NSG rules with subnet associations).

This change also introduces the first embedded data dependency in a plugin. The `//go:embed` directive and the nightly CI refresh pattern may be reused by other plugins that need provider-specific reference data (e.g., an AWS plugin embedding IAM action definitions).

Related: [ADR-0003](ADR-0003-provider-agnostic-core-with-deep-inspection-plugins.md) — Three-layer classification model. This ADR improves the Layer 3 deep inspection capability for Azure privilege analysis.

Related: [ADR-0002](ADR-0002-grpc-plugin-architecture.md) — gRPC plugin architecture. The `Runner.GetResourceChanges` interface used for cross-resource lookup is defined by this architecture.
