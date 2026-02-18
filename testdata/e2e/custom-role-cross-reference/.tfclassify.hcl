plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.1.0"
}

classification "critical" {
  description = "Critical - authorization control access via custom role"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  # CR-0028: Pattern-based control-plane detection
  # Custom role has Microsoft.Authorization/roleAssignments/write
  # which should be cross-referenced from the plan and matched
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
