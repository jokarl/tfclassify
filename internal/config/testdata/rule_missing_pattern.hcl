classification "standard" {
  description = "Standard change"

  rule {
    actions = ["delete"]
  }
}

precedence = ["standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
