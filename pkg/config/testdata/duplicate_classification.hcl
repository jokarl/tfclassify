classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }
}

classification "standard" {
  description = "Duplicate!"

  rule {
    resource = ["*"]
  }
}

precedence = ["standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
