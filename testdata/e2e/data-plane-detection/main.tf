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
  subscription_id                 = "0a271008-02cf-4a50-9bb3-afc7c4aed74c"
  resource_provider_registrations = "none"
}

resource "random_id" "suffix" {
  byte_length = 4
}

data "azurerm_resource_group" "lab" {
  name = "rg-lab-johan.karlsson"
}

# Create managed identity to receive role assignments
resource "azurerm_user_assigned_identity" "test" {
  name                = "id-verify-dataplane-${random_id.suffix.hex}"
  resource_group_name = data.azurerm_resource_group.lab.name
  location            = data.azurerm_resource_group.lab.location
}

# Storage Blob Data Owner - has data-plane actions (should trigger critical via data_actions = ["Microsoft.Storage/*"])
# This role has:
# - dataActions: ["Microsoft.Storage/storageAccounts/blobServices/containers/blobs/*"]
# - actions: ["Microsoft.Storage/storageAccounts/blobServices/containers/*", ...]
resource "azurerm_role_assignment" "storage_data_owner" {
  scope                = data.azurerm_resource_group.lab.id
  role_definition_name = "Storage Blob Data Owner"
  principal_id         = azurerm_user_assigned_identity.test.principal_id
}

# Reader - only has control-plane read actions, no data actions
# Should NOT trigger critical (no storage data actions), falls through to standard
resource "azurerm_role_assignment" "reader" {
  scope                = data.azurerm_resource_group.lab.id
  role_definition_name = "Reader"
  principal_id         = azurerm_user_assigned_identity.test.principal_id
}
