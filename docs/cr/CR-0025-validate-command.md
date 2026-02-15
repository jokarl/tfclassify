---
name: cr-0025-validate-command
description: Add tfclassify validate command that checks .tfclassify.hcl for correctness without requiring a Terraform plan.
id: "CR-0025"
status: "proposed"
date: 2026-02-15
requestor: jokarl
stakeholders:
  - jokarl
priority: "low"
target-version: backlog
---

# `tfclassify validate` Command

> **Note:** This CR is a skeleton that must be further refined before implementation.

## Change Summary

Add a `tfclassify validate` subcommand that checks `.tfclassify.hcl` for correctness without requiring a Terraform plan. The command verifies HCL syntax, validates all cross-references (precedence list references existing classifications, `defaults.unclassified` and `defaults.no_changes` reference existing classifications, plugin blocks inside classifications reference enabled plugins), and warns about unreachable rules (rules shadowed by earlier catch-all patterns in higher-precedence classifications).

## Motivation and Background

Currently, configuration errors are only discovered at classification time when a plan is provided. Syntax errors, dangling references, and unreachable rules go undetected until the tool is run against a real plan. A dedicated validation command enables:

- Pre-commit hooks that catch config errors before they reach CI
- IDE integration for config file linting
- Onboarding validation for new users writing their first config

## Proposed Validations

| Check | Severity | Description |
|-------|----------|-------------|
| HCL syntax | error | Config file parses without syntax errors |
| Precedence references | error | Every entry in `precedence` matches a `classification` block name |
| Default references | error | `defaults.unclassified` and `defaults.no_changes` reference classification names in precedence |
| Plugin references | error | Plugin-named blocks in classifications reference an enabled `plugin` block |
| Analyzer names | error | Analyzer sub-blocks match known analyzers for the referenced plugin |
| Unreachable rules | warning | Rules in lower-precedence classifications that are fully shadowed by catch-all rules in higher-precedence ones |
| Empty classifications | warning | Classification blocks with no rules and no plugin analyzer blocks |
| Plugin binary exists | warning | Plugin binary found in discovery path (optional, skip if `--syntax-only`) |

## Expected CLI Interface

```bash
# Validate default config file
tfclassify validate

# Validate a specific config file
tfclassify validate --config path/to/.tfclassify.hcl

# Syntax-only check (no plugin binary resolution)
tfclassify validate --syntax-only
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Valid configuration, no warnings |
| 0 | Valid configuration with warnings (warnings printed to stderr) |
| 1 | Invalid configuration (errors printed to stderr) |

## Scope

### In Scope

* `validate` subcommand with `--config` and `--syntax-only` flags
* All validations listed above
* Human-readable error/warning output with file location context

### Out of Scope (to be refined)

* JSON/machine-readable output format
* Auto-fix suggestions
* Plugin schema validation beyond analyzer name matching
* Integration with `tfclassify init` or other subcommands

## Related Items

* CR-0024: Classification-Scoped Plugin Analyzer Rules — introduces plugin blocks inside classifications that need validation
* `pkg/config/validation.go` — existing validation logic to extend
