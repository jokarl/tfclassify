classification "critical" {
  description = "Critical"
}

classification "standard" {
  description = "Standard"

  rule {
    resource = ["*"]
  }
}

precedence = ["critical", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
