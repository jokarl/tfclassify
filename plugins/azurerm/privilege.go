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
	// DataActions is a list of Azure RBAC data-plane action patterns to match (CR-0027).
	DataActions []string `json:"data_actions,omitempty"`
	// Actions is a list of Azure RBAC control-plane action patterns to match (CR-0028).
	Actions []string `json:"actions,omitempty"`
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
	name       string
	score      PermissionScore
	source     roleSource
	definition *RoleDefinition // The full role definition (for pattern matching)
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

	// Get config options
	var scoreThreshold int
	var dataActionPatterns []string
	var actionPatterns []string
	if analyzerCfg != nil {
		scoreThreshold = analyzerCfg.ScoreThreshold
		dataActionPatterns = analyzerCfg.DataActions
		actionPatterns = analyzerCfg.Actions
	}

	// Determine which detection modes are active
	usePatternBasedControlPlane := len(actionPatterns) > 0
	useDataPlaneDetection := len(dataActionPatterns) > 0

	for _, change := range changes {
		scope := stringField(change.After, "scope")
		if scope == "" {
			scope = stringField(change.Before, "scope")
		}

		beforeRole := a.resolveRole(change.Before, db, customRoles, privilegedSet)
		afterRole := a.resolveRole(change.After, db, customRoles, privilegedSet)

		// Check if role is in exclude list
		if excludeSet != nil && excludeSet[strings.ToLower(afterRole.name)] {
			continue
		}

		// Check if we have a roles filter and the role isn't in it
		if rolesFilter != nil && !rolesFilter[strings.ToLower(afterRole.name)] {
			continue
		}

		// Check for control-plane trigger
		controlPlaneTriggered := false
		var controlPlaneMatchedActions []string
		var controlPlaneMatchedPatterns []string

		if usePatternBasedControlPlane {
			// CR-0028: Pattern-based control-plane detection
			controlPlaneMatchedActions, controlPlaneMatchedPatterns = a.matchControlPlanePatterns(afterRole.definition, actionPatterns)
			controlPlaneTriggered = len(controlPlaneMatchedActions) > 0
		} else {
			// Legacy score-based detection
			beforeScoreWeighted := ApplyScopeMultiplier(beforeRole.score.Total, scope)
			afterScoreWeighted := ApplyScopeMultiplier(afterRole.score.Total, scope)

			// Only detect escalation (not de-escalation - CR-0024 removes de-escalation detection)
			if afterScoreWeighted > beforeScoreWeighted && afterScoreWeighted >= scoreThreshold {
				controlPlaneTriggered = true
			}
		}

		// Check for data-plane trigger (CR-0027)
		dataPlaneTriggered := false
		var dataPlaneMatchedActions []string
		var dataPlaneMatchedPatterns []string

		if useDataPlaneDetection {
			dataPlaneMatchedActions, dataPlaneMatchedPatterns = a.matchDataPlanePatterns(afterRole.definition, dataActionPatterns)
			dataPlaneTriggered = len(dataPlaneMatchedActions) > 0
		}

		// Emit decision if either control-plane or data-plane triggered
		if controlPlaneTriggered || dataPlaneTriggered {
			beforeScoreWeighted := ApplyScopeMultiplier(beforeRole.score.Total, scope)
			afterScoreWeighted := ApplyScopeMultiplier(afterRole.score.Total, scope)

			// Determine trigger type and build reason/metadata
			var trigger string
			var reason string
			metadata := map[string]interface{}{
				"analyzer":    "privilege-escalation",
				"before_role": beforeRole.name,
				"after_role":  afterRole.name,
				"scope":       scope,
				"scope_level": ParseScopeLevel(scope).String(),
				"role_source": string(afterRole.source),
			}

			if controlPlaneTriggered && dataPlaneTriggered {
				trigger = "both"
				reason = fmt.Sprintf("role %q grants control-plane and data-plane access matching configured patterns", afterRole.name)
				metadata["matched_actions"] = controlPlaneMatchedActions
				metadata["matched_patterns"] = controlPlaneMatchedPatterns
				metadata["matched_data_actions"] = dataPlaneMatchedActions
				metadata["matched_data_patterns"] = dataPlaneMatchedPatterns
			} else if dataPlaneTriggered {
				trigger = "data-plane"
				reason = fmt.Sprintf("role %q grants data-plane access matching configured patterns", afterRole.name)
				metadata["matched_data_actions"] = dataPlaneMatchedActions
				metadata["matched_patterns"] = dataPlaneMatchedPatterns
			} else if usePatternBasedControlPlane {
				trigger = "control-plane"
				reason = fmt.Sprintf("role %q grants control-plane access matching configured patterns", afterRole.name)
				metadata["matched_actions"] = controlPlaneMatchedActions
				metadata["matched_patterns"] = controlPlaneMatchedPatterns
			} else {
				// Legacy score-based trigger
				trigger = "control-plane"
				if beforeRole.name == "" {
					reason = fmt.Sprintf("privileged role %q assigned", afterRole.name)
				} else {
					reason = fmt.Sprintf("role escalated from %q to %q", beforeRole.name, afterRole.name)
				}
				metadata["direction"] = "escalation"
				metadata["before_score"] = beforeScoreWeighted
				metadata["after_score"] = afterScoreWeighted
				metadata["score_factors"] = afterRole.score.Factors
				metadata["score_threshold"] = scoreThreshold
			}

			metadata["trigger"] = trigger

			decision := &sdk.Decision{
				Classification: classification,
				Reason:         reason,
				Severity:       afterScoreWeighted,
				Metadata:       metadata,
			}

			if err := runner.EmitDecision(a, change, decision); err != nil {
				return fmt.Errorf("failed to emit decision: %w", err)
			}
		}
	}

	return nil
}

// matchControlPlanePatterns matches effective control-plane actions against patterns.
// Returns the list of matched actions and the patterns they matched.
func (a *PrivilegeEscalationAnalyzer) matchControlPlanePatterns(role *RoleDefinition, patterns []string) (matchedActions, matchedPatterns []string) {
	if role == nil || len(patterns) == 0 {
		return nil, nil
	}

	matchedActionsSet := make(map[string]bool)
	matchedPatternsSet := make(map[string]bool)

	for _, perm := range role.Permissions {
		// Compute effective actions (Actions minus NotActions)
		effectiveActions := computeEffectiveActions(perm.Actions, perm.NotActions)

		for _, action := range effectiveActions {
			for _, pattern := range patterns {
				if actionMatchesPattern(action, pattern) {
					matchedActionsSet[action] = true
					matchedPatternsSet[pattern] = true
				}
			}
		}
	}

	// Convert sets to slices
	for action := range matchedActionsSet {
		matchedActions = append(matchedActions, action)
	}
	for pattern := range matchedPatternsSet {
		matchedPatterns = append(matchedPatterns, pattern)
	}

	return matchedActions, matchedPatterns
}

// matchDataPlanePatterns matches effective data-plane actions against patterns.
// Returns the list of matched actions and the patterns they matched.
func (a *PrivilegeEscalationAnalyzer) matchDataPlanePatterns(role *RoleDefinition, patterns []string) (matchedActions, matchedPatterns []string) {
	if role == nil || len(patterns) == 0 {
		return nil, nil
	}

	matchedActionsSet := make(map[string]bool)
	matchedPatternsSet := make(map[string]bool)

	for _, perm := range role.Permissions {
		// Compute effective data actions (DataActions minus NotDataActions)
		effectiveDataActions := computeEffectiveActions(perm.DataActions, perm.NotDataActions)

		for _, action := range effectiveDataActions {
			for _, pattern := range patterns {
				if actionMatchesPattern(action, pattern) {
					matchedActionsSet[action] = true
					matchedPatternsSet[pattern] = true
				}
			}
		}
	}

	// Convert sets to slices
	for action := range matchedActionsSet {
		matchedActions = append(matchedActions, action)
	}
	for pattern := range matchedPatternsSet {
		matchedPatterns = append(matchedPatterns, pattern)
	}

	return matchedActions, matchedPatterns
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
				name:       roleName,
				score:      ScorePermissions(role),
				source:     roleSourceBuiltin,
				definition: role,
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
				name:       name,
				score:      ScorePermissions(role),
				source:     roleSourceBuiltin,
				definition: role,
			}
		}
	}

	// Try custom roles from plan
	if roleName != "" && customRoles != nil {
		if role, found := customRoles[strings.ToLower(roleName)]; found {
			return resolvedRole{
				name:       roleName,
				score:      ScorePermissions(role),
				source:     roleSourcePlanCustom,
				definition: role,
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
