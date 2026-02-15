# Remaining Gaps

All previously identified gaps have been addressed.

## Resolved (CR-0023/CR-0024 Implementation)

### Critical (Resolved)

- ~~**Missing E2E test scenarios from CR-0024 Test Strategy**~~ - Added:
  - `testdata/e2e/role-escalation-threshold/` - Tests graduated thresholds across classifications
  - `testdata/e2e/role-exclusion/` - Tests `exclude = ["AcrPush"]` filtering in privilege analyzer
  - `testdata/e2e/keyvault-destructive/` - Tests key vault destructive permission detection

- ~~**E2E workflow matrix not updated**~~ - Updated `.github/workflows/verify.yml` to include all three new test scenarios

### Important (Resolved)

- ~~**Unit tests for classification-aware privilege analyzer features**~~ - Added to `plugins/azurerm/privilege_test.go`:
  - `TestPrivilege_AnalyzeWithClassification_ScoreThreshold` - Tests score threshold gating
  - `TestPrivilege_AnalyzeWithClassification_RoleExclusion` - Tests role exclusion
  - `TestPrivilege_AnalyzeWithClassification_RolesFilter` - Tests roles filter
  - `TestPrivilege_AnalyzeWithClassification_EmitsClassification` - Tests classification emission
  - `TestPrivilege_AnalyzeWithClassification_EmptyClassification` - Tests backward compatibility
  - `TestPrivilege_AnalyzeWithClassification_CombinedFilters` - Tests combined score_threshold + exclude + roles
  - `TestPrivilege_AnalyzeWithClassification_InvalidJSON` - Tests error handling

## Additional Changes

- Updated Go version from 1.25.6 to 1.25.7 to fix GO-2026-4337 vulnerability (crypto/tls session resumption)
