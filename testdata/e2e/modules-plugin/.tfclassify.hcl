plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.3.0"
}

classification "critical" {
  description = "Requires security review"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  azurerm {
    privilege_escalation {
      actions = ["Microsoft.Authorization/*"]
    }
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
}

precedence = ["critical", "standard", "auto"]

defaults {
  unclassified   = "standard"
  no_changes     = "auto"
  plugin_timeout = "30s"
}
