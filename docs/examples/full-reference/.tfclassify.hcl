# Full Reference Example
#
# Every configurable field in .tfclassify.hcl, annotated.
#
# Run:
#   tfclassify -p plan.json -c .tfclassify.hcl -v
# or with a binary plan (requires terraform on PATH):
#   tfclassify -p tfplan -c .tfclassify.hcl -v

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
  version = "0.3.0"

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
#
# The optional "sarif_level" field controls the SARIF severity when using
# --output sarif. Valid values: "error", "warning", "note", "none".
# When omitted, defaults to "error" for the highest-precedence classification
# and "warning" for all others. The no_changes default gets "none".

classification "critical" {
  description = "Requires security team approval — blocks automated deployment"
  sarif_level = "error"

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

  # Rule 4: Deletions inside production modules.
  # The "module" field filters by Terraform module path. Uses glob patterns
  # with "." as the separator — "*" matches one module level, "**" any depth.
  # Omitting "module" (and "not_module") matches all modules including root.
  rule {
    description = "Production module deletions require security review"
    module      = ["module.production", "module.production.**"]
    resource    = ["*"]
    actions     = ["delete"]
    # "module" restricts this rule to resources inside the named modules.
    # Cannot combine "module" and "not_module" in the same rule.
  }

  # Blast radius thresholds — triggers when plan-wide counts exceed limits.
  # When any threshold is exceeded, ALL resources in the plan receive a
  # decision at this classification level. Omitted fields are not evaluated.
  blast_radius {
    max_deletions    = 5    # Standalone deletions (delete without create)
    max_replacements = 10   # Replacements (delete + create)
    max_changes      = 50   # All resources with non-no-op actions
    exclude_drift    = true  # Don't count drift-corrected resources toward limits
  }

  # Topology thresholds — triggers when a single resource's change propagates
  # to too many downstream resources in the Terraform dependency graph.
  # Uses the plan's configuration block to build a directed dependency graph
  # and computes fan-out via BFS from each changed resource.
  topology {
    max_downstream        = 10  # Flag if change affects 10+ downstream resources
    max_propagation_depth = 3   # Flag if change cascades 3+ levels deep
  }

  # Plugin-specific analyzer configuration for this classification level.
  # Each enabled plugin can have per-analyzer sub-blocks here.
  azurerm {
    # Privilege escalation detection with pattern-based control-plane and data-plane detection.
    # CR-0027/CR-0028: Pattern-based detection gives fine-grained control over what triggers.
    privilege_escalation {
      # Pattern-based control-plane detection (CR-0028).
      # Matches roles with effective control-plane actions matching these patterns.
      actions = ["*", "Microsoft.Authorization/*"]  # Wildcard or auth control access

      # Pattern-based data-plane detection (CR-0027).
      # Matches roles with effective data-plane actions matching these patterns.
      # Data-plane detection is independent from control-plane — either can trigger.
      data_actions = ["*/read"]  # Any data-plane read access (e.g., blob reads)

      # Exclude these roles even if they match patterns above.
      exclude = ["AcrPush", "AcrPull"]

      # Filter which roles to analyze by display name.
      # When set, only role assignments whose resolved display name matches one of
      # these entries are analyzed. Omit to analyze all roles.
      # roles = ["Owner", "User Access Administrator"]

      # Scope-level filtering — only trigger for assignments at these ARM scope levels.
      # Valid values: "management_group", "subscription", "resource_group", "resource".
      # Omit to trigger at all scope levels.
      # scopes = ["management_group", "subscription"]

      # Whether to flag roles whose permissions cannot be resolved (not in the
      # built-in database and not a custom role in the plan). Default: true.
      # flag_unknown_roles = true

      # Enable per-principal evaluation: group role assignments by principal_id,
      # compute the union of effective permissions across all assigned roles, and
      # evaluate the merged set against the same actions/data_actions patterns.
      # Emits one decision per principal instead of per role, with the full
      # effective permission set in metadata. Default: false.
      # merge_principal_roles = true
    }

  }
}

classification "high" {
  description = "Requires team lead approval before merge"
  sarif_level = "warning"

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

  # Rule 5: Deletions outside production modules.
  # "not_module" is the inverse of "module" — matches resources NOT in
  # the listed modules. Cannot combine with "module" in the same rule.
  # rule {
  #   description = "Non-production deletions need team lead review"
  #   not_module  = ["module.production", "module.production.**"]
  #   resource    = ["*"]
  #   actions     = ["delete"]
  # }

  # Less strict blast radius for "high" — catches medium-scale changes.
  blast_radius {
    max_deletions = 2
    max_changes   = 20
  }

  # Less strict topology thresholds for "high".
  topology {
    max_downstream = 20
  }

  # Plugin analyzer configuration for "high" — catch roles with write access
  azurerm {
    privilege_escalation {
      # Pattern-based control-plane detection for write/delete operations.
      # This catches roles that don't match the stricter "critical" patterns above.
      actions = ["*/write", "*/delete"]

      # Pattern-based data-plane detection for write/delete operations.
      data_actions = ["*/write", "*/delete"]
    }
  }
}

classification "standard" {
  description = "Standard change process — peer review required"
  sarif_level = "note"

  # Rule using "not_resource" — matches everything EXCEPT the listed patterns.
  # This is an alternative to using resource = ["*"] as a catch-all.
  #
  # Use "not_resource" when you want to explicitly exclude specific types at
  # THIS level rather than relying on higher-precedence rules to consume them.
  #
  # Cannot combine "resource" and "not_resource" in the same rule.
  rule {
    description  = "All infrastructure changes not covered above"
    not_resource = ["*_monitor_*", "*_log_*", "*_diagnostic_*"]
    # No need for `not_actions = ["no-op"]` — CR-0036 short-circuits no-op
    # resources to defaults.no_changes before rule evaluation runs.
    #
    # In practice, resources already matched by "critical" or "high" above
    # will never reach this rule due to precedence-ordered evaluation.
  }
}

classification "low" {
  description = "Observability changes — auto-approved with notification"
  sarif_level = "note"

  rule {
    description = "Monitoring and logging changes are low-risk"
    resource    = ["*_monitor_*", "*_log_*", "*_diagnostic_*"]
  }
}

classification "auto" {
  description = "No approval needed"
  sarif_level = "none"

  # No rules — the classification named by defaults.no_changes absorbs all
  # no-op resources via short-circuit (CR-0036). Rule evaluation is skipped
  # entirely for resources whose actions are exactly ["no-op"]; they receive
  # this classification with a synthetic rule description explaining why.
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

  # Classification for drift-corrected resources. When set, resources that
  # Terraform plans to change because their actual state has drifted from the
  # desired configuration receive this classification. This separates intentional
  # changes from drift corrections in your approval workflow.
  # Omit to not apply drift-based classification.
  drift_classification = "standard"

  # Timeout for external plugin execution. Parsed as a Go duration string
  # (e.g., "10s", "2m30s", "500ms"). If omitted or invalid, defaults to 30s.
  # Only relevant when external plugins are configured.
  plugin_timeout = "30s"

  # Attributes to ignore when determining if a resource meaningfully changed.
  # If ALL changed attributes on an "update" resource match these prefixes,
  # the resource is reclassified as no-op before classification begins.
  #
  # Uses prefix-based dot-path matching:
  #   "tags"      → covers tags, tags.env, tags.team (but NOT tags_all)
  #   "meta.tags" → covers meta.tags, meta.tags.env (but NOT meta.name)
  #
  # Common use case: module tagging conventions where version bumps cause
  # widespread cosmetic changes (e.g., provenance tag updates).
  ignore_attributes = ["tags", "tags_all"]
}

# ─── Evidence ──────────────────────────────────────────────────────────────────
#
# Evidence artifact output for audit retention. When configured, tfclassify
# produces a self-contained JSON evidence file alongside the normal output.
# The artifact includes input hashes, timestamps, and an optional Ed25519
# signature for tamper evidence.
#
# Usage:
#   tfclassify --plan tfplan --evidence-file evidence.json
#
# If the evidence block is present but --evidence-file is not provided,
# tfclassify writes to the current directory with an auto-generated filename
# and warns to stderr.
evidence {
  # Include per-resource classification decisions in the evidence artifact.
  # Default: true.
  include_resources = true

  # Include the full explain trace (every rule evaluated per resource).
  # Runs the explain pipeline alongside classification — doubles computation
  # but captures full audit trail. Default: false.
  include_trace = false

  # Path to an Ed25519 PEM private key for signing the artifact.
  # Supports environment variable expansion: "$TFCLASSIFY_SIGNING_KEY".
  # When set, the artifact includes signature and signed_content_hash fields.
  # When omitted, the artifact is produced unsigned.
  # signing_key = "$TFCLASSIFY_SIGNING_KEY"
}
