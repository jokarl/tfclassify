// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	"fmt"
	"strings"

	"github.com/jokarl/tfclassify/sdk"
)

// roleSource indicates where a role was resolved from.
type roleSource string

const (
	roleSourceBuiltin        roleSource = "builtin"
	roleSourcePlanCustom     roleSource = "plan-custom-role"
	roleSourceConfigFallback roleSource = "config-fallback"
	roleSourceUnknown        roleSource = "unknown"
)

// resolvedRole contains the resolved role information and score.
type resolvedRole struct {
	name   string
	score  PermissionScore
	source roleSource
}

// PrivilegeEscalationAnalyzer detects privilege escalation in Azure role assignments.
// It uses permission-based scoring, scope weighting, and custom role cross-referencing
// to compute graduated severity based on the actual risk of role changes.
type PrivilegeEscalationAnalyzer struct {
	sdk.DefaultAnalyzer
	config *PluginConfig
}

// NewPrivilegeEscalationAnalyzer creates a new PrivilegeEscalationAnalyzer.
func NewPrivilegeEscalationAnalyzer(config *PluginConfig) *PrivilegeEscalationAnalyzer {
	return &PrivilegeEscalationAnalyzer{config: config}
}

// Name returns the analyzer name.
func (a *PrivilegeEscalationAnalyzer) Name() string {
	return "privilege-escalation"
}

// Enabled returns whether this analyzer is enabled.
func (a *PrivilegeEscalationAnalyzer) Enabled() bool {
	return a.config.PrivilegeEnabled
}

// ResourcePatterns returns the patterns this analyzer is interested in.
func (a *PrivilegeEscalationAnalyzer) ResourcePatterns() []string {
	return []string{"azurerm_role_assignment"}
}

// Analyze inspects role assignments for privilege escalation using permission-based scoring.
func (a *PrivilegeEscalationAnalyzer) Analyze(runner sdk.Runner) error {
	changes, err := runner.GetResourceChanges(a.ResourcePatterns())
	if err != nil {
		return fmt.Errorf("failed to get resource changes: %w", err)
	}

	// Build custom role lookup from plan (if enabled)
	customRoles := a.buildCustomRoleLookup(runner)

	// Get role database (use default if not configured)
	db := a.config.RoleDatabase
	if db == nil {
		db = DefaultRoleDatabase()
	}

	// Build privileged roles set for fallback
	privilegedSet := toSet(a.config.PrivilegedRoles)

	for _, change := range changes {
		scope := stringField(change.After, "scope")
		if scope == "" {
			scope = stringField(change.Before, "scope")
		}

		beforeRole := a.resolveRole(change.Before, db, customRoles, privilegedSet)
		afterRole := a.resolveRole(change.After, db, customRoles, privilegedSet)

		// Apply scope weighting to scores
		beforeScoreWeighted := ApplyScopeMultiplier(beforeRole.score.Total, scope)
		afterScoreWeighted := ApplyScopeMultiplier(afterRole.score.Total, scope)

		// Determine escalation direction
		if afterScoreWeighted > beforeScoreWeighted {
			// Escalation: after is more privileged than before
			reason := fmt.Sprintf("role escalated from %q to %q", beforeRole.name, afterRole.name)
			if beforeRole.name == "" {
				reason = fmt.Sprintf("privileged role %q assigned", afterRole.name)
			}

			decision := &sdk.Decision{
				Classification: "",
				Reason:         reason,
				Severity:       afterScoreWeighted,
				Metadata: map[string]interface{}{
					"analyzer":      "privilege-escalation",
					"before_role":   beforeRole.name,
					"after_role":    afterRole.name,
					"direction":     "escalation",
					"before_score":  beforeScoreWeighted,
					"after_score":   afterScoreWeighted,
					"scope":         scope,
					"scope_level":   ParseScopeLevel(scope).String(),
					"score_factors": afterRole.score.Factors,
					"role_source":   string(afterRole.source),
				},
			}

			if err := runner.EmitDecision(a, change, decision); err != nil {
				return fmt.Errorf("failed to emit decision: %w", err)
			}
		} else if afterScoreWeighted < beforeScoreWeighted && beforeScoreWeighted > 0 {
			// De-escalation: before was more privileged than after
			reason := fmt.Sprintf("role de-escalated from %q to %q", beforeRole.name, afterRole.name)
			if afterRole.name == "" {
				reason = fmt.Sprintf("privileged role %q removed", beforeRole.name)
			}

			decision := &sdk.Decision{
				Classification: "",
				Reason:         reason,
				Severity:       40, // De-escalation always emits severity 40
				Metadata: map[string]interface{}{
					"analyzer":      "privilege-escalation",
					"before_role":   beforeRole.name,
					"after_role":    afterRole.name,
					"direction":     "de-escalation",
					"before_score":  beforeScoreWeighted,
					"after_score":   afterScoreWeighted,
					"scope":         scope,
					"scope_level":   ParseScopeLevel(scope).String(),
					"score_factors": beforeRole.score.Factors,
					"role_source":   string(beforeRole.source),
				},
			}

			if err := runner.EmitDecision(a, change, decision); err != nil {
				return fmt.Errorf("failed to emit decision: %w", err)
			}
		}
		// If scores are equal, no escalation/de-escalation
	}

	return nil
}

// resolveRole resolves a role from the resource state using the four-level fallback chain:
// 1. Built-in role database (by name or ID)
// 2. Custom role from plan (azurerm_role_definition)
// 3. Config fallback (PrivilegedRoles list)
// 4. Unknown role
func (a *PrivilegeEscalationAnalyzer) resolveRole(
	state map[string]interface{},
	db *RoleDatabase,
	customRoles map[string]*RoleDefinition,
	privilegedSet map[string]bool,
) resolvedRole {
	if state == nil {
		return resolvedRole{
			name:   "",
			score:  PermissionScore{Total: 0, Factors: []string{"no role state"}},
			source: roleSourceUnknown,
		}
	}

	roleName := stringField(state, "role_definition_name")
	roleID := stringField(state, "role_definition_id")

	// Try to resolve by name first from built-in database
	if roleName != "" {
		if role, found := db.LookupByName(roleName); found {
			return resolvedRole{
				name:   roleName,
				score:  ScorePermissions(role),
				source: roleSourceBuiltin,
			}
		}
	}

	// Try to resolve by ID from built-in database
	if roleID != "" {
		if role, found := db.LookupByID(roleID); found {
			name := role.Name
			if roleName != "" {
				name = roleName // Prefer the name from the assignment
			}
			return resolvedRole{
				name:   name,
				score:  ScorePermissions(role),
				source: roleSourceBuiltin,
			}
		}
	}

	// Try custom roles from plan
	if roleName != "" && customRoles != nil {
		if role, found := customRoles[strings.ToLower(roleName)]; found {
			return resolvedRole{
				name:   roleName,
				score:  ScorePermissions(role),
				source: roleSourcePlanCustom,
			}
		}
	}

	// Config fallback: check if in PrivilegedRoles list
	if roleName != "" && privilegedSet[roleName] {
		return resolvedRole{
			name: roleName,
			score: PermissionScore{
				Total:   a.config.UnknownPrivilegedSeverity,
				Factors: []string{"configured as privileged role"},
			},
			source: roleSourceConfigFallback,
		}
	}

	// Unknown role
	if roleName == "" && roleID == "" {
		return resolvedRole{
			name:   "",
			score:  PermissionScore{Total: 0, Factors: []string{"no role specified"}},
			source: roleSourceUnknown,
		}
	}

	name := roleName
	if name == "" {
		name = roleID
	}
	return resolvedRole{
		name: name,
		score: PermissionScore{
			Total:   a.config.UnknownRoleSeverity,
			Factors: []string{"unknown role"},
		},
		source: roleSourceUnknown,
	}
}

// buildCustomRoleLookup queries the runner for azurerm_role_definition resources
// and builds a lookup map by role name.
func (a *PrivilegeEscalationAnalyzer) buildCustomRoleLookup(runner sdk.Runner) map[string]*RoleDefinition {
	if !a.config.CrossReferenceCustomRoles {
		return nil
	}

	changes, err := runner.GetResourceChanges([]string{"azurerm_role_definition"})
	if err != nil {
		// Log and continue - don't fail analysis due to cross-reference failure
		return nil
	}

	result := make(map[string]*RoleDefinition)
	for _, change := range changes {
		state := change.After
		if state == nil {
			state = change.Before
		}
		if state == nil {
			continue
		}

		name := stringField(state, "name")
		if name == "" {
			continue
		}

		// Parse permissions from plan JSON
		// Terraform uses snake_case: actions, not_actions, data_actions, not_data_actions
		perms := a.parsePermissionsFromPlan(state)
		if len(perms) == 0 {
			continue
		}

		role := &RoleDefinition{
			Name:        name,
			Permissions: perms,
		}
		result[strings.ToLower(name)] = role
	}

	return result
}

// parsePermissionsFromPlan extracts permissions from a Terraform plan state.
func (a *PrivilegeEscalationAnalyzer) parsePermissionsFromPlan(state map[string]interface{}) []Permission {
	// The permissions field in Terraform is typically a list of permission blocks
	permsRaw, ok := state["permissions"]
	if !ok {
		return nil
	}

	permsList, ok := permsRaw.([]interface{})
	if !ok {
		return nil
	}

	var result []Permission
	for _, p := range permsList {
		permMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		perm := Permission{
			Actions:        toStringSlice(permMap["actions"]),
			NotActions:     toStringSlice(permMap["not_actions"]),
			DataActions:    toStringSlice(permMap["data_actions"]),
			NotDataActions: toStringSlice(permMap["not_data_actions"]),
		}
		result = append(result, perm)
	}

	return result
}
