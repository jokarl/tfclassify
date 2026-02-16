# Full Reference Example
#
# Every configurable field in .tfclassify.hcl, annotated.
#
# Run:
#   tfclassify -p plan.json -c .tfclassify.hcl -v

# ─── Plugins ──────────────────────────────────────────────────────────────────
#
# Plugins extend classification with deep-inspection analyzers that run after
# the core rules below. Plugin decisions are merged via the same precedence
# system — a plugin can escalate a resource's classification but never lower it.
#
# Each plugin block declares a plugin by name. The name determines the binary
# looked up at runtime: "azurerm" → tfclassify-plugin-azurerm.
#
# Binary search order:
#   1. $TFCLASSIFY_PLUGIN_DIR (if set)
#   2. .tfclassify/plugins/   (current working directory)
#   3. ~/.tfclassify/plugins/  (home directory)

# External plugin — downloaded by "tfclassify init" from a GitHub release.
plugin "azurerm" {
  enabled = true

  # GitHub repository containing the plugin release. The installer extracts
  # owner/repo and downloads from GitHub Releases at:
  #   https://github.com/{owner}/{repo}/releases/download/{tag}/
  #     tfclassify-plugin-{name}_{version}_{os}_{arch}.zip
  #
  # Tag auto-detection: the installer compares the repo name against the
  # expected binary name (tfclassify-plugin-{name}):
  #   - Match (standalone repo)  → tag = v{version}
  #   - No match (monorepo)      → tag = tfclassify-plugin-{name}-v{version}
  source = "github.com/jokarl/tfclassify"

  # Semantic version to download.
  version = "0.1.0"

  # Note: plugin-specific configuration is now defined per-classification
  # inside classification blocks (see "critical" and "high" below).
  # The old top-level "config {}" block is deprecated.
}

# Disabled plugin — declared but not executed. Useful for temporarily turning
# off a plugin without removing its configuration.
plugin "aws" {
  enabled = false
  source  = "github.com/example/tfclassify-plugin-aws"
  version = "0.2.0"
}

# ─── Classifications ─────────────────────────────────────────────────────────
#
# Each classification block defines a decision level. A resource matches a
# classification if it matches ANY of the classification's rules.
#
# Every classification has a description that appears in the output report.
# This is what reviewers read when deciding whether to approve a change, so
# make it actionable.

classification "critical" {
  description = "Requires security team approval — blocks automated deployment"

  # Multiple rules per classification are OR'd: match any one and the
  # resource is classified at this level.

  # Rule 1: Deleting identity or access resources.
  # The "description" field is optional but recommended. It appears in verbose
  # and JSON output next to each classified resource, explaining WHY the rule
  # matched. Without it, an auto-generated description is used.
  rule {
    description = "Deleting IAM or role resources requires security review"
    resource    = ["*_role_*", "*_iam_*"]
    actions     = ["delete"]
    # "actions" restricts which Terraform actions trigger this rule.
    # Valid values: "create", "update", "delete", "read", "no-op".
    # Omitting "actions" matches ALL actions.
  }

  # Rule 2: Deleting a key vault (but not its children).
  rule {
    description = "Deleting a key vault destroys all secrets and keys within it"
    resource    = ["*_key_vault"]
    actions     = ["delete"]
    # Pattern "*_key_vault" (no trailing wildcard) matches "azurerm_key_vault"
    # but NOT "azurerm_key_vault_secret" or "azurerm_key_vault_key".
  }

  # Rule 3: Any change to subscription-level resources.
  rule {
    description = "Subscription-level changes affect the entire tenant"
    resource    = ["*_subscription*", "*_management_group*"]
    # No "actions" field — matches create, update, delete, read, and no-op.
  }

  # Plugin-specific analyzer configuration for this classification level.
  # Each enabled plugin can have per-analyzer sub-blocks here.
  azurerm {
    # Privilege escalation detection with high threshold (tier 1-2 roles only)
    privilege_escalation {
      score_threshold = 80  # Only Owner (95) and UAA (85) trigger critical
      exclude = ["AcrPush", "AcrPull"]  # Exclude container registry roles
    }

    # Network exposure detection
    network_exposure {
      permissive_sources = ["*", "0.0.0.0/0", "Internet"]
    }

    # Key vault destructive permission detection
    keyvault_access {}  # Empty block = use defaults
  }
}

classification "high" {
  description = "Requires team lead approval before merge"

  # Rule 1: Creating or updating IAM/role resources (deletes are critical above).
  rule {
    description = "Non-destructive IAM changes need team lead sign-off"
    resource    = ["*_role_*", "*_iam_*"]
    actions     = ["create", "update"]
    # Deletes already caught by "critical" above due to precedence ordering.
  }

  # Rule 2: Network security changes.
  rule {
    description = "Network security changes affect access controls"
    resource    = ["*_security_rule", "*_firewall_*", "*_network_security_group"]
  }

  # Rule 3: Key vault children (secrets, keys, certificates).
  rule {
    description = "Key vault secret/key changes need review"
    resource    = ["*_key_vault_*"]
    # Trailing wildcard means this matches children like
    # azurerm_key_vault_secret but NOT azurerm_key_vault itself.
  }

  # Rule 4: DNS changes.
  rule {
    description = "DNS changes can cause outages"
    resource    = ["*_dns_*"]
  }

  # Plugin analyzer configuration for "high" — lower thresholds than "critical"
  azurerm {
    privilege_escalation {
      # Any privilege escalation triggers "high" (no threshold = any score)
    }
    network_exposure {}
    keyvault_access {}
  }
}

classification "standard" {
  description = "Standard change process — peer review required"

  # Rule using "not_resource" — matches everything EXCEPT the listed patterns.
  # This is an alternative to using resource = ["*"] as a catch-all.
  #
  # Use "not_resource" when you want to explicitly exclude specific types at
  # THIS level rather than relying on higher-precedence rules to consume them.
  #
  # Cannot combine "resource" and "not_resource" in the same rule.
  rule {
    description = "All infrastructure changes not covered above"
    not_resource = ["*_monitor_*", "*_log_*", "*_diagnostic_*"]
    # Matches everything except monitoring/logging resources, which are
    # handled by "low" below.
    #
    # In practice, resources already matched by "critical" or "high" above
    # will never reach this rule due to precedence-ordered evaluation.
  }
}

classification "low" {
  description = "Observability changes — auto-approved with notification"

  rule {
    description = "Monitoring and logging changes are low-risk"
    resource    = ["*_monitor_*", "*_log_*", "*_diagnostic_*"]
  }
}

classification "auto" {
  description = "No approval needed"

  # No-op only: Terraform evaluated the resource but found no changes.
  rule {
    description = "No actual changes detected"
    resource    = ["*"]
    actions     = ["no-op"]
  }
}

# ─── Precedence ───────────────────────────────────────────────────────────────
#
# Ordered list of classification names, highest priority first.
#
# Two purposes:
#
# 1. Rule evaluation order — when a resource matches rules in multiple
#    classifications, the one listed FIRST wins. This is why catch-all rules
#    (resource = ["*"]) are safe at lower levels.
#
# 2. Exit codes — derived from position, counting backward from 0:
#      auto=0, low=1, standard=2, high=3, critical=4
#
#    The overall exit code is the highest across ALL resources in the plan.
#    One critical resource makes the entire plan exit 4.
precedence = ["critical", "high", "standard", "low", "auto"]

# ─── Defaults ─────────────────────────────────────────────────────────────────
#
# Required block that controls fallback behavior.
defaults {
  # Classification for resources that match no rule at all.
  # This is the safety net — an unusual resource type not covered by any glob
  # still gets classified rather than silently ignored.
  unclassified = "standard"

  # Classification when the plan contains zero resource changes.
  # Always produces exit code 0 regardless of the classification's position
  # in the precedence list.
  no_changes = "auto"

  # Timeout for external plugin execution. Parsed as a Go duration string
  # (e.g., "10s", "2m30s", "500ms"). If omitted or invalid, defaults to 30s.
  # Only relevant when external plugins are configured.
  plugin_timeout = "30s"
}
