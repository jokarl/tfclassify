classification "critical" {
  description = "Blast radius exceeded"
  sarif_level = "error"

  blast_radius {
    max_changes = 3
  }
}

classification "standard" {
  description = "Standard change"
  sarif_level = "warning"

  rule {
    resource = ["*"]
  }
}

classification "auto" {
  description = "Auto-approved"
  sarif_level = "none"
}

precedence = ["critical", "standard", "auto"]

defaults {
  unclassified   = "standard"
  no_changes     = "auto"
  plugin_timeout = "30s"
}
