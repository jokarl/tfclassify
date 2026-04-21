# Topology: flag resources whose change propagates to many downstream dependents.
# The vnet has downstream: subnet and nsg_association (2 dependents), exceeding
# the max_downstream=1 threshold → critical.

classification "critical" {
  description = "Topology threshold exceeded"

  topology {
    max_downstream        = 1
    max_propagation_depth = 1
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
