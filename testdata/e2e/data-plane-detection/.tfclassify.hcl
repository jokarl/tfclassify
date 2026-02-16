plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.1.0"
}

classification "critical" {
  description = "Critical - data plane storage access detected"

  rule {
    resource = ["*_role_*", "*_security_group", "*_security_rule"]
    actions  = ["delete"]
  }

  # CR-0027: Trigger critical for data-plane access patterns
  # Storage Blob Data Owner has dataActions: ["Microsoft.Storage/.../blobs/*"]
  # Use "Microsoft.Storage/*" to match any storage data-plane action.
  # Set score_threshold high to suppress legacy control-plane detection;
  # only data-plane pattern matching should trigger critical.
  azurerm {
    privilege_escalation {
      score_threshold = 100
      data_actions    = ["Microsoft.Storage/*"]
    }
  }
}

classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }

  # Catch remaining privilege escalations without data-plane specific patterns
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
