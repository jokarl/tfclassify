classification "critical" {
  description = "Critical"

  rule {
    resource = ["*"]
  }
}

classification "standard" {
  description = "Standard"

  rule {
    resource = ["*_role_*"]
  }
}

precedence = ["critical", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
