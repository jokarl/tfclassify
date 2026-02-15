# Remaining Gaps

## Important

None - all gaps have been resolved.

## Resolved

### Plugin README Documentation (CR-0024)
- Removed de-escalation detection references (previously lines 47, 60)
- Removed old `config {}` syntax inside plugin block
- Added new "Classification-Scoped Plugin Configuration" section with:
  - `azurerm {}` block syntax inside classification blocks
  - `privilege_escalation {}` sub-block with `score_threshold`, `exclude`, `roles` options
  - `network_exposure {}` sub-block with `permissive_sources` option
  - `keyvault_access {}` sub-block documentation
  - Behavior notes for classification-scoped configuration
- Updated Table of Contents to reflect new section
- Updated Full Configuration Example to use new syntax

### Functional Implementation
- CR-0023 `--detailed-exitcode` flag: Fully implemented and tested
- CR-0024 classification-scoped plugin config: Fully implemented and tested
- All unit tests pass across all three modules
- All E2E test scenarios created (role-escalation-threshold, role-exclusion, keyvault-destructive)
- E2E workflow matrix updated
- Proto definition updated with `classification` and `analyzer_config` fields
- SDK `ClassificationAwareAnalyzer` interface implemented
- Privilege analyzer: score_threshold gating, role exclusion, roles filter all implemented
- De-escalation detection removed
- Go version updated to 1.25.7 for vulnerability fix

### Documentation
- Main README updated with `--detailed-exitcode` documentation
- Full reference example (`docs/examples/full-reference/.tfclassify.hcl`) fully updated with new syntax
