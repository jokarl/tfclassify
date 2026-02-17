plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify-plugin-azurerm"
  version = "0.1.0"
}

classification "critical" {
  description = "Critical - pattern-based detection"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  azurerm {
    privilege_escalation {
      # CR-0028: Pattern-based control-plane detection
      actions = ["*", "Microsoft.Authorization/*"]

      # CR-0027: Data-plane pattern detection
      data_actions = ["*/read", "*/write"]

      # CR-0028: Scope filtering
      scopes = ["subscription", "management_group"]

      # CR-0028: Unknown role handling
      flag_unknown_roles = false
    }
  }
}

classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }
}

precedence = ["critical", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
