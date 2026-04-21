classification "critical" {
  description = "Requires security review"

  rule {
    resource = ["*_security_group", "*_security_rule"]
    actions  = ["delete"]
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
