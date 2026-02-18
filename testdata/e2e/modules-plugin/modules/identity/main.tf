variable "name_suffix" {}
variable "resource_group_name" {}
variable "resource_group_id" {}
variable "location" {}

resource "azurerm_user_assigned_identity" "this" {
  name                = "id-mod-${var.name_suffix}"
  resource_group_name = var.resource_group_name
  location            = var.location
}

resource "azurerm_role_assignment" "this" {
  scope                = var.resource_group_id
  role_definition_name = "Owner"
  principal_id         = azurerm_user_assigned_identity.this.principal_id
}
