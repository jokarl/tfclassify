---
name: cr-0026-explain-command
description: Add tfclassify explain command that traces classification decisions for a specific resource through the full pipeline.
id: "CR-0026"
status: "proposed"
date: 2026-02-15
requestor: jokarl
stakeholders:
  - jokarl
priority: "low"
target-version: backlog
---

# `tfclassify explain` Command

> **Note:** This CR is a skeleton that must be further refined before implementation.

## Change Summary

Add a `tfclassify explain` subcommand that shows why a specific resource was classified the way it was. Given a Terraform plan and a resource address, the command traces through the full classification pipeline: precedence order evaluation, which core rules matched or were skipped, which builtin analyzers ran and their results, which plugin analyzers ran, what scores and metadata they produced, and how the final classification was determined via decision aggregation.

## Motivation and Background

When tfclassify classifies a resource as "critical", operators need to understand *why* — especially when the classification seems unexpected. Today, the only way to debug classification is to read the config, mentally simulate the precedence logic, and guess which analyzer produced the winning decision. An explain command provides a structured trace of the entire decision pipeline, making classification behavior transparent and debuggable.

## Expected CLI Interface

```bash
# Explain a specific resource
tfclassify explain --plan plan.json --resource "azurerm_role_assignment.example"

# Explain all resources
tfclassify explain --plan plan.json

# JSON output for programmatic consumption
tfclassify explain --plan plan.json --resource "azurerm_role_assignment.example" --output json
```

## Expected Output (human-readable)

```
Resource: azurerm_role_assignment.example
Actions:  [create]
Final:    critical (from plugin: azurerm/privilege-escalation)

  Evaluation trace:
  1. [critical] rule "*_role_*" actions=["delete"]  → SKIP (action mismatch)
  2. [critical] azurerm/privilege-escalation         → MATCH (severity: 95, reason: "privileged role \"Owner\" assigned")
     Score: base=95 (tier 1: unrestricted wildcard), scope=subscription (1.0x), weighted=95
     Threshold: 80, passed
     Role: Owner (source: builtin)
  3. [standard] rule "*"                             → MATCH (catch-all)
  4. [standard] azurerm/privilege-escalation          → MATCH (severity: 95, reason: "privileged role \"Owner\" assigned")

  Winner: critical (precedence rank 0 beats standard rank 2)
```

## Scope

### In Scope

* `explain` subcommand with `--plan`, `--resource`, and `--output` flags
* Structured trace of precedence evaluation, rule matching, and analyzer results
* Human-readable and JSON output formats
* Severity score breakdown (base score, scope multiplier, weighted score, threshold comparison)

### Out of Scope (to be refined)

* Interactive/TUI mode
* Diff-based explain (comparing two plans)
* Integration with `--output github` format
* Explain for hypothetical resources (without a plan)

## Related Items

* CR-0024: Classification-Scoped Plugin Analyzer Rules — the classification-scoped model this command needs to trace through
* CR-0017: Privilege Analyzer Rewrite — the scoring system whose factors are displayed in the trace
* `pkg/classify/classifier.go` — classification logic that needs to expose its decision trace
