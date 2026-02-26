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

  # Exclude AcrPush from privilege escalation detection
  azurerm {
    privilege_escalation {
      exclude = ["AcrPush"]
    }
  }
}

classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }

  # Standard also excludes AcrPush - the role should not trigger any plugin decisions
  azurerm {
    privilege_escalation {
      exclude = ["AcrPush"]
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
