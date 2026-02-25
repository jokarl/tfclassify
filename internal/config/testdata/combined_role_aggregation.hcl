plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify-plugin-azurerm"
  version = "0.1.0"
}

classification "critical" {
  description = "Critical - combined role aggregation"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  azurerm {
    privilege_escalation {
      actions      = ["Microsoft.Authorization/roleAssignments/write"]
      data_actions = ["Microsoft.Storage/storageAccounts/blobServices/containers/blobs/*"]
      scopes       = ["subscription"]

      # Enable principal-level evaluation: merge roles per identity
      merge_principal_roles = true
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
