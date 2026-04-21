classification "standard" {
  description = "Standard review required"

  rule {
    resource = ["*"]
    actions  = ["update", "create", "delete"]
  }
}

classification "auto" {
  description = "Auto-approved — cosmetic or computed-only changes"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
  rule {
    resource = ["*"]
    actions  = ["read"]
  }
}

precedence = ["standard", "auto"]

defaults {
  unclassified      = "standard"
  no_changes        = "auto"
  ignore_attributes = ["tags"]

  # CR-0035: scoped rule — ignore azapi_resource's computed output attribute,
  # which refreshes on every plan and is not a user-authored change.
  ignore_attribute "azapi_output" {
    description = "azapi_resource.output is a computed read-back of the API response; not a user change."
    resource    = ["azapi_resource"]
    attributes  = ["output"]
  }
}
