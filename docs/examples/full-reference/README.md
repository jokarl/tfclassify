# Full Reference

A comprehensive configuration using every available field: multiple classification levels, rule descriptions, `not_resource` patterns, plugin configuration, and all `defaults` options. This example serves as an annotated reference for all `.tfclassify.hcl` capabilities.

## Configuration

See [`.tfclassify.hcl`](.tfclassify.hcl) for the fully annotated configuration file. Key features demonstrated:

| Feature | Where |
|---------|-------|
| Plugin declaration (enabled + disabled) | `plugin "azurerm"`, `plugin "aws"` |
| Classification-scoped plugin config | `azurerm { privilege_escalation { ... } }` inside classification blocks |
| Multiple rules per classification | `classification "critical"` has 3 rules |
| Rule descriptions | `description = "..."` on each rule block |
| Action filtering | `actions = ["delete"]` on critical rules |
| `not_resource` exclusion | `classification "standard"` uses `not_resource` |
| Glob precision (`*_key_vault` vs `*_key_vault_*`) | Rules 2-3 in critical/high |
| Five-level precedence with exit codes | `precedence = ["critical", "high", "standard", "low", "auto"]` |
| `plugin_timeout` default | `defaults { plugin_timeout = "30s" }` |

## Terraform Plan

The plan includes seven resources across all five classification levels:

| Resource | Type | Action | Expected Classification | Why |
|----------|------|--------|------------------------|-----|
| `azurerm_role_assignment.admin` | `azurerm_role_assignment` | delete | critical | Matches `*_role_*` with `actions = ["delete"]` |
| `azurerm_subscription.production` | `azurerm_subscription` | update | critical | Matches `*_subscription*` (any action) |
| `azurerm_network_security_rule.allow_ssh` | `azurerm_network_security_rule` | create | high | Matches `*_security_rule` |
| `azurerm_key_vault_secret.db_password` | `azurerm_key_vault_secret` | create | high | Matches `*_key_vault_*` |
| `azurerm_monitor_diagnostic_setting.logs` | `azurerm_monitor_diagnostic_setting` | create | low | Matches `*_diagnostic_*` |
| `azurerm_virtual_network.main` | `azurerm_virtual_network` | create | standard | Matches `not_resource` rule (not monitoring/logging) |
| `azurerm_resource_group.production` | `azurerm_resource_group` | no-op | auto | Matches `resource = ["*"]` with `actions = ["no-op"]` |

## Running

```bash
tfclassify \
  -p docs/examples/full-reference/plan.json \
  -c docs/examples/full-reference/.tfclassify.hcl \
  -v
```

## Expected Output

```
Classification: critical
Exit code: 4
Resources: 7

[critical] (2 resources)
  Requires security team approval — blocks automated deployment
  - azurerm_role_assignment.admin (azurerm_role_assignment) [delete]
    Rule: Deleting IAM or role resources requires security review (resource: *_role_*, ...)
  - azurerm_subscription.production (azurerm_subscription) [update]
    Rule: Subscription-level changes affect the entire tenant (resource: *_subscription*, ...)

[high] (2 resources)
  Requires team lead approval before merge
  - azurerm_network_security_rule.allow_ssh (azurerm_network_security_rule) [create]
    Rule: Network security changes affect access controls (resource: *_security_rule, ...)
  - azurerm_key_vault_secret.db_password (azurerm_key_vault_secret) [create]
    Rule: Key vault secret/key changes need review (resource: *_key_vault_*)

[standard] (1 resources)
  Standard change process — peer review required
  - azurerm_virtual_network.main (azurerm_virtual_network) [create]
    Rule: All infrastructure changes not covered above (not_resource: *_monitor_*, ...)

[low] (1 resources)
  Observability changes — auto-approved with notification
  - azurerm_monitor_diagnostic_setting.logs (azurerm_monitor_diagnostic_setting) [create]
    Rule: Monitoring and logging changes are low-risk (resource: *_monitor_*, ...)

[auto] (1 resources)
  No approval needed
  - azurerm_resource_group.production (azurerm_resource_group) [no-op]
    Rule: No actual changes detected (resource: *)
```

Exit code **4** corresponds to `critical` in a five-level precedence list (auto=0, low=1, standard=2, high=3, critical=4).

## What This Demonstrates

| Concept | Detail |
|---------|--------|
| Five precedence levels | `critical` → `high` → `standard` → `low` → `auto` with exit codes 4-0 |
| Rule descriptions in output | Custom descriptions appear instead of auto-generated ones |
| Glob precision | `*_key_vault` matches the vault itself; `*_key_vault_*` matches children only |
| `not_resource` | Standard catches everything except monitoring resources |
| `actions` filtering | Same resource type can be critical (delete) vs high (create/update) |
| No-op classification | Resources with no changes classified as auto (exit code 0) |
| Plugin configuration | Classification-scoped plugin config with graduated thresholds |
| Sensitive attributes | `db_password` has `after_sensitive.value = true` (detected by builtin analyzer) |
