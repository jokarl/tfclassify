plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.1.0"
}

classification "critical" {
  description = "Critical - combined role aggregation detects privilege escalation"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  # merge_principal_roles enables per-principal evaluation: group all role
  # assignments by principal_id, compute the union of effective permissions,
  # and match against actions/data_actions patterns.
  #
  # In this scenario, the custom role's role_definition_id is computed at plan
  # time (cross-reference), so it triggers via the unresolved-custom-role path.
  # Reader alone would not match the pattern.
  azurerm {
    privilege_escalation {
      actions              = ["Microsoft.Authorization/roleAssignments/write"]
      merge_principal_roles = true
      flag_unknown_roles    = false
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
