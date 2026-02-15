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

resource "azurerm_route_table" "test" {
  name                = "rt-verify-${random_id.suffix.hex}"
  resource_group_name = data.azurerm_resource_group.lab.name
  location            = data.azurerm_resource_group.lab.location
}

resource "azurerm_route" "test" {
  name                = "internet-route"
  resource_group_name = data.azurerm_resource_group.lab.name
  route_table_name    = azurerm_route_table.test.name
  address_prefix      = "10.100.0.0/16"
  next_hop_type       = "Internet"
}
