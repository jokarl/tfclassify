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

# Use current client identity — principal_id is known at plan time,
# which allows the combined role aggregation pass to group by principal.
data "azurerm_client_config" "current" {}

# Reader role — only grants */read actions at resource group scope.
# Does NOT individually trigger the per-role "actions" pattern.
resource "azurerm_role_assignment" "reader" {
  scope                = data.azurerm_resource_group.lab.id
  role_definition_name = "Reader"
  principal_id         = data.azurerm_client_config.current.object_id
}

# Custom role with Microsoft.Authorization/roleAssignments/write.
# Scoped to resource group so CI can create/destroy without subscription-level perms.
# Does NOT individually trigger the narrow per-role pattern (delete),
# but DOES contribute to the combined action set for this principal.
resource "azurerm_role_definition" "auth_writer" {
  name        = "Custom Auth Writer ${random_id.suffix.hex}"
  scope       = data.azurerm_resource_group.lab.id
  description = "Custom role with authorization write access for combined role aggregation e2e"

  permissions {
    actions = [
      "Microsoft.Authorization/roleAssignments/write",
    ]
    not_actions = []
  }

  assignable_scopes = [
    data.azurerm_resource_group.lab.id,
  ]
}

resource "azurerm_role_assignment" "auth_writer" {
  scope              = data.azurerm_resource_group.lab.id
  role_definition_id = azurerm_role_definition.auth_writer.role_definition_resource_id
  principal_id       = data.azurerm_client_config.current.object_id
}
