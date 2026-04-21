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
  ignore_attributes = ["tags"]

  ignore_attribute "azapi_output" {
    description = "azapi computed refresh"
    resource    = ["azapi_resource"]
    attributes  = ["output"]
  }

  ignore_attribute "timestamps" {
    description = "transient timestamps"
    resource    = ["azapi_*"]
    attributes  = ["properties.*.createdAt", "properties.*.updatedAt"]
  }
}
