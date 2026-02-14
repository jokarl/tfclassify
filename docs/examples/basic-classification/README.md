# Basic Classification

Demonstrates resource type pattern matching using glob rules. Resources matching `*_role_*` or `*_iam_*` are classified as **critical**, while all other resources fall into **standard** using a simple `resource = ["*"]` catch-all pattern.

The catch-all pattern works because the classifier evaluates rules in precedence order: higher-precedence classifications (critical) are checked first, so the **standard** rule only matches resources that didn't match anything above.

## Configuration

```hcl
classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*", "*_iam_*"]
  }
}

classification "standard" {
  description = "Standard change process"

  rule {
    resource = ["*"]
    # Catches everything not matched by critical above.
  }
}

classification "auto" {
  description = "Automatic approval"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

precedence = ["critical", "standard", "auto"]

defaults {
  unclassified = "standard"
  no_changes   = "auto"
}
```

## Terraform Plan

The plan creates three resources: a role assignment, a virtual network, and a resource group.

```json
{
  "format_version": "1.2",
  "terraform_version": "1.9.0",
  "resource_changes": [
    {
      "address": "azurerm_role_assignment.admin",
      "mode": "managed",
      "type": "azurerm_role_assignment",
      "name": "admin",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["create"],
        "before": null,
        "after": {
          "principal_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
          "role_definition_name": "Owner",
          "scope": "/subscriptions/00000000-0000-0000-0000-000000000000"
        },
        "after_unknown": { "id": true },
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_virtual_network.main",
      "mode": "managed",
      "type": "azurerm_virtual_network",
      "name": "main",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["create"],
        "before": null,
        "after": {
          "name": "vnet-production",
          "address_space": ["10.0.0.0/16"],
          "location": "westeurope",
          "resource_group_name": "rg-production"
        },
        "after_unknown": { "id": true },
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_resource_group.production",
      "mode": "managed",
      "type": "azurerm_resource_group",
      "name": "production",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["create"],
        "before": null,
        "after": {
          "name": "rg-production",
          "location": "westeurope"
        },
        "after_unknown": { "id": true },
        "before_sensitive": false,
        "after_sensitive": false
      }
    }
  ]
}
```

## Running

```bash
tfclassify \
  -p docs/examples/basic-classification/plan.json \
  -c docs/examples/basic-classification/.tfclassify.hcl \
  -v
```

## Expected Output

```
Classification: critical
Exit code: 2
Resources: 3

[critical] (1 resources)
  - azurerm_role_assignment.admin (azurerm_role_assignment) [create]
    Rule: critical rule 1 (resource: *_role_*, ...)

[standard] (2 resources)
  - azurerm_virtual_network.main (azurerm_virtual_network) [create]
    Rule: standard rule 1 (resource: *)
  - azurerm_resource_group.production (azurerm_resource_group) [create]
    Rule: standard rule 1 (resource: *)
```

Exit code **2** corresponds to `critical` (highest precedence = highest exit code).

## What This Demonstrates

| Concept | Detail |
|---------|--------|
| `resource` glob matching | `*_role_*` matches `azurerm_role_assignment` |
| `resource = ["*"]` catch-all | Matches everything not already classified by higher-precedence rules |
| Precedence-ordered evaluation | Rules are checked in precedence order; first match wins |
| Precedence | `critical` (index 0) gets exit code 2, `standard` (index 1) gets 1, `auto` (index 2) gets 0 |
| Overall classification | The highest-precedence classification across all resources wins |
