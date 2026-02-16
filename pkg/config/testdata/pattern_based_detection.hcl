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
