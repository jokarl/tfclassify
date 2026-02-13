// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	"fmt"

	"github.com/jokarl/tfclassify/sdk"
)

// PrivilegeEscalationAnalyzer detects privilege escalation in Azure role assignments.
// It compares before and after values of role_definition_name to detect when roles
// are changed to or from privileged roles.
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
	changes, err := runner.GetResourceChanges(a.ResourcePatterns())
	if err != nil {
		return fmt.Errorf("failed to get resource changes: %w", err)
	}

	privileged := toSet(a.config.PrivilegedRoles)

	for _, change := range changes {
		beforeRole := stringField(change.Before, "role_definition_name")
		afterRole := stringField(change.After, "role_definition_name")

		// Skip if no role change
		if beforeRole == afterRole {
			continue
		}

		beforePrivileged := privileged[beforeRole]
		afterPrivileged := privileged[afterRole]

		// Escalation: non-privileged -> privileged
		if !beforePrivileged && afterPrivileged {
			reason := fmt.Sprintf("role escalated from %q to %q", beforeRole, afterRole)
			if beforeRole == "" {
				reason = fmt.Sprintf("privileged role %q assigned", afterRole)
			}

			decision := &sdk.Decision{
				Classification: "", // Let host determine
				Reason:         reason,
				Severity:       90,
				Metadata: map[string]interface{}{
					"analyzer":    "privilege-escalation",
					"before_role": beforeRole,
					"after_role":  afterRole,
					"direction":   "escalation",
				},
			}

			if err := runner.EmitDecision(a, change, decision); err != nil {
				return fmt.Errorf("failed to emit decision: %w", err)
			}
		}

		// De-escalation: privileged -> non-privileged
		if beforePrivileged && !afterPrivileged {
			reason := fmt.Sprintf("role de-escalated from %q to %q", beforeRole, afterRole)
			if afterRole == "" {
				reason = fmt.Sprintf("privileged role %q removed", beforeRole)
			}

			decision := &sdk.Decision{
				Classification: "",
				Reason:         reason,
				Severity:       40,
				Metadata: map[string]interface{}{
					"analyzer":    "privilege-escalation",
					"before_role": beforeRole,
					"after_role":  afterRole,
					"direction":   "de-escalation",
				},
			}

			if err := runner.EmitDecision(a, change, decision); err != nil {
				return fmt.Errorf("failed to emit decision: %w", err)
			}
		}
	}

	return nil
}
