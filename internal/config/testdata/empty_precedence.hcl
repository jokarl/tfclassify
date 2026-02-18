classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }
}

precedence = []

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
