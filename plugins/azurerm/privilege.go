// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jokarl/tfclassify/sdk"
)

// PluginAnalyzerConfig holds per-analyzer configuration from classification blocks.
// This matches the JSON structure sent from the host.
type PluginAnalyzerConfig struct {
	PrivilegeEscalation *PrivilegeEscalationAnalyzerConfig `json:"PrivilegeEscalation,omitempty"`
	NetworkExposure     *NetworkExposureAnalyzerConfig     `json:"NetworkExposure,omitempty"`
	KeyVaultAccess      *KeyVaultAccessAnalyzerConfig      `json:"KeyVaultAccess,omitempty"`
}

// PrivilegeEscalationAnalyzerConfig holds per-classification configuration for the privilege analyzer.
type PrivilegeEscalationAnalyzerConfig struct {
	ScoreThreshold int      `json:"score_threshold,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	Exclude        []string `json:"exclude,omitempty"`
}

// NetworkExposureAnalyzerConfig holds per-classification configuration for the network analyzer.
type NetworkExposureAnalyzerConfig struct {
	PermissiveSources []string `json:"permissive_sources,omitempty"`
}

// KeyVaultAccessAnalyzerConfig holds per-classification configuration for the keyvault analyzer.
type KeyVaultAccessAnalyzerConfig struct {
	DestructivePermissions []string `json:"destructive_permissions,omitempty"`
}

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
	return a.analyzeWithConfig(runner, "", nil)
}

// AnalyzeWithClassification implements sdk.ClassificationAwareAnalyzer.
// It receives classification context and per-analyzer configuration.
func (a *PrivilegeEscalationAnalyzer) AnalyzeWithClassification(runner sdk.Runner, classification string, analyzerConfigJSON []byte) error {
	// Parse the analyzer config
	var pluginConfig PluginAnalyzerConfig
	if len(analyzerConfigJSON) > 0 {
		if err := json.Unmarshal(analyzerConfigJSON, &pluginConfig); err != nil {
			return fmt.Errorf("failed to parse analyzer config: %w", err)
		}
	}
	return a.analyzeWithConfig(runner, classification, pluginConfig.PrivilegeEscalation)
}

// analyzeWithConfig is the core analysis logic with optional classification-scoped config.
func (a *PrivilegeEscalationAnalyzer) analyzeWithConfig(runner sdk.Runner, classification string, analyzerCfg *PrivilegeEscalationAnalyzerConfig) error {
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

	// Build exclude set from classification config
	var excludeSet map[string]bool
	if analyzerCfg != nil && len(analyzerCfg.Exclude) > 0 {
		excludeSet = make(map[string]bool)
		for _, role := range analyzerCfg.Exclude {
			excludeSet[strings.ToLower(role)] = true
		}
	}

	// Build roles filter from classification config (limit to specific roles)
	var rolesFilter map[string]bool
	if analyzerCfg != nil && len(analyzerCfg.Roles) > 0 {
		rolesFilter = make(map[string]bool)
		for _, role := range analyzerCfg.Roles {
			rolesFilter[strings.ToLower(role)] = true
		}
	}

	// Get score threshold
	scoreThreshold := 0
	if analyzerCfg != nil {
		scoreThreshold = analyzerCfg.ScoreThreshold
	}

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

		// Only detect escalation (not de-escalation - CR-0024 removes de-escalation detection)
		if afterScoreWeighted > beforeScoreWeighted {
			// Check if role is in exclude list
			if excludeSet != nil && excludeSet[strings.ToLower(afterRole.name)] {
				continue
			}

			// Check if we have a roles filter and the role isn't in it
			if rolesFilter != nil && !rolesFilter[strings.ToLower(afterRole.name)] {
				continue
			}

			// Check score threshold
			if afterScoreWeighted < scoreThreshold {
				continue
			}

			// Escalation: after is more privileged than before
			reason := fmt.Sprintf("role escalated from %q to %q", beforeRole.name, afterRole.name)
			if beforeRole.name == "" {
				reason = fmt.Sprintf("privileged role %q assigned", afterRole.name)
			}

			decision := &sdk.Decision{
				Classification: classification, // Set classification from context
				Reason:         reason,
				Severity:       afterScoreWeighted,
				Metadata: map[string]interface{}{
					"analyzer":        "privilege-escalation",
					"before_role":     beforeRole.name,
					"after_role":      afterRole.name,
					"direction":       "escalation",
					"before_score":    beforeScoreWeighted,
					"after_score":     afterScoreWeighted,
					"scope":           scope,
					"scope_level":     ParseScopeLevel(scope).String(),
					"score_factors":   afterRole.score.Factors,
					"role_source":     string(afterRole.source),
					"score_threshold": scoreThreshold,
				},
			}

			if err := runner.EmitDecision(a, change, decision); err != nil {
				return fmt.Errorf("failed to emit decision: %w", err)
			}
		}
		// De-escalation is no longer detected (removed per CR-0024)
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
