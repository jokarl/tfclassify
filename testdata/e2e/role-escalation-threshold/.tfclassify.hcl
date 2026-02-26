plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.3.0"
}

classification "critical" {
  description = "Requires security review - high privilege escalation"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  # CR-0028: Pattern-based control-plane detection
  # Owner has Microsoft.Authorization/* actions (triggers critical)
  # Contributor has NotActions: ["Microsoft.Authorization/*"] so does NOT match
  azurerm {
    privilege_escalation {
      actions = ["Microsoft.Authorization/*"]
    }
  }
}

classification "standard" {
  description = "Standard change - moderate privilege escalation"

  rule {
    resource = ["*"]
  }

  # Catch all privilege escalations with any write action
  # Contributor has many write actions (triggers here)
  azurerm {
    privilege_escalation {
      actions = ["*/write", "*/delete", "*/action"]
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
