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

data "azurerm_subscription" "current" {}

# Create managed identity to receive role assignments
resource "azurerm_user_assigned_identity" "test" {
  name                = "id-verify-custom-role-${random_id.suffix.hex}"
  resource_group_name = data.azurerm_resource_group.lab.name
  location            = data.azurerm_resource_group.lab.location
}

# Custom role with Microsoft.Authorization actions — should trigger critical
# via pattern-based cross-referencing from the plan
resource "azurerm_role_definition" "auth_writer" {
  name        = "Custom Auth Writer ${random_id.suffix.hex}"
  scope       = data.azurerm_subscription.current.id
  description = "Custom role with authorization write access for e2e testing"

  permissions {
    actions = [
      "Microsoft.Authorization/roleAssignments/write",
      "Microsoft.Authorization/roleAssignments/delete",
    ]
    not_actions = []
  }

  assignable_scopes = [
    data.azurerm_subscription.current.id,
  ]
}

# Assign the custom role — plugin should cross-reference the role_definition
# above and match via actions = ["Microsoft.Authorization/*"]
resource "azurerm_role_assignment" "custom" {
  scope              = data.azurerm_resource_group.lab.id
  role_definition_id = azurerm_role_definition.auth_writer.role_definition_resource_id
  principal_id       = azurerm_user_assigned_identity.test.principal_id
}
