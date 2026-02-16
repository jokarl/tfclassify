# Remaining Gaps

## Important

(No remaining gaps.)

## Resolved

### CI Workflow E2E Matrix Updated
- Added `data-plane-detection` and `control-plane-patterns` to the e2e matrix in `.github/workflows/ci.yml` so both new test cases run in CI.


### CR-0027: Data-Plane Action Detection — Fully Implemented
- `DataActions` field added to `PrivilegeEscalationConfig` (`pkg/config/config.go`) and `PrivilegeEscalationAnalyzerConfig` (`plugins/azurerm/privilege.go`)
- `data_actions` parsing added to `parsePrivilegeEscalationConfig` (`pkg/config/loader.go`)
- `matchDataPlanePatterns` implemented in `plugins/azurerm/privilege.go` using `computeEffectiveActions` and `actionMatchesPattern`
- Independent data-plane and control-plane triggering with `trigger` metadata
- All 10 unit tests from the CR test strategy pass
- Config parsing test (`TestLoad_PatternBasedDetection`) passes
- E2E test fixture created (`testdata/e2e/data-plane-detection/`)

### CR-0028: Pattern-Based Control-Plane Detection — Implemented (Proportional to Skeleton CR)
- `Actions` field added to config structs
- `actions` parsing added to loader
- `matchControlPlanePatterns` implemented, overrides `score_threshold` when configured
- 6 unit tests covering pattern matching, NotActions subtraction, and override behavior
- E2E test fixture created (`testdata/e2e/control-plane-patterns/`)

### Documentation Updated
- `plugins/azurerm/README.md`: Added "Data-Plane Detection" and "Pattern-Based Control-Plane Detection" sections
- `docs/examples/full-reference/.tfclassify.hcl`: Updated with `actions` and `data_actions` examples

### All CI Checks Pass Locally
- `go build ./...` — success
- `go test -race ./...` — all tests pass across all 3 modules
- `go vet ./...` — no issues
- `govulncheck ./...` — no vulnerabilities
