variable "name_suffix" {}
variable "resource_group_name" {}
variable "location" {}

resource "azurerm_network_security_group" "this" {
  name                = "nsg-mod-${var.name_suffix}"
  resource_group_name = var.resource_group_name
  location            = var.location
}

resource "azurerm_route_table" "this" {
  name                = "rt-mod-${var.name_suffix}"
  resource_group_name = var.resource_group_name
  location            = var.location
}
