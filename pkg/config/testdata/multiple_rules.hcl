classification "critical" {
  description = "Critical changes"

  rule {
    resource = ["*_role_*"]
    actions  = ["delete"]
  }

  rule {
    resource = ["*_key_vault*"]
    actions  = ["delete"]
  }
}

classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }
}

precedence = ["critical", "standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
