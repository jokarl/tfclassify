classification "standard" {
  description = "Standard"
  rule { resource = ["*"] }
}

precedence = ["standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"

  ignore_attribute "dup" {
    description = "first"
    attributes  = ["a"]
  }

  ignore_attribute "dup" {
    description = "second"
    attributes  = ["b"]
  }
}
