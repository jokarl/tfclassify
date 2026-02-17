# This fixture tests opaque config body passthrough — the top-level config {}
# block is forwarded to the plugin as raw HCL. The field names here are
# intentionally arbitrary (not real azurerm config) to verify that the host
# preserves the body without interpreting it.
plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify-plugin-azurerm"
  version = "1.0.0"

  config {
    privileged_roles = ["Owner", "Contributor"]
    max_severity     = 100
  }
}

classification "standard" {
  description = "Standard change"

  rule {
    resource = ["*"]
  }
}

precedence = ["standard"]

defaults {
  unclassified = "standard"
  no_changes   = "standard"
}
