# Remaining Gaps

## Critical

- **`custom-role-cross-reference` e2e test missing (CR-0028)**: CR-0028 explicitly requires a new e2e scenario `testdata/e2e/custom-role-cross-reference/` that validates end-to-end custom role resolution via `azurerm_role_definition` with pattern matching. The directory does not exist and is not in the CI matrix. This is listed under "E2E Tests to Add" and "Affected Components" in CR-0028.

- **`refresh-role-data.yml` workflow not updated (CR-0028 Req 17)**: CR-0028 requires: "A scheduled maintenance workflow MUST refresh the action registry alongside the existing role database refresh." The `.github/workflows/refresh-role-data.yml` only runs `make generate-roles` — it does not run `make generate-actions`. Must add `make generate-actions` step and also check/commit `plugins/azurerm/actiondata/actions.json` changes.

- **README has extensive stale references to removed scoring system**: `plugins/azurerm/README.md` still documents `score_threshold`, `UnknownRoleSeverity`, `UnknownPrivilegedSeverity`, `PrivilegedRoles`, and scoring tiers as if they exist. The full reference example at line 237 shows `score_threshold = 80`. Multiple sections (lines 140, 162, 183, 194-195, 237, 414, 416, 492, 553) reference removed concepts. This is actively misleading to users.

## Important

- **Missing config parsing tests for `scopes` and `flag_unknown_roles` (CR-0028)**: CR-0028 Test Strategy requires `TestLoadPrivilegeEscalation_Scopes` and `TestLoadPrivilegeEscalation_FlagUnknownRoles` in `pkg/config/loader_test.go`. Neither exists. The `pattern_based_detection.hcl` testdata also omits these fields.

- **Missing CR-0027 integration test for `NotDataActions` subtraction**: CR-0027 AC-2 requires testing that `NotDataActions` removes data actions before matching. No privilege_test.go test exercises this path with non-empty `not_data_actions`. The only `not_data_actions` references are empty lists in tests (lines 631, 746). Needed: test with `dataActions: ["Microsoft.Storage/.../blobs/*"]`, `notDataActions: ["Microsoft.Storage/.../blobs/read"]`, and `data_actions = ["*/read"]` — should NOT trigger.

- **Missing CR-0027 tests for empty effective data actions (AC-4)** and **write-only not matching read patterns (AC-3)**: These are listed in the CR-0027 Test Strategy as required unit tests but are absent.

- **Action registry thresholds too low for AC-15**: The `TestActionRegistry_EmbeddedData` test checks for `>= 3000` control-plane actions, `>= 500` data-plane actions, and `>= 100` providers. CR-0028 AC-15 specifies at least 15,000 control-plane actions, 3,000 data-plane actions, and 200 providers. The lower thresholds are because the registry is generated from the role database fallback rather than from Microsoft Docs. If the goal is to match AC-15 thresholds, the registry should be regenerated from Microsoft Docs, or the acceptance criteria should be revised.

- **`PrivilegedRoles` is dead code in `plugin.go`**: `PrivilegedRoles` field and its default values remain in `PluginConfig` and `DefaultConfig()` but are never used by the privilege analyzer (which now uses `flag_unknown_roles`). `plugin_test.go` still tests the default `PrivilegedRoles` count. This dead code should be removed or the field repurposed.

- **CI verification workflow not run**: The original prompt required "Run the verification workflow to verify e2e tests, using gh cli." The `gh` CLI cannot resolve the remote because the git remote uses a custom SSH alias (`git@personal:`), preventing `gh workflow run` and `gh run list` from working. E2e tests against real Azure infrastructure have not been verified on this branch.

## Minor

- **Full-reference example has stale comment**: `docs/examples/full-reference/.tfclassify.hcl` line ~107 says "When 'actions' is set, it overrides score_threshold-based detection" — but `score_threshold` no longer exists at all. The comment should simply describe what `actions` does.

- **`role-assignment-privileged` e2e scenario not updated**: CR-0028 Affected Components says to "Update config to use `actions` patterns instead of relying on default scoring." However, this scenario uses only Layer 1 core rules (no `azurerm {}` block), so it was never using scoring. The scenario works correctly as-is, but it could benefit from adding pattern-based detection to exercise the plugin in this scenario.

- **No per-classification data_actions test**: CR-0027 AC-10 requires testing that different classifications match different data-plane patterns (e.g., critical matches reads, standard matches writes). No test explicitly validates this cross-classification behavior with data-plane patterns.
