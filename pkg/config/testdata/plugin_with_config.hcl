plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify-plugin-azurerm"
  version = "1.0.0"

  config {
    privileged_roles = ["Owner", "Contributor"]
    max_severity     = 100
  }
}

classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }
}

precedence = ["standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
