---
name: cr-0028-pattern-based-control-plane-detection
description: Replace opinionated control-plane scoring tiers with user-configured pattern-based detection, aligning control-plane with the data-plane approach from CR-0027.
id: "CR-0028"
status: "proposed"
date: 2026-02-16
requestor: jokarl
stakeholders:
  - jokarl
priority: "medium"
target-version: next
---

# Pattern-Based Control-Plane Detection

> **Skeleton CR** — this document outlines the direction and scope. It must be refined with detailed design, acceptance criteria, and implementation plan before implementation begins.

## Change Summary

Replace the opinionated control-plane scoring model (7 tiers from 0-95, scope multipliers, `score_threshold`) with the same pattern-based approach used for data-plane detection in CR-0027. Users configure which control-plane action patterns are risky via an `actions` field in classification-scoped config, instead of relying on built-in scores and thresholds.

## Motivation

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

## Proposed Configuration

```hcl
classification "critical" {
  description = "Requires security review"

  azurerm {
    privilege_escalation {
      actions      = ["*", "Microsoft.Authorization/*"]
      data_actions = ["*/read"]
    }
  }
}

classification "standard" {
  description = "Standard change"

  azurerm {
    privilege_escalation {
      actions      = ["*/write", "*/delete"]
      data_actions = ["*/write", "*/delete"]
    }
  }
}
```

- `actions` replaces `score_threshold` — users list control-plane action patterns that matter per classification
- `data_actions` from CR-0027 works alongside it
- `score_threshold`, `ScorePermissions`, and the scoring tier system are removed
- Scope weighting may be retained or replaced with scope-based pattern filtering (to be refined)

## Scope

### Items to Remove

* `ScorePermissions` function and scoring tiers in `scoring.go`
* `score_threshold` config field
* Scope multiplier calculations (or repurpose as scope-based filtering)
* Severity score in privilege escalation decisions

### Items to Add

* `actions` config field on `privilege_escalation` (list of control-plane action patterns)
* Control-plane pattern matching using existing `actionMatchesPattern` and `computeEffectiveActions`
* Unified decision output format for both control-plane and data-plane triggers

### Open Questions (Require Refinement)

1. **Scope handling**: Should scope weighting be removed entirely, or converted to a scope filter (e.g., `scopes = ["subscription", "management_group"]`)?
2. **Migration path**: How to help users migrate from `score_threshold` to `actions` patterns? Should there be a transitional period supporting both?
3. **Custom role cross-referencing**: Does the pattern-based approach still benefit from cross-referencing `azurerm_role_definition` resources in the plan?
4. **Default patterns**: Should there be a set of recommended (but not default) patterns documented for common use cases?

## Dependencies

* CR-0024: Classification-scoped config infrastructure (implemented)
* CR-0027: Data-plane pattern-based detection (must be implemented first to establish the pattern)

## Related Items

* CR-0016: Permission Scoring Algorithm — the scoring system this CR replaces
* CR-0017: Privilege Analyzer Rewrite — introduced the current scoring approach
* CR-0027: Data-Plane Action Detection — establishes the pattern-based model extended here
