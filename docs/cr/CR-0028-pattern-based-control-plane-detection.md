---
name: cr-0028-pattern-based-control-plane-detection
description: Replace opinionated control-plane scoring tiers with user-configured pattern-based detection, aligning control-plane with the data-plane approach from CR-0027.
id: "CR-0028"
status: "approved"
date: 2026-02-16
requestor: jokarl
stakeholders:
  - jokarl
priority: "medium"
target-version: next
---

# Pattern-Based Control-Plane Detection

## Change Summary

Replace the opinionated control-plane scoring model (7 tiers from 0-95, scope multipliers, `score_threshold`) with the same pattern-based approach used for data-plane detection in CR-0027. Users configure which control-plane action patterns are risky via an `actions` field in classification-scoped config, instead of relying on built-in scores and thresholds. Additionally, introduce a `scopes` filter on role assignment scope level and a `flag_unknown_roles` safety net for roles whose permissions cannot be resolved.

## Motivation and Background

The current scoring system assigns fixed severity scores to permission patterns:

| Tier | Score | Pattern |
|------|-------|---------|
| 1 | 95 | Unrestricted `*` without auth exclusion |
| 2 | 85 | `Microsoft.Authorization/*` control |
| 3 | 75 | Targeted `roleAssignments/write` |
| 4 | 70 | `*` with `Microsoft.Authorization` excluded |
| 5 | 50-65 | Provider wildcards |
| 6 | 30 | Limited write |
| 7 | 15 | Read-only |

These tiers are reasonable approximations but are fundamentally opinionated. What constitutes a "dangerous" control-plane action varies by organization — one org may consider `Microsoft.Authorization/*` critical while another treats it as standard. The `score_threshold` mechanism from CR-0024 helps but still relies on fixed scores that may not align with organizational risk models.

CR-0027 introduces pattern-based detection for data-plane actions. This CR extends the same model to control-plane actions, creating a unified pattern-based approach across both planes.

## Change Drivers

* Scoring tiers encode opinionated risk judgments that don't apply universally
* `score_threshold` is an indirect mechanism — users must reverse-engineer which tiers fall above/below a number instead of declaring what actions matter
* Data-plane (CR-0027) and control-plane should use the same detection model for consistency
* Scope weighting via numeric multipliers is coarse — a `scopes` filter is more explicit and user-controllable
* Roles whose permissions can't be resolved are silently scored with a fallback number, hiding the uncertainty from the user

## Current State

### Control-Plane Scoring

`ScorePermissions` in `scoring.go` evaluates `Actions`/`NotActions` and assigns a score from 0-95 based on 7 tiers. `ApplyScopeMultiplier` adjusts the score: management group 1.1x, subscription 1.0x, resource group 0.8x, resource 0.6x. The weighted score is compared against `score_threshold` to decide whether to emit a decision.

### Data-Plane Detection (CR-0027)

CR-0027 adds `data_actions` pattern matching. Effective data actions (`DataActions` minus `NotDataActions`) are matched against user-configured patterns. Data-plane triggers are independent from control-plane scoring.

### Unknown Role Handling

Roles not found in the built-in database, not cross-referenced from plan custom roles, and not in the `PrivilegedRoles` config fallback are assigned a fixed severity (`UnknownRoleSeverity`, default 50). This silently masks the fact that the role's actual permissions are unknown.

## Proposed Change

### Configuration

```hcl
classification "critical" {
  description = "Requires security review"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  azurerm {
    privilege_escalation {
      actions            = ["*", "Microsoft.Authorization/*"]
      data_actions       = ["*/read"]
      scopes             = ["subscription", "management_group"]
      exclude            = ["AcrPush"]
      flag_unknown_roles = true
    }
  }
}

classification "standard" {
  description = "Standard change"

  rule { resource = ["*"] }

  azurerm {
    privilege_escalation {
      actions      = ["*/write", "*/delete"]
      data_actions = ["*/write", "*/delete"]
      scopes       = ["subscription", "management_group", "resource_group"]
    }
  }
}

classification "auto-approved" {
  description = "No approval needed"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

precedence = ["critical", "standard", "auto-approved"]
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `actions` | `[]string` | `[]` (skip) | Control-plane action patterns to match against effective `Actions` minus `NotActions`. Omit to skip control-plane detection. |
| `data_actions` | `[]string` | `[]` (skip) | Data-plane action patterns to match against effective `DataActions` minus `NotDataActions`. From CR-0027. |
| `scopes` | `[]string` | `[]` (any scope) | Assignment scope levels to consider. Values: `management_group`, `subscription`, `resource_group`, `resource`. Omit to match any scope. |
| `exclude` | `[]string` | `[]` | Role names to skip (case-insensitive). Unchanged from CR-0024. |
| `flag_unknown_roles` | `bool` | `true` | Flag roles whose permissions cannot be resolved. See [Unknown Role Handling](#unknown-role-handling). |

### How It Works

When a role assignment is analyzed for a classification:

1. **Resolve role** — look up permissions via built-in database, plan cross-reference (`azurerm_role_definition`), or config fallback. If resolution fails, handle via `flag_unknown_roles`.
2. **Check scope filter** — if `scopes` is configured, skip unless the assignment's ARM scope matches one of the listed levels. Uses existing `ParseScopeLevel`.
3. **Check exclude** — skip if the role name is in the `exclude` list.
4. **Control-plane pattern matching** — if `actions` is configured, compute effective actions via `computeEffectiveActions(perm.Actions, perm.NotActions)`, match against configured patterns. If any effective action matches any pattern, flag.
5. **Data-plane pattern matching** — if `data_actions` is configured, compute effective data actions via `computeEffectiveActions(perm.DataActions, perm.NotDataActions)`, match against configured patterns. If any effective data action matches any pattern, flag.
6. **Either can trigger independently** — control-plane match, data-plane match, or both. Any match emits a decision.

### Unknown Role Handling

When a role's permissions cannot be resolved (not in built-in DB, no `azurerm_role_definition` in plan, ID not resolvable):

- If `flag_unknown_roles = true` (default): emit a decision with diagnostic metadata explaining why resolution failed
- If `flag_unknown_roles = false`: skip silently

Decision output for unknown roles:

```json
{
  "classification": "critical",
  "reason": "unknown role 'Custom Deploy Agent' flagged (role permissions could not be resolved)",
  "metadata": {
    "analyzer": "privilege-escalation",
    "trigger": "unknown-role",
    "role_name": "Custom Deploy Agent",
    "resolution_attempts": [
      "not found in built-in role database",
      "no azurerm_role_definition resource in plan",
      "role definition ID not resolvable"
    ]
  }
}
```

This gives users a clear path to resolve: add the `azurerm_role_definition` to their plan, or add the role to `exclude` if they know it's safe.

### Decision Output

Decisions no longer carry a severity score. The classification name is the output.

**Control-plane trigger:**
```json
{
  "classification": "critical",
  "reason": "role 'User Access Administrator' grants control-plane access matching configured patterns",
  "metadata": {
    "analyzer": "privilege-escalation",
    "trigger": "control-plane",
    "matched_actions": ["Microsoft.Authorization/roleAssignments/write"],
    "matched_patterns": ["Microsoft.Authorization/*"],
    "role_source": "builtin",
    "scope": "/subscriptions/...",
    "scope_level": "subscription"
  }
}
```

**Data-plane trigger** (unchanged from CR-0027):
```json
{
  "classification": "critical",
  "reason": "role 'Storage Blob Data Owner' grants data-plane access matching configured patterns",
  "metadata": {
    "analyzer": "privilege-escalation",
    "trigger": "data-plane",
    "matched_data_actions": ["Microsoft.Storage/.../blobs/read"],
    "matched_patterns": ["*/read"],
    "role_source": "builtin",
    "scope": "/subscriptions/...",
    "scope_level": "subscription"
  }
}
```

### NotActions Subtraction

Applies identically to both planes, using existing `computeEffectiveActions`:

```
# Config: actions = ["Microsoft.Authorization/*"]

# Contributor role:
# Actions: ["*"]
# NotActions: ["Microsoft.Authorization/*"]
# Effective: ["*"] (wildcard kept as-is, but auth is excluded)
# Does NOT match "Microsoft.Authorization/*" — Contributor is safe

# User Access Administrator role:
# Actions: ["Microsoft.Authorization/*", "Microsoft.Support/*", ...]
# NotActions: []
# Effective includes Microsoft.Authorization/* -> MATCHES -> flagged
```

## Items Removed

| Item | Location | Replacement |
|------|----------|-------------|
| `ScorePermissions` function | `scoring.go` | `actions` / `data_actions` pattern matching |
| Scoring tiers (0-95) | `scoring.go` | User-configured patterns |
| `scorePermissionBlock` | `scoring.go` | Removed |
| `ApplyScopeMultiplier` | `scoring.go` | `scopes` filter |
| Scope multiplier constants | `scoring.go` | `scopes` filter |
| `score_threshold` config field | `config.go`, `loader.go`, `plugin.go` | `actions` field |
| `PermissionScore` struct | `scoring.go` | Not needed — no numeric score |
| `UnknownRoleSeverity` config | `plugin.go` | `flag_unknown_roles` |
| `UnknownPrivilegedSeverity` config | `plugin.go` | `flag_unknown_roles` |
| `Severity` field on privilege decisions | `privilege.go` | Removed — classification name is the output |

## Items Retained

| Item | Location | Reason |
|------|----------|--------|
| `actionMatchesPattern` | `scoring.go` | Core matching logic, reused by both planes |
| `matchesAny` | `scoring.go` | Used by pattern matching |
| `computeEffectiveActions` | `scoring.go` | Used by both planes for NotActions subtraction |
| `ParseScopeLevel` | `scoring.go` | Used by `scopes` filter |
| `RoleDatabase` and built-in roles | `roles.go` | Permission source for built-in role resolution |
| Custom role cross-referencing | `privilege.go` | Permission source for plan-defined custom roles |
| `exclude` config field | Unchanged | Role name exclusion |
| `roles` config field | Unchanged | Role name inclusion filter |

## Requirements

### Functional Requirements

1. The `privilege_escalation` analyzer **MUST** support an `actions` field containing a list of Azure RBAC control-plane action patterns
2. When `actions` is configured, the analyzer **MUST** compute effective control-plane actions by subtracting `NotActions` from `Actions` using `computeEffectiveActions`, then match against configured patterns using `actionMatchesPattern`
3. Control-plane and data-plane pattern matching **MUST** trigger independently — either can cause a decision
4. When `actions` is omitted or empty, the analyzer **MUST NOT** evaluate control-plane actions
5. The `scopes` field **MUST** filter role assignments by ARM scope level before pattern matching
6. When `scopes` is omitted or empty, the analyzer **MUST** match role assignments at any scope level
7. The `scopes` filter **MUST** apply to both control-plane and data-plane triggers
8. When `flag_unknown_roles` is `true` (default), the analyzer **MUST** emit a decision for roles whose permissions cannot be resolved, with metadata listing resolution attempts
9. When `flag_unknown_roles` is `false`, the analyzer **MUST** silently skip unresolvable roles
10. `ScorePermissions`, scoring tiers, scope multipliers, and `score_threshold` **MUST** be removed
11. Privilege escalation decisions **MUST NOT** carry a numeric severity score
12. Custom role cross-referencing from `azurerm_role_definition` plan resources **MUST** be retained

### Non-Functional Requirements

1. Pattern matching **MUST** reuse existing `actionMatchesPattern`, `matchesAny`, and `computeEffectiveActions` functions
2. The `scopes` filter **MUST** reuse existing `ParseScopeLevel`
3. The change is intentionally breaking — no backward compatibility with `score_threshold` is required

## Affected Components

| File | Change |
|------|--------|
| `plugins/azurerm/scoring.go` | Remove `ScorePermissions`, `scorePermissionBlock`, `PermissionScore`, `ApplyScopeMultiplier`, scope multiplier constants. Retain `actionMatchesPattern`, `matchesAny`, `computeEffectiveActions`, `ParseScopeLevel`. |
| `plugins/azurerm/privilege.go` | Replace score-based escalation detection with pattern matching. Add `scopes` filter. Add `flag_unknown_roles` handling with diagnostic metadata. Remove severity from decisions. |
| `plugins/azurerm/plugin.go` | Replace `PrivilegeEscalationAnalyzerConfig` fields: remove `ScoreThreshold`, add `Actions`, `Scopes`, `FlagUnknownRoles`. Remove `UnknownRoleSeverity`, `UnknownPrivilegedSeverity` from `PluginConfig`. |
| `pkg/config/config.go` | Replace `PrivilegeEscalationConfig` fields: remove `ScoreThreshold`, add `Actions`, `Scopes`, `FlagUnknownRoles`. |
| `pkg/config/loader.go` | Update `parsePrivilegeEscalationConfig`: remove `score_threshold` parsing, add `actions`, `scopes`, `flag_unknown_roles` parsing. |
| `plugins/azurerm/scoring_test.go` | Remove tests for `ScorePermissions`, `scorePermissionBlock`, `ApplyScopeMultiplier`. Add/update tests for retained functions. |
| `plugins/azurerm/privilege_test.go` | Rewrite tests: replace score-based assertions with pattern-based assertions. Add tests for `scopes` filter, `flag_unknown_roles`, unknown role diagnostics. |
| `testdata/e2e/role-escalation-threshold/` | Rewrite: replace `score_threshold` with `actions` patterns. |
| `testdata/e2e/role-assignment-privileged/` | Update config to use `actions` patterns instead of relying on default scoring. |
| `testdata/e2e/custom-role-cross-reference/` | **New.** E2e scenario with `azurerm_role_definition` in plan, pattern-matched via cross-referencing. |

## Acceptance Criteria

### AC-1: Control-plane pattern matching

```gherkin
Given a classification "critical" with privilege_escalation { actions = ["Microsoft.Authorization/*"] }
  And a plan that assigns "User Access Administrator" (Actions include Microsoft.Authorization/*)
When the plugin analyzes the resource
Then a decision is emitted with Classification = "critical"
  And metadata contains trigger = "control-plane"
  And metadata contains matched_patterns = ["Microsoft.Authorization/*"]
```

### AC-2: NotActions subtraction prevents false positives

```gherkin
Given a classification "critical" with privilege_escalation { actions = ["Microsoft.Authorization/*"] }
  And a plan that assigns "Contributor" (Actions: ["*"], NotActions: ["Microsoft.Authorization/*"])
When effective control-plane actions are computed
Then Microsoft.Authorization actions are excluded
  And the role does NOT match "Microsoft.Authorization/*"
  And no decision is emitted
```

### AC-3: Scopes filter restricts matching

```gherkin
Given a classification "critical" with privilege_escalation { actions = ["*"], scopes = ["subscription", "management_group"] }
  And a plan that assigns "Owner" at resource group scope
When the plugin analyzes the resource
Then the role is skipped because "resource_group" is not in the scopes filter
  And no decision is emitted for this classification
```

### AC-4: Scopes filter applies to data-plane triggers

```gherkin
Given a classification "critical" with privilege_escalation { data_actions = ["*/read"], scopes = ["subscription"] }
  And a plan that assigns "Storage Blob Data Owner" at resource group scope
When the plugin analyzes the resource
Then the role is skipped because "resource_group" is not in the scopes filter
  And no data-plane decision is emitted
```

### AC-5: Omitted scopes matches any scope

```gherkin
Given a classification with privilege_escalation { actions = ["*"] } (no scopes field)
  And a plan that assigns "Owner" at resource scope
When the plugin analyzes the resource
Then the role is matched regardless of scope level
  And a decision is emitted
```

### AC-6: Unknown role flagged with diagnostics

```gherkin
Given a classification with privilege_escalation { actions = ["*"], flag_unknown_roles = true }
  And a plan that assigns a role not in the built-in database
  And no azurerm_role_definition for that role in the plan
When the plugin analyzes the resource
Then a decision is emitted with trigger = "unknown-role"
  And metadata contains resolution_attempts listing why resolution failed
```

### AC-7: Unknown role silently skipped when disabled

```gherkin
Given a classification with privilege_escalation { flag_unknown_roles = false }
  And a plan that assigns an unresolvable role
When the plugin analyzes the resource
Then no decision is emitted for the unknown role
```

### AC-8: Custom role cross-referenced and pattern-matched

```gherkin
Given a classification with privilege_escalation { actions = ["Microsoft.Authorization/*"] }
  And a plan containing an azurerm_role_definition with Actions: ["Microsoft.Authorization/roleAssignments/write"]
  And a role assignment using that custom role
When the plugin analyzes the resource
Then the custom role's permissions are resolved from the plan
  And the role matches "Microsoft.Authorization/*"
  And a decision is emitted with role_source = "plan-custom-role"
```

### AC-9: No severity score on decisions

```gherkin
Given any privilege escalation trigger (control-plane, data-plane, or unknown-role)
When a decision is emitted
Then the decision does NOT contain a numeric severity score
  And the classification name is the sole output signal
```

### AC-10: score_threshold is removed

```gherkin
Given a configuration with privilege_escalation { score_threshold = 80 }
When the config is loaded
Then a validation error is returned indicating score_threshold is no longer supported
```

### AC-11: Control-plane and data-plane trigger independently

```gherkin
Given a classification with privilege_escalation { actions = ["Microsoft.Authorization/*"], data_actions = ["*/read"] }
  And a plan with:
    - Role A: User Access Administrator (control-plane match, no data actions)
    - Role B: Storage Blob Data Owner (no control-plane match, data-plane match)
When the plugin analyzes both resources
Then Role A triggers via control-plane
  And Role B triggers via data-plane
  And both emit decisions with the same classification
```

### AC-12: Backward-incompatible change is intentional

```gherkin
Given a configuration using the old score_threshold syntax
When the user runs tfclassify
Then a clear error message indicates the configuration must be updated
  And the error references the new actions/data_actions syntax
```

## Test Strategy

### Unit Tests to Rewrite

| Test File | Change |
|-----------|--------|
| `plugins/azurerm/scoring_test.go` | Remove `TestScorePermissions*`, `TestApplyScopeMultiplier*`. Keep tests for `actionMatchesPattern`, `matchesAny`, `computeEffectiveActions`, `ParseScopeLevel`. |
| `plugins/azurerm/privilege_test.go` | Rewrite all tests to use `actions`/`data_actions` patterns instead of score assertions. Add `scopes` filter tests, `flag_unknown_roles` tests, unknown role diagnostic tests. |

### Unit Tests to Add

| Test File | Test Name | Description |
|-----------|-----------|-------------|
| `privilege_test.go` | `TestPrivilege_Actions_PatternMatch` | Control-plane action patterns match effective actions |
| `privilege_test.go` | `TestPrivilege_Actions_NotActionsSubtraction` | NotActions prevents false positives (Contributor pattern) |
| `privilege_test.go` | `TestPrivilege_Actions_OmittedSkips` | No `actions` field means no control-plane evaluation |
| `privilege_test.go` | `TestPrivilege_Scopes_Filter` | Role at RG scope skipped when scopes = ["subscription"] |
| `privilege_test.go` | `TestPrivilege_Scopes_OmittedMatchesAny` | No scopes field matches any scope level |
| `privilege_test.go` | `TestPrivilege_Scopes_AppliesToDataPlane` | Scopes filter applies to data-plane triggers too |
| `privilege_test.go` | `TestPrivilege_FlagUnknownRoles_True` | Unknown role flagged with diagnostic metadata |
| `privilege_test.go` | `TestPrivilege_FlagUnknownRoles_False` | Unknown role silently skipped |
| `privilege_test.go` | `TestPrivilege_FlagUnknownRoles_Diagnostics` | Resolution attempts list correct reasons |
| `privilege_test.go` | `TestPrivilege_NoSeverityOnDecisions` | Decisions have no severity field |
| `privilege_test.go` | `TestPrivilege_CustomRole_PatternMatched` | Cross-referenced custom role matched via patterns |
| `config/loader_test.go` | `TestLoadPrivilegeEscalation_Actions` | Parse `actions` attribute from HCL |
| `config/loader_test.go` | `TestLoadPrivilegeEscalation_Scopes` | Parse `scopes` attribute from HCL |
| `config/loader_test.go` | `TestLoadPrivilegeEscalation_FlagUnknownRoles` | Parse `flag_unknown_roles` attribute from HCL |
| `config/loader_test.go` | `TestLoadPrivilegeEscalation_ScoreThresholdRejected` | `score_threshold` produces validation error |

### E2E Tests to Update

| Scenario | Change |
|----------|--------|
| `role-escalation-threshold` | Rewrite config: replace `score_threshold` with `actions` patterns that achieve same graduated behavior |
| `role-assignment-privileged` | Update config to use `actions` patterns |

### E2E Tests to Add

| Scenario | Description |
|----------|-------------|
| `custom-role-cross-reference` | Plan includes `azurerm_role_definition` with specific actions. Role assignment uses the custom role. Config matches via `actions` patterns. Validates end-to-end custom role resolution and pattern matching. |

## Implementation Approach

### Phase 1: Config Changes

1. Remove `ScoreThreshold` from `PrivilegeEscalationConfig`, add `Actions`, `Scopes`, `FlagUnknownRoles`
2. Update `parsePrivilegeEscalationConfig` in loader
3. Add validation error for `score_threshold` (removed field)
4. Mirror changes in plugin-side `PrivilegeEscalationAnalyzerConfig`

### Phase 2: Remove Scoring

1. Remove `ScorePermissions`, `scorePermissionBlock`, `PermissionScore` from `scoring.go`
2. Remove `ApplyScopeMultiplier` and scope multiplier constants
3. Remove `UnknownRoleSeverity`, `UnknownPrivilegedSeverity` from `PluginConfig`
4. Update `scoring_test.go` — remove scoring tests, keep pattern matching tests

### Phase 3: Rewrite Analyzer

1. Replace score-based detection with pattern matching in `analyzeWithConfig`
2. Add `scopes` filter using `ParseScopeLevel`
3. Add `flag_unknown_roles` handling with diagnostic metadata
4. Remove severity from emitted decisions
5. Rewrite `privilege_test.go`

### Phase 4: E2E

1. Rewrite `role-escalation-threshold` and `role-assignment-privileged` configs
2. Add `custom-role-cross-reference` scenario
3. Update CI matrix if needed

## Risks and Mitigation

### Risk 1: Breaking change to all privilege escalation configs

**Likelihood:** certain
**Impact:** medium
**Mitigation:** Intentionally breaking. Produce a clear validation error when `score_threshold` is encountered, pointing users to the new `actions`/`data_actions` syntax. Document migration examples.

### Risk 2: Loss of graduated detection without scoring

**Likelihood:** low
**Impact:** low
**Mitigation:** Pattern-based detection achieves graduation through different patterns per classification. Critical catches `["*", "Microsoft.Authorization/*"]`, standard catches `["*/write"]`. The `scopes` filter adds another graduation axis. This is more explicit than numeric thresholds.

### Risk 3: Unknown role handling changes behavior

**Likelihood:** medium
**Impact:** low
**Mitigation:** `flag_unknown_roles` defaults to `true`, which is more conservative than the current fixed-severity approach. Users who want the old behavior (always emit with a score) get a clearer signal — the diagnostic metadata tells them exactly why the role is unknown.

## Dependencies

* CR-0024: Classification-scoped config infrastructure (implemented)
* CR-0027: Data-plane pattern-based detection (must be implemented first — establishes `data_actions` and the pattern matching flow in the analyzer)

## Related Items

* CR-0016: Permission Scoring Algorithm — the scoring system this CR removes
* CR-0017: Privilege Analyzer Rewrite — introduced the scoring approach being replaced
* CR-0024: Classification-Scoped Plugin Analyzer Rules — introduced `score_threshold` being removed
* CR-0027: Data-Plane Action Detection — establishes the pattern-based model extended here
* `plugins/azurerm/scoring.go` — functions being removed and retained
* `plugins/azurerm/privilege.go` — analyzer being rewritten
