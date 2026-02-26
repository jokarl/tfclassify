plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.3.0"
}

classification "critical" {
  description = "Critical - authorization control access detected"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  # CR-0028: Pattern-based control-plane detection
  # User Access Administrator has Microsoft.Authorization/* actions
  azurerm {
    privilege_escalation {
      actions = ["Microsoft.Authorization/*", "*"]
    }
  }
}

classification "standard" {
  description = "Standard change - read-only access"

  rule {
    resource = ["*"]
  }

  # Catch read-only roles via pattern matching
  azurerm {
    privilege_escalation {
      actions = ["*/read"]
    }
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
