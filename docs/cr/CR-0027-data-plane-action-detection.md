---
name: cr-0027-data-plane-action-detection
description: Add pattern-based data-plane action detection to the privilege escalation analyzer, enabling users to configure which data-plane action patterns trigger classifications.
id: "CR-0027"
status: "approved"
date: 2026-02-16
requestor: jokarl
stakeholders:
  - jokarl
priority: "high"
target-version: next
---

# Data-Plane Action Detection

## Change Summary

Add pattern-based data-plane action detection to the privilege escalation analyzer. Users configure which Azure RBAC data-plane action patterns (e.g., `*/read`, `Microsoft.Storage/*`) are relevant per classification. The analyzer computes effective data actions (`DataActions` minus `NotDataActions`) and matches them against configured patterns. Data-plane and control-plane triggers are independent — either can cause a role to be flagged.

## Motivation and Background

The privilege escalation analyzer's `ScorePermissions` function evaluates only control-plane actions (`Actions`/`NotActions`). Data-plane actions (`DataActions`/`NotDataActions`) are completely ignored. This means 127 built-in Azure roles that have *only* data-plane permissions score 0 — including roles like "Storage Blob Data Owner" and "FHIR Data Contributor" that grant broad data access.

Data-plane risk is fundamentally organization-dependent. A `/read` on storage is catastrophic in banking (data exfiltration) but benign in many other domains. Universal scoring doesn't work for data-plane actions — there are no reasonable defaults. Instead, users should configure which data-plane action patterns they consider risky, and the analyzer should flag roles that match.

## Change Drivers

* 127 built-in Azure roles with data-plane-only permissions score 0, creating a blind spot
* Data-plane risk is organization-dependent — no universal scoring is possible
* `NotDataActions` can make a role entirely acceptable (e.g., write-only without read = no exfiltration risk), but this is currently invisible
* Users need a simple, opt-in mechanism to detect data-plane access patterns relevant to their domain
* The pattern-based approach aligns with Azure RBAC's own action matching semantics

## Current State

### ScorePermissions Ignores DataActions

`scoring.go:ScorePermissions` iterates `role.Permissions` and calls `scorePermissionBlock`, which only examines `perm.Actions` and `perm.NotActions`. The `DataActions` and `NotDataActions` fields are parsed (they exist on the `Permission` struct and are extracted from plan JSON in `parsePermissionsFromPlan`) but never evaluated for scoring.

### Impact on Built-in Roles

The embedded role database contains 400+ Azure built-in roles. Of these, 127 roles have *only* data-plane permissions (empty `Actions`, non-empty `DataActions`). These roles all score 0 regardless of how broad their data access is:

| Role | DataActions | Score |
|------|-------------|-------|
| Storage Blob Data Owner | `Microsoft.Storage/storageAccounts/blobServices/containers/*` | 0 |
| FHIR Data Contributor | `Microsoft.HealthcareApis/services/fhir/resources/*` | 0 |
| Cognitive Services OpenAI User | `Microsoft.CognitiveServices/accounts/OpenAI/*` | 0 |
| Key Vault Crypto User | `Microsoft.KeyVault/vaults/keys/read`, `.../encrypt/action`, etc. | 0 |

### Existing Infrastructure

The codebase already has the building blocks needed:

- `computeEffectiveActions(actions, notActions []string) []string` — subtracts notActions from actions using pattern matching. Works identically for data-plane actions.
- `actionMatchesPattern(action, pattern string) bool` — Azure RBAC glob matching (`*`, `Microsoft.X/*`, `*/read`). Case-insensitive.
- `matchesAny(action string, patterns []string) bool` — checks if an action matches any pattern in a list.
- `Permission` struct has `DataActions` and `NotDataActions` fields already populated.

## Proposed Change

### Configuration

Extends the CR-0024 classification-scoped analyzer config with a `data_actions` field:

```hcl
classification "critical" {
  description = "Requires security review"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  azurerm {
    privilege_escalation {
      score_threshold = 80
      data_actions    = ["*/read"]  # flag roles with ANY data-plane read
    }
  }
}

classification "standard" {
  description = "Standard change"

  rule { resource = ["*"] }

  azurerm {
    privilege_escalation {
      data_actions = ["*/write", "*/delete"]  # flag data-plane writes/deletes
    }
  }
}

classification "auto-approved" {
  description = "No approval needed"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }

  # No data_actions configured — data plane is not checked
  azurerm {
    privilege_escalation {}
  }
}
```

- `data_actions` is a list of Azure RBAC action patterns (reuses existing `actionMatchesPattern()` from `scoring.go`)
- `"*/read"` matches any data read, `"Microsoft.Storage/*"` matches all storage data actions, `"*"` matches everything
- If omitted or empty, data plane is not checked (opt-in)

### How It Works

When a role assignment is analyzed:

1. **Control-plane scoring** — existing logic unchanged. Score permissions, apply scope multiplier, check `score_threshold`.
2. **Compute effective data actions** — `computeEffectiveActions(perm.DataActions, perm.NotDataActions)`. Reuses existing function.
3. **Match effective data actions against `data_actions` patterns** — if ANY effective data action matches ANY configured pattern, flag.
4. **Either can trigger independently** — a role triggers if its control-plane score exceeds threshold OR its effective data actions match patterns. Both are checked for each classification.

### NotDataActions Subtraction

`NotDataActions` removes permissions before matching. This naturally handles exclusion scenarios:

```
# Banking config: data_actions = ["*/read"]

# Role: Storage Blob Data Owner
# dataActions: ["Microsoft.Storage/.../blobs/*"]
# notDataActions: []
# Effective includes reads -> MATCHES "*/read" -> flagged as critical

# Custom write-only role (notDataActions blocks reads)
# dataActions: ["Microsoft.Storage/.../blobs/*"]
# notDataActions: ["Microsoft.Storage/.../blobs/read"]
# Effective: write/delete only -> NO read actions -> does NOT match "*/read"
# Write-only is acceptable -- not flagged

# Role fully neutralized by notDataActions
# dataActions: ["Microsoft.Storage/.../blobs/read"]
# notDataActions: ["Microsoft.Storage/.../blobs/read"]
# Effective: empty -> matches nothing -> not flagged

# Standard config: data_actions = ["*/write", "*/delete"]
# Same write-only role from above -> MATCHES "*/write" -> flagged as standard
```

### Decision Output

When data-plane actions trigger a classification, the emitted decision includes:

```json
{
  "classification": "critical",
  "reason": "role 'Storage Blob Data Owner' grants data-plane access matching configured patterns",
  "metadata": {
    "analyzer": "privilege-escalation",
    "trigger": "data-plane",
    "matched_data_actions": ["Microsoft.Storage/.../blobs/read"],
    "matched_patterns": ["*/read"],
    "role_source": "builtin"
  }
}
```

No severity score for data-plane triggers — the classification name is the output. The role either matches the patterns in a classification or it doesn't.

When control-plane scoring triggers (existing behavior), the decision output is unchanged — `trigger` is `"control-plane"` (or absent for backward compatibility).

### Key Design Decisions

1. **No universal data-plane scoring** — Risk of `/read` depends on what's being read and who's reading. Banking vs. generic orgs have fundamentally different risk profiles. Users configure what matters.

2. **Reuse `computeEffectiveActions` and `actionMatchesPattern`** — `NotDataActions` subtraction and pattern matching already work correctly in `scoring.go`. No new matching logic needed.

3. **`data_actions` is per-classification** — Different classifications can care about different data-plane patterns. Critical might flag reads; standard might flag writes. This fits naturally into the CR-0024 classification-scoped config.

4. **Omit = don't check** — If `data_actions` is not configured, data plane is not evaluated. This is the right default — users should opt in to data-plane detection because there are no reasonable defaults.

5. **Control and data triggers are independent** — A role can trigger on control-plane score alone, data-plane patterns alone, or both. Both are checked for each classification.

6. **`NotDataActions` makes roles acceptable by subtraction** — A role with broad data access but `NotDataActions` blocking reads won't match `data_actions = ["*/read"]`. This handles write-only-is-acceptable naturally without special-casing.

## Requirements

### Functional Requirements

1. The privilege escalation analyzer **MUST** support an optional `data_actions` configuration field containing a list of Azure RBAC action patterns
2. When `data_actions` is configured, the analyzer **MUST** compute effective data actions by subtracting `NotDataActions` from `DataActions` using `computeEffectiveActions`
3. The analyzer **MUST** match effective data actions against configured `data_actions` patterns using `actionMatchesPattern` (case-insensitive, supporting `*`, `Microsoft.X/*`, `*/read` patterns)
4. A role **MUST** trigger a classification if ANY effective data action matches ANY configured `data_actions` pattern
5. Data-plane triggering **MUST** be independent from control-plane score triggering — either can cause a decision independently
6. When `data_actions` is omitted or empty, the analyzer **MUST NOT** evaluate data-plane actions (opt-in behavior)
7. The emitted decision for data-plane triggers **MUST** include `trigger: "data-plane"`, `matched_data_actions`, and `matched_patterns` in metadata
8. The `data_actions` field **MUST** be per-classification, allowing different classifications to match different data-plane patterns
9. `NotDataActions` subtraction **MUST** occur before pattern matching — a role whose data actions are fully neutralized by `NotDataActions` **MUST NOT** trigger

### Non-Functional Requirements

1. The change **MUST** be backward-compatible — configurations without `data_actions` behave identically to current behavior
2. Pattern matching **MUST** reuse existing `actionMatchesPattern`, `matchesAny`, and `computeEffectiveActions` functions without modification
3. Performance **SHOULD NOT** degrade for configurations without `data_actions` — the data-plane path is only entered when the field is configured

## Affected Components

| File | Change |
|------|--------|
| `plugins/azurerm/privilege.go` | Add `data_actions` pattern matching after control-plane scoring in `analyzeWithConfig()`. Compute effective data actions, match against config patterns. Emit data-plane decision with appropriate metadata. |
| `plugins/azurerm/privilege_test.go` | Add tests for data-plane pattern matching, `NotDataActions` subtraction, write-only not matching read patterns, empty effective actions, combined control-plane and data-plane triggers |
| `plugins/azurerm/scoring.go` | No change — reuse `computeEffectiveActions`, `actionMatchesPattern`, `matchesAny` as-is |
| `plugins/azurerm/plugin.go` | Add `DataActions []string` field to `PrivilegeEscalationAnalyzerConfig` |
| `pkg/config/config.go` | Add `DataActions []string` field to `PrivilegeEscalationConfig` with `hcl:"data_actions,optional"` and `json:"data_actions,omitempty"` tags |
| `pkg/config/loader.go` | Parse `data_actions` attribute from `privilege_escalation` blocks (extends CR-0024 parsing in `parsePrivilegeEscalationConfig`) |

## Scope Boundaries

### In Scope

* `data_actions` configuration field on `privilege_escalation` analyzer config
* Effective data action computation using existing `computeEffectiveActions`
* Pattern matching using existing `actionMatchesPattern`
* Decision metadata for data-plane triggers
* Unit tests covering pattern matching, subtraction, and edge cases
* Backward compatibility (no `data_actions` = no data-plane evaluation)

### Out of Scope

* Applying pattern-based detection to control-plane actions (deferred to CR-0028)
* Changes to `ScorePermissions` or the scoring tier system
* Default data-plane action patterns (there are no reasonable defaults)
* Changes to the gRPC protocol (the `analyzer_config` bytes field from CR-0024 already transports arbitrary config)
* New analyzers or plugin changes beyond the privilege escalation analyzer

## Acceptance Criteria

### AC-1: Data-plane pattern matching triggers classification

```gherkin
Given a classification "critical" with privilege_escalation { data_actions = ["*/read"] }
  And a plan that assigns "Storage Blob Data Owner" (dataActions: ["Microsoft.Storage/.../blobs/*"])
When the plugin analyzes the resource
Then a decision is emitted with Classification = "critical"
  And metadata contains trigger = "data-plane"
  And metadata contains matched_patterns = ["*/read"]
```

### AC-2: NotDataActions subtraction prevents false positives

```gherkin
Given a classification "critical" with privilege_escalation { data_actions = ["*/read"] }
  And a role with dataActions: ["Microsoft.Storage/.../blobs/*"]
  And notDataActions: ["Microsoft.Storage/.../blobs/read"]
When effective data actions are computed
Then read actions are removed from the effective set
  And the effective actions do not match "*/read"
  And no decision is emitted for data-plane
```

### AC-3: Write-only role does not match read patterns

```gherkin
Given a classification "critical" with privilege_escalation { data_actions = ["*/read"] }
  And a write-only role (dataActions blocked reads via notDataActions)
When the plugin analyzes the resource
Then no data-plane decision is emitted for "critical"
  But the same role DOES match a standard config with data_actions = ["*/write"]
```

### AC-4: Empty effective data actions match nothing

```gherkin
Given a role with dataActions: ["Microsoft.Storage/.../blobs/read"]
  And notDataActions: ["Microsoft.Storage/.../blobs/read"]
When effective data actions are computed
Then the effective set is empty
  And no data-plane pattern matches
  And no data-plane decision is emitted
```

### AC-5: Omitted data_actions skips data-plane evaluation

```gherkin
Given a classification with privilege_escalation {} (no data_actions field)
  And a plan that assigns "Storage Blob Data Owner"
When the plugin analyzes the resource
Then data-plane actions are not evaluated
  And the role is only checked against control-plane score_threshold
```

### AC-6: Control-plane and data-plane triggers are independent

```gherkin
Given a classification "critical" with privilege_escalation { score_threshold = 80, data_actions = ["*/read"] }
  And a plan with:
    - Role A: Owner (control-plane score 95, no data actions)
    - Role B: Storage Blob Data Owner (control-plane score 0, data-plane read actions)
When the plugin analyzes both resources
Then Role A triggers via control-plane (score 95 >= 80)
  And Role B triggers via data-plane (matches "*/read")
  And both emit decisions with Classification = "critical"
```

### AC-7: Multiple data_actions patterns match independently

```gherkin
Given a classification with privilege_escalation { data_actions = ["*/read", "*/write"] }
  And a role with effective data actions including only write operations
When the plugin analyzes the resource
Then the role matches "*/write" (even though it does not match "*/read")
  And a decision is emitted
```

### AC-8: Backward compatibility with existing configurations

```gherkin
Given an existing configuration with privilege_escalation { score_threshold = 80 }
  And no data_actions field
When the config is loaded and the plugin analyzes resources
Then behavior is identical to before this change
  And no data-plane evaluation occurs
```

### AC-9: Data-plane decision metadata is complete

```gherkin
Given a data-plane trigger
When a decision is emitted
Then metadata includes:
  | key                  | description                                  |
  | analyzer             | "privilege-escalation"                        |
  | trigger              | "data-plane"                                  |
  | matched_data_actions | list of effective data actions that matched    |
  | matched_patterns     | list of configured patterns that were matched  |
  | role_source          | "builtin", "plan-custom-role", or "unknown"    |
```

### AC-10: Per-classification data_actions patterns

```gherkin
Given classification "critical" with data_actions = ["*/read"]
  And classification "standard" with data_actions = ["*/write", "*/delete"]
  And a write-only role (reads blocked by notDataActions)
When the plugin analyzes the resource for each classification
Then the role does NOT match "critical" (no read actions)
  And the role DOES match "standard" (has write actions)
```

## Test Strategy

### Unit Tests to Add

| Test File | Test Name | Description |
|-----------|-----------|-------------|
| `plugins/azurerm/privilege_test.go` | `TestPrivilege_DataActions_PatternMatch` | Role with data-plane read actions matches `*/read` pattern |
| `plugins/azurerm/privilege_test.go` | `TestPrivilege_DataActions_NotDataActionsSubtraction` | `NotDataActions` removes actions before pattern matching |
| `plugins/azurerm/privilege_test.go` | `TestPrivilege_DataActions_WriteOnlyNotMatchingRead` | Write-only role (reads blocked) does not match `*/read` |
| `plugins/azurerm/privilege_test.go` | `TestPrivilege_DataActions_EmptyEffective` | Role with fully neutralized data actions matches nothing |
| `plugins/azurerm/privilege_test.go` | `TestPrivilege_DataActions_OmittedSkipsEvaluation` | No `data_actions` config means no data-plane evaluation |
| `plugins/azurerm/privilege_test.go` | `TestPrivilege_DataActions_IndependentFromControlPlane` | Control-plane and data-plane trigger independently |
| `plugins/azurerm/privilege_test.go` | `TestPrivilege_DataActions_MultiplePatterns` | Multiple patterns in `data_actions` are OR'd |
| `plugins/azurerm/privilege_test.go` | `TestPrivilege_DataActions_DecisionMetadata` | Verify trigger, matched_data_actions, matched_patterns in metadata |
| `plugins/azurerm/privilege_test.go` | `TestPrivilege_DataActions_PerClassification` | Different classifications match different data-plane patterns |
| `plugins/azurerm/privilege_test.go` | `TestPrivilege_DataActions_BackwardCompatibility` | Existing configs without `data_actions` behave identically |

### Unit Tests Unchanged

| Test File | Reason |
|-----------|--------|
| `plugins/azurerm/scoring_test.go` | No changes to scoring functions |
| `plugins/azurerm/privilege_test.go` (existing) | All existing tests remain valid — `data_actions` is opt-in, so existing behavior is unchanged |

### Config Parsing Tests

| Test File | Test Name | Description |
|-----------|-----------|-------------|
| `pkg/config/loader_test.go` | `TestLoadPrivilegeEscalationWithDataActions` | Parse `data_actions` attribute from HCL |
| `pkg/config/loader_test.go` | `TestLoadPrivilegeEscalationWithoutDataActions` | Omitted `data_actions` results in nil/empty slice |

## Implementation Approach

### Phase 1: Config

1. Add `DataActions []string` to `PrivilegeEscalationConfig` in `pkg/config/config.go`
2. Add `data_actions` parsing case in `parsePrivilegeEscalationConfig` in `pkg/config/loader.go`
3. Add `DataActions []string` to `PrivilegeEscalationAnalyzerConfig` in `plugins/azurerm/plugin.go`

### Phase 2: Analyzer Logic

1. In `analyzeWithConfig()`, after the control-plane escalation check, add data-plane pattern matching
2. For each role assignment, resolve the role and iterate its permission blocks
3. Compute effective data actions via `computeEffectiveActions(perm.DataActions, perm.NotDataActions)`
4. Match effective actions against configured `data_actions` patterns via `matchesAny`
5. If any match, emit a decision with data-plane metadata

### Phase 3: Tests

1. Add unit tests for data-plane pattern matching scenarios
2. Add config parsing tests
3. Verify all existing tests still pass

## Risks and Mitigation

### Risk 1: Wildcard data actions with computeEffectiveActions

**Likelihood:** low
**Impact:** low
**Mitigation:** `computeEffectiveActions` has special handling for `["*"]` — it returns `["*"]` as-is. When `data_actions` config contains `"*/read"`, the pattern `"*/read"` will match against `"*"` via `actionMatchesPattern("*", "*/read")` which returns false (correct — `"*"` is not a read action, it's a wildcard). If users want to match wildcard data actions, they can use `data_actions = ["*"]`. This behavior is consistent with Azure RBAC semantics.

### Risk 2: Performance with large role databases

**Likelihood:** low
**Impact:** low
**Mitigation:** Data-plane matching only runs when `data_actions` is configured. The matching loop is O(effective_actions * configured_patterns), both of which are small in practice (typically < 20 effective actions, < 5 patterns).

### Risk 3: Confusion between control-plane and data-plane triggers

**Likelihood:** medium
**Impact:** low
**Mitigation:** The `trigger` metadata field clearly distinguishes `"control-plane"` from `"data-plane"`. Decision reasons also state the trigger type.

## Dependencies

* CR-0024 (classification-scoped plugin analyzer rules) — must be implemented first for per-classification config
* No new external dependencies
* No gRPC protocol changes (config transported via existing `analyzer_config` bytes field)

## Related Items

* CR-0024: Classification-Scoped Plugin Analyzer Rules — provides the per-classification config infrastructure
* CR-0028: Pattern-Based Control-Plane Detection — future CR to align control-plane with this same pattern-based approach
* CR-0016: Permission Scoring Algorithm — the scoring system that currently ignores data-plane actions
* CR-0017: Privilege Analyzer Rewrite — introduced the scoring-based approach extended here
* `plugins/azurerm/scoring.go:201-222` — `computeEffectiveActions` reused for data-plane
* `plugins/azurerm/scoring.go:144-178` — `actionMatchesPattern` reused for data-plane
