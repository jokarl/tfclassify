plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.1.0"
}

classification "critical" {
  description = "Requires security review - high privilege escalation"

  rule {
    resource = ["*_role_*", "*_security_group", "*_security_rule"]
    actions  = ["delete"]
  }

  # Trigger critical for privilege escalation with score >= 70
  # Owner at RG scope = 95 * 0.8 = 76 (triggers)
  # Contributor at RG scope = 70 * 0.8 = 56 (does not trigger)
  azurerm {
    privilege_escalation {
      score_threshold = 70
    }
  }
}

classification "standard" {
  description = "Standard change - moderate privilege escalation"

  rule {
    resource = ["*"]
  }

  # Catch all privilege escalations not caught by critical (any score)
  # Contributor at RG scope = 56 (triggers here)
  azurerm {
    privilege_escalation {}
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
