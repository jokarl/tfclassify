---
status: approved
date: 2026-02-13
decision-makers: Johan
---

# Provider-Agnostic Core with Deep Inspection Plugins

## Context and Problem Statement

tfclassify must classify Terraform plan changes, but classification requirements vary by organization - there are no universal classification levels or rules. Additionally, resources from different providers (Azure, AWS, GCP) have provider-specific semantics that require domain knowledge to analyze deeply (e.g., understanding that an Azure role change from "Reader" to "Owner" is a privilege escalation).

How should classification responsibilities be split between the core engine and plugins, and what role does a bundled plugin play?

## Decision Drivers

* Classification levels and rules are organization-specific - the tool must not prescribe them
* Basic pattern-based classification (resource name globs + action types) should work without plugins
* Deep inspection of provider-specific resource semantics requires extensibility
* The tool must be useful out of the box with minimal configuration
* Following TFLint's model: a bundled plugin is required for baseline functionality

## Considered Options

* Provider-agnostic core with pattern matching + bundled cross-provider plugin + deep inspection plugins
* All classification in plugins (core is only a framework)
* Monolithic analyzer with provider-specific modules

## Decision Outcome

Chosen option: "Provider-agnostic core with pattern matching + bundled cross-provider plugin + deep inspection plugins", because it provides a clear separation of concerns: the core handles config-driven classification, the bundled plugin adds cross-provider analysis beyond pattern matching, and deep plugins add provider-specific intelligence.

### Three-Layer Classification Model

```
┌─────────────────────────────────────────────────────────────┐
│ Layer 3: Deep Inspection Plugins (provider-specific)        │
│ e.g., tfclassify-plugin-azurerm, tfclassify-plugin-aws     │
│ → Understands resource field semantics                      │
│ → Detects privilege escalation by inspecting role names     │
│ → Analyzes CIDR ranges in security rules                    │
│ → Emits decisions with severity and reasoning               │
├─────────────────────────────────────────────────────────────┤
│ Layer 2: Builtin Analyzers (cross-provider, in-process)     │
│ → Detects resource deletions and replacements               │
│ → Flags changes to sensitive-marked attributes              │
│ → Analyzes action patterns beyond simple glob matching      │
│ → Enabled by default, runs in-process (no gRPC overhead)    │
├─────────────────────────────────────────────────────────────┤
│ Layer 1: Core Engine (config-driven pattern matching)       │
│ → Reads org-defined classification levels from config       │
│ → Matches resource types via globs (e.g., *_role_*)         │
│ → Matches action types (create, update, delete)             │
│ → Applies precedence rules                                  │
│ → Aggregates plugin decisions                               │
└─────────────────────────────────────────────────────────────┘
```

### Layer 1: Core Engine

The core engine reads the organization's `.tfclassify.hcl` config and applies pattern-based classification. Classification levels, rules, and precedence are fully org-defined. The tool ships with example configs but prescribes nothing.

Config-driven rules match on:
- Resource type patterns (glob): `*_role_assignment`, `*_security_rule`
- Action types: `create`, `update`, `delete`, `replace`
- Negation patterns: `notResource` for exclusion-based rules

### Layer 2: Builtin Analyzers (cross-provider)

> **Note:** This layer was originally designed as a bundled "terraform" plugin running as a separate process (TFLint pattern). CR-0018 inlined these analyzers into the core classification engine as in-process `BuiltinAnalyzer` implementations in `pkg/classify/`, eliminating process spawn overhead while preserving the same functionality. The `plugin "terraform"` config block is accepted for backward compatibility but has no effect.

The following cross-provider analyzers are enabled by default:

- **Deletion analyzer**: flags resource deletions with context (is it a standalone delete or part of a replacement?)
- **Sensitive attribute analyzer**: detects changes to attributes marked as sensitive in the Terraform state
- **Replace analyzer**: identifies destroy-and-recreate changes that may cause downtime

These analyzers work across all providers because they operate on Terraform-level concepts (actions, sensitive markers) rather than provider-specific field semantics.

### Layer 3: Deep Inspection Plugins

Provider-specific plugins that understand resource field semantics:

| Plugin | Example Analysis |
|--------|-----------------|
| `tfclassify-plugin-azurerm` | Inspects `role_definition_name` on `azurerm_role_assignment` to detect privilege escalation (e.g., Reader to Owner) |
| `tfclassify-plugin-aws` | Inspects IAM policy documents for overly permissive statements (`*` actions, `*` resources) |

Deep plugins query the Runner for resource changes matching their patterns, inspect before/after attribute values, and emit decisions with severity scores and reasoning.

### Decision Aggregation

The core engine aggregates decisions from all sources:

1. Core pattern-matching rules produce base classifications
2. Bundled plugin decisions can override or augment base classifications
3. Deep plugin decisions can override or augment further
4. Org-defined precedence determines the final classification per resource
5. The highest-precedence classification across all resources determines the overall result

### Organization-Defined Configuration

Classification levels, rules, and precedence are fully configurable. The tool ships with no defaults - only examples. An organization might define:

```hcl
# Example - NOT a recommendation
classification "critical" {
  description = "Requires security team approval"
  rules {
    resource = ["*_role_*", "*_iam_*"]
    actions  = ["delete", "update"]
  }
}

classification "review" {
  description = "Requires team lead review"
  rules {
    resource = ["*_security_rule", "*_firewall_*"]
  }
}

precedence = ["critical", "review", "standard", "auto"]
```

### Consequences

* Good, because organizations have full control over classification semantics
* Good, because the tool is useful with just the core engine and config (no plugins required for basic pattern matching)
* Good, because the bundled plugin adds value beyond pattern matching without requiring provider knowledge
* Good, because deep plugins can be developed and released independently per provider
* Good, because the same deep plugin architecture works for any provider
* Bad, because three layers add conceptual complexity that must be well-documented
* Bad, because decision aggregation across layers needs clear precedence rules

### Confirmation

* Core engine classifies changes using only config rules without any plugins loaded
* Bundled terraform plugin detects deletions and sensitive changes across providers
* A deep inspection plugin can override a core classification with a more informed decision
* Config accepts arbitrary classification level names (not hardcoded)
* Example configs are provided but no defaults are baked in

## Pros and Cons of the Options

### Provider-agnostic core + bundled plugin + deep inspection plugins

* Good, because clear separation: config rules, generic analysis, deep analysis
* Good, because core is useful standalone for simple pattern-based classification
* Good, because deep plugins are truly optional - only needed for provider-specific intelligence
* Good, because follows TFLint's proven layered model
* Neutral, because three layers require clear documentation of precedence and override behavior
* Bad, because the boundary between "pattern matching" and "cross-provider analysis" must be well-defined

### All classification in plugins (core is only a framework)

* Good, because simplest core - it only loads plugins and aggregates
* Bad, because basic pattern matching requires a plugin, making the tool useless without one
* Bad, because config-driven rules would live in a plugin, mixing concerns
* Bad, because higher barrier to initial adoption

### Monolithic analyzer with provider-specific modules

* Good, because single codebase, no IPC overhead
* Bad, because every provider addition requires a release of the entire tool
* Bad, because scaling to many providers bloats the binary and dependency tree
* Bad, because no third-party extensibility without forking

## More Information

Related: [ADR-0002](ADR-0002-grpc-plugin-architecture.md) defines the plugin communication protocol.

Related: [ADR-0004](ADR-0004-hcl-configuration-format.md) defines the configuration format where organizations specify their classification rules.

TFLint's bundled plugin approach: https://github.com/terraform-linters/tflint/blob/master/docs/user-guide/plugins.md
