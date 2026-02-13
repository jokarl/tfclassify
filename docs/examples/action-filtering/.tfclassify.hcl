# Action Filtering Example
#
# Demonstrates how the "actions" field narrows classification rules.
# The same resource type (azurerm_role_assignment) is classified differently
# depending on whether the action is a delete or an update.
#
# Run:
#   tfclassify -p plan.json -c .tfclassify.hcl --no-plugins -v

# "critical" — deleting identity/access resources.
#
# Both "resource" and "actions" must match for the rule to apply.
# This means: only a DELETE of a role/IAM resource triggers "critical".
# A create or update of the same resource type will skip this rule.
classification "critical" {
  description = "Requires security team approval"

  rule {
    resource = ["*_role_*", "*_iam_*"]
    actions  = ["delete"]
    # Matches: azurerm_role_assignment being DELETED
    # Skips:   azurerm_role_assignment being created or updated
  }
}

# "review" — any change to identity/access resources (all actions).
#
# No "actions" field means this rule matches any action.
#
# Because rules are evaluated in precedence order (critical first, then
# review), a role deletion will match "critical" above and never reach
# this rule. A role update or create will skip "critical" (wrong action)
# and then match here.
#
# This pattern — a narrow rule at higher precedence, a broad rule at lower
# precedence — lets you escalate specific actions while still flagging the
# resource type in general.
classification "review" {
  description = "Requires team lead review"

  rule {
    resource = ["*_role_*", "*_iam_*"]
    # No "actions" filter: matches create, update, delete, read, no-op.
    # In practice, deletes are already caught by "critical" above,
    # so this rule handles creates and updates.
  }
}

# "standard" — everything that isn't identity/access.
classification "standard" {
  description = "Standard change process"

  rule {
    not_resource = ["*_role_*", "*_iam_*"]
    # Matches any resource whose type does NOT contain "_role_" or "_iam_".
  }
}

# "auto" — no-op changes only.
classification "auto" {
  description = "Automatic approval"

  rule {
    resource = ["*"]
    actions  = ["no-op"]
  }
}

# Four precedence levels produce exit codes 0–3:
#   auto=0, standard=1, review=2, critical=3
#
# The overall exit code is determined by the highest-precedence classification
# across ALL resources in the plan. One critical resource makes the entire
# plan exit with code 3.
precedence = ["critical", "review", "standard", "auto"]

defaults {
  unclassified = "standard"
  no_changes   = "auto"
}
