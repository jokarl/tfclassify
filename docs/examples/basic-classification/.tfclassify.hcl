# Basic Classification Example
#
# Demonstrates resource type pattern matching using glob rules.
# Resources are classified solely by their type name — no action filtering.
#
# Run:
#   tfclassify -p plan.json -c .tfclassify.hcl --no-plugins -v

# "critical" — any change to identity/access resources, regardless of action.
#
# The "resource" field accepts glob patterns matched against the Terraform
# resource type (e.g. "azurerm_role_assignment", "aws_iam_role").
# The "*" character matches any sequence of characters.
#
# No "actions" field means this rule matches creates, updates, AND deletes.
classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*", "*_iam_*"]
    # Matches: azurerm_role_assignment, aws_iam_policy, google_project_iam_member, ...
    # Does NOT match: azurerm_virtual_network, azurerm_storage_account, ...
  }
}

# "standard" — everything that isn't identity/access.
#
# "not_resource" is the inverse of "resource": a resource matches this rule
# if its type does NOT match any of the listed patterns. This is useful as
# a catch-all for "everything else" without listing every resource type.
classification "standard" {
  description = "Standard change process"

  rule {
    not_resource = ["*_role_*", "*_iam_*"]
    # Matches: azurerm_virtual_network, azurerm_resource_group, aws_s3_bucket, ...
    # Does NOT match: azurerm_role_assignment, aws_iam_policy, ...
  }
}

# "auto" — only no-op changes (no actual modifications).
#
# The "actions" field restricts which Terraform actions trigger this rule.
# Valid actions: "create", "update", "delete", "read", "no-op".
# A "no-op" means Terraform evaluated the resource but found no changes.
classification "auto" {
  description = "Automatic approval"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

# Precedence order determines two things:
#
# 1. Rule evaluation: when a resource matches rules in multiple classifications,
#    the classification listed FIRST wins (highest precedence).
#
# 2. Exit codes: derived from position. The last entry gets exit code 0,
#    and codes increase toward the first entry.
#    Here: auto=0, standard=1, critical=2
precedence = ["critical", "standard", "auto"]

# Defaults handle edge cases where no rule matches.
defaults {
  # Resources that match no classification rule get this classification.
  unclassified = "standard"

  # Plans with zero resource changes get this classification (exit code 0).
  no_changes = "auto"
}
