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
	Roles   []string `json:"roles,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
	// Actions is a list of Azure RBAC control-plane action patterns to match (CR-0028).
	Actions []string `json:"actions,omitempty"`
	// DataActions is a list of Azure RBAC data-plane action patterns to match (CR-0027).
	DataActions []string `json:"data_actions,omitempty"`
	// Scopes limits triggering to specific ARM scope levels (CR-0028).
	Scopes []string `json:"scopes,omitempty"`
	// FlagUnknownRoles controls whether unresolvable roles emit decisions (CR-0028).
	// Default: true (nil means true).
	FlagUnknownRoles *bool `json:"flag_unknown_roles,omitempty"`
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
	roleSourceBuiltin    roleSource = "builtin"
	roleSourcePlanCustom roleSource = "plan-custom-role"
	roleSourceUnknown    roleSource = "unknown"
)

// resolvedRole contains the resolved role information.
type resolvedRole struct {
	name               string
	source             roleSource
	definition         *RoleDefinition // The full role definition (for pattern matching)
	resolutionAttempts []string        // Why resolution failed (for unknown roles)
}

// PrivilegeEscalationAnalyzer detects privilege escalation in Azure role assignments.
// It uses pattern-based detection for both control-plane and data-plane actions,
// with scope filtering and unknown role handling.
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

// Analyze inspects role assignments for privilege escalation.
func (a *PrivilegeEscalationAnalyzer) Analyze(runner sdk.Runner) error {
	return a.analyzeWithConfig(runner, "", nil)
}

// AnalyzeWithClassification implements sdk.ClassificationAwareAnalyzer.
// It receives classification context and per-analyzer configuration.
func (a *PrivilegeEscalationAnalyzer) AnalyzeWithClassification(runner sdk.Runner, classification string, analyzerConfigJSON []byte) error {
	var pluginConfig PluginAnalyzerConfig
	if len(analyzerConfigJSON) > 0 {
		if err := json.Unmarshal(analyzerConfigJSON, &pluginConfig); err != nil {
			return fmt.Errorf("failed to parse analyzer config: %w", err)
		}
	}
	return a.analyzeWithConfig(runner, classification, pluginConfig.PrivilegeEscalation)
}

// scopeConfigName maps ScopeLevel values to the config names used in the scopes field.
var scopeConfigName = map[ScopeLevel]string{
	ScopeLevelManagementGroup: "management_group",
	ScopeLevelSubscription:    "subscription",
	ScopeLevelResourceGroup:   "resource_group",
	ScopeLevelResource:        "resource",
}

// matchesScopeFilter checks if the scope matches the configured scopes filter.
// Returns true if scopes is empty/nil (match any) or if the scope level is in the filter.
func matchesScopeFilter(scope string, configuredScopes []string) bool {
	if len(configuredScopes) == 0 {
		return true
	}
	level := ParseScopeLevel(scope)
	configName, ok := scopeConfigName[level]
	if !ok {
		return false
	}
	for _, s := range configuredScopes {
		if strings.EqualFold(s, configName) {
			return true
		}
	}
	return false
}

// flagUnknownRolesEnabled returns whether unknown roles should be flagged.
// Default is true when the pointer is nil.
func flagUnknownRolesEnabled(cfg *PrivilegeEscalationAnalyzerConfig) bool {
	if cfg == nil || cfg.FlagUnknownRoles == nil {
		return true
	}
	return *cfg.FlagUnknownRoles
}

// analyzeWithConfig is the core analysis logic with optional classification-scoped config.
func (a *PrivilegeEscalationAnalyzer) analyzeWithConfig(runner sdk.Runner, classification string, analyzerCfg *PrivilegeEscalationAnalyzerConfig) error {
	changes, err := runner.GetResourceChanges(a.ResourcePatterns())
	if err != nil {
		return fmt.Errorf("failed to get resource changes: %w", err)
	}

	// Build custom role lookup from plan (if enabled)
	customRoles := a.buildCustomRoleLookup(runner)

	// Get role database
	db := a.config.RoleDatabase
	if db == nil {
		db = DefaultRoleDatabase()
	}

	// Build exclude set from classification config
	var excludeSet map[string]bool
	if analyzerCfg != nil && len(analyzerCfg.Exclude) > 0 {
		excludeSet = make(map[string]bool)
		for _, role := range analyzerCfg.Exclude {
			excludeSet[strings.ToLower(role)] = true
		}
	}

	// Build roles filter from classification config
	var rolesFilter map[string]bool
	if analyzerCfg != nil && len(analyzerCfg.Roles) > 0 {
		rolesFilter = make(map[string]bool)
		for _, role := range analyzerCfg.Roles {
			rolesFilter[strings.ToLower(role)] = true
		}
	}

	// Get config options
	var actionPatterns []string
	var dataActionPatterns []string
	var scopeFilter []string
	if analyzerCfg != nil {
		actionPatterns = analyzerCfg.Actions
		dataActionPatterns = analyzerCfg.DataActions
		scopeFilter = analyzerCfg.Scopes
	}

	useControlPlane := len(actionPatterns) > 0
	useDataPlane := len(dataActionPatterns) > 0

	for _, change := range changes {
		scope := stringField(change.After, "scope")
		if scope == "" {
			scope = stringField(change.Before, "scope")
		}

		// Check scope filter before resolving role (optimization)
		if !matchesScopeFilter(scope, scopeFilter) {
			continue
		}

		afterRole := a.resolveRole(change.After, db, customRoles)

		// Check exclude list
		if excludeSet != nil && excludeSet[strings.ToLower(afterRole.name)] {
			continue
		}

		// Check roles filter
		if rolesFilter != nil && !rolesFilter[strings.ToLower(afterRole.name)] {
			continue
		}

		// Handle unknown roles
		if afterRole.source == roleSourceUnknown && afterRole.name != "" {
			if flagUnknownRolesEnabled(analyzerCfg) {
				decision := &sdk.Decision{
					Classification: classification,
					Reason:         fmt.Sprintf("unknown role %q flagged (role permissions could not be resolved)", afterRole.name),
					Metadata: map[string]interface{}{
						"analyzer":            "privilege-escalation",
						"trigger":             "unknown-role",
						"role_name":           afterRole.name,
						"resolution_attempts": afterRole.resolutionAttempts,
						"scope":               scope,
						"scope_level":         ParseScopeLevel(scope).String(),
					},
				}
				if err := runner.EmitDecision(a, change, decision); err != nil {
					return fmt.Errorf("failed to emit decision: %w", err)
				}
			}
			continue
		}

		// Pattern-based detection
		controlPlaneTriggered := false
		var controlPlaneMatchedActions []string
		var controlPlaneMatchedPatterns []string

		if useControlPlane {
			controlPlaneMatchedActions, controlPlaneMatchedPatterns = a.matchControlPlanePatterns(afterRole.definition, actionPatterns)
			controlPlaneTriggered = len(controlPlaneMatchedActions) > 0
		}

		dataPlaneTriggered := false
		var dataPlaneMatchedActions []string
		var dataPlaneMatchedPatterns []string

		if useDataPlane {
			dataPlaneMatchedActions, dataPlaneMatchedPatterns = a.matchDataPlanePatterns(afterRole.definition, dataActionPatterns)
			dataPlaneTriggered = len(dataPlaneMatchedActions) > 0
		}

		if !controlPlaneTriggered && !dataPlaneTriggered {
			continue
		}

		// Build decision
		var trigger string
		var reason string
		metadata := map[string]interface{}{
			"analyzer":    "privilege-escalation",
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
		} else {
			trigger = "control-plane"
			reason = fmt.Sprintf("role %q grants control-plane access matching configured patterns", afterRole.name)
			metadata["matched_actions"] = controlPlaneMatchedActions
			metadata["matched_patterns"] = controlPlaneMatchedPatterns
		}

		metadata["trigger"] = trigger

		decision := &sdk.Decision{
			Classification: classification,
			Reason:         reason,
			Metadata:       metadata,
		}

		if err := runner.EmitDecision(a, change, decision); err != nil {
			return fmt.Errorf("failed to emit decision: %w", err)
		}
	}

	return nil
}

// matchControlPlanePatterns matches effective control-plane actions against patterns.
func (a *PrivilegeEscalationAnalyzer) matchControlPlanePatterns(role *RoleDefinition, patterns []string) (matchedActions, matchedPatterns []string) {
	if role == nil || len(patterns) == 0 {
		return nil, nil
	}

	matchedActionsSet := make(map[string]bool)
	matchedPatternsSet := make(map[string]bool)

	for _, perm := range role.Permissions {
		effectiveActions := computeEffectiveActionsWithRegistry(perm.Actions, perm.NotActions, false, nil)

		for _, action := range effectiveActions {
			for _, pattern := range patterns {
				if actionMatchesPattern(action, pattern) {
					matchedActionsSet[action] = true
					matchedPatternsSet[pattern] = true
				}
			}
		}
	}

	for action := range matchedActionsSet {
		matchedActions = append(matchedActions, action)
	}
	for pattern := range matchedPatternsSet {
		matchedPatterns = append(matchedPatterns, pattern)
	}

	return matchedActions, matchedPatterns
}

// matchDataPlanePatterns matches effective data-plane actions against patterns.
func (a *PrivilegeEscalationAnalyzer) matchDataPlanePatterns(role *RoleDefinition, patterns []string) (matchedActions, matchedPatterns []string) {
	if role == nil || len(patterns) == 0 {
		return nil, nil
	}

	matchedActionsSet := make(map[string]bool)
	matchedPatternsSet := make(map[string]bool)

	for _, perm := range role.Permissions {
		effectiveDataActions := computeEffectiveActionsWithRegistry(perm.DataActions, perm.NotDataActions, true, nil)

		for _, action := range effectiveDataActions {
			for _, pattern := range patterns {
				if actionMatchesPattern(action, pattern) {
					matchedActionsSet[action] = true
					matchedPatternsSet[pattern] = true
				}
			}
		}
	}

	for action := range matchedActionsSet {
		matchedActions = append(matchedActions, action)
	}
	for pattern := range matchedPatternsSet {
		matchedPatterns = append(matchedPatterns, pattern)
	}

	return matchedActions, matchedPatterns
}

// resolveRole resolves a role from the resource state.
// Resolution order:
// 1. Built-in role database (by name or ID)
// 2. Custom role from plan (azurerm_role_definition) by name or ID
// 3. Unknown role (with resolution attempt details)
func (a *PrivilegeEscalationAnalyzer) resolveRole(
	state map[string]interface{},
	db *RoleDatabase,
	customRoles *customRoleLookup,
) resolvedRole {
	if state == nil {
		return resolvedRole{
			name:   "",
			source: roleSourceUnknown,
		}
	}

	roleName := stringField(state, "role_definition_name")
	roleID := stringField(state, "role_definition_id")

	if roleName == "" && roleID == "" {
		return resolvedRole{
			name:   "",
			source: roleSourceUnknown,
		}
	}

	var attempts []string

	// Try built-in database by name
	if roleName != "" {
		if role, found := db.LookupByName(roleName); found {
			return resolvedRole{
				name:       roleName,
				source:     roleSourceBuiltin,
				definition: role,
			}
		}
		attempts = append(attempts, "not found in built-in role database")
	}

	// Try built-in database by ID
	if roleID != "" {
		if role, found := db.LookupByID(roleID); found {
			name := role.Name
			if roleName != "" {
				name = roleName
			}
			return resolvedRole{
				name:       name,
				source:     roleSourceBuiltin,
				definition: role,
			}
		}
		if roleName == "" {
			attempts = append(attempts, "not found in built-in role database (by ID)")
		}
	}

	// Try custom roles from plan (by name)
	if roleName != "" && customRoles != nil {
		if role, found := customRoles.lookupByName(roleName); found {
			return resolvedRole{
				name:       roleName,
				source:     roleSourcePlanCustom,
				definition: role,
			}
		}
		attempts = append(attempts, "no matching azurerm_role_definition by name in plan")
	}

	// Try custom roles from plan (by ID)
	if roleID != "" && customRoles != nil {
		if role, found := customRoles.lookupByID(roleID); found {
			return resolvedRole{
				name:       role.Name,
				source:     roleSourcePlanCustom,
				definition: role,
			}
		}
		attempts = append(attempts, "no matching azurerm_role_definition by ID in plan")
	}

	if customRoles == nil {
		attempts = append(attempts, "custom role cross-referencing disabled")
	}

	name := roleName
	if name == "" {
		name = roleID
	}
	return resolvedRole{
		name:               name,
		source:             roleSourceUnknown,
		resolutionAttempts: attempts,
	}
}

// customRoleLookup holds custom role definitions indexed by name and by resource ID.
type customRoleLookup struct {
	byName map[string]*RoleDefinition // lowercase name → definition
	byID   map[string]*RoleDefinition // lowercase role_definition_resource_id → definition
}

// lookupByName finds a custom role by name.
func (c *customRoleLookup) lookupByName(name string) (*RoleDefinition, bool) {
	if c == nil {
		return nil, false
	}
	role, ok := c.byName[strings.ToLower(name)]
	return role, ok
}

// lookupByID finds a custom role by its role_definition_resource_id.
func (c *customRoleLookup) lookupByID(id string) (*RoleDefinition, bool) {
	if c == nil {
		return nil, false
	}
	role, ok := c.byID[strings.ToLower(id)]
	return role, ok
}

// buildCustomRoleLookup queries the runner for azurerm_role_definition resources
// and builds a lookup indexed by role name and role_definition_resource_id.
func (a *PrivilegeEscalationAnalyzer) buildCustomRoleLookup(runner sdk.Runner) *customRoleLookup {
	if !a.config.CrossReferenceCustomRoles {
		return nil
	}

	changes, err := runner.GetResourceChanges([]string{"azurerm_role_definition"})
	if err != nil {
		return nil
	}

	result := &customRoleLookup{
		byName: make(map[string]*RoleDefinition),
		byID:   make(map[string]*RoleDefinition),
	}
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

		perms := a.parsePermissionsFromPlan(state)
		if len(perms) == 0 {
			continue
		}

		role := &RoleDefinition{
			Name:        name,
			Permissions: perms,
		}
		result.byName[strings.ToLower(name)] = role

		// Also index by role_definition_resource_id for ID-based lookups
		resourceID := stringField(state, "role_definition_resource_id")
		if resourceID != "" {
			result.byID[strings.ToLower(resourceID)] = role
		}
	}

	return result
}

// parsePermissionsFromPlan extracts permissions from a Terraform plan state.
func (a *PrivilegeEscalationAnalyzer) parsePermissionsFromPlan(state map[string]interface{}) []Permission {
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
