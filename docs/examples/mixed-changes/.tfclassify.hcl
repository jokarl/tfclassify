# Mixed Changes Example
#
# A realistic configuration with multiple classification levels, several
# rules per level, and careful glob patterns that distinguish parent
# resources from their children (e.g. key vaults vs. key vault secrets).
#
# Run:
#   tfclassify -p plan.json -c .tfclassify.hcl --no-plugins -v

# "critical" — destructive changes to sensitive resource types.
#
# A classification block can contain multiple rule blocks. A resource
# matches this classification if it matches ANY of the rules.
classification "critical" {
  description = "Requires security team approval"

  # Rule 1: deleting identity/access resources.
  rule {
    resource = ["*_role_*", "*_iam_*"]
    actions  = ["delete"]
  }

  # Rule 2: deleting a key vault itself.
  #
  # Note the glob pattern "*_key_vault" has no trailing wildcard.
  # This matches "azurerm_key_vault" but NOT "azurerm_key_vault_secret"
  # or "azurerm_key_vault_key". Deleting the vault is critical;
  # modifying its children is handled by "review" below.
  rule {
    resource = ["*_key_vault"]
    actions  = ["delete"]
  }
}

# "review" — changes to network security and key vault children.
classification "review" {
  description = "Requires team lead review"

  # Rule 1: any change to security or firewall rules.
  # No "actions" filter — creating, updating, or deleting these all
  # require review because they affect network access controls.
  rule {
    resource = ["*_security_rule", "*_firewall_*"]
    # Matches: azurerm_network_security_rule, azurerm_firewall_policy, ...
  }

  # Rule 2: any change to key vault children (secrets, keys, certificates).
  #
  # The pattern "*_key_vault_*" requires characters AFTER "_key_vault_",
  # so it matches "azurerm_key_vault_secret" and "azurerm_key_vault_key"
  # but NOT "azurerm_key_vault" (the vault itself — that's handled above).
  rule {
    resource = ["*_key_vault_*"]
    # Matches: azurerm_key_vault_secret, azurerm_key_vault_key, ...
    # Does NOT match: azurerm_key_vault (no characters after "_key_vault")
  }
}

# "standard" — everything not covered above.
#
# The not_resource list must include ALL patterns from higher-precedence
# classifications to avoid accidentally catching those resources here.
# If a resource matches both "critical" and "standard" rules, precedence
# ensures "critical" wins — but it's cleaner to exclude them explicitly.
classification "standard" {
  description = "Standard change process"

  rule {
    not_resource = [
      "*_role_*",         # covered by critical
      "*_iam_*",          # covered by critical
      "*_key_vault*",     # note: trailing * catches both _key_vault and _key_vault_secret
      "*_security_rule",  # covered by review
      "*_firewall_*",     # covered by review
    ]
  }
}

# "auto" — no actual changes.
classification "auto" {
  description = "Automatic approval"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

# Exit codes: auto=0, standard=1, review=2, critical=3
precedence = ["critical", "review", "standard", "auto"]

defaults {
  # A resource that matches no rule at all (e.g. an unusual resource type
  # not covered by any glob) gets classified as "standard".
  unclassified = "standard"

  # A plan with zero resource changes exits with "auto" (exit code 0).
  no_changes = "auto"
}
