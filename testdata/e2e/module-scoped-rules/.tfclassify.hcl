# Module-scoped rules: classify deletions inside module.network as critical,
# while the same deletion at root level stays standard.

classification "critical" {
  description = "Critical - module-scoped deletion"

  rule {
    description = "Deleting resources inside the network module requires approval"
    module      = ["module.network"]
    resource    = ["*"]
    actions     = ["delete"]
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
}

precedence = ["critical", "standard", "auto"]

defaults {
  unclassified   = "standard"
  no_changes     = "auto"
  plugin_timeout = "30s"
}
