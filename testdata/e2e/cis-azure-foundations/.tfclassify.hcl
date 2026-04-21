# CIS Azure Foundations Benchmark
#
# Demonstrates naming classifications after CIS benchmark sections.
# Each classification maps a CIS control to an azurerm plugin analyzer
# that verifies the control requirement at Terraform plan time.
#
# Azure Policy mapping reference:
#   https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0
#
# CIS controls verified:
#   1.23 — No custom admin role assignments → privilege_escalation

plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.3.0"
}

# ── CIS Section 1: Identity and Access Management ──────────────────────────
#
# 1.23: Ensure that no custom subscription administrator roles exist
#       https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0#ensure-that-no-custom-subscription-administrator-roles-exist
#
# The privilege_escalation analyzer detects role assignments granting
# wildcard (*) or Microsoft.Authorization/* control-plane permissions.

classification "CIS-1" {
  description = "CIS 1 – IAM: Restrict privileged role assignments"

  rule {
    description = "CIS 1.23: Deleting role assignments or definitions requires review"
    resource    = ["*_role_assignment", "*_role_definition"]
    actions     = ["delete"]
  }

  azurerm {
    privilege_escalation {
      actions = ["*", "Microsoft.Authorization/*"]
    }
  }
}

classification "standard" {
  description = "Standard change – peer review required"

  rule {
    resource = ["*"]
  }
}

classification "auto" {
  description = "No approval needed"
}

# Precedence maps to exit codes:
#   CIS-1=2, standard=1, auto=0
precedence = ["CIS-1", "standard", "auto"]

defaults {
  unclassified   = "standard"
  no_changes     = "auto"
  plugin_timeout = "30s"
}
