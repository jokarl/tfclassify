plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.1.0"
}

classification "critical" {
  description = "Requires security review"

  rule {
    resource = ["*_role_*", "*_security_group", "*_security_rule", "*_key_vault*"]
    actions  = ["delete"]
  }

  # Detect key vault access policies with destructive permissions
  azurerm {
    keyvault_access {}
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
