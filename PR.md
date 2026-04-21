# CR-0036: Skip classification rule evaluation for no-op resources

## What

No-op resource changes (`actions = ["no-op"]`) now short-circuit in the classifier: they bypass rule iteration, inherit `defaults.no_changes`, and get a synthetic matched-rule description. Overall classification and text-output breakdown already exclude them; this makes the bypass the primary path instead of a workaround.

## Why

Since **CR-0034** (`ignore_attributes`), tag-only updates get downgraded to `["no-op"]` — routinely. The classifier still evaluated rules over them, so any rule without an explicit `not_actions = ["no-op"]` silently matched and elevated `Overall`. The text renderer hid the no-op resources from view, so the output looked like this:

```
Classification: major
Resources: 1
[minor] (1 resources)
  - data.azapi_resource_action.account_keys[0] (azapi_resource_action) [read]
(95 no-op resources hidden)
```

`major` — with zero visible major resources. The output was lying.

Every one of the 16 e2e configs in this repo carried the same workaround rule to absorb no-ops at the lowest precedence — strong evidence the pattern was boilerplate the mental model imposed, not genuine configuration.

## Value

- **Honest output**: `Classification: minor` now means "the highest real change is minor". No mental translation through the filter.
- **Zero boilerplate**: rule authors no longer need `not_actions = ["no-op"]` on every rule, and no longer need a catch-all `actions = ["no-op"]` rule.
- **Transparency**: hidden no-op count is broken down by classification (`(95 no-op resources hidden — major: 4, minor: 91)`) so authors can sanity-check what the filter absorbed.
- **Backward compatible**: existing configs with the workaround rule still parse and run. The rule just becomes dead code.

## Usage

Before (required workaround):

```hcl
classification "major" {
  rule {
    resource    = ["azurerm_key_vault_key"]
    not_actions = ["no-op"]   # boilerplate to dodge cosmetic matches
  }
}

classification "auto" {
  rule {
    resource = ["*"]
    actions  = ["no-op"]   # catch-all to absorb no-ops
  }
}
```

After:

```hcl
classification "major" {
  rule {
    resource = ["azurerm_key_vault_key"]
  }
}

classification "auto" {
  description = "No approval needed"
  # no rules — no-op resources short-circuit here via defaults.no_changes
}

defaults {
  no_changes        = "auto"
  ignore_attributes = ["tags", "tags_all"]
}
```

## Scope

- `classifyResource` and `explainResource` short-circuit on `isNoOp(change.Actions)`.
- `Classify` and `AddPluginDecisions` continue to exclude no-op decisions from `Overall` (defense in depth for plugin decisions).
- Text output: per-classification breakdown of hidden no-op counts.
- Validator exempts the classification referenced by `defaults.no_changes` from the "has no rules" warning.
- Workaround rule removed from all 16 e2e configs and the full-reference example; `newTestConfig` updated.
- CR-0034 gets a follow-up note pointing at CR-0036.

## Verification

- `make ci` — build, test, vet, lint, govulncheck all pass.
- `bash testdata/e2e/run.sh --build --fixtures` — 16/16 pass after workaround removal.
- New tests: `TestClassifyResource_NoOpShortCircuit`, `TestClassifyResource_NativeNoOp`, `TestExplainClassify_NoOpSingleTraceEntry`, `TestClassify_NoOpDoesNotElevateOverall`, `TestFormatText_MixedNoOpBreakdownVerbose`.

Full spec: `docs/cr/CR-0036-skip-noop-rule-evaluation.md`.
