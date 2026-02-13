# Action Filtering

Demonstrates how the `actions` field in rules narrows classification. The same resource type (`azurerm_role_assignment`) is classified differently depending on whether the action is a `delete` or an `update`.

## Configuration

```hcl
classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*", "*_iam_*"]
    actions  = ["delete"]
  }
}

classification "review" {
  description = "Requires team lead review"

  rule {
    resource = ["*_role_*", "*_iam_*"]
  }
}

classification "standard" {
  description = "Standard change process"

  rule {
    not_resource = ["*_role_*", "*_iam_*"]
  }
}

classification "auto" {
  description = "Automatic approval"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

precedence = ["critical", "review", "standard", "auto"]

defaults {
  unclassified = "standard"
  no_changes   = "auto"
}
```

The key detail: `critical` requires both a role/IAM resource **and** a `delete` action. `review` matches role/IAM resources with **any** action (no `actions` filter). Because the classifier checks rules in precedence order, a role deletion matches `critical` first and stops; a role update skips `critical` (wrong action) and matches `review`.

## Terraform Plan

Three changes: a role assignment deletion, a role assignment update, and a storage account update.

```json
{
  "format_version": "1.2",
  "terraform_version": "1.9.0",
  "resource_changes": [
    {
      "address": "azurerm_role_assignment.legacy",
      "mode": "managed",
      "type": "azurerm_role_assignment",
      "name": "legacy",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["delete"],
        "before": {
          "principal_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
          "role_definition_name": "Contributor",
          "scope": "/subscriptions/00000000-0000-0000-0000-000000000000"
        },
        "after": null,
        "after_unknown": {},
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_role_assignment.reader",
      "mode": "managed",
      "type": "azurerm_role_assignment",
      "name": "reader",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["update"],
        "before": {
          "principal_id": "11111111-2222-3333-4444-555555555555",
          "role_definition_name": "Reader",
          "scope": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-dev"
        },
        "after": {
          "principal_id": "11111111-2222-3333-4444-555555555555",
          "role_definition_name": "Reader",
          "scope": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-staging"
        },
        "after_unknown": {},
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_storage_account.data",
      "mode": "managed",
      "type": "azurerm_storage_account",
      "name": "data",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["update"],
        "before": {
          "name": "stdata001",
          "account_tier": "Standard",
          "account_replication_type": "LRS"
        },
        "after": {
          "name": "stdata001",
          "account_tier": "Standard",
          "account_replication_type": "GRS"
        },
        "after_unknown": {},
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
  -p docs/examples/action-filtering/plan.json \
  -c docs/examples/action-filtering/.tfclassify.hcl \
  --no-plugins -v
```

## Expected Output

```
Classification: critical
Exit code: 3
Resources: 3

[critical] (1 resources)
  - azurerm_role_assignment.legacy (azurerm_role_assignment) [delete]
    Rule: critical rule 1 (resource: *_role_*, ...)

[review] (1 resources)
  - azurerm_role_assignment.reader (azurerm_role_assignment) [update]
    Rule: review rule 1 (resource: *_role_*, ...)

[standard] (1 resources)
  - azurerm_storage_account.data (azurerm_storage_account) [update]
    Rule: standard rule 1 (not_resource: *_role_*, ...)
```

Exit code **3** corresponds to `critical` in a 4-level precedence list.

## What This Demonstrates

| Concept | Detail |
|---------|--------|
| Action filtering | `actions = ["delete"]` restricts `critical` to deletions only |
| Rules without actions | `review` has no `actions` field, so it matches any action on role resources |
| Precedence-ordered evaluation | `critical` is checked first; a role `delete` matches there and stops. A role `update` skips `critical` (action mismatch), then matches `review` |
| Same type, different classification | Both `azurerm_role_assignment` resources get different classifications based on their action |
