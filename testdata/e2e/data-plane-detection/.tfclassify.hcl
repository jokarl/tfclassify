plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.3.0"
}

classification "critical" {
  description = "Critical - data plane storage access detected"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  # CR-0027: Trigger critical for data-plane access patterns
  # Storage Blob Data Owner has dataActions: ["Microsoft.Storage/.../blobs/*"]
  # Use "Microsoft.Storage/*" to match any storage data-plane action.
  # Only data-plane pattern matching triggers critical here.
  azurerm {
    privilege_escalation {
      data_actions = ["Microsoft.Storage/*"]
    }
  }
}

classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }

  # Catch remaining role assignments with any control-plane write actions
  azurerm {
    privilege_escalation {
      actions = ["*/write", "*/delete", "*/action", "*/read"]
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
