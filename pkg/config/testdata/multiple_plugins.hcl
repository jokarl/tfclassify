plugin "terraform" {
  enabled = true
}

plugin "azurerm" {
  enabled = true
}

plugin "aws" {
  enabled = false
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
