# CIS Azure Foundations Benchmark — e2e scenario
#
# Creates resources that violate three real CIS controls to demonstrate
# how tfclassify classifications can be named after benchmark sections.
#
# CIS controls violated (Azure Policy mapping: https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0):
#   6.2  — SSH open to the Internet     (network_exposure analyzer)
#          https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0#ensure-that-ssh-access-from-the-internet-is-evaluated-and-restricted
#   1.23 — Privileged role assignment    (privilege_escalation analyzer)
#          https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0#ensure-that-no-custom-subscription-administrator-roles-exist
#   8.5  — Destructive key vault access  (keyvault_access analyzer)
#          https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0#ensure-the-key-vault-is-recoverable

terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "azurerm" {
  features {
    key_vault {
      purge_soft_delete_on_destroy = false
    }
  }
  resource_provider_registrations = "none"
}

resource "random_id" "suffix" {
  byte_length = 4
}

variable "resource_group_name" {}

data "azurerm_resource_group" "lab" {
  name = var.resource_group_name
}

data "azurerm_client_config" "current" {}

# ── CIS 6.2 violation: SSH open to the Internet ─────────────────────────────

resource "azurerm_network_security_group" "cis" {
  name                = "nsg-cis-${random_id.suffix.hex}"
  resource_group_name = data.azurerm_resource_group.lab.name
  location            = data.azurerm_resource_group.lab.location
}

resource "azurerm_network_security_rule" "ssh_open" {
  name                        = "allow-ssh-from-internet"
  priority                    = 100
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Tcp"
  source_port_range           = "*"
  destination_port_range      = "22"
  source_address_prefix       = "*"
  destination_address_prefix  = "*"
  resource_group_name         = data.azurerm_resource_group.lab.name
  network_security_group_name = azurerm_network_security_group.cis.name
}

# ── CIS 1.23 violation: Privileged role assignment ──────────────────────────

resource "azurerm_user_assigned_identity" "cis" {
  name                = "id-cis-${random_id.suffix.hex}"
  resource_group_name = data.azurerm_resource_group.lab.name
  location            = data.azurerm_resource_group.lab.location
}

resource "azurerm_role_assignment" "owner" {
  scope                = data.azurerm_resource_group.lab.id
  role_definition_name = "Owner"
  principal_id         = azurerm_user_assigned_identity.cis.principal_id
}

# ── CIS 8.5 violation: Destructive key vault permissions ────────────────────

resource "azurerm_key_vault" "cis" {
  name                       = "kv-cis-${random_id.suffix.hex}"
  location                   = data.azurerm_resource_group.lab.location
  resource_group_name        = data.azurerm_resource_group.lab.name
  tenant_id                  = data.azurerm_client_config.current.tenant_id
  sku_name                   = "standard"
  soft_delete_retention_days = 7
  purge_protection_enabled   = false
}

resource "azurerm_key_vault_access_policy" "destructive" {
  key_vault_id = azurerm_key_vault.cis.id
  tenant_id    = data.azurerm_client_config.current.tenant_id
  object_id    = data.azurerm_client_config.current.object_id

  secret_permissions = [
    "Get",
    "List",
    "Set",
    "Delete",
    "Purge",
  ]
}
