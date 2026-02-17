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

resource "azurerm_route_table" "test" {
  name                = "rt-verify-${random_id.suffix.hex}"
  resource_group_name = data.azurerm_resource_group.lab.name
  location            = data.azurerm_resource_group.lab.location
}

resource "azurerm_route" "test" {
  name                = "internet-route"
  resource_group_name = data.azurerm_resource_group.lab.name
  route_table_name    = azurerm_route_table.test.name
  address_prefix      = "203.0.113.0/24"
  next_hop_type       = "Internet"
}
