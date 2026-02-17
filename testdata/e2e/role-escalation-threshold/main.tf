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

# Create managed identity to receive role assignments
resource "azurerm_user_assigned_identity" "test" {
  name                = "id-verify-threshold-${random_id.suffix.hex}"
  resource_group_name = data.azurerm_resource_group.lab.name
  location            = data.azurerm_resource_group.lab.location
}

# Owner role assignment - has Microsoft.Authorization/* actions, triggers critical
resource "azurerm_role_assignment" "owner" {
  scope                = data.azurerm_resource_group.lab.id
  role_definition_name = "Owner"
  principal_id         = azurerm_user_assigned_identity.test.principal_id
}

# Contributor role assignment - has NotActions: ["Microsoft.Authorization/*"], triggers standard via */write
resource "azurerm_role_assignment" "contributor" {
  scope                = data.azurerm_resource_group.lab.id
  role_definition_name = "Contributor"
  principal_id         = azurerm_user_assigned_identity.test.principal_id
}
