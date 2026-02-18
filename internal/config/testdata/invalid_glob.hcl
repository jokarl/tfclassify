classification "critical" {
  description = "Critical"

  rule {
    resource = ["*_role_[*"]
  }
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
