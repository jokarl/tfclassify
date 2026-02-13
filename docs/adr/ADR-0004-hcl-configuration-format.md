---
status: approved
date: 2026-02-13
decision-makers: Johan
---

# HCL Configuration Format

## Context and Problem Statement

tfclassify needs a configuration format for defining classification levels, rules, plugin settings, and operational defaults. The configuration is central to the tool's value since classification levels and rules are fully organization-defined. The format must be expressive enough for complex rule definitions while being familiar to the target audience (Terraform practitioners).

What configuration format should tfclassify use?

## Decision Drivers

* Target users are Terraform practitioners who are fluent in HCL
* Consistency with TFLint (which uses `.tflint.hcl`) and the Terraform ecosystem
* Expressiveness for nested rule definitions and plugin configuration blocks
* Tooling support (syntax highlighting, formatting, validation)

## Considered Options

* HCL (`.tfclassify.hcl`)
* JSON (`.tfclassify.json`)
* YAML (`.tfclassify.yaml`)

## Decision Outcome

Chosen option: "HCL", because it is the native configuration language of the Terraform ecosystem, immediately familiar to our target users, and follows the convention established by TFLint.

### Configuration Structure

```hcl
# .tfclassify.hcl

plugin "terraform" {
  enabled = true
}

plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify-plugin-azurerm"
  version = "0.1.0"

  config {
    privileged_roles  = ["Owner", "User Access Administrator"]
    sensitive_scopes  = ["/subscriptions/"]
  }
}

classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*", "*_iam_*"]
    actions  = ["delete"]
  }
}

classification "review" {
  description = "Standard approval required"

  rule {
    resource = ["*_role_*", "*_iam_*", "*_identity*"]
  }
}

precedence = ["critical", "review", "standard", "auto"]

defaults {
  unclassified   = "review"
  no_changes     = "auto"
  plugin_timeout = "30s"
}
```

### Configuration File Discovery

In order of precedence:

1. `--config` CLI flag
2. `.tfclassify.hcl` in current directory
3. `~/.tfclassify.hcl` for user-level defaults

### Consequences

* Good, because Terraform users can read and write config immediately without learning a new format
* Good, because HCL block syntax maps naturally to plugin and classification definitions
* Good, because consistent with TFLint's `.tflint.hcl` convention
* Good, because HCL tooling (formatters, language servers) is available
* Bad, because adds `hashicorp/hcl/v2` as a dependency
* Bad, because HCL is less common outside the Terraform ecosystem, potentially limiting adoption by non-Terraform users

### Confirmation

* Config loading parses `.tfclassify.hcl` successfully with org-defined classification blocks
* Plugin blocks configure and enable/disable plugins correctly
* Invalid config produces clear error messages with file/line references
* Example configs are provided in the repository documentation

## Pros and Cons of the Options

### HCL (`.tfclassify.hcl`)

* Good, because native to Terraform ecosystem - no learning curve for target users
* Good, because block syntax is ideal for nested plugin and rule definitions
* Good, because supports comments (unlike JSON)
* Good, because HCL library handles schema validation with useful error messages
* Neutral, because requires `hashicorp/hcl/v2` dependency (well-maintained)
* Bad, because less familiar to users outside the Terraform ecosystem

### JSON (`.tfclassify.json`)

* Good, because universally understood format with zero learning curve
* Good, because native parsing in Go with no external dependencies
* Bad, because no comments - cannot annotate classification rationale inline
* Bad, because verbose for nested structures (plugin configs, rule definitions)
* Bad, because less idiomatic in the Terraform ecosystem

### YAML (`.tfclassify.yaml`)

* Good, because widely used for configuration in cloud-native tooling
* Good, because supports comments and is relatively concise
* Bad, because not used in the Terraform/HCL ecosystem - odd choice for this tool
* Bad, because YAML parsing quirks (implicit type coercion, indentation sensitivity)
* Bad, because adds an external dependency (`gopkg.in/yaml.v3`) with less ecosystem alignment

## More Information

Related: [ADR-0003](ADR-0003-provider-agnostic-core-with-deep-inspection-plugins.md) defines the classification model that the config drives.

HCL specification: https://github.com/hashicorp/hcl
TFLint configuration: https://github.com/terraform-linters/tflint/blob/master/docs/user-guide/config.md
