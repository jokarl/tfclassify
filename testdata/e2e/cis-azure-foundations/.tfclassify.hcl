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
#   6.1/6.2 — Restrict RDP/SSH from the Internet  → network_exposure
#   1.23    — No custom admin role assignments     → privilege_escalation
#   8.5     — Key Vault must be recoverable        → keyvault_access

plugin "azurerm" {
  enabled = true
  source  = "github.com/jokarl/tfclassify"
  version = "0.1.0"
}

# ── CIS Section 6: Networking ───────────────────────────────────────────────
#
# 6.1: Ensure that RDP access from the Internet is evaluated and restricted
#      https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0#ensure-that-rdp-access-from-the-internet-is-evaluated-and-restricted
# 6.2: Ensure that SSH access from the Internet is evaluated and restricted
#      https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0#ensure-that-ssh-access-from-the-internet-is-evaluated-and-restricted
#
# The network_exposure analyzer detects NSG rules that allow inbound traffic
# from overly broad sources — the exact condition CIS 6.1/6.2 prohibit.

classification "CIS-6" {
  description = "CIS 6 – Networking: Restrict inbound Internet access"

  rule {
    description = "CIS 6.1/6.2: Deleting network security controls requires review"
    resource    = ["*_security_group", "*_security_rule"]
    actions     = ["delete"]
  }

  azurerm {
    network_exposure {
      permissive_sources = ["*", "0.0.0.0/0", "Internet"]
    }
  }
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

# ── CIS Section 8: Key Vault ───────────────────────────────────────────────
#
# 8.5: Ensure the Key Vault is Recoverable
#      https://learn.microsoft.com/en-us/azure/governance/policy/samples/cis-azure-2-0-0#ensure-the-key-vault-is-recoverable
#
# The keyvault_access analyzer detects access policies granting destructive
# permissions (Delete, Purge) that could compromise key vault recoverability.

classification "CIS-8" {
  description = "CIS 8 – Key Vault: Protect secret management infrastructure"

  rule {
    description = "CIS 8.5: Deleting key vault resources compromises recoverability"
    resource    = ["*_key_vault*"]
    actions     = ["delete"]
  }

  azurerm {
    keyvault_access {}
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

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

# Precedence maps to exit codes:
#   CIS-6=4, CIS-1=3, CIS-8=2, standard=1, auto=0
precedence = ["CIS-6", "CIS-1", "CIS-8", "standard", "auto"]

defaults {
  unclassified   = "standard"
  no_changes     = "auto"
  plugin_timeout = "30s"
}
