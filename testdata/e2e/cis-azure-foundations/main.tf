# CIS Azure Foundations Benchmark — e2e scenario
#
# Creates resources that violate a real CIS control to demonstrate
# how tfclassify classifications can be named after benchmark sections.
#
# CIS controls violated (Azure Policy mapping: https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0):
#   1.23 — Privileged role assignment    (privilege_escalation analyzer)
#          https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0#ensure-that-no-custom-subscription-administrator-roles-exist

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
  features {}
  resource_provider_registrations = "none"
}

resource "random_id" "suffix" {
  byte_length = 4
}

variable "resource_group_name" {}

data "azurerm_resource_group" "lab" {
  name = var.resource_group_name
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
