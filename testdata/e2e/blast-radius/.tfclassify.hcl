classification "critical" {
  description = "Blast radius exceeded"

  blast_radius {
    max_changes = 3
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
