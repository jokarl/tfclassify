classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }
}

precedence = ["standard"]

defaults {
  unclassified      = "standard"
  no_changes        = "standard"
  ignore_attributes = ["tags", ""]
}
