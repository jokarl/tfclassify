plugin "azurerm" {
  enabled = true

  config {
    privilege_enabled = false
    network_enabled   = true
    keyvault_enabled  = false
  }
}

classification "critical" {
  description = "Requires security review"

  rule {
    resource = ["*_role_*", "*_security_group", "*_security_rule"]
    actions  = ["delete"]
  }
}

classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }
}

classification "auto" {
  description = "Auto-approved"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

precedence = ["critical", "standard", "auto"]

defaults {
  unclassified   = "standard"
  no_changes     = "auto"
  plugin_timeout = "30s"
}
