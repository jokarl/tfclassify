# Mixed Changes

A realistic scenario with six resource changes spanning creates, updates, deletes, and a replacement. Demonstrates multiple classification levels, several rules per level, and how the overall classification is determined by the highest-precedence resource.

## Configuration

```hcl
classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*", "*_iam_*"]
    actions  = ["delete"]
  }

  rule {
    resource = ["*_key_vault"]
    actions  = ["delete"]
  }
}

classification "review" {
  description = "Requires team lead review"

  rule {
    resource = ["*_security_rule", "*_firewall_*"]
  }

  rule {
    resource = ["*_key_vault_*"]
  }
}

classification "standard" {
  description = "Standard change process"

  rule {
    resource = ["*"]
    # Catches everything not matched by critical or review above.
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

Key design choices:
- `*_key_vault` (no trailing wildcard) matches the vault resource itself; `*_key_vault_*` matches child resources like secrets and keys
- Deleting a vault is `critical`; modifying a secret is `review`
- Security rules and firewall rules always require `review` regardless of action
- Everything else is `standard` using `resource = ["*"]` as a catch-all (precedence handles the hierarchy)

## Terraform Plan

Six resources representing a typical infrastructure change:

| Resource | Action | Expected Classification |
|----------|--------|------------------------|
| `azurerm_key_vault.secrets` | delete | critical |
| `azurerm_network_security_rule.allow_https` | create | review |
| `azurerm_key_vault_secret.db_password` | update | review |
| `azurerm_subnet.backend` | replace (delete + create) | standard |
| `azurerm_storage_account.logs` | update | standard |
| `azurerm_resource_group.monitoring` | create | standard |

```json
{
  "format_version": "1.2",
  "terraform_version": "1.9.0",
  "resource_changes": [
    {
      "address": "azurerm_key_vault.secrets",
      "mode": "managed",
      "type": "azurerm_key_vault",
      "name": "secrets",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["delete"],
        "before": {
          "name": "kv-secrets-prod",
          "location": "westeurope",
          "resource_group_name": "rg-production",
          "sku_name": "standard",
          "purge_protection_enabled": true
        },
        "after": null,
        "after_unknown": {},
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_network_security_rule.allow_https",
      "mode": "managed",
      "type": "azurerm_network_security_rule",
      "name": "allow_https",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["create"],
        "before": null,
        "after": {
          "name": "allow-https-inbound",
          "priority": 100,
          "direction": "Inbound",
          "access": "Allow",
          "protocol": "Tcp",
          "source_port_range": "*",
          "destination_port_range": "443",
          "source_address_prefix": "*",
          "destination_address_prefix": "10.0.1.0/24"
        },
        "after_unknown": { "id": true },
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_subnet.backend",
      "mode": "managed",
      "type": "azurerm_subnet",
      "name": "backend",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["delete", "create"],
        "before": {
          "name": "snet-backend",
          "address_prefixes": ["10.0.2.0/24"],
          "virtual_network_name": "vnet-production"
        },
        "after": {
          "name": "snet-backend",
          "address_prefixes": ["10.0.3.0/24"],
          "virtual_network_name": "vnet-production"
        },
        "after_unknown": { "id": true },
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_storage_account.logs",
      "mode": "managed",
      "type": "azurerm_storage_account",
      "name": "logs",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["update"],
        "before": {
          "name": "stlogs001",
          "account_tier": "Standard",
          "account_replication_type": "LRS",
          "min_tls_version": "TLS1_2"
        },
        "after": {
          "name": "stlogs001",
          "account_tier": "Standard",
          "account_replication_type": "GRS",
          "min_tls_version": "TLS1_2"
        },
        "after_unknown": {},
        "before_sensitive": false,
        "after_sensitive": false
      }
    },
    {
      "address": "azurerm_key_vault_secret.db_password",
      "mode": "managed",
      "type": "azurerm_key_vault_secret",
      "name": "db_password",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["update"],
        "before": {
          "name": "db-password",
          "value": "old-password-value",
          "key_vault_id": "/subscriptions/.../Microsoft.KeyVault/vaults/kv-secrets-prod"
        },
        "after": {
          "name": "db-password",
          "value": "new-password-value",
          "key_vault_id": "/subscriptions/.../Microsoft.KeyVault/vaults/kv-secrets-prod"
        },
        "after_unknown": {},
        "before_sensitive": { "value": true },
        "after_sensitive": { "value": true }
      }
    },
    {
      "address": "azurerm_resource_group.monitoring",
      "mode": "managed",
      "type": "azurerm_resource_group",
      "name": "monitoring",
      "provider_name": "registry.terraform.io/hashicorp/azurerm",
      "change": {
        "actions": ["create"],
        "before": null,
        "after": {
          "name": "rg-monitoring",
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
  -p docs/examples/mixed-changes/plan.json \
  -c docs/examples/mixed-changes/.tfclassify.hcl \
  -v
```

## Expected Output

```
Classification: critical
Exit code: 3
Resources: 6

[critical] (1 resources)
  - azurerm_key_vault.secrets (azurerm_key_vault) [delete]
    Rule: critical rule 2 (resource: *_key_vault)

[review] (2 resources)
  - azurerm_network_security_rule.allow_https (azurerm_network_security_rule) [create]
    Rule: review rule 1 (resource: *_security_rule, ...)
  - azurerm_key_vault_secret.db_password (azurerm_key_vault_secret) [update]
    Rule: review rule 2 (resource: *_key_vault_*)

[standard] (3 resources)
  - azurerm_subnet.backend (azurerm_subnet) [delete create]
    Rule: standard rule 1 (resource: *)
  - azurerm_storage_account.logs (azurerm_storage_account) [update]
    Rule: standard rule 1 (resource: *)
  - azurerm_resource_group.monitoring (azurerm_resource_group) [create]
    Rule: standard rule 1 (resource: *)
```

JSON output (`-o json`):

```json
{
  "overall": "critical",
  "exit_code": 3,
  "no_changes": false,
  "resources": [
    {
      "address": "azurerm_key_vault.secrets",
      "type": "azurerm_key_vault",
      "actions": ["delete"],
      "classification": "critical",
      "matched_rule": "critical rule 2 (resource: *_key_vault)"
    },
    {
      "address": "azurerm_network_security_rule.allow_https",
      "type": "azurerm_network_security_rule",
      "actions": ["create"],
      "classification": "review",
      "matched_rule": "review rule 1 (resource: *_security_rule, ...)"
    },
    {
      "address": "azurerm_subnet.backend",
      "type": "azurerm_subnet",
      "actions": ["delete", "create"],
      "classification": "standard",
      "matched_rule": "standard rule 1 (resource: *)"
    },
    {
      "address": "azurerm_storage_account.logs",
      "type": "azurerm_storage_account",
      "actions": ["update"],
      "classification": "standard",
      "matched_rule": "standard rule 1 (resource: *)"
    },
    {
      "address": "azurerm_key_vault_secret.db_password",
      "type": "azurerm_key_vault_secret",
      "actions": ["update"],
      "classification": "review",
      "matched_rule": "review rule 2 (resource: *_key_vault_*)"
    },
    {
      "address": "azurerm_resource_group.monitoring",
      "type": "azurerm_resource_group",
      "actions": ["create"],
      "classification": "standard",
      "matched_rule": "standard rule 1 (resource: *)"
    }
  ]
}
```

## What This Demonstrates

| Concept | Detail |
|---------|--------|
| Multiple rules per classification | `critical` has two rules: role/IAM deletes and key vault deletes |
| Glob precision | `*_key_vault` matches the vault itself; `*_key_vault_*` matches secrets, keys, etc. |
| Resource replacement | `azurerm_subnet.backend` has actions `["delete", "create"]`, classified as `standard` by core rules |
| Sensitive attributes | `azurerm_key_vault_secret.db_password` has `before_sensitive`/`after_sensitive` marking the `value` field (relevant for plugin-based analysis) |
| Overall = highest precedence | One `critical` resource among six makes the overall result `critical` with exit code 3 |
| Exit code mapping | 4 levels: `auto`=0, `standard`=1, `review`=2, `critical`=3 |
