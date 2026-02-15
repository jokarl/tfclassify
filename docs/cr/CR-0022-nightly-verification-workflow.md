---
id: "CR-0022"
status: implemented
date: 2026-02-15
requestor: jokarl
stakeholders:
  - jokarl
priority: medium
target-version: next
---

# Nightly Verification Workflow

## Change Summary

Add a GitHub Actions workflow that runs nightly (and on-demand) to verify the latest released tfclassify binaries against real Azure infrastructure. Four use cases exercise the full stack: Terraform plan parsing, plugin gRPC communication, classification output, and exit code correctness. A pre-flight cleanup job ensures the target resource group starts empty, and failures automatically open GitHub issues with diagnostic context.

## Motivation and Background

Unit tests cover classification logic in isolation, but they cannot catch regressions in:

- Terraform plan JSON format changes across provider versions
- Plugin gRPC communication between separately-released CLI and plugin binaries
- Azure provider behavior changes that affect resource attribute shapes
- Exit code calculation against real plan output

The existing CI pipeline (CR-0020) validates build and test on every PR, but uses synthetic test fixtures. A nightly verification against real infrastructure provides confidence that the latest released binaries work end-to-end.

## Change Drivers

* No integration testing against real Azure resources
* CLI and plugin are released independently -- version compatibility needs ongoing validation
* Terraform provider updates can change plan JSON structure without warning
* Exit code correctness is critical for CI/CD consumers of tfclassify

## Proposed Change

### Use Cases

Four parallel use cases, each in its own folder under `testdata/e2e/`:

| Use case | Resources | Plugin | Create exit | Destroy exit |
|---|---|---|---|---|
| `role-assignment-privileged` | Managed identity + Owner role assignment | azurerm (privilege) | 1 (standard) | 2 (critical) |
| `nsg-open-inbound` | NSG + permissive inbound rule | azurerm (network) | 1 (standard) | 2 (critical) |
| `route-table` | Route table + Internet route | none | 1 (standard) | 1 (standard) |
| `role-assignment-reader` | Managed identity + Reader role assignment | azurerm (privilege) | 1 (standard) | 2 (critical) |

### Workflow Structure

Two jobs:

1. **cleanup** -- Uses `azure/login` + `az cli` to delete all existing resources and role assignments from the lab resource group, ensuring each run starts fresh even if a previous run failed and left resources behind.

2. **verify** (matrix x4, fail-fast: false) -- Downloads latest released CLI and plugin binaries, runs Terraform init/plan/apply/destroy against each use case, and compares tfclassify exit codes against expected values in `expected.json`.

### Authentication

GitHub OIDC federated credentials with the Azure service principal. The Terraform azurerm provider authenticates via `ARM_*` environment variables. The cleanup job uses `azure/login@v2` for `az cli` access.

### Failure Handling

- Per-use-case `terraform destroy -auto-approve` cleanup runs on `if: always()` when apply was attempted
- Pre-flight cleanup job deletes all resources before the matrix starts
- On failure, `gh issue create` opens an issue with binary versions, expected vs actual exit codes, and a link to the workflow run

## Files Created

```
.github/workflows/verify.yml
testdata/e2e/role-assignment-privileged/{main.tf,.tfclassify.hcl,expected.json}
testdata/e2e/nsg-open-inbound/{main.tf,.tfclassify.hcl,expected.json}
testdata/e2e/route-table/{main.tf,.tfclassify.hcl,expected.json}
testdata/e2e/role-assignment-reader/{main.tf,.tfclassify.hcl,expected.json}
```

## Requirements

### Functional Requirements

1. The workflow **MUST** run on a nightly schedule (cron) and support manual `workflow_dispatch`
2. The workflow **MUST** download the latest released CLI and plugin binaries (not build from source)
3. Each use case **MUST** run Terraform plan, classify with tfclassify, and compare exit codes to expected values
4. The workflow **MUST** clean up Azure resources after each use case, including on failure
5. The workflow **MUST** delete all pre-existing resources from the lab resource group before tests start
6. The workflow **MUST** open a GitHub issue on failure with diagnostic information

### Non-Functional Requirements

1. Matrix jobs **MUST** use `fail-fast: false` so one failure does not cancel others
2. The workflow **MUST** use OIDC authentication (no stored secrets for Azure credentials)
3. Plan JSON output **MUST** be logged in collapsible groups for debugging

## Acceptance Criteria

### AC-1: Nightly schedule triggers verification

```gherkin
Given the verify workflow is configured with cron schedule '0 2 * * *'
When the scheduled time arrives
Then all four matrix jobs run against real Azure infrastructure
  And each job compares tfclassify exit codes against expected.json
```

### AC-2: Clean start on every run

```gherkin
Given previous resources may exist in the lab resource group
When the workflow starts
Then the cleanup job deletes all role assignments and resources
  And the verify jobs begin with an empty resource group
```

### AC-3: Resource cleanup on failure

```gherkin
Given a use case where terraform apply succeeded
When a subsequent step fails
Then terraform destroy runs to clean up created resources
```

### AC-4: Issue creation on failure

```gherkin
Given a use case where the actual exit code differs from expected
When the job fails
Then a GitHub issue is created with label "verification"
  And the issue body contains binary versions and exit code comparison
  And the issue body contains a link to the workflow run
```

### AC-5: Exit code verification

```gherkin
Given use case "role-assignment-privileged" with Owner role assignment
When tfclassify classifies the create plan
Then the exit code is 1 (standard)
When tfclassify classifies the destroy plan
Then the exit code is 2 (critical)
```

## Scope Boundaries

### In Scope

* Nightly verification workflow with cleanup and matrix jobs
* Four e2e use cases covering plugin and core-only classification
* Automatic issue creation on failure
* Pre-flight resource cleanup via az cli

### Out of Scope

* Slack/Teams notifications on failure
* Automatic retry of failed runs
* Performance benchmarking
* Additional cloud providers (AWS, GCP)

## Impact Assessment

### User Impact

None -- this is a CI-only workflow with no changes to the CLI or library.

### Technical Impact

Adds a new workflow and test fixtures. No changes to existing code, tests, or build artifacts.

## Dependencies

* Azure OIDC federated credential configured for the GitHub repository
* Lab resource group `rg-lab-johan.karlsson` must exist in the Azure subscription
* At least one CLI release and one plugin release must exist

## Related Items

* CR-0020: GitHub Actions CI/CD Workflows (established the CI pipeline this builds upon)
* CR-0006: gRPC Protocol and Plugin Host (the plugin communication being verified)
