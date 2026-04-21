# CR-0036: Skip classification rule evaluation for no-op resources

## What

No-op resource changes (`actions = ["no-op"]`) now short-circuit in the classifier: they bypass rule iteration, inherit `defaults.no_changes`, and get a synthetic matched-rule description. Downgraded resources remain diagnosable in the output — verbose mode lists them in full; compact mode splits the hidden count between `ignore_attributes` downgrades and native no-ops.

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
- **Diagnosable**: verbose output shows every downgraded resource with its original action, the attribute paths the filter absorbed, and the synthetic matched rule — no need to drop to `tfclassify explain` for the common case.
- **CI-friendly**: compact output surfaces the downgrade footprint as a single count line so logs stay tight, with a hint pointing to `-v` when detail is wanted.
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

Verbose output against a plan with one real data-source read, two tag-only downgrades, and one native no-op:

```
Classification: minor
  Plumbing
Exit code: 1
Resources: 1

[minor] (1 resources)
  Plumbing
  - module.aiwz.data.azapi_resource_action.account_keys[0] (azapi_resource_action) [read]
    Rule: Data reads

Downgraded to no-op by ignore_attributes (2):
  - module.aiwz.azurerm_key_vault_key.cmk (azurerm_key_vault_key)
    Originally: [update]  (ignored: tags.tf-module-l2)
    Rule: no-op (downgraded by ignore_attributes: tags.tf-module-l2)
  - module.aiwz.azurerm_resource_group.ai_app (azurerm_resource_group)
    Originally: [update]  (ignored: tags.tf-module-l2)
    Rule: no-op (downgraded by ignore_attributes: tags.tf-module-l2)

(1 native no-op resources hidden)
```

Compact output (same plan):

```
Classification: minor
Exit code: 1
Resources: 1

  [minor] module.aiwz.data.azapi_resource_action.account_keys[0]
  (3 no-op resources hidden — 2 downgraded by ignore_attributes, 1 native; rerun with -v for detail)
```

## Scope

- `classifyResource` and `explainResource` short-circuit on `isNoOp(change.Actions)`.
- `Classify` and `AddPluginDecisions` continue to exclude no-op decisions from `Overall` (defense in depth for plugin decisions).
- Verbose text output: dedicated "Downgraded to no-op by ignore_attributes" section per resource, plus a native-no-op count line.
- Compact text output: split count between downgrades and native no-ops, with a rerun-verbose hint when downgrades exist.
- Validator exempts the classification referenced by `defaults.no_changes` from the "has no rules" warning.
- Workaround rule removed from all 16 e2e configs and the full-reference example; `newTestConfig` updated.
- CR-0034 gets a follow-up note pointing at CR-0036.

## Test Coverage

Classifier (`internal/classify/classifier_test.go`):

- `TestClassifyResource_NoOpShortCircuit` — downgraded no-op inherits `defaults.no_changes`, synthetic rule references `ignore_attributes` and the ignored path, and does not reference the major rule that would otherwise match the type.
- `TestClassifyResource_NativeNoOp` — native no-op gets `"no-op (no change)"` synthetic rule.
- `TestExplainClassify_NoOpSingleTraceEntry` — explain emits exactly one synthetic trace entry instead of iterating rules.
- `TestClassify_NoOpDoesNotElevateOverall` — no-op decisions do not raise `Overall` above the highest real classification.
- `TestClassify_AllNoOpReportsNoChanges` and `TestClassify_MixedNoOpAndRealNotNoChanges` continue to pass against the updated `newTestConfig` (auto classification has no rules).

Text output (`internal/output/formatter_test.go`):

- `TestFormatText_DowngradedSectionVerbose` — verbose output renders the "Downgraded to no-op by ignore_attributes (N):" header with per-resource address, type, original actions, ignored paths, and synthetic rule; native no-ops collapse into a count line and are not listed per-resource.
- `TestFormatText_HiddenCountSplitCompact` — compact line splits "X downgraded by ignore_attributes, Y native" with the rerun hint.
- `TestFormatText_HiddenCountOnlyDowngradedCompact` — only downgrades → "downgraded by ignore_attributes; rerun with -v".
- `TestFormatText_HiddenCountOnlyNativeCompact` — only native no-ops → "native", no `ignore_attributes` mention, no rerun hint.
- `TestFormatText_NoChangesWithDowngradedVerbose` and `TestFormatText_NoChangesWithDowngradedNonVerbose` — pre-existing all-no-op cases continue to pass.

End-to-end:

- `make ci` green (build, test, vet, golangci-lint, govulncheck).
- `bash testdata/e2e/run.sh --build --fixtures` — 16/16 pass after workaround removal.

Full spec: `docs/cr/CR-0036-skip-noop-rule-evaluation.md`.
