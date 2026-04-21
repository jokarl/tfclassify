classification "standard" {
  description = "Standard"
  rule { resource = ["*"] }
}

precedence = ["standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"

  ignore_attribute "bad" {
    description = "unparseable glob"
    attributes  = ["["]
  }
}
