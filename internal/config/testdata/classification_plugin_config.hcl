plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify-plugin-azurerm"
  version = "0.1.0"
}

classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  azurerm {
    privilege_escalation {
      actions = ["Microsoft.Authorization/*"]
      exclude = ["AcrPush", "AcrPull"]
    }

    network_exposure {
      permissive_sources = ["*", "0.0.0.0/0", "Internet"]
    }

    keyvault_access {}
  }
}

classification "high" {
  description = "Requires team lead approval"

  rule {
    resource = ["*_role_*"]
    actions  = ["create", "update"]
  }

  azurerm {
    privilege_escalation {
      roles = ["Contributor", "Storage Blob Data Owner"]
    }
  }
}

classification "standard" {
  description = "Standard change process"

  rule {
    resource = ["*"]
  }
}

precedence = ["critical", "high", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
