classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }
}

precedence = ["standard"]

defaults {
  unclassified = "missing"
  no_changes   = "standard"
}
